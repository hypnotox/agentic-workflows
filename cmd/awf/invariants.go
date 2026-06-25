package main

import (
	"fmt"

	"agentic-workflows/internal/project"
)

func runInvariants(root string) error {
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	findings, err := p.CheckInvariants()
	if err != nil {
		return err
	}
	if len(findings) == 0 {
		fmt.Println("awf invariants: clean")
		return nil
	}
	for _, f := range findings {
		fmt.Printf("  %s — invariant %q has no backing `// invariant: <slug>` test\n", f.ADR, f.Slug)
	}
	return fmt.Errorf("awf invariants: %d unbacked invariant(s)", len(findings))
}
