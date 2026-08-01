package project

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
)

// gatedCommandsDisplay is the backticked, comma-joined clispec gated set - the
// single source both doc surfaces consume, so it cannot drift from the code.
// The expectation derives from clispec too: a literal would pass while the code
// read from the wrong projection.
// invariant: tooling/cli:gated-commands-generated (TestGatedCommandsDisplay)
func TestGatedCommandsDisplay(t *testing.T) {
	quote := func(names []string) []string {
		out := make([]string, len(names))
		for i, n := range names {
			out[i] = "`" + n + "`"
		}
		return out
	}
	gated := quote(clispec.GatedCommandNames())
	want := strings.Join(gated, ", ")
	if got := gatedCommandsDisplay(); got != want {
		t.Errorf("gatedCommandsDisplay() = %q, want %q", got, want)
	}
}
