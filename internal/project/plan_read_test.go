package project

import (
	"bytes"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
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

func TestProjectReadPlanUsesFixedDocsDirectory(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {}\ndomains: []\n")
	path := filepath.Join(root, "docs/plans/2026-08-02-project-seam.md")
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

func TestProjectReadPlanV2ComposesTaskScope(t *testing.T) {
	root := scaffold(t, sampleYAML)
	body := strings.Replace(projectPlanV1, "format: plan-v1", "format: plan-v2", 1)
	body = strings.Replace(body, "- The configured plan reads exactly.", "- `dod: complete` The configured plan reads exactly.", 1)
	body = strings.Replace(body, "**Execution mode: inline.**", "**Execution mode: inline.**\n\nCompletes: [\"complete\"]", 1)
	testsupport.WriteFile(t, filepath.Join(root, "docs/plans/2026-08-02-project-v2.md"), body)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.ReadPlan("2026-08-02-project-v2", "1.1")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("Scope notice: only this task is in scope")) || !bytes.Contains(got, []byte("Completed outcomes")) {
		t.Fatalf("v2 projection = %s", got)
	}
}

func TestProjectReadPlanV2PhaseUnionsTaskContext(t *testing.T) {
	root := scaffold(t, sampleYAML)
	body := strings.Replace(projectPlanV1, "format: plan-v1", "format: plan-v2", 1)
	body = strings.Replace(body, "- The configured plan reads exactly.", "- `dod: complete` Done.", 1)
	body = strings.Replace(body, "**Execution mode: inline.**", "**Execution mode: inline.**\n\nCompletes: [\"complete\"]", 1)
	testsupport.WriteFile(t, filepath.Join(root, "docs/plans/2026-08-02-project-v2-phase.md"), body)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.ReadPlan("2026-08-02-project-v2-phase", "1")
	if err != nil || !bytes.Contains(got, []byte("### Task 1.1")) {
		t.Fatalf("phase projection = %s, %v", got, err)
	}
}

func TestProjectReadPlanV2RejectsUnresolvedDecisionReferences(t *testing.T) {
	for _, tc := range []struct{ name, field string }{{"Applying", "Applying: [\"missing:first\"]"}, {"Context", "Context: [\"missing:first\"]"}} {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffold(t, sampleYAML)
			body := strings.Replace(projectPlanV1, "format: plan-v1", "format: plan-v2", 1)
			body = strings.Replace(body, "- The configured plan reads exactly.", "- `dod: complete` Done.", 1)
			body = strings.Replace(body, "**Execution mode: inline.**", "**Execution mode: inline.**\n\nCompletes: [\"complete\"]", 1)
			body = strings.Replace(body, "### Task 1.1: Return bytes\n", "### Task 1.1: Return bytes\n"+tc.field+"\n", 1)
			testsupport.WriteFile(t, filepath.Join(root, "docs/plans/2026-08-02-unresolved.md"), body)
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = p.ReadPlan("2026-08-02-unresolved", "1.1"); err == nil || !strings.Contains(err.Error(), "ADR not found") {
				t.Fatalf("ReadPlan error = %v", err)
			}
		})
	}
}

func TestProjectReadPlanV2RejectsMissingSelector(t *testing.T) {
	root := scaffold(t, sampleYAML)
	body := strings.Replace(projectPlanV1, "format: plan-v1", "format: plan-v2", 1)
	body = strings.Replace(body, "- The configured plan reads exactly.", "- `dod: complete` Done.", 1)
	body = strings.Replace(body, "**Execution mode: inline.**", "**Execution mode: inline.**\n\nCompletes: [\"complete\"]", 1)
	testsupport.WriteFile(t, filepath.Join(root, "docs/plans/2026-08-02-selector.md"), body)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReadPlan("2026-08-02-selector", "9"); err == nil || !strings.Contains(err.Error(), "selector") {
		t.Fatalf("missing v2 selector error = %v", err)
	}
}

func TestProjectReadPlanV2SelectorErrorsPrecedeMalformedCorpus(t *testing.T) {
	root := scaffold(t, sampleYAML)
	body := strings.Replace(projectPlanV1, "format: plan-v1", "format: plan-v2", 1)
	body = strings.Replace(body, "- The configured plan reads exactly.", "- `dod: complete` Done.", 1)
	body = strings.Replace(body, "**Execution mode: inline.**", "**Execution mode: inline.**\n\nCompletes: [\"complete\"]", 1)
	testsupport.WriteFile(t, filepath.Join(root, "docs/plans/2026-08-02-selector-precedence.md"), body)
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-broken.md"), "---\nstatus: [\n---\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, selector := range []string{"01", "9"} {
		_, err := p.ReadPlan("2026-08-02-selector-precedence", selector)
		if selector == "01" {
			var invalid *plan.InvalidSelectorError
			if !errors.As(err, &invalid) || invalid.Value != selector || !reflect.DeepEqual(invalid.Available, []string{"1", "1.1"}) {
				t.Fatalf("invalid selector error = %#v", err)
			}
			continue
		}
		var missing *plan.NotFoundError
		if !errors.As(err, &missing) || missing.Kind != "selector" || missing.Value != selector || !reflect.DeepEqual(missing.Available, []string{"1", "1.1"}) {
			t.Fatalf("missing selector error = %#v", err)
		}
	}
}

func TestProjectReadPlanV2SurfacesCorpusFailureBeforeProjection(t *testing.T) {
	root := scaffold(t, sampleYAML)
	body := strings.Replace(projectPlanV1, "format: plan-v1", "format: plan-v2", 1)
	body = strings.Replace(body, "- The configured plan reads exactly.", "- `dod: complete` Done.", 1)
	body = strings.Replace(body, "**Execution mode: inline.**", "**Execution mode: inline.**\n\nCompletes: [\"complete\"]", 1)
	testsupport.WriteFile(t, filepath.Join(root, "docs/plans/2026-08-02-broken-corpus.md"), body)
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-broken.md"), "---\nstatus: [\n---\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReadPlan("2026-08-02-broken-corpus", "1.1"); err == nil {
		t.Fatal("ReadPlan accepted malformed ADR corpus")
	}
}

func TestProjectReadPlanPreservesTypedErrors(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {}\ndomains: []\n")
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
