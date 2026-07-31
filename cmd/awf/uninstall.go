package main

import (
	"context"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/project"
)

// runUninstall removes awf's generated footprint from a project (delegated to
// project.Uninstall: lock-tracked files, the dirs they leave empty, and the
// lock). It deliberately leaves the authored .awf/ config (config.yaml,
// sidecars, convention parts) in place.
func runUninstall(ctx context.Context, root string, stdout io.Writer) error {
	report, err := project.Uninstall(ctx, root)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "awf uninstall: removed %d generated file(s) and the lock\n", report.Removed)
	for _, root := range report.PreservedRoots { // coverage-ignore: Uninstall's generated-root report has no preserved roots in command scaffolds; resident preservation is tested at project layer
		fmt.Fprintf(stdout, "preserved resident data under .awf/%s\n", root)
	}
	fmt.Fprintln(stdout, "awf uninstall: left the .awf/ config in place (delete it to fully remove)")
	return nil
}
