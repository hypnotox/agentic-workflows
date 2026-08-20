package project

import (
	"errors"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/clispec"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
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
// invariant: tooling/audit-commands:audit-full-profile-only (TestRequireCapabilityRefusesCoreFullOnlyCommands)
// invariant: tooling/context-and-topic:context-full-profile-only (TestRequireCapabilityRefusesCoreFullOnlyCommands)
func TestRequireCapabilityRefusesCoreFullOnlyCommands(t *testing.T) {
	var fullOnly []string
	var visit func(prefix string, commands []clispec.Command)
	visit = func(prefix string, commands []clispec.Command) {
		for _, command := range commands {
			name := strings.TrimSpace(prefix + " " + command.Name)
			if command.FullOnly {
				fullOnly = append(fullOnly, name)
			}
			visit(name, command.Children)
		}
	}
	visit("", clispec.Commands)
	if len(fullOnly) == 0 {
		t.Fatal("command specification declares no Full-only capability")
	}
	for _, name := range fullOnly {
		err := RequireCapability(catalog.ProfileCore, name, true)
		var refusal *CapabilityError
		if !errors.As(err, &refusal) {
			t.Errorf("%s: error = %v, want CapabilityError", name, err)
			continue
		}
		if refusal.Error() != "awf "+name+" is unavailable in the selected core governance footprint" {
			t.Errorf("%s: error = %q", name, refusal)
		}
		diagnostic, err := refusal.Diagnostic()
		if err != nil {
			t.Errorf("%s: diagnostic = %v", name, err)
		} else {
			if diagnostic.Cause != "the command requires the Full governance footprint" {
				t.Errorf("%s: diagnostic cause = %q", name, diagnostic.Cause)
			}
			document, err := diagnostic.Document()
			if err != nil {
				t.Errorf("%s: diagnostic document = %v", name, err)
			} else {
				var rendered strings.Builder
				if err := presentation.Render(&rendered, document); err != nil {
					t.Errorf("%s: render diagnostic = %v", name, err)
				} else if !strings.Contains(rendered.String(), "selected governance footprint: core") {
					t.Errorf("%s: diagnostic omitted exact footprint field:\n%s", name, rendered.String())
				}
			}
		}
		if err := RequireCapability(catalog.ProfileFull, name, true); err != nil {
			t.Errorf("%s: Full refusal = %v", name, err)
		}
	}
	if err := RequireCapability(catalog.ProfileCore, "render", false); err != nil {
		t.Fatalf("shared refusal = %v", err)
	}
}
