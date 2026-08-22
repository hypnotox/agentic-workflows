package project

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// These remaining project-local helpers prepare Phase 2 context universes; they
// retain their own failure and empty-HEAD contracts while Phase 1 routes checks
// through currentstatecoord.
func TestCurrentStateContextSubstrateFailuresAndEmptyHead(t *testing.T) {
	if _, err := workingCurrentState(filepath.Join(t.TempDir(), "missing"), nil, context.Background()); err == nil {
		t.Fatal("missing filesystem universe accepted")
	}

	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "awf.lock"), "{")
	if _, err := workingCurrentState(root, nil, context.Background()); err == nil {
		t.Fatal("malformed filesystem lock accepted")
	}

	tree, err := snapshot.NewTree(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lockFromTree(tree); err == nil {
		t.Fatal("missing staged lock accepted")
	}
}

func currentStateFindings(r CurrentStateReport) []string {
	var out []string
	for _, finding := range r.Static {
		out = append(out, finding.Message)
	}
	for _, coverage := range r.Coverage {
		if coverage.Severity == severity.Error {
			out = append(out, coverage.Message())
		}
	}
	for _, drift := range r.PlanDrift {
		out = append(out, fmt.Sprintf("%s %s: %s", drift.Kind, drift.Path, drift.Detail))
	}
	return out
}
