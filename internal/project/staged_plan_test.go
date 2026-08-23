package project

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

// plansFromTree parses only regular plan files from one immutable universe.
// The caller supplies that universe's configured docs directory; it never falls
// back to working-tree paths or bytes.
func plansFromTree(tree *snapshot.Tree, docsDir string) ([]plan.Plan, []manifest.Drift, error) {
	prefix := filepath.ToSlash(filepath.Join(docsDir, "plans")) + "/"
	var sources []plan.Source
	for _, file := range tree.List() {
		if !file.Scannable() {
			continue
		}
		name, ok := strings.CutPrefix(file.Path, prefix)
		if !ok || strings.Contains(name, "/") {
			continue
		}
		sources = append(sources, plan.Source{Filename: name, Path: file.Path, Bytes: file.Bytes})
	}
	plans, err := plan.ParseSources(sources)
	if err == nil {
		return plans, nil, nil
	}
	var out []manifest.Drift
	var diagnostics *plan.DiagnosticsError
	if errors.As(err, &diagnostics) {
		for _, diagnostic := range diagnostics.Diagnostics {
			out = append(out, manifest.Drift{Path: filepath.ToSlash(filepath.Join(docsDir, "plans", diagnostic.Path)), Kind: "plan-" + diagnostic.Category, Detail: diagnostic.Detail})
		}
		return plans, out, nil
	}
	return nil, nil, fmt.Errorf("parse staged plans: %w", err) // coverage-ignore: ParseSources converts every parser failure into DiagnosticsError
}

// TestCheckStagedCleanWithCoverage stages a new owned-but-unscoped file over an
// unchanged HEAD topic set: the transition is clean while the index coverage
// reports the uncovered path, proving both sides load and the HEAD-to-index diff
// runs.
func TestPlansFromTreeUsesOnlySnapshotSources(t *testing.T) {
	valid := strings.Replace(projectPlanV1, "format: plan-v1", "format: plan-v2", 1)
	valid = strings.Replace(valid, "- The configured plan reads exactly.", "- `dod: complete` Done.", 1)
	valid = strings.Replace(valid, "**Execution mode: inline.**", "**Execution mode: inline.**\n\nCompletes: [\"complete\"]", 1)
	tree, err := snapshot.NewTree([]snapshot.File{{Path: "guide/plans/2026-08-02-v2.md", Mode: snapshot.Regular, Bytes: []byte(valid)}, {Path: "guide/plans/not-a-plan.md", Mode: snapshot.Regular, Bytes: []byte("ignored")}, {Path: "guide/plans/2026-08-03-broken.md", Mode: snapshot.Regular, Bytes: []byte("---\nformat: plan-v2\n---\n")}})
	if err != nil {
		t.Fatal(err)
	}
	plans, drift, err := plansFromTree(tree, "guide")
	if err != nil || len(plans) != 1 || plans[0].Filename != "2026-08-02-v2.md" || len(drift) != 1 || drift[0].Kind != "plan-structure" {
		t.Fatalf("plansFromTree = %#v %#v %v", plans, drift, err)
	}
}

func TestPlansFromTreeSkipsUnscannableAndNestedSources(t *testing.T) {
	tree, err := snapshot.NewTree([]snapshot.File{
		{Path: "guide/plans/2026-08-02-link.md", Mode: snapshot.Symlink, Bytes: []byte("outside")},
		{Path: "guide/plans/nested/2026-08-02-nested.md", Mode: snapshot.Regular, Bytes: []byte("ignored")},
	})
	if err != nil {
		t.Fatal(err)
	}
	plans, drift, err := plansFromTree(tree, "guide")
	if err != nil || plans != nil || drift != nil {
		t.Fatalf("plansFromTree ignored snapshot sources = %#v %#v %v", plans, drift, err)
	}
}
