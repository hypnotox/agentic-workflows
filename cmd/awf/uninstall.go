package main

import (
	"context"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/resident"
)

// runUninstall removes awf's generated footprint from a project (delegated to
// resident.Uninstall: lock-tracked files, the dirs they leave empty, and the
// lock). It deliberately leaves the authored .awf/ config (config.yaml,
// sidecars, convention parts) in place.
func runUninstall(ctx context.Context, root string, stdout io.Writer) error {
	report, err := resident.Uninstall(ctx, root)
	if err != nil {
		return err
	}
	document, err := report.Document()
	if err != nil { // coverage-ignore: Uninstall returns a count and validated resident-root names; the owner mapping uses fixed grammar
		return err
	}
	return presentation.Render(stdout, document)
}
