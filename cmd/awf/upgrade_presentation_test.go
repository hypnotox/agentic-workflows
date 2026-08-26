package main

import (
	"context"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

// upgradeSyncDependencies keeps the former command-composition fault seam in
// tests. Production upgrade composition now passes its covering lease directly
// to Publisher, so this test helper must not ship as an alternate publication
// route.
type upgradeSyncDependencies struct {
	publisherSync func(context.Context, *project.ProjectState, *config.Config) (publisher.Result, error)
}

func upgradeSyncMutationWith(ctx context.Context, root string, dependencies upgradeSyncDependencies) (presentation.Mutation, error) {
	loader, err := newProjectLoader(root)
	if err != nil {
		return presentation.Mutation{}, err
	}
	state, cfg, err := loader.OpenForOperation(ctx, root)
	if err != nil {
		return presentation.Mutation{}, err
	}
	result, syncErr := dependencies.publisherSync(ctx, state, cfg)
	mutation, mutationErr := result.Mutation()
	if mutationErr != nil {
		return presentation.Mutation{}, mutationErr
	}
	return mutation, syncErr
}
