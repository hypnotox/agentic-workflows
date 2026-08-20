package prosegate

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func TestEmptyResults(t *testing.T) {
	categories, err := Categories(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (presentation.Report{Status: "completed", Categories: categories}).Document(); err != nil {
		t.Fatal(err)
	}
}

func TestCategoriesOwnExactCheckVocabularyAndOrder(t *testing.T) {
	pin := 1
	categories, err := Categories([]Finding{{Path: "z.md", Rune: '\u2014', Count: 2, Pinned: &pin}}, []string{"binary.bin"})
	if err != nil {
		t.Fatal(err)
	}
	report := presentation.Report{Status: "warnings", Categories: categories}
	document, err := report.Document()
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := presentation.Render(&out, document); err != nil {
		t.Fatal(err)
	}
	want := "status: warnings\n\nfindings:\n  warnings:\n    prose | z.md: em-dash (U+2014) appears 2 time(s); the exemption pins 1\n"
	if out.String() != want {
		t.Fatalf("categories = %q, want %q", out.String(), want)
	}
}
