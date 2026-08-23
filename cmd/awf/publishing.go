package main

import (
	"context"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/contextinput"
	"github.com/hypnotox/agentic-workflows/internal/currentstatecoord"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
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

func preparedPublisher(prep *currentstatecoord.ContextPreparation) *publisher.Publisher {
	return publisher.New(prep.State, prep.Config, prep.Reader, project.Version)
}

func workingContextState(ctx context.Context, state *project.ProjectState, repo *awfgit.Repo) (contextinput.Input, error) {
	prep, err := currentstatecoord.PrepareWorkingContext(state.OutputState(), repo, ctx)
	if err != nil {
		return contextinput.Input{}, err
	}
	prepared, err := preparePublisher(preparedPublisher(prep))
	if err != nil {
		return contextinput.Input{}, err
	}
	return currentstatecoord.CompleteContext(prep, prepared.ADRs(), prepared.Topics(), prepared.Plans(), prepared.Plan().Declarations()), nil
}

func stagedContextState(ctx context.Context, root string) (contextinput.Input, error) {
	prep, err := currentstatecoord.PrepareStagedContext(ctx, root)
	if err != nil {
		return contextinput.Input{}, err
	}
	prepared, err := preparePublisher(preparedPublisher(prep))
	if err != nil {
		return contextinput.Input{}, err
	}
	return currentstatecoord.CompleteContext(prep, prepared.ADRs(), prepared.Topics(), prepared.Plans(), prepared.Plan().Declarations()), nil
}
