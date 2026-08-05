package project

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func TestEmptyCheckCategories(t *testing.T) {
	if categories, err := DriftCategories(nil, false); err != nil || len(categories) != 0 {
		t.Fatalf("empty drift = %#v, %v", categories, err)
	}
	if categories, err := CurrentStateCategories(CurrentStateReport{}, false); err != nil || len(categories) != 0 {
		t.Fatalf("empty state = %#v, %v", categories, err)
	}
}

func TestCurrentStateCategoriesRejectsEmptyFinding(t *testing.T) {
	if _, err := CurrentStateCategories(CurrentStateReport{Static: []currentstate.Finding{{}}}, false); err == nil {
		t.Fatal("empty current-state finding accepted")
	}
}

func TestDriftCategoriesOwnExactCheckVocabularyAndOrder(t *testing.T) {
	categories, err := DriftCategories([]manifest.Drift{{Kind: "stale", Path: "z", Detail: "later"}, {Kind: "hand-edited", Path: "a", Detail: "first"}}, true)
	if err != nil {
		t.Fatal(err)
	}
	document, err := (presentation.Report{Status: "failed", Categories: categories}).Document()
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := presentation.Render(&out, document); err != nil {
		t.Fatal(err)
	}
	want := "status: failed\n\nfindings:\n  errors:\n    staged drift | stale: z: later\n    staged drift | hand-edited: a: first\n"
	if out.String() != want {
		t.Fatalf("categories = %q, want %q", out.String(), want)
	}
}
