package prosegate

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func TestDisabledCategoryAndEmptyResults(t *testing.T) {
	empty, err := Categories(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, categories := range [][]presentation.ReportCategory{mustDisabledCategories(t), empty} {
		document, err := (presentation.Report{Status: "warnings", Categories: categories}).Document()
		if err != nil {
			t.Fatal(err)
		}
		var out strings.Builder
		if err := presentation.Render(&out, document); err != nil {
			t.Fatal(err)
		}
	}
}

func mustDisabledCategories(t *testing.T) []presentation.ReportCategory {
	t.Helper()
	categories, err := DisabledCategory()
	if err != nil {
		t.Fatal(err)
	}
	return categories
}

func TestCategoriesOwnExactCheckVocabularyAndOrder(t *testing.T) {
	pin := 1
	categories, err := Categories([]Finding{{Path: "z.md", Rune: '\u2014', Count: 2, Pinned: &pin}}, []string{"binary.bin"})
	if err != nil {
		t.Fatal(err)
	}
	report := presentation.Report{Status: "failed", Categories: categories}
	document, err := report.Document()
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := presentation.Render(&out, document); err != nil {
		t.Fatal(err)
	}
	want := "status: failed\n\nfindings:\n  errors:\n    prose | z.md: em-dash (U+2014) appears 2 time(s); the exemption pins 1\n  warnings:\n    prose | skipped binary: binary.bin\n"
	if out.String() != want {
		t.Fatalf("categories = %q, want %q", out.String(), want)
	}
}
