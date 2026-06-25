package main

import (
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/project"
)

func runCheck(root string) error {
	if err := gate(root); err != nil {
		return err
	}
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	drift, err := p.Check()
	if err != nil {
		return err
	}
	findings, err := p.CheckInvariants()
	if err != nil {
		return err
	}
	for _, d := range drift {
		fmt.Printf("  %-14s %s — %s\n", d.Kind, d.Path, d.Detail)
	}
	for _, f := range findings {
		fmt.Printf("  %-14s %s — invariant %q %s\n", "invariant", f.ADR, f.Slug, f.Detail())
	}
	if len(drift) == 0 && len(findings) == 0 {
		fmt.Println("awf check: clean")
		return nil
	}
	return fmt.Errorf("awf check: %d drift(s), %d invariant issue(s)", len(drift), len(findings))
}
