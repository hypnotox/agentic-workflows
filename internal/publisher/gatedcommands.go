package publisher

import (
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
)

// gatedCommandsDisplay renders the gated-command list from the command spec.
func gatedCommandsDisplay() string {
	return strings.Join(backtick(clispec.GatedCommandNames()), ", ")
}

func backtick(names []string) []string {
	out := make([]string, len(names))
	for i, name := range names {
		out[i] = "`" + name + "`"
	}
	return out
}
