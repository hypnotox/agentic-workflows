package currentstatecoord

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

func TestCurrentStateResultDerivesOnlyCoverage(t *testing.T) {
	report, err := classifyCurrentState(CurrentStateReport{})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.CurrentResult.Findings()) != 0 {
		t.Fatalf("current result = %#v", report.CurrentResult.Findings())
	}
}

func TestResidualPlanResultIsPreserved(t *testing.T) {
	result, err := checkresult.New([]checkresult.Finding{{Rank: severity.Error, Property: "authority", Evidence: checkresult.Evidence{Kind: "plan-reference", Path: "docs/plans/p.md", Detail: "missing"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	report, err := classifyCurrentState(CurrentStateReport{PlanDrift: []manifest.Drift{{Kind: "plan-frontmatter", Path: "docs/plans/bad.md", Detail: "malformed"}}, PlanResult: result})
	if err != nil {
		t.Fatal(err)
	}
	if got := report.Result().Findings(); len(got) != 2 || got[0].Property != propertyPlanArtifact || got[1].Property != "authority" {
		t.Fatalf("result findings = %#v", got)
	}
}
