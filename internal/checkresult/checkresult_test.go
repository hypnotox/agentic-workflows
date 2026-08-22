package checkresult

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/severity"
)

func TestNewPreservesRankedAndUnrankedClassification(t *testing.T) {
	result, err := New(
		[]Finding{
			{Rank: severity.Error, Property: "reproducibility", Evidence: Evidence{Kind: "missing", Path: "AGENTS.md", Detail: "is not rendered"}},
			{Rank: severity.Warn, Property: "prose style", Evidence: Evidence{Kind: "punctuation", Path: "docs/guide.md", Detail: "uses prohibited punctuation"}},
		},
		[]Information{{Evidence: Evidence{Kind: "unset-variable", Detail: "title is unset"}}},
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	findings := result.Findings()
	if len(findings) != 2 {
		t.Fatalf("Findings() length = %d, want 2", len(findings))
	}
	if findings[0].Rank != severity.Error || findings[1].Rank != severity.Warn {
		t.Fatalf("Findings() ranks = %v, %v, want Error, Warn", findings[0].Rank, findings[1].Rank)
	}
	information := result.Information()
	if len(information) != 1 {
		t.Fatalf("Information() length = %d, want 1", len(information))
	}
	if information[0].Evidence.Detail != "title is unset" {
		t.Fatalf("Information() = %#v", information)
	}
}

func TestNewDefensivelySnapshotsInputsAndProjections(t *testing.T) {
	findings := []Finding{{Rank: severity.Error, Property: "correctness", Evidence: Evidence{Kind: "reference", Path: "docs/plan.md", Detail: "is broken"}}}
	information := []Information{{Evidence: Evidence{Kind: "note", Detail: "consider adding a tag"}}}
	result, err := New(findings, information)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	findings[0].Evidence.Detail = "input mutation"
	information[0].Evidence.Detail = "input mutation"
	projectedFindings := result.Findings()
	projectedInformation := result.Information()
	projectedFindings[0].Evidence.Detail = "projection mutation"
	projectedInformation[0].Evidence.Detail = "projection mutation"

	if got := result.Findings()[0].Evidence.Detail; got != "is broken" {
		t.Fatalf("Findings() retained %q after mutation, want original value", got)
	}
	if got := result.Information()[0].Evidence.Detail; got != "consider adding a tag" {
		t.Fatalf("Information() retained %q after mutation, want original value", got)
	}
}

func TestNewRefusesInvalidRankedFinding(t *testing.T) {
	tests := []struct {
		name    string
		finding Finding
		want    string
	}{
		{"unknown rank", Finding{Rank: severity.Rank(2), Property: "correctness", Evidence: Evidence{Kind: "reference", Detail: "is broken"}}, "not Error or Warn"},
		{"empty property", Finding{Rank: severity.Error, Evidence: Evidence{Kind: "reference", Detail: "is broken"}}, "protected property is empty"},
		{"empty kind", Finding{Rank: severity.Warn, Property: "prose style", Evidence: Evidence{Detail: "is awkward"}}, "kind is empty"},
		{"empty detail", Finding{Rank: severity.Warn, Property: "prose style", Evidence: Evidence{Kind: "punctuation"}}, "detail is empty"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New([]Finding{test.finding}, nil)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("New() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestZeroValuesAreDeliberate(t *testing.T) {
	var result Result
	if result.Findings() != nil {
		t.Fatalf("zero Result Findings() = %#v, want nil", result.Findings())
	}
	if result.Information() != nil {
		t.Fatalf("zero Result Information() = %#v, want nil", result.Information())
	}

	result, err := New(nil, nil)
	if err != nil {
		t.Fatalf("New(nil, nil) error = %v", err)
	}
	if result.Findings() != nil || result.Information() != nil {
		t.Fatalf("New(nil, nil) = %#v, want zero projections", result)
	}

	if _, err := New([]Finding{{}}, nil); err == nil {
		t.Fatal("New() accepted zero Finding")
	}
	if _, err := New(nil, []Information{{}}); err == nil {
		t.Fatal("New() accepted zero Information")
	}
}
