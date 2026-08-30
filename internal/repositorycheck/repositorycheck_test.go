package repositorycheck

import (
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

func result(t *testing.T, findings []checkresult.Finding, information []checkresult.Information) checkresult.Result {
	t.Helper()
	out, err := checkresult.New(findings, information)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func finding(kind, path, detail string) checkresult.Finding {
	return checkresult.Finding{Rank: severity.Error, Property: "correctness", Evidence: checkresult.Evidence{Kind: kind, Path: path, Detail: detail}}
}

func TestComposePreservesExplicitSlotOrderAndCompatibilityProjections(t *testing.T) {
	producer := result(t, []checkresult.Finding{finding("first", "one", "first error")}, []checkresult.Information{{Evidence: checkresult.Evidence{Kind: "unused", Path: "two", Detail: "producer information"}}})
	advisories := result(t, []checkresult.Finding{{Rank: severity.Warn, Property: "heuristic", Evidence: checkresult.Evidence{Kind: "unrelated-spelling", Detail: "ordinary warning"}}}, []checkresult.Information{{Evidence: checkresult.Evidence{Kind: "also-unrelated", Detail: "ordinary information"}}})
	tracking := result(t, nil, []checkresult.Information{{Evidence: checkresult.Evidence{Kind: "anything", Detail: "tracking information"}}})

	report, err := Compose(Inputs{
		Tracking:            Slot{},
		ProducerResults:     []Slot{{Result: producer, IncludeInformationInDrift: true}},
		OrdinaryAdvisories:  Slot{Result: advisories},
		TrackingInformation: Slot{Result: tracking},
	})
	if err != nil {
		t.Fatal(err)
	}
	var driftKinds []string
	for _, drift := range report.Drift {
		driftKinds = append(driftKinds, drift.Kind)
	}
	if want := []string{"first", "unused"}; !slices.Equal(driftKinds, want) {
		t.Fatalf("drift order = %v, want %v", driftKinds, want)
	}
	if !slices.Equal(report.Warnings, []string{"ordinary warning"}) {
		t.Fatalf("ordinary warnings = %v", report.Warnings)
	}
	if !slices.Equal(report.Information, []string{"ordinary information"}) {
		t.Fatalf("aggregate information = %v", report.Information)
	}
	if !slices.Equal(report.TrackingInformation, []string{"tracking information"}) {
		t.Fatalf("tracking information = %v", report.TrackingInformation)
	}

	if !slices.Equal(report.Notes, []string{"ordinary information", "ordinary warning"}) || !slices.Equal(report.TrackingNotes, []string{"tracking information"}) {
		t.Fatalf("compatibility notes = %#v", report)
	}
}

func TestPresentationUsesExplicitRanksAndPreservesEvidence(t *testing.T) {
	mixed := result(t, []checkresult.Finding{
		finding("error-kind", "error.md", "error detail"),
		{Rank: severity.Warn, Property: "heuristic", Evidence: checkresult.Evidence{Kind: "warning-kind", Path: "warning.md", Detail: "warning detail"}},
	}, []checkresult.Information{{Evidence: checkresult.Evidence{Kind: "information-kind", Path: "information.md", Detail: "information detail"}}})

	plain, err := Present(mixed, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if len(plain.Errors) != 1 || len(plain.Warnings) != 1 || len(plain.Information) != 1 {
		t.Fatalf("plain presentation = %#v", plain)
	}
	evidence, err := PresentEvidence(mixed, "owner")
	if err != nil {
		t.Fatal(err)
	}
	combined := (Presentation{}).Append(plain).Append(evidence)
	if len(combined.Errors) != 2 || len(combined.Warnings) != 2 || len(combined.Information) != 2 {
		t.Fatalf("combined presentation = %#v", combined)
	}
	if _, err := Present(mixed, ""); err == nil {
		t.Fatal("presentation accepted an empty check label")
	}
	whitespaceFinding := result(t, []checkresult.Finding{{Rank: severity.Error, Property: "correctness", Evidence: checkresult.Evidence{Kind: "kind", Detail: " "}}}, nil)
	if _, err := Present(whitespaceFinding, "owner"); err == nil {
		t.Fatal("presentation accepted whitespace-only finding detail")
	}
	whitespaceInformation := result(t, nil, []checkresult.Information{{Evidence: checkresult.Evidence{Kind: "kind", Detail: " "}}})
	if _, err := Present(whitespaceInformation, "owner"); err == nil {
		t.Fatal("presentation accepted whitespace-only information detail")
	}
	if !HasErrors(mixed) || HasErrors(result(t, nil, nil)) {
		t.Fatal("HasErrors did not follow explicit ranks")
	}
}

func TestComposeRejectsResultsInWrongExplicitSlots(t *testing.T) {
	errorResult := result(t, []checkresult.Finding{finding("error", "path", "detail")}, nil)
	warningResult := result(t, []checkresult.Finding{{Rank: severity.Warn, Property: "heuristic", Evidence: checkresult.Evidence{Kind: "warning", Detail: "detail"}}}, nil)
	for _, input := range []Inputs{
		{OrdinaryAdvisories: Slot{Result: errorResult}},
		{TrackingInformation: Slot{Result: warningResult}},
	} {
		if _, err := Compose(input); err == nil {
			t.Fatalf("Compose accepted wrong explicit slot: %#v", input)
		}
	}
}

func TestComposeDefensivelyCopiesOwnerResults(t *testing.T) {
	owner := result(t, []checkresult.Finding{finding("first", "path", "detail")}, nil)
	report, err := Compose(Inputs{ProducerResults: []Slot{{Result: owner}}})
	if err != nil {
		t.Fatal(err)
	}
	findings := report.Result.Findings()
	findings[0].Evidence.Detail = "mutated"
	if got := report.Result.Findings()[0].Evidence.Detail; got != "detail" {
		t.Fatalf("aggregated result leaked mutable findings: %q", got)
	}
	ownerFindings := owner.Findings()
	ownerFindings[0].Evidence.Detail = "owner mutation"
	if got := report.Result.Findings()[0].Evidence.Detail; got != "detail" {
		t.Fatalf("report aliases owner result: %q", got)
	}
}

func TestSplitWarningsAndProducerEvidenceValidation(t *testing.T) {
	mixed := result(t, []checkresult.Finding{finding("error", "path", "error"), {Rank: severity.Warn, Property: "heuristic", Evidence: checkresult.Evidence{Kind: "warning", Detail: "warning"}}}, nil)
	withoutWarnings, warnings, err := SplitWarnings(mixed)
	if err != nil {
		t.Fatal(err)
	}
	if len(withoutWarnings.Findings()) != 1 || len(warnings.Findings()) != 1 {
		t.Fatalf("split = %#v %#v", withoutWarnings, warnings)
	}
	if _, err := checkresult.New([]checkresult.Finding{{Rank: severity.Error, Property: "correctness", Evidence: checkresult.Evidence{Kind: "invalid"}}}, nil); err == nil {
		t.Fatal("invalid producer evidence was accepted before aggregation")
	}

}
