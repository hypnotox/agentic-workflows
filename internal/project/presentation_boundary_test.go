package project

import (
	"bytes"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func TestPitfallScaffoldPresentationBoundary(t *testing.T) {
	document, err := PitfallScaffoldDocument(".awf/docs/pitfalls/example.md")
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := presentation.Render(&output, document); err != nil {
		t.Fatal(err)
	}
	const want = "status: pitfall created\nauthored path: .awf/docs/pitfalls/example.md\n"
	if output.String() != want {
		t.Fatalf("presentation = %q, want %q", output.String(), want)
	}
	if _, err := PitfallScaffoldDocument("bad\npath"); err == nil {
		t.Fatal("invalid presentation path accepted")
	}
}
