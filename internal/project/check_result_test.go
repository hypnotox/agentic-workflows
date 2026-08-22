package project

import (
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

func TestClassifiedCheckResultNamesRankAndProtectedProperty(t *testing.T) {
	report := CheckReport{
		Drift: []manifest.Drift{
			{Kind: "missing", Path: "AGENTS.md", Detail: "file absent"},
			{Kind: "plan-adr-link", Path: "docs/plans/example.md", Detail: "ADR not found"},
			{Kind: "dead-reference", Path: "docs/example.md", Detail: "missing.md"},
			{Kind: "unused-var", Path: ".awf/config.yaml", Detail: "var is unused"},
		},
		Warnings:            []string{"heuristic warning"},
		PlanWarnings:        []string{"plan warning"},
		Information:         []string{"optional cleanup"},
		TrackingInformation: []string{"tracking unavailable"},
	}

	result, err := classifiedCheckResult(report)
	if err != nil {
		t.Fatal(err)
	}
	findings := result.Findings()
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
	information := result.Information()
	gotInformationKinds := make([]string, 0, len(information))
	for _, item := range information {
		gotInformationKinds = append(gotInformationKinds, item.Evidence.Kind)
	}
	if wantKinds := []string{"unused-var", "advisory", "tracking"}; !slices.Equal(gotInformationKinds, wantKinds) {
		t.Fatalf("information kinds = %v, want %v", gotInformationKinds, wantKinds)
	}

	report.Result = result
	report.classified = true
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

func TestClassifiedCheckResultRefusesIncompleteOwnerEvidence(t *testing.T) {
	_, err := classifiedCheckResult(CheckReport{Drift: []manifest.Drift{{Kind: "missing", Path: "AGENTS.md"}}})
	if err == nil {
		t.Fatal("classifiedCheckResult accepted a finding without owner evidence detail")
	}
}
