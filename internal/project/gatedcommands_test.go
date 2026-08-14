package project

import (
	"errors"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
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

	p := &Project{Cfg: &config.Config{}}
	registry, err := p.placeholderRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if got := registry["gatedCommands"]; got != want {
		t.Errorf("placeholder gatedCommands = %q, want %q", got, want)
	}
	if got := p.data(config.Sidecar{}, map[string]bool{})["gatedCommands"]; got != want {
		t.Errorf("agent-guide gatedCommands = %q, want %q", got, want)
	}
}

// invariant: tooling/cli:gated-commands-generated (TestRequireCapabilityRefusesCoreFullOnlyCommands)
func TestRequireCapabilityRefusesCoreFullOnlyCommands(t *testing.T) {
	err := RequireCapability(catalog.ProfileCore, "audit", true)
	var refusal *CapabilityError
	if !errors.As(err, &refusal) {
		t.Fatalf("error = %v, want CapabilityError", err)
	}
	if refusal.Error() != "awf audit is unavailable for the selected core profile" {
		t.Fatalf("error = %q", refusal)
	}
	diagnostic, err := refusal.Diagnostic()
	if err != nil || diagnostic.State != "configuration" || diagnostic.Condition != refusal.Error() {
		t.Fatalf("diagnostic = %#v, %v", diagnostic, err)
	}
	if err := RequireCapability(catalog.ProfileFull, "audit", true); err != nil {
		t.Fatalf("full refusal = %v", err)
	}
	if err := RequireCapability(catalog.ProfileCore, "render", false); err != nil {
		t.Fatalf("shared refusal = %v", err)
	}
}
