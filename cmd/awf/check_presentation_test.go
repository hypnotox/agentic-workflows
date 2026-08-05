package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

const completedCheckReport = "status: completed\n\nsummary:\n  findings: 0 errors, 0 warnings\n"

func requireCheckReport(t *testing.T, got string) {
	t.Helper()
	if got != completedCheckReport {
		t.Fatalf("report = %q, want %q", got, completedCheckReport)
	}
}

func TestCheckReportRejectsInvalidPresentationValues(t *testing.T) {
	if _, err := checkReport([]string{"\n"}, nil); err == nil {
		t.Fatal("invalid advisory value accepted")
	}
}

func TestRenderCheckCollectionPropagatesConstructionAndWriterFailures(t *testing.T) {
	var clean strings.Builder
	if err := renderCheckCollection(&clean, checkCollection{}); err != nil {
		t.Fatal(err)
	}
	requireCheckReport(t, clean.String())
	if err := renderCheckCollection(&failOnWrite{failAt: 1, err: errors.New("writer failed")}, checkCollection{}); err == nil {
		t.Fatal("writer failure accepted")
	}
	if err := renderCheckCollection(&failOnWrite{failAt: 1, err: errors.New("unused")}, checkCollection{notes: []string{"\n"}}); err == nil {
		t.Fatal("invalid report fact accepted")
	}
	if err := renderCheckCollection(&strings.Builder{}, checkCollection{categories: []presentation.ReportCategory{{Label: "errors", Schema: []string{"check", "detail"}, Records: []presentation.Record{{}}}}}); err == nil {
		t.Fatal("invalid report category accepted")
	}
}

func TestProducedCheckFailureDelegatesError(t *testing.T) {
	failure := errors.New("failed")
	produced := producedCheckFailure{err: failure}
	if got := produced.Error(); got != "failed" {
		t.Fatalf("Error() = %q", got)
	}
	if !errors.Is(produced, failure) {
		t.Fatal("Unwrap() lost the cause")
	}
}

func TestCheckCollectionAppendPreservesOrdinaryEvidence(t *testing.T) {
	record := func(t *testing.T, text string) presentation.Record {
		t.Helper()
		check, err := presentation.Prose("check")
		if err != nil {
			t.Fatal(err)
		}
		detail, err := presentation.Prose(text)
		if err != nil {
			t.Fatal(err)
		}
		value, err := presentation.NewRecord(check, detail)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	first := checkCollection{notes: []string{"same"}, categories: []presentation.ReportCategory{{Label: "warnings", Schema: []string{"check", "detail"}, Records: []presentation.Record{record(t, "same")}}}}
	second := checkCollection{notes: []string{"same", "next"}, categories: []presentation.ReportCategory{{Label: "warnings", Schema: []string{"check", "detail"}, Records: []presentation.Record{record(t, "same")}}, {Label: "errors", Schema: []string{"check", "detail"}, Records: []presentation.Record{record(t, "next")}}}}
	got := first.append(second)
	if len(got.notes) != 3 || len(got.categories) != 3 {
		t.Fatalf("source-ordered collection = %#v", got)
	}
}
