package main

import (
	"context"
	"errors"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

// newProjectLoader composes the project-opening policy for one invocation: the
// standard catalog, the seam's one resident-root resolution, and the Git handle
// the opened project reads through. A fresh non-repository tree takes the
// explicit no-repository constructor; malformed repositories are returned.
func newProjectLoader(root string) (*project.Loader, error) {
	repo, _, err := awfgit.OpenContaining(root)
	if err != nil {
		if !errors.Is(err, awfgit.ErrNotARepository) {
			return nil, err
		}
		return project.NewLoaderWithoutRepository(config.Load, catalog.Standard, awfgit.ProjectResidentRoot), nil
	}
	return project.NewLoader(config.Load, catalog.Standard, awfgit.ProjectResidentRoot, repo), nil
}

func runSync(ctx context.Context, root string, stdout io.Writer) error {
	loader, err := newProjectLoader(root)
	if err != nil {
		return err
	}
	return runSyncPrinting(ctx, loader, root, stdout)
}

func runSyncPrinting(ctx context.Context, loader *project.Loader, root string, stdout io.Writer) error {
	residentRoot := awfgit.ProjectResidentRoot(ctx, root)
	lease, err := filesystem.AcquireProjectLease(ctx, root, residentRoot)
	if err != nil {
		return err
	}
	state, cfg, err := loader.OpenForOperation(ctx, root)
	if err != nil {
		return errors.Join(err, lease.Release())
	}
	composed := composePublisher(state, cfg)
	result, syncErr := composed.SyncLeased(ctx, lease)
	return finishSyncPrinting(stdout, result, syncErr, lease.Release())
}

func finishSyncPrinting(stdout io.Writer, result publisher.Result, syncErr, releaseErr error) error {
	combined := errors.Join(syncErr, releaseErr)
	if syncErr == nil && releaseErr != nil {
		combined = &publisher.PartialError{Result: result, Cause: releaseErr}
	}
	if combined != nil {
		if presentErr := renderPartialSync(stdout, combined); presentErr != nil {
			return errors.Join(combined, presentErr)
		}
		return combined
	}
	mutation, err := result.Mutation()
	if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
		return err
	}
	return renderSyncMutation(stdout, mutation)
}

func renderPartialSync(stdout io.Writer, syncErr error) error {
	var partial *publisher.PartialError
	if !errors.As(syncErr, &partial) {
		return nil
	}
	mutation, err := partial.Result.PartialMutation()
	if err != nil {
		return err
	}
	return renderSyncMutation(stdout, mutation)
}

func renderSyncMutation(stdout io.Writer, mutation presentation.Mutation) error {
	document, err := mutation.Document()
	if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
		return err
	}
	return presentation.Render(stdout, document)
}
