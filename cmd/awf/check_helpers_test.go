package main

import (
	"errors"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/checkop"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

const completedCheckReport = "status: completed\n\nsummary:\n  findings: 0 errors, 0 warnings, 0 information\n"

func TestRenderCheckOutcomePropagatesWriterFailureAndProducedStatus(t *testing.T) {
	document, err := (presentation.Report{Status: "completed"}).Document()
	if err != nil {
		t.Fatal(err)
	}
	writerFailure := errors.New("writer failed")
	if err := renderCheckOutcome(&failOnWrite{failAt: 1, err: writerFailure}, checkop.Outcome{Document: document}); !errors.Is(err, writerFailure) {
		t.Fatalf("writer error = %v, want %v", err, writerFailure)
	}
	producedFailure := errors.New("produced failure")
	if err := renderCheckOutcome(&discardWriter{}, checkop.Outcome{Document: document, Failure: producedFailure}); !errors.Is(err, producedFailure) {
		t.Fatalf("produced error = %v, want %v", err, producedFailure)
	}
}

type discardWriter struct{}

func (*discardWriter) Write(p []byte) (int, error) { return len(p), nil }
