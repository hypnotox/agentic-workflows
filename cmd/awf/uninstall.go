package main

import (
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/project"
)

// runUninstall removes awf's generated footprint from a project (delegated to
// project.Uninstall: lock-tracked files, the dirs they leave empty, and the
// lock). It deliberately leaves the authored .awf/ config (config.yaml,
// sidecars, convention parts) in place.
func runUninstall(root string, stdout io.Writer) error {
	removed, err := project.Uninstall(root)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "awf uninstall: removed %d generated file(s) and the lock\n", removed)
	fmt.Fprintln(stdout, "awf uninstall: left the .awf/ config in place (delete it to fully remove)")
	return nil
}
