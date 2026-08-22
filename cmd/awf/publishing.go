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

func preparePublisher(composed *publisher.Publisher) (publisher.Preparation, error) {
	return composed.Prepare()
}

func operationPreparation(state *project.ProjectState, cfg *config.Config) (publisher.Preparation, error) {
	return preparePublisher(composePublisher(state, cfg))
}

func operationPlan(state *project.ProjectState, cfg *config.Config) (outputplan.Plan, error) {
	return composePublisher(state, cfg).Plan()
}

func projectSemantics(prepared publisher.Preparation) project.OperationSemantics {
	return project.OperationSemantics{
		ADRs: prepared.ADRs(), Pitfalls: prepared.Pitfalls(), Topics: prepared.Topics(),
		EffectiveSkills: prepared.EffectiveSkills(), Plans: prepared.Plans(), PlansError: prepared.PlansError(), GeneratedOutput: prepared.GeneratedOutput(),
		Vocabulary: prepared.Vocabulary(),
	}
}

func preparedPublisher(prep *project.ContextPreparation) *publisher.Publisher {
	return publisher.New(prep.State, prep.Config, prep.Reader, project.Version)
}

func workingContextState(ctx context.Context, state *project.ProjectState, repo *awfgit.Repo) (project.ContextState, error) {
	prep, err := project.PrepareContextState(state, repo, ctx)
	if err != nil {
		return project.ContextState{}, err
	}
	prepared, err := preparePublisher(preparedPublisher(prep))
	if err != nil {
		return project.ContextState{}, err
	}
	return project.CompleteContextState(prep, prepared.Plan()), nil
}

func stagedDrift(ctx context.Context, root string) ([]manifest.Drift, error) {
	prep, err := project.PrepareStagedContextState(ctx, root)
	if err != nil {
		return nil, err
	}
	prepared, err := preparePublisher(preparedPublisher(prep))
	if err != nil {
		return nil, err
	}
	return project.CheckStagedDrift(prep, prepared.Plan())
}

func stagedContextState(ctx context.Context, root string) (project.ContextState, error) {
	prep, err := project.PrepareStagedContextState(ctx, root)
	if err != nil {
		return project.ContextState{}, err
	}
	prepared, err := preparePublisher(preparedPublisher(prep))
	if err != nil {
		return project.ContextState{}, err
	}
	return project.CompleteStagedContextState(prep, prepared.Plan()), nil
}
