package project

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
)

// gatedCommandsDisplay is the backticked, comma-joined clispec gated set plus an
// except clause naming the ungated group children - the single source both doc
// surfaces consume, so it cannot drift from the code. Both parts derive from
// clispec here too: a literal expectation would pass while the code read from
// the wrong projection.
// invariant: tooling/cli:gated-commands-generated
func TestGatedCommandsDisplay(t *testing.T) {
	quote := func(names []string) []string {
		out := make([]string, len(names))
		for i, n := range names {
			out[i] = "`" + n + "`"
		}
		return out
	}
	gated := quote(clispec.GatedCommandNames())
	exclusions := quote(clispec.UngatedGroupChildren())
	if len(exclusions) < 2 {
		t.Fatalf("expected at least two ungated group children to exercise the clause, got %v", exclusions)
	}
	want := strings.Join(gated, ", ") + ", except " +
		strings.Join(exclusions[:len(exclusions)-1], ", ") + ", and " + exclusions[len(exclusions)-1]
	if got := gatedCommandsDisplay(); got != want {
		t.Errorf("gatedCommandsDisplay() = %q, want %q", got, want)
	}
	// The gated list stays top-level-only: no exclusion appears among the gated names.
	for _, ex := range exclusions {
		if strings.Contains(strings.Join(gated, ", "), ex) {
			t.Errorf("gated list must not contain the exclusion %s", ex)
		}
	}
}
