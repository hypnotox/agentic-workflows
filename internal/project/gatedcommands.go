package project

import (
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
)

// gatedCommandsDisplay renders the gated-command list as a backticked,
// comma-joined list of gated top-level names (ADR-0094 Decision 6). The list
// comes from clispec, never from a literal. It is a tool constant (identical for
// every adopter - the same awf binary), so it takes no config input.
func gatedCommandsDisplay() string {
	return strings.Join(backtick(clispec.GatedCommandNames()), ", ")
}

// backtick wraps each name in backticks, preserving order.
func backtick(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = "`" + n + "`"
	}
	return out
}
