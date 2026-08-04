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

type typedDiagnosticError struct {
	diagnostic presentation.Diagnostic
	err        error
}

func (e typedDiagnosticError) Error() string { return "typed diagnostic" }

func (e typedDiagnosticError) Diagnostic() (presentation.Diagnostic, error) {
	return e.diagnostic, e.err
}

func TestDiagnosticOutcomeUsesTypedDiagnostic(t *testing.T) {
	value, err := presentation.Prose("retry")
	if err != nil {
		t.Fatal(err)
	}
	outcome := diagnosticOutcome(typedDiagnosticError{diagnostic: presentation.Diagnostic{
		Condition: "operation refused", State: "add", Steps: []presentation.Value{value},
	}})
	if outcome.stream != commandStderr || outcome.exit != 1 || outcome.err == nil {
		t.Fatalf("typed outcome = %#v", outcome)
	}
	var stdout, stderr bytes.Buffer
	if code := writeOutcome(&stdout, &stderr, outcome); code != 1 || stdout.Len() != 0 {
		t.Fatalf("typed diagnostic streams: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	const want = "condition: operation refused\nstate: add\n\ndiagnostic:\n  steps:\n    step 1: retry\n"
	if stderr.String() != want {
		t.Fatalf("typed diagnostic stderr = %q, want %q", stderr.String(), want)
	}

	mappingErr := errors.New("mapping failed")
	if got := diagnosticOutcome(typedDiagnosticError{err: mappingErr}); !errors.Is(got.err, mappingErr) {
		t.Fatalf("mapping failure outcome = %#v", got)
	}
	invalid := diagnosticOutcome(typedDiagnosticError{diagnostic: presentation.Diagnostic{Condition: " "}})
	if invalid.err == nil {
		t.Fatalf("document failure outcome = %#v", invalid)
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
