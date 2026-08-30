// Package testselection owns the conservative typed behavioral selection policy.
package testselection

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

const PolicyVersion = 2

var requiredLanes = []string{
	"go",
	"pi-runtime",
	"platform-sensitive",
	"release-archive",
	"render-template",
}

// LanePolicy assigns repository paths, including dynamically read inputs, to a
// behavioral lane.
type LanePolicy struct {
	Name     string   `json:"name"`
	Patterns []string `json:"patterns"`
}

// Policy is the versioned repository selection policy.
type Policy struct {
	Version             int          `json:"version"`
	Lanes               []LanePolicy `json:"lanes"`
	SharedPathPatterns  []string     `json:"shared_path_patterns"`
	GeneratedGoPatterns []string     `json:"generated_go_patterns"`
}

// Lane is one selected behavioral lane and its stable reasons.
type Lane struct {
	Name    string   `json:"name"`
	Reasons []string `json:"reasons"`
}

// Package is one selected Go package and its stable reasons.
type Package struct {
	Path    string   `json:"path"`
	Reasons []string `json:"reasons"`
}

// Result is the stable machine-readable selection interface. Outcome is one of
// selected, empty, widened, or refused. Every slice is emitted, including when
// empty, so consumers do not need null handling.
type Result struct {
	Version     int       `json:"version"`
	Outcome     string    `json:"outcome"`
	Lanes       []Lane    `json:"lanes"`
	Packages    []Package `json:"packages"`
	Diagnostics []string  `json:"diagnostics"`
}

// Load reads and validates the typed selection policy.
func Load(filename string) (Policy, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return Policy{}, fmt.Errorf("read selection policy: %w", err)
	}
	var policy Policy
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&policy); err != nil {
		return Policy{}, fmt.Errorf("parse selection policy: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Policy{}, fmt.Errorf("parse selection policy: trailing content")
	}
	if policy.Version != PolicyVersion {
		return Policy{}, fmt.Errorf("selection policy version %d is unsupported", policy.Version)
	}

	remaining := make(map[string]bool, len(requiredLanes))
	for _, name := range requiredLanes {
		remaining[name] = true
	}
	for _, lane := range policy.Lanes {
		if !remaining[lane.Name] || len(lane.Patterns) == 0 {
			return Policy{}, fmt.Errorf("selection policy has invalid or duplicate lane %q", lane.Name)
		}
		delete(remaining, lane.Name)
		if err := validatePatterns(lane.Patterns); err != nil {
			return Policy{}, fmt.Errorf("selection policy lane %q: %w", lane.Name, err)
		}
	}
	if len(remaining) != 0 {
		return Policy{}, fmt.Errorf("selection policy omits required lanes: %s", strings.Join(sortedKeys(remaining), ", "))
	}
	if len(policy.SharedPathPatterns) == 0 || len(policy.GeneratedGoPatterns) == 0 {
		return Policy{}, fmt.Errorf("selection policy requires shared and generated Go path patterns")
	}
	if err := validatePatterns(policy.SharedPathPatterns); err != nil {
		return Policy{}, fmt.Errorf("selection policy shared paths: %w", err)
	}
	if err := validatePatterns(policy.GeneratedGoPatterns); err != nil {
		return Policy{}, fmt.Errorf("selection policy generated Go paths: %w", err)
	}
	return policy, nil
}

func validatePatterns(patterns []string) error {
	seen := map[string]bool{}
	for _, pattern := range patterns {
		if pattern == "" || seen[pattern] || !doublestar.ValidatePattern(pattern) {
			return fmt.Errorf("invalid or duplicate path pattern %q", pattern)
		}
		seen[pattern] = true
	}
	return nil
}

// Select discovers repository packages and selects typed behavioral lanes. Go
// changes close over production reverse dependencies and test importers. A
// shared, generated, build-tagged, deleted, or unclassified input widens
// visibly instead of guessing at incomplete assurance.
func Select(ctx context.Context, root string, policy Policy, changedPaths []string) (Result, error) {
	paths, err := normalizePaths(changedPaths)
	if err != nil {
		return refused(policy, err), err
	}
	if len(paths) == 0 {
		return empty(policy), nil
	}
	graph, err := discover(ctx, root)
	if err != nil {
		err = fmt.Errorf("discover Go packages: %w", err)
		return refused(policy, err), err
	}
	if shared, reason := sharedChange(policy, paths); shared {
		return widened(policy, graph, reason), nil
	}

	lanes, classified := selectedLanes(policy, paths)
	if widenedGo, reason := packageWideningChange(root, policy, paths); widenedGo {
		if strings.HasPrefix(reason, "build-tag-change:") {
			lanes = addLane(lanes, "platform-sensitive", "build-constraint:"+strings.TrimPrefix(reason, "build-tag-change:"))
		}
		return widenedPackages(policy, graph, lanes, reason), nil
	}
	for _, changed := range paths {
		if !classified[changed] {
			return widened(policy, graph, "unclassified-change:"+changed), nil
		}
	}
	packages, widenReason := selectGoPackages(policy, graph, paths)
	if widenReason != "" {
		return widened(policy, graph, widenReason), nil
	}
	return Result{
		Version:     policy.Version,
		Outcome:     "selected",
		Lanes:       lanes,
		Packages:    packages,
		Diagnostics: []string{},
	}, nil
}

func empty(policy Policy) Result {
	return Result{
		Version:     policy.Version,
		Outcome:     "empty",
		Lanes:       []Lane{},
		Packages:    []Package{},
		Diagnostics: []string{"no-relevant-changes"},
	}
}

func refused(policy Policy, err error) Result {
	return Result{
		Version:     policy.Version,
		Outcome:     "refused",
		Lanes:       []Lane{},
		Packages:    []Package{},
		Diagnostics: []string{err.Error()},
	}
}

func selectedLanes(policy Policy, paths []string) ([]Lane, map[string]bool) {
	selected := map[string]map[string]bool{}
	classified := map[string]bool{}
	for _, changed := range paths {
		for _, lane := range policy.Lanes {
			if matchesAny(lane.Patterns, changed) {
				addReason(selected, lane.Name, "changed:"+changed)
				classified[changed] = true
			}
		}
	}
	names := sortedKeys(selected)
	lanes := make([]Lane, 0, len(names))
	for _, name := range names {
		lanes = append(lanes, Lane{Name: name, Reasons: sortedKeys(selected[name])})
	}
	return lanes, classified
}

func matchesAny(patterns []string, filename string) bool {
	for _, pattern := range patterns {
		matched, err := doublestar.Match(pattern, filename)
		if err != nil || matched {
			return true
		}
	}
	return false
}

type graph struct {
	packages map[string]node
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
	type platform struct{ os, arch string }
	platforms := []platform{{"linux", "amd64"}, {"linux", "arm64"}, {"darwin", "amd64"}, {"darwin", "arm64"}}
	imports := map[string]map[string]bool{}
	testImports := map[string]map[string]bool{}
	for _, target := range platforms {
		packages, err := discoverPlatform(ctx, root, target.os, target.arch)
		if err != nil {
			return graph{}, err
		}
		for packagePath, item := range packages {
			if imports[packagePath] == nil {
				imports[packagePath] = map[string]bool{}
				testImports[packagePath] = map[string]bool{}
			}
			for _, imported := range item.imports {
				imports[packagePath][imported] = true
			}
			for _, imported := range item.testImports {
				testImports[packagePath][imported] = true
			}
		}
	}
	if len(imports) == 0 {
		return graph{}, fmt.Errorf("go list returned no packages")
	}
	packages := make(map[string]node, len(imports))
	for packagePath := range imports {
		packages[packagePath] = node{
			imports:     sortedKeys(imports[packagePath]),
			testImports: sortedKeys(testImports[packagePath]),
		}
	}
	return graph{packages: packages}, nil
}

func discoverPlatform(ctx context.Context, root, goos, goarch string) (map[string]node, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-json", "./...")
	cmd.Dir = root
	cmd.Env = platformEnvironment(goos, goarch)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list for %s/%s: %w", goos, goarch, err)
	}

	decoder := json.NewDecoder(bytes.NewReader(output))
	byImport := map[string]string{}
	type rawNode struct{ imports, testImports []string }
	raw := map[string]rawNode{}
	for {
		var item goListPackage
		if err := decoder.Decode(&item); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("decode go list for %s/%s: %w", goos, goarch, err)
		}
		if item.Error != nil {
			return nil, fmt.Errorf("go list package %q for %s/%s: %s", item.ImportPath, goos, goarch, item.Error.Err)
		}
		relative, err := filepath.Rel(root, item.Dir)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("go list package %q is outside repository", item.ImportPath)
		}
		packagePath := "./"
		if relative != "." {
			packagePath += filepath.ToSlash(relative)
		}
		byImport[item.ImportPath] = packagePath
		raw[item.ImportPath] = rawNode{
			imports:     append([]string(nil), item.Imports...),
			testImports: append(append([]string(nil), item.TestImports...), item.XTestImports...),
		}
	}

	packages := make(map[string]node, len(raw))
	for importPath, item := range raw {
		packages[byImport[importPath]] = node{
			imports:     localPackages(item.imports, byImport),
			testImports: localPackages(item.testImports, byImport),
		}
	}
	return packages, nil
}

func platformEnvironment(goos, goarch string) []string {
	environment := make([]string, 0, len(os.Environ())+3)
	for _, value := range os.Environ() {
		if strings.HasPrefix(value, "GOOS=") || strings.HasPrefix(value, "GOARCH=") || strings.HasPrefix(value, "CGO_ENABLED=") {
			continue
		}
		environment = append(environment, value)
	}
	return append(environment, "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0")
}

func localPackages(imports []string, byImport map[string]string) []string {
	local := map[string]bool{}
	for _, imported := range imports {
		if packagePath, ok := byImport[imported]; ok {
			local[packagePath] = true
		}
	}
	return sortedKeys(local)
}

func normalizePaths(paths []string) ([]string, error) {
	normalized := map[string]bool{}
	for _, raw := range paths {
		if raw == "" || strings.ContainsRune(raw, '\x00') || strings.Contains(raw, `\`) {
			return nil, fmt.Errorf("invalid changed path %q", raw)
		}
		candidate := strings.TrimPrefix(raw, "./")
		cleaned := path.Clean(candidate)
		if candidate == "" || candidate != cleaned || cleaned == "." || strings.HasPrefix(cleaned, "/") || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
			return nil, fmt.Errorf("invalid changed path %q", raw)
		}
		normalized[cleaned] = true
	}
	return sortedKeys(normalized), nil
}

func sharedChange(policy Policy, paths []string) (bool, string) {
	for _, changed := range paths {
		if matchesAny(policy.SharedPathPatterns, changed) {
			return true, "shared-change:" + changed
		}
	}
	return false, ""
}

func packageWideningChange(root string, policy Policy, paths []string) (bool, string) {
	for _, changed := range paths {
		if matchesAny(policy.GeneratedGoPatterns, changed) {
			return true, "generated-go-change:" + changed
		}
		if strings.HasSuffix(changed, ".go") {
			contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(changed)))
			if err != nil {
				return true, "unreadable-go-change:" + changed
			}
			if hasBuildConstraint(contents) {
				return true, "build-tag-change:" + changed
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

func selectGoPackages(policy Policy, graph graph, paths []string) ([]Package, string) {
	selected := map[string]map[string]bool{}
	direct := map[string]bool{}
	productionDirect := map[string]bool{}
	for _, changed := range paths {
		if !laneMatches(policy, "go", changed) {
			continue
		}
		owner, owned := packageOwner(graph, changed)
		if !owned {
			return nil, "unowned-go-change:" + changed
		}
		if strings.HasSuffix(changed, ".go") {
			exactOwner := "./"
			if directory := path.Dir(changed); directory != "." {
				exactOwner += directory
			}
			if owner != exactOwner {
				return nil, "unowned-go-change:" + changed
			}
			addReason(selected, owner, "changed-package:"+changed)
			if !strings.HasSuffix(changed, "_test.go") {
				productionDirect[owner] = true
			}
		} else {
			addReason(selected, owner, "changed-package-input:"+changed)
			productionDirect[owner] = true
		}
		direct[owner] = true
	}

	productionAffected := map[string]bool{}
	for changed := range productionDirect {
		for packagePath := range productionReverseClosure(graph, changed) {
			productionAffected[packagePath] = true
			if packagePath != changed {
				addReason(selected, packagePath, "reverse-dependent:"+changed)
			}
		}
	}
	for packagePath, item := range graph.packages {
		if direct[packagePath] || productionAffected[packagePath] {
			continue
		}
		for _, imported := range item.testImports {
			if productionAffected[imported] {
				addReason(selected, packagePath, "test-reverse-dependent:"+imported)
				break
			}
		}
	}
	return packages(selected), ""
}

func laneMatches(policy Policy, laneName, changed string) bool {
	for _, lane := range policy.Lanes {
		if lane.Name == laneName {
			return matchesAny(lane.Patterns, changed)
		}
	}
	return false
}

func packageOwner(graph graph, filename string) (string, bool) {
	for directory := path.Dir(filename); ; directory = path.Dir(directory) {
		packagePath := "./"
		if directory != "." {
			packagePath += directory
		}
		if _, ok := graph.packages[packagePath]; ok {
			return packagePath, true
		}
		if directory == "." {
			return "", false
		}
	}
}

func productionReverseClosure(graph graph, changed string) map[string]bool {
	seen := map[string]bool{changed: true}
	for progressed := true; progressed; {
		progressed = false
		for packagePath, item := range graph.packages {
			if seen[packagePath] {
				continue
			}
			for _, imported := range item.imports {
				if seen[imported] {
					seen[packagePath] = true
					progressed = true
					break
				}
			}
		}
	}
	return seen
}

func widened(policy Policy, graph graph, reason string) Result {
	laneNames := append([]string(nil), requiredLanes...)
	sort.Strings(laneNames)
	lanes := make([]Lane, 0, len(laneNames))
	for _, name := range laneNames {
		lanes = append(lanes, Lane{Name: name, Reasons: []string{"widened:" + reason}})
	}
	return widenedPackages(policy, graph, lanes, reason)
}

func widenedPackages(policy Policy, graph graph, lanes []Lane, reason string) Result {
	selectedPackages := map[string]map[string]bool{}
	for packagePath := range graph.packages {
		addReason(selectedPackages, packagePath, "widened:"+reason)
	}
	return Result{
		Version:     policy.Version,
		Outcome:     "widened",
		Lanes:       lanes,
		Packages:    packages(selectedPackages),
		Diagnostics: []string{reason},
	}
}

func addLane(lanes []Lane, name, reason string) []Lane {
	selected := map[string]map[string]bool{}
	for _, lane := range lanes {
		for _, existing := range lane.Reasons {
			addReason(selected, lane.Name, existing)
		}
	}
	addReason(selected, name, reason)
	names := sortedKeys(selected)
	out := make([]Lane, 0, len(names))
	for _, laneName := range names {
		out = append(out, Lane{Name: laneName, Reasons: sortedKeys(selected[laneName])})
	}
	return out
}

func packages(selected map[string]map[string]bool) []Package {
	paths := sortedKeys(selected)
	packages := make([]Package, 0, len(paths))
	for _, packagePath := range paths {
		packages = append(packages, Package{Path: packagePath, Reasons: sortedKeys(selected[packagePath])})
	}
	return packages
}

func addReason(selected map[string]map[string]bool, key, reason string) {
	if selected[key] == nil {
		selected[key] = map[string]bool{}
	}
	selected[key][reason] = true
}

func sortedKeys[V any](items map[string]V) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
