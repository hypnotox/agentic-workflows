package project

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// A first adoption whose decisions dir carries two ADRs with the same number
// fails at corpus load: duplicate identity has one home (ADR-0202 item 4), so
// the refusal precedes every consumer rather than being re-derived by each.
func TestInitializeReportSurfacesDuplicateADRIdentity(t *testing.T) {
	root := scaffold(t, sampleYAML)
	for _, name := range []string{"0001-alpha.md", "0001-beta.md"} {
		testsupport.WriteFile(t, filepath.Join(root, "docs/decisions", name),
			testsupport.ADR("Accepted", testsupport.WithDate("2026-07-13"),
				testsupport.WithTitle("0001: A"), testsupport.WithBody("## Context\nx\n")))
	}
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := initializeReportProject(p, InitAuthority{InitializedWithVersion: Version}); err == nil ||
		!strings.Contains(err.Error(), "ADR number 0001 is declared by more than one file") {
		t.Fatalf("expected duplicate ADR identity to fail the corpus load, got %v", err)
	}
}

func TestInitializeReportAcceptsBrownfieldGovernedRecord(t *testing.T) {
	root := scaffold(t, sampleYAML)
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions", "0001-governed.md"),
		"---\nformat: current-state-v2\nstatus: Proposed\ndate: 2026-07-13\n---\n# ADR-0001: Governed\n\n## Context\n\nC.\n\n## Decision\n\n1. D.\n\n## State changes\n\nNone.\n\n## Consequences\n\nC.\n\n## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-07-13: Proposed\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, "docs/decisions", "0001-governed.md"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := initializeReportProject(p, InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatalf("initialize governed brownfield: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(root, "docs/decisions", "0001-governed.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("initialization rewrote the existing ADR")
	}
}
