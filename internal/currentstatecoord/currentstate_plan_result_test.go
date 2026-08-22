package currentstatecoord

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

func TestClassifyCurrentStateRejectsInvalidOwnerEvidence(t *testing.T) {
	if _, err := classifyCurrentState(CurrentStateReport{Static: []currentstate.Finding{{}}}); err == nil {
		t.Fatal("classification accepted invalid owner evidence")
	}
}

func TestCurrentStateResultIsStableAfterCompatibilityMutation(t *testing.T) {
	report, err := classifyCurrentState(CurrentStateReport{Static: []currentstate.Finding{{Message: "original"}}})
	if err != nil {
		t.Fatal(err)
	}
	report.Static[0].Message = "mutated"
	result := report.Result()
	if got := result.Findings()[0].Evidence.Detail; got != "original" {
		t.Fatalf("owner result changed with compatibility slice: %q", got)
	}
	if got := report.CurrentResult.Findings()[0].Evidence.Detail; got != "original" {
		t.Fatalf("current-state partition changed with compatibility slice: %q", got)
	}
	if len(report.PlanArtifactResult.Findings()) != 0 {
		t.Fatalf("empty plan partition = %#v", report.PlanArtifactResult.Findings())
	}
}

func TestAppendStagedPlanResultPreservesRankedProjections(t *testing.T) {
	result, err := checkresult.New([]checkresult.Finding{
		{Rank: severity.Error, Property: "authority", Evidence: checkresult.Evidence{Kind: "plan-reference", Path: "docs/plans/p.md", Detail: "missing"}},
		{Rank: severity.Warn, Property: "plan-detail-quality", Evidence: checkresult.Evidence{Kind: "unrelated-spelling", Detail: "assignment"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	report, err := classifyCurrentState(CurrentStateReport{
		PlanDrift:  []manifest.Drift{{Kind: "plan-frontmatter", Path: "docs/plans/bad.md", Detail: "malformed"}},
		PlanResult: result,
	})
	if err != nil {
		t.Fatal(err)
	}
	appendStagedPlanResult(&report, result)
	if len(report.PlanDrift) != 2 || report.PlanDrift[0].Kind != "plan-frontmatter" || report.PlanDrift[1].Kind != "plan-reference" {
		t.Fatalf("plan drift = %#v", report.PlanDrift)
	}
	if len(report.PlanNotes) != 1 || report.PlanNotes[0] != "assignment" {
		t.Fatalf("plan notes = %#v", report.PlanNotes)
	}
	findings := report.Result().Findings()
	if len(findings) != 3 || findings[0].Rank != severity.Error || findings[0].Property != propertyPlanArtifact || findings[0].Evidence.Detail != "plan-frontmatter docs/plans/bad.md: malformed" || findings[1].Rank != severity.Error || findings[1].Property != "authority" || findings[1].Evidence.Detail != "plan-reference docs/plans/p.md: missing" || findings[2].Rank != severity.Warn || findings[2].Property != "plan-detail-quality" {
		t.Fatalf("typed staged plan findings = %#v", findings)
	}
	if len(report.CurrentResult.Findings()) != 0 || len(report.PlanArtifactResult.Findings()) != 3 {
		t.Fatalf("typed staged partitions = current %#v plan %#v", report.CurrentResult.Findings(), report.PlanArtifactResult.Findings())
	}
	report.PlanNotes[0] = "mutated compatibility note"
	if got := report.Result().Findings()[2].Evidence.Detail; got != "assignment" {
		t.Fatalf("typed staged plan warning changed with compatibility projection: %q", got)
	}
}
