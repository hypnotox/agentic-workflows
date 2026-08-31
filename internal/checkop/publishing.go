package checkop

import (
	"context"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/currentstatecoord"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

func composePublisher(session *project.Session) *publisher.Publisher {
	return publisher.New(session, project.Version)
}

func preparePublisher(composed *publisher.Publisher) (publisher.Preparation, error) {
	return composed.Prepare()
}

func operationPreparation(session *project.Session) (publisher.Preparation, error) {
	return preparePublisher(composePublisher(session))
}

func stagedDriftResult(ctx context.Context, root string) (checkresult.Result, error) {
	prep, err := currentstatecoord.PrepareStagedOutput(ctx, root)
	if err != nil {
		return checkresult.Result{}, err
	}
	prepared, err := preparePublisher(publisher.New(prep.Session, project.Version))
	if err != nil {
		return checkresult.Result{}, err
	}
	return prep.Check(prepared.Plan())
}
