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

// invariant: tooling/cli:check-severity-by-protected-property (TestCheckSeverityByProtectedProperty)
func TestCheckSeverityByProtectedProperty(t *testing.T) {
	category := func(label, detail string) presentation.ReportCategory {
		t.Helper()
		check, err := presentation.Prose("check")
		if err != nil {
			t.Fatal(err)
		}
		value, err := presentation.Prose(detail)
		if err != nil {
			t.Fatal(err)
		}
		record, err := presentation.NewRecord(check, value)
		if err != nil {
			t.Fatal(err)
		}
		return presentation.ReportCategory{Label: label, Schema: []string{"check", "detail"}, Records: []presentation.Record{record}}
	}
	render := func(collection checkCollection) (string, error) {
		t.Helper()
		var out strings.Builder
		err := renderCheckCollection(&out, collection)
		return out.String(), err
	}

	errorCategory := category("errors", "invalid authority")
	warningCategory := category("warnings", "style heuristic")
	informationCategory := category("information", "optional cleanup")
	errorFailure := producedCheckFailure{errors.New("invalid authority")}
	cases := []struct {
		name           string
		direct         checkCollection
		aggregateParts []checkCollection
		want           string
		wantError      bool
	}{
		{
			name:           "Error-only fails",
			direct:         checkCollection{categories: []presentation.ReportCategory{errorCategory}, failures: []error{errorFailure}},
			aggregateParts: []checkCollection{{categories: []presentation.ReportCategory{errorCategory}}, {failures: []error{errorFailure}}},
			want:           "status: failed\n\nsummary:\n  findings: 1 errors, 0 warnings, 0 information\n\nfindings:\n  errors:\n    check | invalid authority\n",
			wantError:      true,
		},
		{
			name:           "Warning-only succeeds",
			direct:         checkCollection{categories: []presentation.ReportCategory{warningCategory}},
			aggregateParts: []checkCollection{{categories: []presentation.ReportCategory{warningCategory}}},
			want:           "status: warnings\n\nsummary:\n  findings: 0 errors, 1 warnings, 0 information\n\nfindings:\n  warnings:\n    check | style heuristic\n",
		},
		{
			name:           "Information-only succeeds",
			direct:         checkCollection{categories: []presentation.ReportCategory{informationCategory}},
			aggregateParts: []checkCollection{{categories: []presentation.ReportCategory{informationCategory}}},
			want:           "status: completed\n\nsummary:\n  findings: 0 errors, 0 warnings, 1 information\n\nfindings:\n  information:\n    check | optional cleanup\n",
		},
		{
			name: "mixed normalizes category order and fails",
			direct: checkCollection{categories: []presentation.ReportCategory{
				informationCategory, errorCategory, warningCategory,
			}, failures: []error{errorFailure}},
			aggregateParts: []checkCollection{
				{categories: []presentation.ReportCategory{informationCategory}},
				{categories: []presentation.ReportCategory{errorCategory}, failures: []error{errorFailure}},
				{categories: []presentation.ReportCategory{warningCategory}},
			},
			want:      "status: failed\n\nsummary:\n  findings: 1 errors, 1 warnings, 1 information\n\nfindings:\n  errors:\n    check | invalid authority\n  warnings:\n    check | style heuristic\n  information:\n    check | optional cleanup\n",
			wantError: true,
		},
	}
	for _, tc := range cases {
		for _, path := range []struct {
			name       string
			collection func() checkCollection
		}{
			{name: "direct", collection: func() checkCollection { return tc.direct }},
			{name: "aggregate", collection: func() checkCollection {
				var aggregate checkCollection
				for _, part := range tc.aggregateParts {
					aggregate = aggregate.append(part)
				}
				return aggregate
			}},
		} {
			t.Run(tc.name+"/"+path.name, func(t *testing.T) {
				got, err := render(path.collection())
				if (err != nil) != tc.wantError {
					t.Fatalf("render error = %v, wantError %t", err, tc.wantError)
				}
				if got != tc.want {
					t.Fatalf("report = %q, want %q", got, tc.want)
				}
			})
		}
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
