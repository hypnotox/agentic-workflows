// Package testselection owns the conservative affected-Go-package selection
// policy used by the separate local feedback command.
package testselection

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

var provingUnitName = regexp.MustCompile(`^(Test|Example|Fuzz)[A-Za-z0-9_]+$`)

// Policy is the versioned repository selection policy.
type Policy struct {
	Version                int                 `json:"version"`
	MetaSuites             []SuitePolicy       `json:"meta_suites"`
	ReverseDependentSuites map[string][]string `json:"reverse_dependent_suites,omitempty"`
	SharedPathPatterns     []string            `json:"shared_path_patterns"`
	GeneratedGoPatterns    []string            `json:"generated_go_patterns"`
}

// SuitePolicy declares one closed set of representative top-level proving
// units. Suite IDs, packages, and proving-unit names are repository policy, not
// operator-supplied regular expressions.
type SuitePolicy struct {
	ID      string   `json:"id"`
	Package string   `json:"package"`
	Tests   []string `json:"tests"`
}

// Package is one selected Go package and its stable, accumulated reasons.
type Package struct {
	Path    string   `json:"path"`
	Reasons []string `json:"reasons"`
}

// Suite is one selected declared suite and its stable, accumulated reasons.
type Suite struct {
	ID      string   `json:"id"`
	Package string   `json:"package"`
	Tests   []string `json:"tests"`
	Reasons []string `json:"reasons"`
}

// Result is machine-consumable selection evidence. Outcome is selected, empty,
// widened, or refused; refused is returned alongside an error to retain the
// diagnostic when package discovery is unavailable.
type Result struct {
	Version  int       `json:"version"`
	Outcome  string    `json:"outcome"`
	Packages []Package `json:"packages"`
	Suites   []Suite   `json:"suites"`
	Reasons  []string  `json:"reasons,omitempty"`
}

// Load reads and validates a versioned selection policy.
func Load(path string) (Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Policy{}, fmt.Errorf("read selection policy: %w", err)
	}
	var policy Policy
	if err := json.Unmarshal(data, &policy); err != nil {
		return Policy{}, fmt.Errorf("parse selection policy: %w", err)
	}
	if policy.Version != 1 {
		return Policy{}, fmt.Errorf("selection policy version %d is unsupported", policy.Version)
	}
	if len(policy.MetaSuites) == 0 {
		return Policy{}, fmt.Errorf("selection policy declares no meta suites")
	}
	declared := map[string]bool{}
	for _, suite := range policy.MetaSuites {
		if suite.ID == "" || declared[suite.ID] {
			return Policy{}, fmt.Errorf("selection policy has invalid or duplicate meta suite %q", suite.ID)
		}
		declared[suite.ID] = true
		if !validPackage(suite.Package) || len(suite.Tests) == 0 {
			return Policy{}, fmt.Errorf("selection policy has invalid meta suite %q", suite.ID)
		}
		seenTests := map[string]bool{}
		for _, test := range suite.Tests {
			if !provingUnitName.MatchString(test) || seenTests[test] {
				return Policy{}, fmt.Errorf("selection policy meta suite %q has invalid or duplicate proving unit %q", suite.ID, test)
			}
			seenTests[test] = true
		}
	}
	for _, pattern := range append(append([]string(nil), policy.SharedPathPatterns...), policy.GeneratedGoPatterns...) {
		if !doublestar.ValidatePattern(pattern) {
			return Policy{}, fmt.Errorf("selection policy has invalid path pattern %q", pattern)
		}
	}
	for packagePath, suiteIDs := range policy.ReverseDependentSuites {
		if !validPackage(packagePath) || len(suiteIDs) == 0 {
			return Policy{}, fmt.Errorf("selection policy has invalid reverse-dependent suite mapping for %q", packagePath)
		}
		seen := map[string]bool{}
		for _, id := range suiteIDs {
			if !declared[id] || seen[id] {
				return Policy{}, fmt.Errorf("selection policy reverse-dependent mapping for %q has invalid or duplicate suite %q", packagePath, id)
			}
			seen[id] = true
		}
	}
	return policy, nil
}

func validPackage(path string) bool {
	return strings.HasPrefix(path, "./") && path != "./..." && !strings.Contains(path, "..")
}

// Select discovers repository packages then selects changed package ownership,
// their reverse dependents, and declared meta suites. Production dependency
// effects close transitively; test-only imports select their importing package
// without falsely making that package's production consumers affected. Any
// shared or uncertain path widens visibly; unavailable package discovery
// refuses rather than guessing at a partial dependency graph.
func Select(ctx context.Context, root string, policy Policy, changedPaths []string) (Result, error) {
	paths, err := normalizePaths(changedPaths)
	if err != nil {
		return refused(policy, err), err
	}
	if len(paths) == 0 {
		return Result{Version: policy.Version, Outcome: "empty", Packages: []Package{}, Suites: []Suite{}, Reasons: []string{"no-relevant-changes"}}, nil
	}
	graph, err := discover(ctx, root)
	if err != nil {
		err = fmt.Errorf("discover Go packages: %w", err)
		return refused(policy, err), err
	}
	shared, reason := sharedChange(root, policy, paths)
	if shared {
		return widened(policy, graph, reason), nil
	}
	return selectPaths(policy, graph, paths), nil
}

func refused(policy Policy, err error) Result {
	return Result{Version: policy.Version, Outcome: "refused", Packages: []Package{}, Suites: []Suite{}, Reasons: []string{err.Error()}}
}

type graph struct {
	packages map[string]node // package pattern by repository directory
}

type node struct {
	imports     []string
	testImports []string
}

type goListPackage struct {
	ImportPath   string
	Dir          string
	Imports      []string
	TestImports  []string
	XTestImports []string
	Error        *struct{ Err string }
}

func discover(ctx context.Context, root string) (graph, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-json", "./...")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return graph{}, fmt.Errorf("go list: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(out))
	packages := map[string]node{}
	byImport := map[string]string{}
	type rawNode struct{ imports, testImports []string }
	raw := map[string]rawNode{}
	for {
		var item goListPackage
		if err := decoder.Decode(&item); err != nil {
			if err == io.EOF {
				break
			}
			return graph{}, fmt.Errorf("decode go list: %w", err)
		}
		if item.Error != nil {
			return graph{}, fmt.Errorf("go list package %q: %s", item.ImportPath, item.Error.Err)
		}
		rel, err := filepath.Rel(root, item.Dir)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return graph{}, fmt.Errorf("go list package %q is outside repository", item.ImportPath)
		}
		pattern := "./"
		if rel != "." {
			pattern += filepath.ToSlash(rel)
		}
		packages[pattern] = node{}
		byImport[item.ImportPath] = pattern
		raw[item.ImportPath] = rawNode{
			imports:     append([]string(nil), item.Imports...),
			testImports: append(append([]string(nil), item.TestImports...), item.XTestImports...),
		}
	}
	if len(packages) == 0 {
		return graph{}, fmt.Errorf("go list returned no packages")
	}
	for importPath, item := range raw {
		localImports := localPackages(item.imports, byImport)
		localTests := localPackages(item.testImports, byImport)
		packages[byImport[importPath]] = node{imports: localImports, testImports: localTests}
	}
	return graph{packages: packages}, nil
}

func localPackages(imports []string, byImport map[string]string) []string {
	set := map[string]bool{}
	for _, imported := range imports {
		if target, ok := byImport[imported]; ok {
			set[target] = true
		}
	}
	out := make([]string, 0, len(set))
	for path := range set {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func normalizePaths(paths []string) ([]string, error) {
	set := map[string]bool{}
	for _, raw := range paths {
		path := strings.TrimPrefix(filepath.ToSlash(raw), "./")
		if path == "" || strings.HasPrefix(path, "/") || path == ".." || strings.HasPrefix(path, "../") {
			return nil, fmt.Errorf("invalid changed path %q", raw)
		}
		set[path] = true
	}
	out := make([]string, 0, len(set))
	for path := range set {
		out = append(out, path)
	}
	sort.Strings(out)
	return out, nil
}

func sharedChange(root string, policy Policy, paths []string) (bool, string) {
	for _, path := range paths {
		for _, pattern := range policy.SharedPathPatterns {
			matched, err := doublestar.Match(pattern, path)
			if err != nil || matched {
				return true, "shared-change:" + path
			}
		}
		for _, pattern := range policy.GeneratedGoPatterns {
			matched, err := doublestar.Match(pattern, path)
			if err != nil || matched {
				return true, "generated-go-change:" + path
			}
		}
		if strings.HasSuffix(path, ".go") {
			contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
			if err != nil {
				return true, "unreadable-go-change:" + path
			}
			if hasBuildConstraint(contents) {
				return true, "build-tag-change:" + path
			}
		}
	}
	return false, ""
}

func hasBuildConstraint(contents []byte) bool {
	for _, raw := range bytes.Split(contents, []byte("\n")) {
		line := strings.TrimSpace(string(raw))
		if strings.HasPrefix(line, "package ") {
			return false
		}
		if strings.HasPrefix(line, "//go:build ") || strings.HasPrefix(line, "// +build ") {
			return true
		}
	}
	return false
}

func selectPaths(policy Policy, graph graph, paths []string) Result {
	selected := map[string]map[string]bool{}
	selectedSuites := map[string]map[string]bool{}
	direct := map[string]bool{}
	productionDirect := map[string]bool{}
	for _, path := range paths {
		if !strings.HasSuffix(path, ".go") {
			continue
		}
		owner := "./"
		if dir := filepath.ToSlash(filepath.Dir(path)); dir != "." {
			owner += dir
		}
		if _, ok := graph.packages[owner]; !ok {
			return widened(policy, graph, "unowned-go-change:"+path)
		}
		direct[owner] = true
		if !strings.HasSuffix(path, "_test.go") {
			productionDirect[owner] = true
		}
		addReason(selected, owner, "changed-package:"+path)
	}
	if len(selected) == 0 {
		return Result{Version: policy.Version, Outcome: "empty", Packages: []Package{}, Suites: []Suite{}, Reasons: []string{"no-relevant-changes"}}
	}

	productionAffected := map[string]bool{}
	for changed := range productionDirect {
		closure := productionReverseClosure(graph, changed)
		for packagePath := range closure {
			productionAffected[packagePath] = true
			if packagePath == changed {
				continue
			}
			selectDependent(policy, selected, selectedSuites, packagePath, "reverse-dependent:"+changed)
		}
	}
	for packagePath, item := range graph.packages {
		if direct[packagePath] || productionAffected[packagePath] {
			continue
		}
		for _, imported := range item.testImports {
			if productionAffected[imported] {
				selectDependent(policy, selected, selectedSuites, packagePath, "test-reverse-dependent:"+imported)
				break
			}
		}
	}
	for _, suite := range policy.MetaSuites {
		if _, ok := graph.packages[suite.Package]; !ok {
			return widened(policy, graph, "unavailable-meta-suite:"+suite.ID)
		}
		addReason(selectedSuites, suite.ID, "declared-meta-suite")
	}
	return result(policy, "selected", selected, selectedSuites, nil)
}

func selectDependent(policy Policy, selected, selectedSuites map[string]map[string]bool, packagePath, reason string) {
	if suiteIDs := policy.ReverseDependentSuites[packagePath]; len(suiteIDs) > 0 {
		for _, id := range suiteIDs {
			addReason(selectedSuites, id, "representative-"+reason)
		}
		return
	}
	addReason(selected, packagePath, reason)
}

func productionReverseClosure(g graph, changed string) map[string]bool {
	seen := map[string]bool{changed: true}
	for progressed := true; progressed; {
		progressed = false
		for packagePath, item := range g.packages {
			if seen[packagePath] {
				continue
			}
			for _, imported := range item.imports {
				if seen[imported] {
					seen[packagePath], progressed = true, true
					break
				}
			}
		}
	}
	return seen
}

func widened(policy Policy, graph graph, reason string) Result {
	selected := map[string]map[string]bool{}
	for path := range graph.packages {
		addReason(selected, path, "widened:"+reason)
	}
	return result(policy, "widened", selected, map[string]map[string]bool{}, []string{reason})
}

func addReason(selected map[string]map[string]bool, path, reason string) {
	if selected[path] == nil {
		selected[path] = map[string]bool{}
	}
	selected[path][reason] = true
}

func result(policy Policy, outcome string, selected, selectedSuites map[string]map[string]bool, reasons []string) Result {
	suiteByID := map[string]SuitePolicy{}
	for _, suite := range policy.MetaSuites {
		suiteByID[suite.ID] = suite
		if _, full := selected[suite.Package]; full {
			if suiteReasons := selectedSuites[suite.ID]; len(suiteReasons) > 0 {
				for reason := range suiteReasons {
					addReason(selected, suite.Package, "contains-suite:"+suite.ID+":"+reason)
				}
				delete(selectedSuites, suite.ID)
			}
		}
	}

	paths := sortedKeys(selected)
	packages := make([]Package, 0, len(paths))
	for _, path := range paths {
		packages = append(packages, Package{Path: path, Reasons: sortedKeys(selected[path])})
	}
	suiteIDs := sortedKeys(selectedSuites)
	suites := make([]Suite, 0, len(suiteIDs))
	for _, id := range suiteIDs {
		declaration := suiteByID[id]
		tests := append([]string(nil), declaration.Tests...)
		sort.Strings(tests)
		suites = append(suites, Suite{ID: id, Package: declaration.Package, Tests: tests, Reasons: sortedKeys(selectedSuites[id])})
	}
	sort.Strings(reasons)
	return Result{Version: policy.Version, Outcome: outcome, Packages: packages, Suites: suites, Reasons: reasons}
}

func sortedKeys[V any](items map[string]V) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
