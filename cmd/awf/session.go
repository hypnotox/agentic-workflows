package main

import (
	"context"
	"errors"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

// loadProjectSession selects one configuration tree and one repository handle,
// then constructs the invocation's authoritative project Session.
func loadProjectSession(ctx context.Context, root string) (*project.Session, error) {
	repo, _, repoErr := awfgit.OpenContaining(root)
	if repoErr != nil && !errors.Is(repoErr, awfgit.ErrNotARepository) {
		return nil, repoErr
	}
	if repo == nil {
		return project.NewLoaderWithoutRepository(config.Load, catalog.Standard, awfgit.ProjectResidentRoot).Load(ctx, root)
	}
	return project.NewLoader(config.Load, catalog.Standard, awfgit.ProjectResidentRoot, repo).Load(ctx, root)
}
