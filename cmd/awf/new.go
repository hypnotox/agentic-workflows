package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/project"
)

// runNew scaffolds a new templated artifact. Today only kind == "adr" is
// valid; titleWords is every positional argument after kind, joined with a
// single space to form the one title NewADR takes.
// invariant: adr-new-version-gated
func runNew(root, kind string, titleWords []string, stdout io.Writer) error {
	if kind != "adr" {
		return &usageErr{fmt.Sprintf("unknown kind %q (want: adr)", kind)}
	}
	if err := gate(root); err != nil {
		return err
	}
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	path, err := p.NewADR(strings.Join(titleWords, " "))
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, path)
	return nil
}
