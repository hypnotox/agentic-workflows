package project

import (
	"fmt"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

// TestCurrentStateContextSubstrateFailuresAndEmptyHead retains staged helper contracts.
func TestCurrentStateContextSubstrateFailuresAndEmptyHead(t *testing.T) {
	tree, err := snapshot.NewTree(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lockFromTree(tree); err == nil {
		t.Fatal("missing staged lock accepted")
	}
}

func TestLoadTreeCurrentStatePropagatesAuthorityParseFailure(t *testing.T) {
	tree, err := snapshot.NewTree([]snapshot.File{
		{Path: ".awf/config.yaml", Mode: snapshot.Regular, Bytes: []byte("prefix: test\nprofile: full\nintegrationBranch: main\ndomains: [tooling]\n")},
		{Path: "docs/decisions/bad.md", Mode: snapshot.Regular, Bytes: []byte("not an ADR\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadTreeCurrentState(t.TempDir(), tree, nil); err == nil {
		t.Fatal("malformed selected authority was accepted")
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
