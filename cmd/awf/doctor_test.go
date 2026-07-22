package main

import (
	"io"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
)

func TestDoctorGrammarAndWriteFailure(t *testing.T) {
	spec, ok := clispec.Lookup("doctor")
	if !ok {
		t.Fatal("doctor command missing")
	}
	const usage = "Usage: awf doctor [--effort ID] [--session ID] [--phase PHASE] [--since RFC3339] [--until RFC3339] [--json]"
	if !strings.HasPrefix(spec.HelpBody, usage+"\n") || spec.MinPos != 0 || spec.MaxPos != 0 {
		t.Fatalf("doctor grammar = %#v", spec)
	}
	root := telemetryProject(t)
	if err := runDoctor(&cmdCtx{root: root, inv: invocation{bools: map[string]bool{}, values: map[string]string{}}, stdout: failingMetricsWriter{}}); err == nil {
		t.Fatal("doctor write failure ignored")
	}
	if err := runDoctor(&cmdCtx{root: root, inv: invocation{bools: map[string]bool{}, values: map[string]string{"--phase": "unknown"}}, stdout: io.Discard}); err == nil {
		t.Fatal("doctor invalid selector accepted")
	}
}
