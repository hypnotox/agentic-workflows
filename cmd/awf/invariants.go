package main

import (
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/project"
)

func runInvariants(root string, stdout io.Writer) error {
	if err := gate(root); err != nil {
		return err
	}
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	findings, err := p.CheckInvariants()
	if err != nil {
		return err
	}
	if len(findings) == 0 {
		fmt.Fprintln(stdout, "awf invariants: clean")
		return nil
	}
	for _, f := range findings {
		fmt.Fprintf(stdout, "  %s\n", f.Line())
	}
	return fmt.Errorf("awf invariants: %d invariant issue(s)", len(findings))
}
