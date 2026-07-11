package project

import (
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
)

// gatedCommandsDisplay renders the gated-command list as a backticked, comma-joined
// list from the single clispec source (ADR-0094 Decision 6). It is a tool constant
// (identical for every adopter — the same awf binary), so it takes no config input.
// invariant: gated-commands-generated
func gatedCommandsDisplay() string {
	names := clispec.GatedCommandNames()
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = "`" + n + "`"
	}
	return strings.Join(quoted, ", ")
}
