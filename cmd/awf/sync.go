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

func runSyncPrinting(ctx context.Context, loader *project.Loader, root string, seed *project.InitAuthority, stdout io.Writer) error {
	mutation, _, _, err := syncMutation(ctx, loader, root, seed)
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

func syncMutation(ctx context.Context, loader *project.Loader, root string, seed *project.InitAuthority) (presentation.Mutation, *project.ProjectState, *config.Config, error) {
	state, cfg, err := loader.OpenForOperation(ctx, root)
	if err != nil {
		return presentation.Mutation{}, nil, nil, err
	}
	plan, err := operationPlan(state, cfg)
	if err != nil { // coverage-ignore: OpenForOperation validated the same immutable tree; Publisher planning failures are covered at the owner boundary
		return presentation.Mutation{}, nil, nil, err
	}
	var backups []project.Backup
	var changes []project.Change
	var pruned []string
	if seed == nil {
		backups, changes, pruned, err = project.SyncReport(state, cfg, plan)
	} else {
		backups, changes, pruned, err = project.InitializeReport(state, cfg, *seed, plan)
	}
	if err != nil {
		return presentation.Mutation{}, nil, nil, err
	}
	mutation, err := project.SyncMutation(backups, changes, pruned)
	if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
		return presentation.Mutation{}, nil, nil, err
	}
	return mutation, state, cfg, nil
}
