package worktree

import (
	"github.com/hypnotox/agentic-workflows/internal/effort"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// Mutation maps managed-topology facts into the common readable mutation.
func (r Result) Mutation() (presentation.Mutation, error) {
	identity := []presentation.Field{}
	for _, fact := range []struct{ label, value string }{{"worktree", r.Path}, {"branch", r.Branch}} {
		if fact.value == "" {
			continue
		}
		value, err := presentation.Prose(fact.value)
		if err != nil { // coverage-ignore: typed result values and fixed presentation grammar are validated before this mapping
			return presentation.Mutation{}, err
		}
		field, err := presentation.NewField(fact.label, value)
		if err != nil { // coverage-ignore: typed result values and fixed presentation grammar are validated before this mapping
			return presentation.Mutation{}, err
		}
		identity = append(identity, field)
	}
	changes := []presentation.MutationChange{}
	if r.ChangedTopology {
		value, err := presentation.Prose("managed topology")
		if err != nil { // coverage-ignore: typed result values and fixed presentation grammar are validated before this mapping
			return presentation.Mutation{}, err
		}
		changes = append(changes, presentation.MutationChange{Label: "completed", Values: []presentation.Value{value}})
	}
	next, err := presentation.Prose(r.NextAction)
	if err != nil { // coverage-ignore: typed result values and fixed presentation grammar are validated before this mapping
		return presentation.Mutation{}, err
	}
	return presentation.Mutation{Status: r.Condition, Identity: identity, Changes: changes, NextActions: []presentation.Value{next}}, nil
}

// NewEffortMutation maps the worktree-owned creation orchestration and its
// published effort identity into one completed mutation.
func (r Result) NewEffortMutation(record effort.Record) (presentation.Mutation, error) {
	mutation, err := r.Mutation()
	if err != nil { // coverage-ignore: typed result values and fixed presentation grammar are validated before this mapping
		return presentation.Mutation{}, err
	}
	identity := make([]presentation.Field, 0, 3)
	for _, fact := range []struct{ label, value string }{{"effort", record.Slug}, {"title", record.Title}, {"memory", record.MemoryPath}} {
		value, err := presentation.Prose(fact.value)
		if err != nil { // coverage-ignore: typed result values and fixed presentation grammar are validated before this mapping
			return presentation.Mutation{}, err
		}
		field, err := presentation.NewField(fact.label, value)
		if err != nil { // coverage-ignore: typed result values and fixed presentation grammar are validated before this mapping
			return presentation.Mutation{}, err
		}
		identity = append(identity, field)
	}
	mutation.Identity = append(identity, mutation.Identity...)
	return mutation, nil
}
