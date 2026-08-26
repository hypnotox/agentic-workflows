package main

import (
	"context"
	"errors"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
	"github.com/hypnotox/agentic-workflows/internal/resident"
)

// runUninstall removes awf's generated footprint from a project (delegated to
// resident.Uninstall: lock-tracked files, the dirs they leave empty, and the
// lock). It deliberately leaves the authored .awf/ config (config.yaml,
// sidecars, convention parts) in place.
func runUninstall(ctx context.Context, root string, stdout io.Writer) (returnErr error) {
	lease, err := filesystem.AcquireProjectLease(ctx, root, awfgit.ProjectResidentRoot(ctx, root))
	if err != nil {
		return err
	}
	defer func() { returnErr = errors.Join(returnErr, lease.Release()) }()
	report, uninstallErr := resident.UninstallLeased(ctx, root, publisher.IsLocalDocTemplate, lease)
	if uninstallErr != nil {
		var partial *resident.PartialUninstallError
		if !errors.As(uninstallErr, &partial) {
			return uninstallErr
		}
		document, documentErr := partial.Document()
		if documentErr != nil {
			return errors.Join(uninstallErr, documentErr)
		}
		if renderErr := presentation.Render(stdout, document); renderErr != nil {
			return errors.Join(uninstallErr, renderErr)
		}
		return uninstallErr
	}
	document, err := report.Document()
	if err != nil { // coverage-ignore: Uninstall returns a count and validated resident-root names; the owner mapping uses fixed grammar
		return err
	}
	return presentation.Render(stdout, document)
}
