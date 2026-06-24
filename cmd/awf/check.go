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
	if len(drift) == 0 {
		fmt.Println("awf check: clean")
		return nil
	}
	for _, d := range drift {
		fmt.Printf("  %-12s %s — %s\n", d.Kind, d.Path, d.Detail)
	}
	return fmt.Errorf("awf check: %d drift(s) found", len(drift))
}
