package main

import (
	"context"
	"errors"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
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
	return runSyncPrinting(ctx, loader, root, nil, stdout)
}

func runSyncInitialized(ctx context.Context, root string, seed project.InitAuthority, stdout io.Writer) error {
	loader, err := newProjectLoader(root)
	if err != nil {
		return err
	}
	return runSyncPrinting(ctx, loader, root, &seed, stdout)
}

func runSyncPrinting(ctx context.Context, loader *project.Loader, root string, seed *project.InitAuthority, stdout io.Writer) error {
	p, err := loader.Open(ctx, root)
	if err != nil {
		return err
	}
	var backups []project.Backup
	var changes []project.Change
	var pruned []string
	if seed == nil {
		backups, changes, pruned, err = p.SyncReport(ctx)
	} else {
		backups, changes, pruned, err = p.InitializeReport(ctx, *seed)
	}
	if err != nil {
		return err
	}
	mutation, err := project.SyncMutation(backups, changes, pruned)
	if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
		return err
	}
	document, err := mutation.Document()
	if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
		return err
	}
	return presentation.Render(stdout, document)
}
