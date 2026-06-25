package main

import (
	"fmt"

	"agentic-workflows/internal/project"
)

func runCheck(root string) error {
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
		fmt.Printf("  %-14s %s — invariant %q has no backing // invariant: test\n", "unbacked-inv", f.ADR, f.Slug)
	}
	if len(drift) == 0 && len(findings) == 0 {
		fmt.Println("awf check: clean")
		return nil
	}
	return fmt.Errorf("awf check: %d drift(s), %d unbacked invariant(s)", len(drift), len(findings))
}
