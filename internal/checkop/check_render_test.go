package checkop

import (
	"context"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/execution"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// The operation's controlled dependency tests render semantic outcomes locally;
// production composition is exclusively through Run.
func renderCheckCollection(stdout io.Writer, collection checkCollection) error {
	out, err := outcome(collection)
	if err != nil {
		return err
	}
	if err := presentation.Render(stdout, out.Document); err != nil {
		return err
	}
	return out.Failure
}

func runRepoCheckSelection(ctx context.Context, root string, stdout io.Writer, selected []execution.StepID, policy execution.FailurePolicy, aggregate bool, deps repoCheckDependencies) error {
	return runRepoCheckSelectionWithPlanNotes(ctx, root, stdout, selected, policy, aggregate, nil, planNoteSink{}, deps)
}

func runRepoCheckSelectionWithPlanNotes(ctx context.Context, root string, stdout io.Writer, selected []execution.StepID, policy execution.FailurePolicy, aggregate bool, leadingNotes []string, planNotes planNoteSink, deps repoCheckDependencies) error {
	collection, err := collectRepoCheckSelectionWithPlanNotes(ctx, root, selected, policy, aggregate, leadingNotes, planNotes, deps)
	if err != nil {
		return err
	}
	return renderCheckCollection(stdout, collection)
}
