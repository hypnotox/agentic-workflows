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

func runSyncPrinting(ctx context.Context, loader *project.Loader, root string, stdout io.Writer) (returnErr error) {
	residentRoot := awfgit.ProjectResidentRoot(ctx, root)
	lease, err := filesystem.AcquireProjectLease(ctx, root, residentRoot)
	if err != nil {
		return err
	}
	defer func() { returnErr = errors.Join(returnErr, lease.Release()) }()
	state, cfg, err := loader.OpenForOperation(ctx, root)
	if err != nil {
		return err
	}
	composed := composePublisher(state, cfg)
	result, err := composed.SyncLeased(ctx, lease)
	if err != nil {
		if presentErr := renderPartialSync(stdout, err); presentErr != nil {
			return errors.Join(err, presentErr)
		}
		return err
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
