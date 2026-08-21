package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

// openProjectOperation selects one configuration tree and one repository
// handle, then constructs immutable state from those exact inputs.
func openProjectOperation(ctx context.Context, root string) (*project.ProjectState, *config.Config, *awfgit.Repo, error) {
	repo, _, repoErr := awfgit.OpenContaining(root)
	if repoErr != nil && !errors.Is(repoErr, awfgit.ErrNotARepository) {
		return nil, nil, nil, repoErr
	}
	cfg, err := config.Load(config.RootDir(root))
	if err != nil {
		return nil, nil, nil, err
	}
	load := func(dir string) (*config.Config, error) {
		if dir != config.RootDir(root) { // coverage-ignore: Loader.OpenForOperation requests exactly the selected root's config directory
			return nil, fmt.Errorf("unexpected config root %q", dir)
		}
		return cfg, nil
	}
	if repo == nil {
		state, selected, openErr := project.NewLoaderWithoutRepository(load, catalog.Standard, awfgit.ProjectResidentRoot).OpenForOperation(ctx, root)
		return state, selected, nil, openErr
	}
	state, selected, err := project.NewLoader(load, catalog.Standard, awfgit.ProjectResidentRoot, repo).OpenForOperation(ctx, root)
	return state, selected, repo, err
}
