package project

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

func TestAppendStagedPlanResultPreservesClassifiedProjections(t *testing.T) {
	result, err := checkresult.New([]checkresult.Finding{
		{Rank: severity.Error, Property: "authority", Evidence: checkresult.Evidence{Kind: "plan-reference", Path: "docs/plans/p.md", Detail: "missing"}},
		{Rank: severity.Warn, Property: "plan-detail-quality", Evidence: checkresult.Evidence{Kind: "plan-advisory", Detail: "assignment"}},
		{Rank: severity.Warn, Property: "plan-detail-quality", Evidence: checkresult.Evidence{Kind: "other", Detail: "not projected"}},
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
