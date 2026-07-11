package main

import (
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/project"
)

func runSync(root string, stdout io.Writer) error {
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	backups, changes, pruned, err := p.SyncReport()
	if err != nil {
		return err
	}
	for _, b := range backups {
		fmt.Fprintf(stdout, "backed up %s → %s\n", b.Path, b.Bak)
		if b.Index {
			fmt.Fprintf(stdout, "  note: awf now generates %s; retire any external generator for it\n", b.Path)
		}
	}
	for _, c := range changes {
		if c.Cause == "added" {
			fmt.Fprintf(stdout, "awf sync: added %s\n", c.Path)
			continue
		}
		fmt.Fprintf(stdout, "awf sync: changed %s (%s)\n", c.Path, c.Cause)
	}
	for _, path := range pruned {
		fmt.Fprintf(stdout, "awf sync: pruned %s\n", path)
	}
	fmt.Fprintln(stdout, "awf sync: done")
	return nil
}
