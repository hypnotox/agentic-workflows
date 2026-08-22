package publisher

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
	"github.com/hypnotox/agentic-workflows/internal/config"
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
	var names []string
	for _, cmd := range clispec.Commands {
		if cmd.Gating != clispec.Ungated {
			names = append(names, cmd.Name)
		}
	}
	want := strings.Join(quote(names), ", ")
	if got := gatedCommandsDisplay(); got != want {
		t.Errorf("gatedCommandsDisplay() = %q, want %q", got, want)
	}

	cfg := &config.Config{}
	p := testState()
	inputs := newRenderInputs(p, cfg, nil, "test")
	registry, err := placeholderRegistry(inputs)
	if err != nil {
		t.Fatal(err)
	}
	if got := registry["gatedCommands"]; got != want {
		t.Errorf("placeholder gatedCommands = %q, want %q", got, want)
	}
	if got := projectData(inputs, config.Sidecar{}, map[string]bool{})["gatedCommands"]; got != want {
		t.Errorf("agent-guide gatedCommands = %q, want %q", got, want)
	}
}
