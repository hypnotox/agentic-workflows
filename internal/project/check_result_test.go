package project

import (
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

func TestProducerBatchNamesRankAndProtectedProperty(t *testing.T) {
	batch := checkBatch{}
	batch.error(propertyReproducibility, "missing", "AGENTS.md", "file absent")
	batch.error(propertyAuthority, "plan-adr-link", "docs/plans/example.md", "ADR not found")
	batch.error(propertyCorrectness, "dead-reference", "docs/example.md", "missing.md")
	batch.informationItem("unused-var", ".awf/config.yaml", "var is unused")
	batch.warning(propertyHeuristic, "advisory", "heuristic warning")
	batch.warning(propertyPlanDetail, "plan-advisory", "plan warning")
	batch.informationItem("advisory", "", "optional cleanup")
	batch.informationItem("tracking", "", "tracking unavailable")

	report, err := reportFromBatch(batch)
	if err != nil {
		t.Fatal(err)
	}
	findings := report.Result.Findings()
	want := []struct {
		rank     severity.Rank
		property checkresult.Property
		kind     string
	}{
		{severity.Error, propertyReproducibility, "missing"},
		{severity.Error, propertyAuthority, "plan-adr-link"},
		{severity.Error, propertyCorrectness, "dead-reference"},
		{severity.Warn, propertyHeuristic, "advisory"},
		{severity.Warn, propertyPlanDetail, "plan-advisory"},
	}
	if len(findings) != len(want) {
		t.Fatalf("findings = %#v, want %d", findings, len(want))
	}
	for i, expected := range want {
		if findings[i].Rank != expected.rank || findings[i].Property != expected.property || findings[i].Evidence.Kind != expected.kind {
			t.Errorf("finding %d = %#v, want rank=%v property=%q kind=%q", i, findings[i], expected.rank, expected.property, expected.kind)
		}
	}
	var informationKinds []string
	for _, item := range report.Result.Information() {
		informationKinds = append(informationKinds, item.Evidence.Kind)
	}
	if wantKinds := []string{"unused-var", "advisory", "tracking"}; !slices.Equal(informationKinds, wantKinds) {
		t.Fatalf("information kinds = %v, want %v", informationKinds, wantKinds)
	}
	if got := report.OrdinaryWarnings(); !slices.Equal(got, []string{"heuristic warning"}) {
		t.Fatalf("OrdinaryWarnings() = %v", got)
	}
	if got := report.PlanWarningNotes(); !slices.Equal(got, []string{"plan warning"}) {
		t.Fatalf("PlanWarningNotes() = %v", got)
	}
	if got := report.AggregateInformation(); !slices.Equal(got, []string{"optional cleanup"}) {
		t.Fatalf("AggregateInformation() = %v", got)
	}
	if got := report.DirectTrackingInformation(); !slices.Equal(got, []string{"tracking unavailable"}) {
		t.Fatalf("DirectTrackingInformation() = %v", got)
	}
}

func TestBatchSeparatesDeferredWarningsWithoutChangingSemanticProjections(t *testing.T) {
	batch := checkBatch{}
	batch.error(propertyAuthority, "plan-reference", "plan.md", "missing ADR")
	batch.warning(propertyPlanDetail, "plan-advisory", "assignment missing")
	batch.informationItem("advisory", "", "optional note")
	withoutWarnings := batch.withoutWarnings()
	result, err := withoutWarnings.result()
	if err != nil || len(result.Findings()) != 1 || len(result.Information()) != 1 {
		t.Fatalf("withoutWarnings result = %#v, %v", result, err)
	}
	warnings := batch.warningsOnly()
	result, err = warnings.result()
	if err != nil || len(result.Findings()) != 1 || len(result.Information()) != 0 {
		t.Fatalf("warningsOnly result = %#v, %v", result, err)
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

func TestReportFinalizerRefusesIncompleteProducerEvidence(t *testing.T) {
	batch := checkBatch{}
	batch.error(propertyCorrectness, "missing", "AGENTS.md", "")
	if _, err := reportFromBatch(batch); err == nil {
		t.Fatal("report finalizer accepted a finding without evidence detail")
	}
}
