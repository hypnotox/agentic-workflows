package main

import (
	"context"

	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

func composePublisher(state *project.ProjectState, cfg *config.Config) *publisher.Publisher {
	return publisher.New(state.OutputState(), cfg, publisher.NewFilesystemReader(state.Root()), project.Version)
}

func operationPlan(state *project.ProjectState, cfg *config.Config) (outputplan.Plan, error) {
	return composePublisher(state, cfg).Plan()
}

func preparedPublisher(prep *project.ContextPreparation) *publisher.Publisher {
	return publisher.New(prep.State, prep.Config, prep.Reader, project.Version)
}

func workingContextState(ctx context.Context, state *project.ProjectState, repo *awfgit.Repo) (project.ContextState, error) {
	prep, err := project.PrepareContextState(state, repo, ctx)
	if err != nil {
		return project.ContextState{}, err
	}
	plan, err := preparedPublisher(prep).Plan()
	if err != nil { // coverage-ignore: preparation already validated this immutable tree; Publisher error propagation is covered at its planning boundary
		return project.ContextState{}, err
	}
	return project.CompleteContextState(prep, plan), nil
}

func stagedDrift(ctx context.Context, root string) ([]manifest.Drift, error) {
	prep, err := project.PrepareStagedContextState(ctx, root)
	if err != nil {
		return nil, err
	}
	plan, err := preparedPublisher(prep).Plan()
	if err != nil { // coverage-ignore: staged preparation already validated this immutable index tree; Publisher error propagation is covered at its planning boundary
		return nil, err
	}
	return project.CheckStagedDrift(prep, plan)
}

func stagedContextState(ctx context.Context, root string) (project.ContextState, error) {
	prep, err := project.PrepareStagedContextState(ctx, root)
	if err != nil {
		return project.ContextState{}, err
	}
	plan, err := preparedPublisher(prep).Plan()
	if err != nil { // coverage-ignore: staged preparation already validated this immutable index tree; Publisher error propagation is covered at its planning boundary
		return project.ContextState{}, err
	}
	return project.CompleteStagedContextState(prep, plan), nil
}
