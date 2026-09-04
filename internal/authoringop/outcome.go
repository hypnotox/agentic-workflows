package authoringop

import (
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
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

// Outcome retains visible source and publication facts even when Run returns an error.
type Outcome struct {
	Kind, Name, Part string
	SourcePath       string
	Source           SourceEffect
	CreatedParents   []string
	Publisher        publisher.Result
}

// Document maps the observed semantic result to the common presentation tree.
func (o Outcome) Document() (presentation.Document, error) {
	identity := []presentation.Field{}
	for _, fact := range []struct{ label, value string }{
		{"artifact", fmt.Sprintf("%s %s", o.Kind, o.Name)},
		{"part", o.Part}, {"source", o.SourcePath}, {"source effect", string(o.Source)},
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
	appendPaths := func(label string, paths []string) error {
		values := make([]presentation.Value, 0, len(paths))
		for _, path := range paths {
			value, err := presentation.Literal(path)
			if err != nil {
				return err
			}
			values = append(values, value)
		}
		if len(values) != 0 {
			changes = append(changes, presentation.MutationChange{Label: label, Values: values})
		}
		return nil
	}
	if err := appendPaths("created parents", o.CreatedParents); err != nil {
		return presentation.Document{}, err
	}
	published := make([]string, 0, len(o.Publisher.Changes()))
	for _, change := range o.Publisher.Changes() {
		published = append(published, change.Path)
	}
	if err := appendPaths("outputs", published); err != nil {
		return presentation.Document{}, err
	}
	if err := appendPaths("pruned", o.Publisher.Pruned()); err != nil {
		return presentation.Document{}, err
	}
	return (presentation.Mutation{Status: "artifact part authored", Identity: identity, Changes: changes}).Document()
}
