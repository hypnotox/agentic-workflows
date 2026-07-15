package project

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
)

// gatedCommandsDisplay is the backticked, comma-joined clispec gated set - the
// single source both doc surfaces consume, so it cannot drift from the code.
// invariant: gated-commands-generated
func TestGatedCommandsDisplay(t *testing.T) {
	names := clispec.GatedCommandNames()
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = "`" + n + "`"
	}
	want := strings.Join(quoted, ", ")
	if got := gatedCommandsDisplay(); got != want {
		t.Errorf("gatedCommandsDisplay() = %q, want %q", got, want)
	}
}
