package checkop

import (
	"context"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/adr"
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

func projectSemantics(root string, prepared publisher.Preparation) (project.OperationSemantics, error) {
	corpus, err := adr.LoadCorpus(filepath.Join(root, config.DocsDir, "decisions"))
	if err != nil {
		return project.OperationSemantics{}, err
	}
	return project.OperationSemantics{
		ADRs: corpus, Pitfalls: prepared.Pitfalls(), Topics: prepared.Topics(),
		EffectiveSkills: prepared.EffectiveSkills(), GeneratedOutput: prepared.GeneratedOutput(),
		Glossary: prepared.Glossary(),
	}, nil
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
