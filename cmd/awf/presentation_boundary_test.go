package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
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
		Condition: "operation refused", State: "operation", Steps: []presentation.Value{value},
	}})
	if outcome.stream != commandStderr || outcome.exit != 1 || outcome.err == nil {
		t.Fatalf("typed outcome = %#v", outcome)
	}
	var stdout, stderr bytes.Buffer
	if code := writeOutcome(&stdout, &stderr, outcome); code != 1 || stdout.Len() != 0 {
		t.Fatalf("typed diagnostic streams: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	const want = "condition: operation refused\nstate: operation\n\ndiagnostic:\n  steps:\n    step 1: retry\n"
	if stderr.String() != want {
		t.Fatalf("typed diagnostic stderr = %q, want %q", stderr.String(), want)
	}
}

func TestTypedDiagnosticFallbackRendersOriginalFailureOnce(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "mapping", err: typedDiagnosticError{err: errors.New("mapping failed")}},
		{name: "document", err: typedDiagnosticError{diagnostic: presentation.Diagnostic{Condition: " "}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := writeOutcome(&stdout, &stderr, diagnosticOutcome(test.err)); code != 1 {
				t.Fatalf("writeOutcome exit = %d", code)
			}
			if stdout.Len() != 0 {
				t.Fatalf("writeOutcome stdout = %q", stdout.String())
			}
			failure := "mapping failed"
			if test.name == "document" {
				failure = "presentation value is empty"
			}
			if strings.Count(stderr.String(), failure) != 1 {
				t.Fatalf("writeOutcome stderr = %q, want %q exactly once", stderr.String(), failure)
			}
			if strings.Contains(stderr.String(), "presentation document is empty") {
				t.Fatalf("writeOutcome replaced original failure: %q", stderr.String())
			}

			var dispatchStdout, dispatchStderr bytes.Buffer
			if code := dispatchFailure(&dispatchStdout, &dispatchStderr, test.err); code != 1 || dispatchStdout.Len() != 0 || dispatchStderr.String() != stderr.String() {
				t.Fatalf("dispatchFailure exit=%d stdout=%q stderr=%q", code, dispatchStdout.String(), dispatchStderr.String())
			}
		})
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
