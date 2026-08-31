package checkop

import (
	"errors"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

const completedCheckReport = "status: completed\n\nsummary:\n  findings: 0 errors, 0 warnings, 0 information\n"

func requireCheckReport(t *testing.T, got string) {
	t.Helper()
	if got != completedCheckReport {
		t.Fatalf("report = %q, want %q", got, completedCheckReport)
	}
}

func checkTestResult(t *testing.T, findings []checkresult.Finding, information []checkresult.Information) checkresult.Result {
	t.Helper()
	result, err := checkresult.New(findings, information)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func checkFinding(rank severity.Rank, property checkresult.Property, kind, path, detail string) checkresult.Finding {
	return checkresult.Finding{Rank: rank, Property: property, Evidence: checkresult.Evidence{Kind: kind, Path: path, Detail: detail}}
}

func collectionWithResult(check string, result checkresult.Result, failure error) checkCollection {
	collection := checkCollection{}
	collection.add(check, result, false)
	if failure != nil {
		collection.failures = append(collection.failures, failure)
	}
	return collection
}

// invariant: tooling/cli:check-severity-by-protected-property (TestCheckSeverityByProtectedProperty)
func TestCheckSeverityByProtectedProperty(t *testing.T) {
	errorResult := checkTestResult(t, []checkresult.Finding{checkFinding(severity.Error, "authority", "authority", "", "invalid authority")}, nil)
	warningResult := checkTestResult(t, []checkresult.Finding{checkFinding(severity.Warn, "heuristic-quality", "style", "", "style heuristic")}, nil)
	informationResult := checkTestResult(t, nil, []checkresult.Information{{Evidence: checkresult.Evidence{Kind: "optional", Detail: "optional cleanup"}}})
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
			direct:         collectionWithResult("check", errorResult, errorFailure),
			aggregateParts: []checkCollection{collectionWithResult("check", errorResult, nil), {failures: []error{errorFailure}}},
			want:           "status: failed\n\nsummary:\n  findings: 1 errors, 0 warnings, 0 information\n\nfindings:\n  errors:\n    check | invalid authority\n",
			wantError:      true,
		},
		{
			name:           "Warning-only succeeds",
			direct:         collectionWithResult("check", warningResult, nil),
			aggregateParts: []checkCollection{collectionWithResult("check", warningResult, nil)},
			want:           "status: warnings\n\nsummary:\n  findings: 0 errors, 1 warnings, 0 information\n\nfindings:\n  warnings:\n    check | style heuristic\n",
		},
		{
			name:           "Information-only succeeds",
			direct:         collectionWithResult("check", informationResult, nil),
			aggregateParts: []checkCollection{collectionWithResult("check", informationResult, nil)},
			want:           "status: completed\n\nsummary:\n  findings: 0 errors, 0 warnings, 1 information\n\nfindings:\n  information:\n    check | optional cleanup\n",
		},
		{
			name: "mixed normalizes category order and fails",
			direct: func() checkCollection {
				collection := collectionWithResult("check", checkTestResult(t, []checkresult.Finding{
					checkFinding(severity.Error, "authority", "authority", "", "invalid authority"),
					checkFinding(severity.Warn, "heuristic-quality", "style", "", "style heuristic"),
				}, []checkresult.Information{{Evidence: checkresult.Evidence{Kind: "optional", Detail: "optional cleanup"}}}), errorFailure)
				return collection
			}(),
			aggregateParts: []checkCollection{
				collectionWithResult("check", informationResult, nil),
				collectionWithResult("check", errorResult, errorFailure),
				collectionWithResult("check", warningResult, nil),
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

func TestCheckReportPreservesEvidenceFormattingAndResultOrder(t *testing.T) {
	result := checkTestResult(t, []checkresult.Finding{
		checkFinding(severity.Error, "correctness", "missing", "AGENTS.md", "file absent"),
		checkFinding(severity.Warn, "heuristic-quality", "style", "", "tighten prose"),
	}, []checkresult.Information{{Evidence: checkresult.Evidence{Kind: "tracking", Path: "", Detail: "tracking unavailable"}}})
	report, err := checkReport([]reportedResult{{check: "drift", result: result, evidencePrefix: true}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "failed" {
		t.Fatalf("status = %q, want failed", report.Status)
	}
	document, err := report.Document()
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := presentation.Render(&out, document); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"drift | missing: AGENTS.md: file absent", "drift | style: : tighten prose", "drift | tracking: : tracking unavailable"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("report missing %q: %q", want, out.String())
		}
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
	invalid := checkCollection{}
	invalid.add("", checkTestResult(t, nil, []checkresult.Information{{Evidence: checkresult.Evidence{Kind: "test", Detail: "detail"}}}), false)
	if err := renderCheckCollection(&strings.Builder{}, invalid); err == nil {
		t.Fatal("invalid report label accepted")
	}
}

func TestOutcomeRetainsPresentationFailureWithOperationalFailures(t *testing.T) {
	malformed := checkTestResult(t, nil, []checkresult.Information{{Evidence: checkresult.Evidence{Kind: "test", Detail: " "}}})
	operationFailure := errors.New("operation failed")
	collection := checkCollection{operational: []error{operationFailure}}
	collection.add("check", malformed, false)
	_, err := outcome(collection)
	if !errors.Is(err, operationFailure) || !strings.Contains(err.Error(), "presentation value is empty") {
		t.Fatalf("combined outcome error = %v", err)
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
	firstResult := checkTestResult(t, []checkresult.Finding{checkFinding(severity.Warn, "heuristic-quality", "warning", "", "same")}, nil)
	secondResult := checkTestResult(t, []checkresult.Finding{
		checkFinding(severity.Warn, "heuristic-quality", "warning", "", "same"),
		checkFinding(severity.Error, "correctness", "error", "", "next"),
	}, nil)
	first := collectionWithResult("first", firstResult, nil)
	second := collectionWithResult("second", secondResult, nil)
	got := first.append(second)
	if len(got.results) != 2 || got.results[0].check != "first" || got.results[1].check != "second" || len(got.results[0].result.Findings()) != 1 || len(got.results[1].result.Findings()) != 2 {
		t.Fatalf("source-ordered collection = %#v", got)
	}
}
