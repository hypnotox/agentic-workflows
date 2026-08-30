package checkop

import (
	"context"
	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/config"
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

func stagedDriftResult(ctx context.Context, root string) (checkresult.Result, error) {
	prep, err := project.PrepareStagedOutputState(ctx, root)
	if err != nil {
		return checkresult.Result{}, err
	}
	prepared, err := preparePublisher(publisher.New(prep.State, prep.Config, prep.Reader, project.Version))
	if err != nil {
		return checkresult.Result{}, err
	}
	return project.CheckStagedDriftResult(prep, prepared.Plan())
}
