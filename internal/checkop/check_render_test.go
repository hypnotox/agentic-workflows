package checkop

import (
	"context"
	"io"

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

func runRepoCheckSelection(ctx context.Context, root string, stdout io.Writer, selected []repositoryLane, continueOnFailure, aggregate bool, deps repoCheckDependencies) error {
	return runRepoCheckSelectionWithNotes(ctx, root, stdout, selected, continueOnFailure, aggregate, nil, deps)
}

func runRepoCheckSelectionWithNotes(ctx context.Context, root string, stdout io.Writer, selected []repositoryLane, continueOnFailure, aggregate bool, leadingNotes []string, deps repoCheckDependencies) error {
	collection, err := collectRepoCheckSelection(ctx, root, selected, continueOnFailure, aggregate, leadingNotes, deps)
	if err != nil {
		return err
	}
	return renderCheckCollection(stdout, collection)
}
