package checkop

import (
	"errors"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/repositorycheck"
)

const completedCheckReport = "status: completed\n\nsummary:\n  findings: 0 errors, 0 warnings, 0 information\n"

func requireCheckReport(t *testing.T, got string) {
	t.Helper()
	if got != completedCheckReport {
		t.Fatalf("report = %q, want %q", got, completedCheckReport)
	}
}

func TestCheckReportRejectsInvalidPresentationValues(t *testing.T) {
	if _, err := checkReport(nil, []string{"\n"}, repositorycheck.Presentation{}); err == nil {
		t.Fatal("invalid information value accepted")
	}
	if _, err := checkReport([]string{"\n"}, nil, repositorycheck.Presentation{}); err == nil {
		t.Fatal("invalid warning value accepted")
	}
}

func checkPresentationRecord(t *testing.T, detail string) presentation.Record {
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
	return record
}

// invariant: tooling/cli:check-severity-by-protected-property (TestCheckSeverityByProtectedProperty)
func TestCheckSeverityByProtectedProperty(t *testing.T) {
	errorRecord := checkPresentationRecord(t, "invalid authority")
	warningRecord := checkPresentationRecord(t, "style heuristic")
	informationRecord := checkPresentationRecord(t, "optional cleanup")
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
			direct:         checkCollection{presentation: repositorycheck.Presentation{Errors: []presentation.Record{errorRecord}}, failures: []error{errorFailure}},
			aggregateParts: []checkCollection{{presentation: repositorycheck.Presentation{Errors: []presentation.Record{errorRecord}}}, {failures: []error{errorFailure}}},
			want:           "status: failed\n\nsummary:\n  findings: 1 errors, 0 warnings, 0 information\n\nfindings:\n  errors:\n    check | invalid authority\n",
			wantError:      true,
		},
		{
			name:           "Warning-only succeeds",
			direct:         checkCollection{presentation: repositorycheck.Presentation{Warnings: []presentation.Record{warningRecord}}},
			aggregateParts: []checkCollection{{presentation: repositorycheck.Presentation{Warnings: []presentation.Record{warningRecord}}}},
			want:           "status: warnings\n\nsummary:\n  findings: 0 errors, 1 warnings, 0 information\n\nfindings:\n  warnings:\n    check | style heuristic\n",
		},
		{
			name:           "Information-only succeeds",
			direct:         checkCollection{presentation: repositorycheck.Presentation{Information: []presentation.Record{informationRecord}}},
			aggregateParts: []checkCollection{{presentation: repositorycheck.Presentation{Information: []presentation.Record{informationRecord}}}},
			want:           "status: completed\n\nsummary:\n  findings: 0 errors, 0 warnings, 1 information\n\nfindings:\n  information:\n    check | optional cleanup\n",
		},
		{
			name:   "mixed normalizes category order and fails",
			direct: checkCollection{presentation: repositorycheck.Presentation{Errors: []presentation.Record{errorRecord}, Warnings: []presentation.Record{warningRecord}, Information: []presentation.Record{informationRecord}}, failures: []error{errorFailure}},
			aggregateParts: []checkCollection{
				{presentation: repositorycheck.Presentation{Information: []presentation.Record{informationRecord}}},
				{presentation: repositorycheck.Presentation{Errors: []presentation.Record{errorRecord}}, failures: []error{errorFailure}},
				{presentation: repositorycheck.Presentation{Warnings: []presentation.Record{warningRecord}}},
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
				var out strings.Builder
				err := renderCheckCollection(&out, path.collection())
				if (err != nil) != tc.wantError {
					t.Fatalf("render error = %v, wantError %t", err, tc.wantError)
				}
				if out.String() != tc.want {
					t.Fatalf("report = %q, want %q", out.String(), tc.want)
				}
			})
		}
	}
}

func TestCheckReportIncludesExplicitInformationAndWarnings(t *testing.T) {
	record := checkPresentationRecord(t, "finding")
	report, err := checkReport([]string{"compatibility warning"}, []string{"compatibility information"}, repositorycheck.Presentation{Warnings: []presentation.Record{record}, Information: []presentation.Record{record}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "warnings" {
		t.Errorf("status = %q, want warnings", report.Status)
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
	if err := renderCheckCollection(&strings.Builder{}, checkCollection{presentation: repositorycheck.Presentation{Errors: []presentation.Record{{}}}}); err == nil {
		t.Fatal("invalid report record accepted")
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
	first := checkCollection{information: []string{"same"}, presentation: repositorycheck.Presentation{Warnings: []presentation.Record{checkPresentationRecord(t, "same")}}}
	second := checkCollection{information: []string{"same", "next"}, presentation: repositorycheck.Presentation{Warnings: []presentation.Record{checkPresentationRecord(t, "same")}, Errors: []presentation.Record{checkPresentationRecord(t, "next")}}}
	got := first.append(second)
	if len(got.information) != 3 || len(got.presentation.Warnings) != 2 || len(got.presentation.Errors) != 1 {
		t.Fatalf("source-ordered collection = %#v", got)
	}
}
