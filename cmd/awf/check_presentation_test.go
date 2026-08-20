package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

const completedCheckReport = "status: completed\n\nsummary:\n  findings: 0 errors, 0 warnings, 0 information\n"

func requireCheckReport(t *testing.T, got string) {
	t.Helper()
	if got != completedCheckReport {
		t.Fatalf("report = %q, want %q", got, completedCheckReport)
	}
}

func TestCheckReportRejectsInvalidPresentationValues(t *testing.T) {
	if _, err := checkReport(nil, []string{"\n"}, nil); err == nil {
		t.Fatal("invalid information value accepted")
	}
	if _, err := checkReport([]string{"\n"}, nil, nil); err == nil {
		t.Fatal("invalid warning value accepted")
	}
}

// invariant: tooling/cli:check-severity-by-protected-property (TestCheckReportLabelsUnrankedNotesInformation)
func TestCheckReportLabelsUnrankedNotesInformation(t *testing.T) {
	report, err := checkReport(nil, []string{"optional cleanup"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	document, err := report.Document()
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := presentation.Render(&out, document); err != nil {
		t.Fatal(err)
	}
	want := "status: completed\n\nsummary:\n  findings: 0 errors, 0 warnings, 1 information\n\nfindings:\n  information:\n    advisory | optional cleanup\n"
	if out.String() != want {
		t.Fatalf("report = %q, want %q", out.String(), want)
	}
}

func TestCheckReportIncludesInformationCategory(t *testing.T) {
	check, err := presentation.Prose("check")
	if err != nil {
		t.Fatal(err)
	}
	detail, err := presentation.Prose("information")
	if err != nil {
		t.Fatal(err)
	}
	record, err := presentation.NewRecord(check, detail)
	if err != nil {
		t.Fatal(err)
	}
	report, err := checkReport(nil, nil, []presentation.ReportCategory{{Label: "information", Schema: []string{"check", "detail"}, Records: []presentation.Record{record}}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "completed" {
		t.Errorf("status = %q, want completed", report.Status)
	}
}

func TestCheckReportIncludesWarnings(t *testing.T) {
	check, err := presentation.Prose("check")
	if err != nil {
		t.Fatal(err)
	}
	detail, err := presentation.Prose("warning")
	if err != nil {
		t.Fatal(err)
	}
	record, err := presentation.NewRecord(check, detail)
	if err != nil {
		t.Fatal(err)
	}
	report, err := checkReport(nil, nil, []presentation.ReportCategory{{Label: "warnings", Schema: []string{"check", "detail"}, Records: []presentation.Record{record}}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "warnings" {
		t.Errorf("status = %q, want warnings", report.Status)
	}
}

func TestCheckReportRejectsUnknownCategory(t *testing.T) {
	_, err := checkReport(nil, nil, []presentation.ReportCategory{{Label: "unexpected"}})
	if err == nil || !strings.Contains(err.Error(), `unknown check report category "unexpected"`) {
		t.Fatalf("checkReport error = %v, want unknown-category refusal", err)
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
	if err := renderCheckCollection(&failOnWrite{failAt: 1, err: errors.New("unused")}, checkCollection{information: []string{"\n"}}); err == nil {
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
	first := checkCollection{information: []string{"same"}, categories: []presentation.ReportCategory{{Label: "warnings", Schema: []string{"check", "detail"}, Records: []presentation.Record{record(t, "same")}}}}
	second := checkCollection{information: []string{"same", "next"}, categories: []presentation.ReportCategory{{Label: "warnings", Schema: []string{"check", "detail"}, Records: []presentation.Record{record(t, "same")}}, {Label: "errors", Schema: []string{"check", "detail"}, Records: []presentation.Record{record(t, "next")}}}}
	got := first.append(second)
	if len(got.information) != 3 || len(got.categories) != 3 {
		t.Fatalf("source-ordered collection = %#v", got)
	}
}
