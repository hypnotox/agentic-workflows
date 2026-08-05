package memorycite

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func TestCommitGateDocumentOwnsExactFindingPresentation(t *testing.T) {
	document, err := CommitGateDocument([]Reference{{Path: "commit message", Line: 3, Segment: ".awf/efforts/example/memory.md"}})
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := presentation.Render(&out, document); err != nil {
		t.Fatal(err)
	}
	const want = "check staged commit:\n  errors:\n    commit message line 3 names the effort-owned memory file \".awf/efforts/example/memory.md\"\n"
	if out.String() != want {
		t.Fatalf("document = %q, want %q", out.String(), want)
	}
}

func TestDisabledCategoryAndEmptyResults(t *testing.T) {
	empty, err := Categories(nil)
	if err != nil {
		t.Fatal(err)
	}
	disabled, err := DisabledCategory()
	if err != nil {
		t.Fatal(err)
	}
	for _, categories := range [][]presentation.ReportCategory{empty, disabled} {
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

func TestCategoriesOwnExactCheckVocabularyAndOrder(t *testing.T) {
	categories, err := Categories([]Finding{{Path: "a.md", Lines: []int{3, 7}}, {Path: "b.md", Lines: []int{1}}})
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
	want := "status: failed\n\nfindings:\n  errors:\n    memory | a.md: 2 effort-owned memory citation(s) on line(s) 3, 7; name the .awf/efforts/ directory, use an angle-bracket slug placeholder, or remove the ephemeral file citation\n    memory | b.md: 1 effort-owned memory citation(s) on line(s) 1; name the .awf/efforts/ directory, use an angle-bracket slug placeholder, or remove the ephemeral file citation\n"
	if out.String() != want {
		t.Fatalf("categories = %q, want %q", out.String(), want)
	}
}
