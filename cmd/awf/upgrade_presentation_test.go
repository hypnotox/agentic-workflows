package main

import (
	"context"
	"errors"
	"testing"

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
	if syncErr != nil {
		mutation, mutationErr := result.PartialMutation()
		if mutationErr != nil {
			return presentation.Mutation{}, mutationErr
		}
		return mutation, syncErr
	}
	return result.Mutation()
}

func TestUpgradeSyncMutationWithPreservesPartialPublisherOutcome(t *testing.T) {
	root := scaffoldProject(t)
	cause := errors.New("late publication failure")
	mutation, err := upgradeSyncMutationWith(testContext(t), root, upgradeSyncDependencies{
		publisherSync: func(ctx context.Context, state *project.ProjectState, cfg *config.Config) (publisher.Result, error) {
			result, syncErr := composePublisher(state, cfg).SyncLeased(context.Background(), nil)
			if syncErr != nil {
				t.Fatal(syncErr)
			}
			return result, cause
		},
	})
	if !errors.Is(err, cause) {
		t.Fatalf("error = %v, want late publication cause", err)
	}
	if mutation.Status != "partially committed" || len(mutation.Changes) == 0 {
		t.Fatalf("mutation = %#v, want partial publisher effects", mutation)
	}
}
