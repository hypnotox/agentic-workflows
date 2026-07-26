package project

import (
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
)

// gatedCommandsDisplay renders the gated-command list as a backticked,
// comma-joined list of gated top-level names, followed by an except clause
// naming the group children that lower their parent's gate (ADR-0094 Decision 6,
// ADR-0159 Decision 4). Both parts come from clispec, never from a literal. It
// is a tool constant (identical for every adopter - the same awf binary), so it
// takes no config input.
func gatedCommandsDisplay() string {
	gated := backtick(clispec.GatedCommandNames())
	exclusions := backtick(clispec.UngatedGroupChildren())
	if len(exclusions) == 0 { // coverage-ignore: the table always carries the three ungated check children; the branch keeps the clause from rendering a dangling "except"
		return strings.Join(gated, ", ")
	}
	clause := exclusions[0]
	if len(exclusions) > 1 {
		clause = strings.Join(exclusions[:len(exclusions)-1], ", ") + ", and " + exclusions[len(exclusions)-1]
	}
	return strings.Join(gated, ", ") + ", except " + clause
}

// backtick wraps each name in backticks, preserving order.
func backtick(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = "`" + n + "`"
	}
	return out
}
