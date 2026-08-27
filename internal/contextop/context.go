// Package contextop coordinates command-level context selections over one immutable project universe.
package contextop

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/contextinput"
	"github.com/hypnotox/agentic-workflows/internal/contextq"
	"github.com/hypnotox/agentic-workflows/internal/currentstatecoord"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

// Input is the syntax-level context selection supplied by the command parser.
type Input struct {
	Paths     []string
	Staged    bool
	Range     string
	Uncovered bool
	Full      bool
	Shows     []string
}

// LoadProject opens the command-selected project inputs as immutable state.
type LoadProject func(context.Context, string) (*project.ProjectState, *config.Config, *awfgit.Repo, error)

// Gate validates the selected working or staged command universe.
type Gate func(context.Context, string) error

// UsageError identifies an operation-level syntax refusal for command exit mapping.
type UsageError struct{ Message string }

// Error returns the syntax refusal message.
func (e *UsageError) Error() string { return e.Message }

// Run selects, validates, and queries one context universe. Loading and gate
// callbacks are concrete command-composed dependencies; delivery remains above
// this operation at the CLI boundary.
func Run(ctx context.Context, root string, input Input, load LoadProject, workingGate, stagedGate Gate) ([]byte, error) {
	facets, err := contextq.ParseContextFacets(input.Shows, input.Full)
	if err != nil {
		return nil, &UsageError{Message: "awf context: " + err.Error()}
	}
	if input.Uncovered && (input.Full || len(input.Shows) > 0) {
		return nil, &UsageError{Message: "awf context: --show and --full cannot be combined with --uncovered"}
	}
	if input.Uncovered {
		return uncovered(ctx, root, input, load, workingGate, stagedGate)
	}
	selection := contextq.SelectionExplicit
	if input.Staged {
		selection = contextq.SelectionStaged
	} else if input.Range != "" {
		selection = contextq.SelectionRange
	}
	paths := input.Paths
	if len(paths) == 0 {
		if !input.Staged && input.Range == "" {
			return nil, &UsageError{Message: "usage: awf context <path>... [--show <facet>] [--full] [--staged] [--range <a>..<b>]"}
		}
		repo, _, err := awfgit.OpenContaining(root)
		if err != nil {
			return nil, err
		}
		paths, err = repo.ChangedPaths(ctx, input.Staged, input.Range)
		if err != nil {
			return nil, err
		}
		if len(paths) == 0 {
			return nil, &UsageError{Message: "awf context: no changed paths for the given selector"}
		}
	}
	options := contextq.ContextOptions{Selection: selection, Range: input.Range, Facets: facets}
	result := contextq.ContextResult{Selection: selection, Range: input.Range}
	header := "live state for this project"
	var state contextinput.Input
	if input.Staged {
		if err := stagedGate(ctx, root); err != nil {
			return nil, err
		}
		state, err = stagedState(ctx, root)
		header = "staged state for this project"
	} else if _, statErr := os.Stat(config.ConfigPath(root)); statErr != nil {
		if !errors.Is(statErr, fs.ErrNotExist) {
			return nil, statErr
		}
		return []byte(contextq.RenderContextText(result, "static: not inside an awf project; live classification and authority require an adopted project", facets)), nil
	} else {
		if err := workingGate(ctx, root); err != nil {
			return nil, err
		}
		projectState, _, repo, loadErr := load(ctx, root)
		if loadErr != nil {
			return nil, loadErr
		}
		if input.Range != "" {
			state, err = workingCompleteState(ctx, projectState, repo)
		} else {
			state, err = workingState(ctx, projectState, repo, paths)
		}
	}
	if err != nil {
		return nil, err
	}
	return []byte(contextq.RenderContextText(contextq.New(state).ContextForOptions(paths, options), header, facets)), nil
}

func uncovered(ctx context.Context, root string, input Input, load LoadProject, workingGate, stagedGate Gate) ([]byte, error) {
	if input.Range != "" {
		return nil, &UsageError{Message: "awf context --uncovered takes optional scan-root paths, not --range"}
	}
	header := "coverage gaps for this project"
	var state contextinput.Input
	var err error
	if input.Staged {
		if err = stagedGate(ctx, root); err == nil {
			state, err = stagedState(ctx, root)
		}
		header = "staged coverage gaps for this project"
	} else if _, statErr := os.Stat(config.ConfigPath(root)); statErr != nil {
		if !errors.Is(statErr, fs.ErrNotExist) {
			return nil, statErr
		}
		static := contextq.UncoveredResult{ScanRoots: contextq.NormalizeContextPaths(input.Paths)}
		return []byte(contextq.RenderUncoveredText(static, "static: not inside an awf project; live coverage appears inside one")), nil
	} else if err = workingGate(ctx, root); err == nil {
		projectState, _, repo, loadErr := load(ctx, root)
		if loadErr != nil {
			return nil, loadErr
		}
		state, err = workingCompleteState(ctx, projectState, repo)
	}
	if err != nil {
		return nil, err
	}
	return []byte(contextq.RenderUncoveredText(contextq.New(state).Uncovered(input.Paths), header)), nil
}

func workingState(ctx context.Context, state *project.ProjectState, repo *awfgit.Repo, requested []string) (contextinput.Input, error) {
	var prep *currentstatecoord.ContextPreparation
	var err error
	if repo == nil {
		prep, err = currentstatecoord.PrepareWorkingContext(state.OutputState(), repo, ctx)
	} else {
		prep, err = currentstatecoord.PrepareFocusedWorkingContext(state.OutputState(), repo, ctx, requested)
	}
	if err != nil {
		return contextinput.Input{}, err
	}
	return focused(prep)
}

func workingCompleteState(ctx context.Context, state *project.ProjectState, repo *awfgit.Repo) (contextinput.Input, error) {
	prep, err := currentstatecoord.PrepareWorkingContext(state.OutputState(), repo, ctx)
	if err != nil {
		return contextinput.Input{}, err
	}
	return complete(prep)
}

func stagedState(ctx context.Context, root string) (contextinput.Input, error) {
	prep, err := currentstatecoord.PrepareStagedContext(ctx, root)
	if err != nil {
		return contextinput.Input{}, err
	}
	return complete(prep)
}

func focused(prep *currentstatecoord.ContextPreparation) (contextinput.Input, error) {
	prepared, err := publisher.New(prep.State, prep.Config, prep.Reader, project.Version).PrepareContext()
	if err != nil {
		return contextinput.Input{}, err
	}
	return currentstatecoord.CompleteContext(prep, prepared.ADRs(), prepared.Topics(), prepared.Plans(), prepared.Declarations()), nil
}

func complete(prep *currentstatecoord.ContextPreparation) (contextinput.Input, error) {
	prepared, err := publisher.New(prep.State, prep.Config, prep.Reader, project.Version).Prepare()
	if err != nil {
		return contextinput.Input{}, err
	}
	return currentstatecoord.CompleteContext(prep, prepared.ADRs(), prepared.Topics(), prepared.Plans(), prepared.Plan().Declarations()), nil
}
