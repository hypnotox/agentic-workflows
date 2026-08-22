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
