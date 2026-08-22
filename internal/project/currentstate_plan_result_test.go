package project

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
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
}

func TestAppendStagedPlanResultPreservesRankedProjections(t *testing.T) {
	result, err := checkresult.New([]checkresult.Finding{
		{Rank: severity.Error, Property: "authority", Evidence: checkresult.Evidence{Kind: "plan-reference", Path: "docs/plans/p.md", Detail: "missing"}},
		{Rank: severity.Warn, Property: "plan-detail-quality", Evidence: checkresult.Evidence{Kind: "unrelated-spelling", Detail: "assignment"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	report := CurrentStateReport{}
	appendStagedPlanResult(&report, result)
	if len(report.PlanDrift) != 1 || report.PlanDrift[0].Kind != "plan-reference" {
		t.Fatalf("plan drift = %#v", report.PlanDrift)
	}
	if len(report.PlanNotes) != 1 || report.PlanNotes[0] != "assignment" {
		t.Fatalf("plan notes = %#v", report.PlanNotes)
	}
}
