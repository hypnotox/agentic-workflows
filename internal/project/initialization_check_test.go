package project

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

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
