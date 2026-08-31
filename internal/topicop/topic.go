// Package topicop coordinates command-level topic selection over static or live current-state authority.
package topicop

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstatecoord"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// Input is the syntax-level topic selection supplied by the command parser.
type Input struct {
	Selector             string
	References, Coverage bool
}

// LoadProject opens the command-selected project inputs as immutable state.
type LoadProject func(context.Context, string) (*project.Session, error)

// Gate validates the selected live command universe.
type Gate func(context.Context, string) error

// UsageError identifies an operation-level syntax refusal for command exit mapping.
type UsageError struct{ Message string }

// Error returns the syntax refusal message.
func (e *UsageError) Error() string { return e.Message }

// Run validates syntax before state inspection, then selects static or live
// authority and returns the owner-produced semantic presentation detail.
func Run(ctx context.Context, root string, input Input, load LoadProject, gate Gate) (presentation.Detail, error) {
	if _, _, err := topic.ParseSelector(input.Selector); err != nil {
		return presentation.Detail{}, &UsageError{Message: err.Error()}
	}
	if _, err := os.Stat(config.ConfigPath(root)); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return presentation.Detail{}, err
		}
		return topic.StaticReferenceDetail(), nil
	}
	if err := gate(ctx, root); err != nil {
		return presentation.Detail{}, err
	}
	session, err := load(ctx, root)
	if err != nil {
		return presentation.Detail{}, err
	}
	result, err := currentstatecoord.QueryTopic(session.Root(), session.Repository(), ctx, input.Selector, topic.QueryOptions{References: input.References, Coverage: input.Coverage})
	if err != nil {
		return presentation.Detail{}, err
	}
	return result.Detail(), nil
}
