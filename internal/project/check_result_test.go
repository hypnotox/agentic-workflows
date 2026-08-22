package project

import (
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/repositorycheck"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

func projectTestResult(t *testing.T, findings []checkresult.Finding, information []checkresult.Information) checkresult.Result {
	t.Helper()
	result, err := checkresult.New(findings, information)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestProducerResultsNameRankAndProtectedProperty(t *testing.T) {
	producer := projectTestResult(t, []checkresult.Finding{
		{Rank: severity.Error, Property: propertyReproducibility, Evidence: checkresult.Evidence{Kind: "missing", Path: "AGENTS.md", Detail: "file absent"}},
		{Rank: severity.Error, Property: propertyAuthority, Evidence: checkresult.Evidence{Kind: "plan-adr-link", Path: "docs/plans/example.md", Detail: "ADR not found"}},
		{Rank: severity.Error, Property: propertyCorrectness, Evidence: checkresult.Evidence{Kind: "dead-reference", Path: "docs/example.md", Detail: "missing.md"}},
	}, []checkresult.Information{{Evidence: checkresult.Evidence{Kind: "unused-var", Path: ".awf/config.yaml", Detail: "var is unused"}}})
	advisories := projectTestResult(t, []checkresult.Finding{{Rank: severity.Warn, Property: propertyHeuristic, Evidence: checkresult.Evidence{Kind: "advisory", Detail: "heuristic warning"}}}, []checkresult.Information{{Evidence: checkresult.Evidence{Kind: "advisory", Detail: "optional cleanup"}}})
	tracking := projectTestResult(t, nil, []checkresult.Information{{Evidence: checkresult.Evidence{Kind: "tracking", Detail: "tracking unavailable"}}})
	planWarnings := projectTestResult(t, []checkresult.Finding{{Rank: severity.Warn, Property: propertyPlanDetail, Evidence: checkresult.Evidence{Kind: "plan-advisory", Detail: "plan warning"}}}, nil)

	report, err := repositorycheck.Compose(repositorycheck.Inputs{
		ProducerResults:      []repositorycheck.Slot{{Result: producer, IncludeInformationInDrift: true}},
		OrdinaryAdvisories:   repositorycheck.Slot{Result: advisories},
		TrackingInformation:  repositorycheck.Slot{Result: tracking},
		DeferredPlanWarnings: repositorycheck.Slot{Result: planWarnings},
	})
	if err != nil {
		t.Fatal(err)
	}
	findings := report.Result.Findings()
	wantProperties := []checkresult.Property{propertyReproducibility, propertyAuthority, propertyCorrectness, propertyHeuristic, propertyPlanDetail}
	for i, want := range wantProperties {
		if findings[i].Property != want {
			t.Errorf("finding %d property = %q, want %q", i, findings[i].Property, want)
		}
	}
	if got := report.Warnings; !slices.Equal(got, []string{"heuristic warning"}) {
		t.Fatalf("Warnings = %v", got)
	}
	if got := report.PlanWarnings; !slices.Equal(got, []string{"plan warning"}) {
		t.Fatalf("PlanWarnings = %v", got)
	}
	if got := report.Information; !slices.Equal(got, []string{"optional cleanup"}) {
		t.Fatalf("Information = %v", got)
	}
	if got := report.TrackingInformation; !slices.Equal(got, []string{"tracking unavailable"}) {
		t.Fatalf("TrackingInformation = %v", got)
	}
}

func TestKnownDynamicPlanDiagnosticCategoriesAreClosed(t *testing.T) {
	for _, category := range []string{"field", "frontmatter", "numbering", "path", "paths", "phase-close", "projection", "relationship", "structure"} {
		if !knownDynamicPlanDiagnosticCategory(category) {
			t.Errorf("known category %q was refused", category)
		}
	}
	if knownDynamicPlanDiagnosticCategory("future-category") {
		t.Fatal("unknown dynamic plan category was accepted")
	}
}

func TestOwnerResultBoundaryRefusesIncompleteEvidence(t *testing.T) {
	if _, err := checkresult.New([]checkresult.Finding{{Rank: severity.Error, Property: propertyCorrectness, Evidence: checkresult.Evidence{Kind: "missing", Path: "AGENTS.md"}}}, nil); err == nil {
		t.Fatal("result boundary accepted a finding without evidence detail")
	}
}
