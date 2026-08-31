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


func stagedDriftResult(ctx context.Context, root string) (checkresult.Result, error) {
	prep, err := currentstatecoord.PrepareStagedOutput(ctx, root)
	if err != nil {
		return checkresult.Result{}, err
	}
	operation := publisher.New(prep.Session, project.Version)
	plan, err := operation.Plan()
	if err != nil {
		return checkresult.Result{}, err
	}
	return prep.Check(plan)
}
