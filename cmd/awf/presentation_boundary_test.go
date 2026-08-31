package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// invariant: tooling/cli:typed-command-output-boundary (TestCommandOutputBoundary)
func TestCommandOutputBoundary(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"awf", "bogus"}, &stdout, &stderr); code != 2 {
		t.Fatalf("unknown command exit = %d", code)
	}
	if stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("usage output streams stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	value, err := presentation.Prose("one finding")
	if err != nil {
		t.Fatal(err)
	}
	field, err := presentation.NewField("status", value)
	if err != nil {
		t.Fatal(err)
	}
	document, err := presentation.NewDocument(field)
	if err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	outcome := commandOutcome{document: document, stream: commandStdout, exit: 1, err: errors.New("report failed")}
	if code := writeOutcome(&stdout, &stderr, outcome); code != 1 || stdout.String() != "status: one finding\n" || stderr.Len() != 0 {
		t.Fatalf("produced report streams exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
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
	outcome := diagnosticOutcome(typedDiagnosticError{diagnostic: presentation.Diagnostic{Condition: "operation refused", State: "operation", Steps: []presentation.Value{value}}})
	if outcome.stream != commandStderr || outcome.exit != 1 || outcome.err == nil {
		t.Fatalf("typed outcome = %#v", outcome)
	}
	var stdout, stderr bytes.Buffer
	if code := writeOutcome(&stdout, &stderr, outcome); code != 1 || stdout.Len() != 0 {
		t.Fatalf("typed diagnostic streams: code=%d stdout=%q", code, stdout.String())
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
	}{{"mapping", typedDiagnosticError{err: errors.New("mapping failed")}}, {"document", typedDiagnosticError{diagnostic: presentation.Diagnostic{Condition: " "}}}} {
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
		})
	}
}

func TestWriteStatusValidatesAndPropagatesWrites(t *testing.T) {
	if err := writeStatus(io.Discard, " \n\t"); err == nil {
		t.Fatal("empty normalized status accepted")
	}
	if err := writeStatus(errorWriter{}, "ready"); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("write error = %v", err)
	}
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestRendererFailureFallback(t *testing.T) {
	for _, test := range []struct {
		name  string
		cause error
		want  string
	}{{"ordinary", errors.New("render failed"), "awf: render failed\n"}, {"hostile whitespace", errors.New("\t render\n\u00a0failed \r\n"), "awf: render failed\n"}, {"empty", errors.New(" \t\n\u00a0"), "awf: renderer failed\n"}} {
		t.Run(test.name, func(t *testing.T) {
			var stderr bytes.Buffer
			writeRendererFailure(&stderr, test.cause)
			if got := stderr.String(); got != test.want {
				t.Fatalf("fallback = %q, want %q", got, test.want)
			}
		})
	}
}

func TestWriteOutcomeRendererFailureIsSingleDiagnostic(t *testing.T) {
	value, _ := presentation.Prose("ready")
	field, _ := presentation.NewField("condition", value)
	document, _ := presentation.NewDocument(field)
	var stderr bytes.Buffer
	if got := writeOutcome(errorWriter{}, &stderr, commandOutcome{document: document}); got != 1 {
		t.Fatalf("exit = %d", got)
	}
	if stderr.String() != "awf: write presentation: write failed\n" {
		t.Fatalf("stderr=%q", stderr.String())
	}
}
