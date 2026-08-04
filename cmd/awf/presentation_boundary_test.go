package main

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func TestCommandOutputBoundary(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"awf", "bogus"}, &stdout, &stderr); code != 2 {
		t.Fatalf("unknown command exit = %d", code)
	}
	if stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("usage output streams stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestWriteOutcomeRendererFailureIsSingleDiagnostic(t *testing.T) {
	value, _ := presentation.Prose("ready")
	field, _ := presentation.NewField("condition", value)
	document, _ := presentation.NewDocument(field)
	var stdout, stderr bytes.Buffer
	if got := writeOutcomeWithRenderer(&stdout, &stderr, commandOutcome{document: document}, func(_ io.Writer, _ presentation.Document) error { return errors.New("render failed") }); got != 1 {
		t.Fatalf("exit = %d", got)
	}
	if stdout.Len() != 0 || stderr.String() != "awf: render failed\n" {
		t.Fatalf("streams stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
