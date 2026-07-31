package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
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
	for _, b := range backups {
		fmt.Fprintf(stdout, "backed up %s → %s\n", b.Path, b.Bak)
		if b.Index {
			fmt.Fprintf(stdout, "  note: awf now generates %s; retire any external generator for it\n", b.Path)
		}
	}
	for _, c := range changes {
		if c.Cause == "added" {
			fmt.Fprintf(stdout, "awf render: added %s\n", c.Path)
			continue
		}
		fmt.Fprintf(stdout, "awf render: changed %s (%s)\n", c.Path, c.Cause)
	}
	for _, path := range pruned {
		fmt.Fprintf(stdout, "awf render: pruned %s\n", path)
	}
	fmt.Fprintln(stdout, "awf render: done")
	return nil
}
