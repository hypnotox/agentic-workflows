package main

import (
	"context"
	"errors"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

type upgradeSyncDependencies struct {
	publisherSync func(context.Context, *project.Session) (publisher.Result, error)
}

func upgradeSyncMutationWith(ctx context.Context, root string, dependencies upgradeSyncDependencies) (presentation.Mutation, error) {
	loader, err := newProjectLoader(root)
	if err != nil {
		return presentation.Mutation{}, err
	}
	state, err := loader.Load(ctx, root)
	if err != nil {
		return presentation.Mutation{}, err
	}
	result, syncErr := dependencies.publisherSync(ctx, state)
	mutation, mutationErr := result.Mutation()
	return mutation, errors.Join(syncErr, mutationErr)
}

func TestUpgradeSyncMutationWithPreservesSuccessfulPublisherFactsOnError(t *testing.T) {
	root := scaffoldProject(t)
	cause := errors.New("late publication failure")
	mutation, err := upgradeSyncMutationWith(testContext(t), root, upgradeSyncDependencies{
		publisherSync: func(ctx context.Context, state *project.Session) (publisher.Result, error) {
			result, syncErr := composePublisher(state).SyncLeased(context.Background(), nil)
			if syncErr != nil {
				t.Fatal(syncErr)
			}
			return result, cause
		},
	})
	if !errors.Is(err, cause) {
		t.Fatalf("error = %v, want cause", err)
	}
	if mutation.Status != "completed" {
		t.Fatalf("mutation = %#v, want ordinary successful facts for diagnostic adaptation", mutation)
	}
}
