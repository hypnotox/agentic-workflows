package audit

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

func TestConventionalCommitDocumentOwnsGatePresentation(t *testing.T) {
	document, err := ConventionalCommitDocument([]Finding{{Detail: "subject must use Conventional Commits"}})
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := presentation.Render(&out, document); err != nil {
		t.Fatal(err)
	}
	const want = "check staged commit:\n  errors:\n    subject must use Conventional Commits\n"
	if out.String() != want {
		t.Fatalf("document = %q, want %q", out.String(), want)
	}
	if _, err := ConventionalCommitDocument([]Finding{{Detail: " \n\t"}}); err == nil {
		t.Fatal("invalid finding detail accepted")
	}
}

func TestReportOwnsExactCategoriesAndStatus(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		findings   []Finding
		commits    int
	}{
		{"failed", "status: failed", []Finding{{Rule: "error-rule", Commit: "abc", Detail: "bad", Severity: severity.Error}, {Rule: "warn-rule", Detail: "note", Severity: severity.Warn}}, 2},
		{"warnings", "status: warnings", []Finding{{Rule: "warn-rule", Detail: "note", Severity: severity.Warn}}, 1},
		{"empty", "status: empty", nil, 0},
		{"clean", "status: clean", nil, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report, _, err := reportOutcome(tc.findings, tc.commits, "base", "head")
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
			if !strings.HasPrefix(out.String(), tc.want+"\n") {
				t.Fatalf("report = %q", out.String())
			}
			if tc.name == "failed" {
				want := "findings:\n  errors:\n    error-rule | abc | bad\n  warnings:\n    warn-rule | branch | note\n"
				if !strings.Contains(out.String(), want) {
					t.Fatalf("categories = %q, want %q", out.String(), want)
				}
			}
		})
	}
}

func TestReportRejectsInvalidFindingVocabulary(t *testing.T) {
	if _, _, err := reportOutcome([]Finding{{Rule: "\n", Detail: "bad", Severity: severity.Error}}, 1, "base", "head"); err == nil {
		t.Fatal("invalid finding accepted")
	}
	if _, _, err := reportOutcome(nil, 1, "base\n", "head"); err == nil {
		t.Fatal("invalid scope accepted")
	}
}
