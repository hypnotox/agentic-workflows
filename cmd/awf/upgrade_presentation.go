package main

import (
	"context"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

type upgradeSyncDependencies struct {
	publisherSync func(context.Context, *project.ProjectState, *config.Config) (publisher.Result, error)
}

// productionUpgradeSyncDependencies is the command's concrete Publisher
// composition. Upgrade receives only its focused terminal-sync function.
func productionUpgradeSyncDependencies() upgradeSyncDependencies {
	return upgradeSyncDependencies{publisherSync: func(_ context.Context, state *project.ProjectState, cfg *config.Config) (publisher.Result, error) {
		return composePublisher(state, cfg).Sync()
	}}
}

func upgradeSyncMutation(ctx context.Context, root string) (presentation.Mutation, error) {
	return upgradeSyncMutationWith(ctx, root, productionUpgradeSyncDependencies())
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
	if mutationErr != nil { // coverage-ignore: Publisher.Result exposes only validated committed mutation facts
		return presentation.Mutation{}, mutationErr
	}
	return mutation, syncErr
}
