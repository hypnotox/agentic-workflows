package project

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

const projectPlanV1 = `---
format: plan-v1
date: 2026-08-02
adrs: []
status: Proposed
---
# Plan: Project seam

## Goal

Read through the configured directory without widening scope.

## Architecture summary

Keep Markdown ownership in internal/plan.

## Phase 1: Read

**Execution mode: inline.**

### Task 1.1: Return bytes

Return the model projection unchanged.

### Phase close

Run the gate.

` + "```commit\nfeat(plans): expose reads\n```" + `

## Definition of done

- The configured plan reads exactly.
`

func TestProjectReadPlanUsesConfiguredDocsDirectory(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\ndocsDir: guide\nvars: {}\nskills: []\nagents: []\ndocs: []\ndomains: []\n")
	path := filepath.Join(root, "guide/plans/2026-08-02-project-seam.md")
	testsupport.WriteFile(t, path, projectPlanV1)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.ReadPlan("2026-08-02-project-seam", "1.1")
	if err != nil {
		t.Fatalf("ReadPlan: %v", err)
	}
	if !bytes.Contains(got, []byte("### Task 1.1: Return bytes")) || bytes.Contains(got, []byte("guide/plans")) {
		t.Fatalf("projection = %q", got)
	}
}

func TestProjectReadPlanPreservesTypedErrors(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\nskills: []\nagents: []\ndocs: []\ndomains: []\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/plans/2026-08-02-project-seam.md"), projectPlanV1)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.ReadPlan("missing", "1")
	var notFound *plan.NotFoundError
	if !errors.As(err, &notFound) || notFound.Kind != "name" {
		t.Fatalf("name error = %#v", err)
	}
	_, err = p.ReadPlan("2026-08-02-project-seam", "01")
	var invalid *plan.InvalidSelectorError
	if !errors.As(err, &invalid) {
		t.Fatalf("selector error = %#v", err)
	}
}
