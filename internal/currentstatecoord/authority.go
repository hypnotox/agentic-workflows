package currentstatecoord

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// ResolveTopics reports lexical path attribution from the working current-state
// universe. It intentionally does not require paths to exist.
func ResolveTopics(root string, repo *awfgit.Repo, ctx context.Context, paths []string) (presentation.Detail, error) {
	ws, err := workingCurrentState(root, repo, ctx)
	if err != nil {
		return presentation.Detail{}, err
	}
	if err := validateAuthority(ws); err != nil {
		return presentation.Detail{}, err
	}
	sections := make([]presentation.Section, 0, len(paths))
	for _, path := range paths {
		domains, topics := topic.PathAuthority(ws.Loaded.Topics, path)
		nodes := []presentation.Node{authorityLiteralField("path", path), authorityItems("domains", domains), authorityItems("topics", topics)}
		section, err := presentation.NewSection("resolution", nodes...)
		if err != nil {
			return presentation.Detail{}, err
		}
		sections = append(sections, section)
	}
	return presentation.Detail{Fields: []presentation.Field{authorityField("query", "topic resolution")}, Sections: sections}, nil
}

// UncoveredPaths reports the whole-working-tree unowned census. It is an
// informational query, separate from enforcement coverage and its exclusions.
func UncoveredPaths(root string, repo *awfgit.Repo, ctx context.Context) (presentation.Detail, error) {
	ws, err := workingCurrentState(root, repo, ctx)
	if err != nil {
		return presentation.Detail{}, err
	}
	if err := validateAuthority(ws); err != nil {
		return presentation.Detail{}, err
	}
	generated := map[string]bool{}
	if ws.Lock != nil {
		for path := range ws.Lock.Files {
			generated[path] = true
		}
	}
	paths := authorityCensusPaths(ws.Tree.List(), generated)
	owned := map[string]bool{}
	unowned := []string{}
	for _, path := range paths {
		domains, _ := topic.PathAuthority(ws.Loaded.Topics, path)
		if len(domains) == 0 {
			unowned = append(unowned, path)
			continue
		}
		owned[path] = true
	}
	paths = collapseUnowned(unowned, owned)
	return presentation.Detail{Fields: []presentation.Field{authorityField("query", "uncovered paths")}, Sections: []presentation.Section{authoritySection("uncovered", authorityItems("paths", paths))}}, nil
}

// authorityCensusPaths selects the whole-repository query population. It is
// deliberately independent of contextIgnore, while retaining the independent
// generated-output, resident, and nested-adopter exclusions.
func authorityCensusPaths(files []snapshot.File, generated map[string]bool) []string {
	nested := []string{}
	for _, file := range files {
		if file.Scannable() && !resident.IsResidentPath(file.Path) && strings.HasSuffix(file.Path, "/.awf/config.yaml") {
			nested = append(nested, strings.TrimSuffix(file.Path, "/.awf/config.yaml"))
		}
	}
	paths := []string{}
	for _, file := range files {
		if !file.Scannable() || generated[file.Path] || resident.IsResidentPath(file.Path) {
			continue
		}
		insideNested := false
		for _, root := range nested {
			if file.Path == root || strings.HasPrefix(file.Path, root+"/") {
				insideNested = true
				break
			}
		}
		if !insideNested {
			paths = append(paths, file.Path)
		}
	}
	return paths
}

// collapseUnowned is the focused query form of contextq's established census
// algorithm. Snapshot trees contain files, so owned file descendants mark their
// directory ancestors before each unowned file selects its topmost uncovered
// ancestor; a flat prefix collapse cannot represent that semantic.
func collapseUnowned(unowned []string, owned map[string]bool) []string {
	coveredDirs := map[string]bool{}
	for path := range owned {
		for _, ancestor := range authorityAncestors(path) {
			coveredDirs[ancestor] = true
		}
	}
	entries := map[string]bool{}
	for _, path := range unowned {
		pick := path
		for _, ancestor := range authorityAncestors(path) {
			if !coveredDirs[ancestor] {
				if ancestor == "." {
					pick = "."
				} else {
					pick = ancestor + "/"
				}
				break
			}
		}
		entries[pick] = true
	}
	out := make([]string, 0, len(entries))
	for path := range entries {
		out = append(out, path)
	}
	slices.Sort(out)
	return out
}

func authorityAncestors(path string) []string {
	out := []string{"."}
	parts := strings.Split(path, "/")
	for i := 1; i < len(parts); i++ {
		out = append(out, strings.Join(parts[:i], "/"))
	}
	return out
}

// ReadADR reports one parsed ADR's status, canonical operation progress, and
// plans that link it from the same working authority universe.
func ReadADR(root string, repo *awfgit.Repo, ctx context.Context, identity string) (presentation.Detail, error) {
	ws, err := workingCurrentState(root, repo, ctx)
	if err != nil {
		return presentation.Detail{}, err
	}
	if err := validateAuthority(ws); err != nil {
		return presentation.Detail{}, err
	}
	record, ok := ws.Loaded.Corpus.ByIdentity(identity)
	if !ok {
		return presentation.Detail{}, fmt.Errorf("ADR-%s not found", identity)
	}
	progress, err := record.OperationProgress()
	if err != nil {
		return presentation.Detail{}, err
	}
	plans, drift, err := plansFromTree(ws.Tree, config.DocsDir)
	if err != nil {
		return presentation.Detail{}, err
	}
	if len(drift) != 0 {
		return presentation.Detail{}, fmt.Errorf("parse linked plans: malformed plan")
	}
	linked := []string{}
	for _, p := range plans {
		for _, link := range p.ADRs {
			if link.Identity() == identity {
				linked = append(linked, p.Filename)
				break
			}
		}
	}
	nodes := []presentation.Node{authorityLiteralField("identity", "ADR-"+identity), authorityField("status", record.Status), authorityItems("applied", formatApplied(progress.Applied)), authorityItems("remaining", formatOperations(progress.Remaining)), authorityItems("canceled", formatOperations(progress.Canceled)), authorityItems("linked-plans", linked)}
	return presentation.Detail{Fields: []presentation.Field{authorityField("query", "ADR")}, Sections: []presentation.Section{authoritySection("adr", nodes...)}}, nil
}

func validateAuthority(ws workingState) error {
	findings := currentStateMessages(ws.Loaded.ADRs, ws.Loaded.Topics.All())
	if len(findings) != 0 {
		return fmt.Errorf("current-state validation failed: %s", strings.Join(findings, "; "))
	}
	return nil
}
func currentStateMessages(adrs []adr.ADR, topics []topic.Topic) []string {
	// Keep validation at the coordinator boundary shared by authority queries.
	// QueryTopic has the same existing validation until its later consolidation.
	findings := []string{}
	for _, finding := range currentstate.Check(adrs, topics) {
		findings = append(findings, finding.Message)
	}
	return findings
}

func formatOperations(operations []adr.Operation) []string {
	out := make([]string, len(operations))
	for i, op := range operations {
		out[i] = string(op.Verb) + " " + op.ID
	}
	return out
}
func formatApplied(operations []adr.AppliedOperation) []string {
	out := make([]string, len(operations))
	for i, op := range operations {
		out[i] = string(op.Operation.Verb) + " " + op.Operation.ID
	}
	return out
}
func authorityField(label, text string) presentation.Field {
	v, _ := presentation.Prose(text)
	f, _ := presentation.NewField(label, v)
	return f
}
func authorityLiteralField(label, text string) presentation.Field {
	v, _ := presentation.Literal(text)
	f, _ := presentation.NewField(label, v)
	return f
}
func authorityItems(label string, items []string) presentation.Node {
	if len(items) == 0 {
		return authorityField(label, "none")
	}
	values := make([]presentation.Value, len(items))
	for i, item := range items {
		values[i], _ = presentation.Literal(item)
	}
	l, _ := presentation.NewList(label, values...)
	return l
}
func authoritySection(label string, nodes ...presentation.Node) presentation.Section {
	s, _ := presentation.NewSection(label, nodes...)
	return s
}

// normalizeAuthorityPath makes a command argument repository-relative and
// lexical. Absolute paths outside root and upward traversal are refused.
func NormalizeAuthorityPath(root, value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("path must not be empty")
	}
	candidate := value
	if filepath.IsAbs(candidate) {
		rel, err := filepath.Rel(root, candidate)
		if err != nil {
			return "", err
		}
		candidate = rel
	}
	candidate = filepath.ToSlash(filepath.Clean(candidate))
	if candidate == "." || candidate == ".." || strings.HasPrefix(candidate, "../") {
		return "", fmt.Errorf("path %q is outside the repository", value)
	}
	return candidate, nil
}
