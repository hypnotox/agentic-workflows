package authoringop

import (
	"errors"
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/projectmutation"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

// SourceEffect identifies the authored source effect that committed.
type SourceEffect string

const (
	SourceNone      SourceEffect = "none"
	SourceCreated   SourceEffect = "created"
	SourceReplaced  SourceEffect = "replaced"
	SourceRemoved   SourceEffect = "removed"
	SourceLocalBody SourceEffect = "local-body-replaced"
)

// Outcome retains authoring-owned source/setup effects separately from the
// Publisher-owned synchronization result.
type Outcome struct {
	Kind, Name, Part string
	SourcePath       string
	Source           SourceEffect
	CreatedParents   []string
	Residue          []string
	Publisher        publisher.Result
}

// PartialError retains every committed axis and the original typed cause.
type PartialError struct {
	projectmutation.Partial[Outcome]
}

func (e *PartialError) Error() string { return "authoring partially committed: " + e.Cause.Error() }

func outcomeDocument(status string, outcome Outcome, recovery []string) (presentation.Document, error) {
	identity := []presentation.Field{}
	for _, fact := range []struct{ label, value string }{
		{"artifact", fmt.Sprintf("%s %s", outcome.Kind, outcome.Name)},
		{"part", outcome.Part},
		{"source", outcome.SourcePath},
		{"source effect", string(outcome.Source)},
	} {
		value, err := presentation.Literal(fact.value)
		if err != nil {
			return presentation.Document{}, err
		}
		field, err := presentation.NewField(fact.label, value)
		if err != nil {
			return presentation.Document{}, err
		}
		identity = append(identity, field)
	}
	changes := []presentation.MutationChange{}
	appendChange := func(label, text string) error {
		value, err := presentation.Literal(text)
		if err != nil {
			return err
		}
		changes = append(changes, presentation.MutationChange{Label: label, Values: []presentation.Value{value}})
		return nil
	}
	// Residue is deliberately first: cleanup that can obstruct an ordinary retry
	// must be presented before source and publisher recovery.
	for _, path := range outcome.Residue {
		if err := appendChange("publication residue", path+"; recovery: remove this residue before retrying"); err != nil {
			return presentation.Document{}, err
		}
	}
	for _, path := range outcome.CreatedParents {
		if err := appendChange("setup effects", path+"; recovery: remove only if still empty after recovery"); err != nil {
			return presentation.Document{}, err
		}
	}
	for _, effect := range outcome.Publisher.Effects() {
		if err := appendChange("publisher effects", fmt.Sprintf("%s %s; recovery: %s", effect.Kind, effect.Path, effect.Recovery)); err != nil {
			return presentation.Document{}, err
		}
	}
	next := make([]presentation.Value, 0, len(recovery))
	for _, action := range recovery {
		value, err := presentation.Prose(action)
		if err != nil {
			return presentation.Document{}, err
		}
		next = append(next, value)
	}
	return (presentation.Mutation{Status: status, Identity: identity, Changes: changes, NextActions: next}).Document()
}

// Document maps a complete semantic result to the common presentation tree.
func (o Outcome) Document() (presentation.Document, error) {
	return outcomeDocument("artifact part authored", o, []string{"continue with the rendered project state"})
}

// Document maps a typed partial result to residue-first recovery guidance.
func (e *PartialError) Document() (presentation.Document, error) {
	recovery := e.Recovery
	if len(recovery) == 0 {
		recovery = []string{"remove reported residue first, repair the reported cause, then rerun awf render"}
	}
	return outcomeDocument("artifact part partially committed", e.Outcome, recovery)
}

// AsPartial returns a retained partial outcome when err carries one.
func AsPartial(err error) (*PartialError, bool) {
	var partial *PartialError
	ok := errors.As(err, &partial)
	return partial, ok
}
