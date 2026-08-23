package project

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// TestCheckPlansValidatesFrontmatterAndLinks exercises checkPlans over a
// docs/plans/ set: a plan linking a nonexistent ADR yields plan-adr-link drift,
// a bad status: yields plan-frontmatter drift, a valid plan yields none, and a
// frontmatter-less (grandfathered) plan is skipped. A slug entry resolves
// against a pending record and drifts when it names none (ADR-0202 item 14).
// invariant: adr-system/plan-artifacts:plan-frontmatter-validated (TestCheckPlansValidatesFrontmatterAndLinks)
// invariant: adr-system/plan-artifacts:plan-adr-link-resolved (TestCheckPlansValidatesFrontmatterAndLinks)
func TestCheckPlansValidatesFrontmatterAndLinks(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	// One real ADR (0001) for links to resolve against.
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-real.md"),
		testsupport.ADR("Accepted", testsupport.WithDate("2026-07-12"),
			testsupport.WithTitle("0001: Real"), testsupport.WithBody("## Context\nx\n")))

	write := func(name, content string) {
		testsupport.WriteFile(t, filepath.Join(root, "docs/plans", name), content)
	}
	// One pending record and one already-numbered record that retained its slug,
	// so both slug resolution paths the claim names are exercised.
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/still-pending.md"), pendingADRFixture("still-pending"))
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0002-was-pending.md"),
		strings.Replace(pendingADRFixture("was-pending"), "# ADR-was-pending:", "# ADR-0002:", 1))

	write("2026-07-12-good.md", "---\ndate: 2026-07-12\nadrs: [1]\nstatus: Proposed\n---\n# Plan: Good\n")
	write("2026-07-12-bad-link.md", "---\ndate: 2026-07-12\nadrs: [42]\nstatus: Proposed\n---\n# Plan: Bad Link\n")
	write("2026-07-12-bad-status.md", "---\ndate: 2026-07-12\nadrs: [1]\nstatus: Draft\n---\n# Plan: Bad Status\n")
	write("2026-07-12-slug-link.md", "---\ndate: 2026-07-12\nadrs: [still-pending]\nstatus: Proposed\n---\n# Plan: Slug Link\n")
	write("2026-07-12-retained-slug-link.md", "---\ndate: 2026-07-12\nadrs: [was-pending]\nstatus: Proposed\n---\n# Plan: Retained Slug Link\n")
	write("2026-07-12-bad-slug-link.md", "---\ndate: 2026-07-12\nadrs: [never-authored]\nstatus: Proposed\n---\n# Plan: Bad Slug Link\n")
	write("2026-06-24-legacy.md", "# Plan: Legacy\n\nNo frontmatter, grandfathered.\n")

	drift := checkPlans(renderInputsForTest(p), mustDeriveCorpus(t, p), mustParsePlans(t, p))

	got := map[string]string{}
	for _, d := range drift {
		got[d.Kind+"@"+filepath.Base(d.Path)] = d.Detail
	}
	if len(drift) != 3 {
		t.Fatalf("expected exactly 3 drifts (bad-link, bad-slug-link, bad-status), got %d: %#v", len(drift), drift)
	}
	if d, ok := got["plan-adr-link@2026-07-12-bad-link.md"]; !ok || d != "ADR-0042" {
		t.Errorf("expected plan-adr-link ADR-0042 drift, got %#v", drift)
	}
	if d, ok := got["plan-adr-link@2026-07-12-bad-slug-link.md"]; !ok || d != "ADR-never-authored" {
		t.Errorf("expected plan-adr-link ADR-never-authored drift, got %#v", drift)
	}
	if _, ok := got["plan-adr-link@2026-07-12-slug-link.md"]; ok {
		t.Errorf("slug link to a pending record must resolve, got %#v", drift)
	}
	if _, ok := got["plan-adr-link@2026-07-12-retained-slug-link.md"]; ok {
		t.Errorf("slug link to a numbered record's retained slug must resolve, got %#v", drift)
	}
	if _, ok := got["plan-frontmatter@2026-07-12-bad-status.md"]; !ok {
		t.Errorf("expected plan-frontmatter drift for bad status, got %#v", drift)
	}

	structured := `---
format: plan-v1
date: 2026-07-12
adrs: []
status: Proposed
---
# Plan: Structured

## Goal

Validate frontmatter.

## Architecture summary

Keep parsing in internal/plan.

## Phase 1: Check

**Execution mode: inline.**

### Task 1.1: Check marker

Parse the marker.

### Phase close

Run the gate.

` + "```commit\ntest(plans): validate frontmatter\n```" + `

## Definition of done

- Frontmatter is validated.
`
	for _, status := range []string{"Proposed", "Implemented"} {
		t.Run("plan-v1 "+status, func(t *testing.T) {
			dir := t.TempDir()
			testsupport.WriteFile(t, filepath.Join(dir, "2026-07-12-structured.md"), strings.Replace(structured, "status: Proposed", "status: "+status, 1))
			plans, err := plan.ParseDir(dir)
			if err != nil || len(plans) != 1 || plans[0].Format != "plan-v1" || plans[0].Status != status {
				t.Fatalf("ParseDir plans=%#v err=%v", plans, err)
			}
		})
	}
	for _, tc := range []struct{ name, body, detail string }{
		{"empty format", strings.Replace(structured, "format: plan-v1", `format: ""`, 1), "format must be a nonempty string"},
		{"unknown format", strings.Replace(structured, "format: plan-v1", "format: plan-v3", 1), "format must be exactly plan-v1 or plan-v2"},
		{"duplicate format", strings.Replace(structured, "format: plan-v1", "format: plan-v1\nformat: plan-v1", 1), "duplicate format"},
		{"malformed format", strings.Replace(structured, "format: plan-v1", "format: [plan-v1]", 1), "format must be a nonempty string"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			testsupport.WriteFile(t, filepath.Join(dir, "2026-07-12-structured.md"), tc.body)
			_, err := plan.ParseDir(dir)
			var diagnostic *plan.Diagnostic
			if !errors.As(err, &diagnostic) || diagnostic.Category != "frontmatter" || diagnostic.Detail != tc.detail {
				t.Fatalf("diagnostic=%#v err=%v", diagnostic, err)
			}
		})
	}
}

func TestCheckReportConsumesPreparedPlansWithoutParsing(t *testing.T) {
	source, err := os.ReadFile(filepath.Join(testsupport.RepoRoot(t), "internal/project/check.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if strings.Contains(text, "plan.ParseDir(") || strings.Contains(text, "adr.LoadCorpus(") || strings.Contains(text, "topic.LoadCorpus(") {
		t.Fatal("project check or advisory policy re-derives Publisher-owned operation state")
	}
	if !strings.Contains(text, "semantics OperationSemantics") {
		t.Fatal("checkReport no longer receives the prepared semantic universe")
	}
}

// TestCheckReportMapsPlanDiagnostics proves malformed plan frontmatter reaches
// the stable drift channel while valid sibling plans remain checkable.
func TestCheckReportMapsPlanDiagnostics(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, "docs/plans/2026-07-12-broken.md"),
		"---\nstatus: [unterminated\n---\n# Plan: Broken\n")
	report, err := checkReportProject(p, testContext(t))
	if err != nil {
		t.Fatalf("CheckReport: %v", err)
	}
	if !slices.ContainsFunc(report.Drift, func(d manifest.Drift) bool {
		return d.Path == "docs/plans/2026-07-12-broken.md" && d.Kind == "plan-frontmatter" && strings.Contains(d.Detail, "yaml")
	}) {
		t.Fatalf("plan diagnostic did not reach drift: %#v", report.Drift)
	}
}

func TestBuildCheckReportRefusesUnknownDynamicPlanDiagnosticCategory(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	prepared, err := testPublisher(operationInputs(p, testConfig(p))).Prepare()
	if err != nil {
		t.Fatal(err)
	}
	semantics := OperationSemantics{ADRs: prepared.ADRs(), Pitfalls: prepared.Pitfalls(), Topics: prepared.Topics(), EffectiveSkills: prepared.EffectiveSkills(), Plans: prepared.Plans(), PlansError: &plan.DiagnosticsError{Diagnostics: []*plan.Diagnostic{{Category: "future-category", Path: "example.md", Detail: "future diagnostic"}}}}
	if _, err := BuildCheckReport(p, testConfig(p), testRepo(p), testContext(t), prepared.Plan(), semantics); err == nil || !strings.Contains(err.Error(), "unknown plan diagnostic category") {
		t.Fatalf("BuildCheckReport error = %v, want unknown diagnostic category refusal", err)
	}
}

func TestCheckReportPropagatesPreparedPlanReadError(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	prepared, err := testPublisher(operationInputs(p, testConfig(p))).Prepare()
	if err != nil {
		t.Fatal(err)
	}
	semantics := OperationSemantics{ADRs: prepared.ADRs(), Pitfalls: prepared.Pitfalls(), Topics: prepared.Topics(), EffectiveSkills: prepared.EffectiveSkills(), Plans: prepared.Plans(), PlansError: errors.New("read selected plans: injected fault")}
	if _, err := BuildCheckReport(p, testConfig(p), testRepo(p), testContext(t), prepared.Plan(), semantics); err == nil || !strings.Contains(err.Error(), "injected fault") {
		t.Fatalf("CheckReport error = %v, want prepared plan read failure", err)
	}
}

// TestCheckProjectsPlanDiagnostics proves Check's compatibility projection
// exposes malformed plan frontmatter as drift rather than a process error.
func TestCheckProjectsPlanDiagnostics(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, "docs/plans/2026-07-12-broken.md"),
		"---\nstatus: [unterminated\n---\n# Plan: Broken\n")
	drift, err := checkProject(p, testContext(t))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !slices.ContainsFunc(drift, func(d manifest.Drift) bool { return d.Kind == "plan-frontmatter" }) {
		t.Fatalf("Check omitted plan-frontmatter drift: %#v", drift)
	}
}
