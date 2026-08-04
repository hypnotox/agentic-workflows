package worktree

import "github.com/hypnotox/agentic-workflows/internal/presentation"

// Diagnostic maps a typed refusal into the common readable diagnostic.
func (e *RefusalError) Diagnostic() (presentation.Diagnostic, error) {
	value, err := presentation.Prose(yesNo(e.ChangedTopology))
	if err != nil { // coverage-ignore: yesNo always returns a nonempty prose value
		return presentation.Diagnostic{}, err
	}
	axis, err := presentation.NewField("managed topology", value)
	if err != nil { // coverage-ignore: fixed grammar-valid topology label always validates
		return presentation.Diagnostic{}, err
	}
	actions := e.NextActions
	if len(actions) == 0 {
		actions = []string{e.NextAction}
	}
	steps := make([]presentation.Value, 0, len(actions))
	for _, action := range actions {
		step, err := presentation.Literal(action)
		if err != nil { // coverage-ignore: constructors require nonempty recovery actions
			return presentation.Diagnostic{}, err
		}
		steps = append(steps, step)
	}
	diagnostic := presentation.Diagnostic{Condition: e.Condition, State: e.Category, Changed: []presentation.Field{axis}, Steps: steps}
	if e.Err != nil {
		diagnostic.Cause = e.Err.Error()
	}
	return diagnostic, nil
}

// Diagnostic maps failed creation while retaining both mechanism identities.
func (e *CreationError) Diagnostic() (presentation.Diagnostic, error) {
	changed := make([]presentation.Field, 0, 2)
	for _, fact := range []struct {
		label string
		value bool
	}{{"effort resident", e.ChangedEffort}, {"managed topology", e.ChangedTopology}} {
		value, err := presentation.Prose(yesNo(fact.value))
		if err != nil { // coverage-ignore: yesNo always returns a nonempty prose value
			return presentation.Diagnostic{}, err
		}
		field, err := presentation.NewField(fact.label, value)
		if err != nil { // coverage-ignore: fixed grammar-valid changed-axis labels always validate
			return presentation.Diagnostic{}, err
		}
		changed = append(changed, field)
	}
	steps := make([]presentation.Value, 0, len(e.Steps))
	for _, text := range e.Steps {
		step, err := presentation.Prose(text)
		if err != nil { // coverage-ignore: manager-owned recovery steps are nonempty text
			return presentation.Diagnostic{}, err
		}
		steps = append(steps, step)
	}
	cause := e.Cause.Error()
	if e.RollbackCause != nil {
		cause += " | " + e.RollbackCause.Error()
	}
	return presentation.Diagnostic{Condition: e.Condition, State: "operation", Changed: changed, Cause: cause, Steps: steps}, nil
}

// Mutation maps managed-topology facts into the common readable mutation.
func (r Result) Mutation() (presentation.Mutation, error) {
	identity := []presentation.Field{}
	for _, fact := range []struct {
		label   string
		value   string
		literal bool
	}{{"worktree", r.Path, true}, {"branch", r.Branch, false}} {
		if fact.value == "" {
			continue
		}
		var value presentation.Value
		var err error
		if fact.literal {
			value, err = presentation.Literal(fact.value)
		} else {
			value, err = presentation.Prose(fact.value)
		}
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
	next, err := presentation.Literal(r.NextAction)
	if err != nil { // coverage-ignore: typed result values and fixed presentation grammar are validated before this mapping
		return presentation.Mutation{}, err
	}
	return presentation.Mutation{Status: r.Condition, Identity: identity, Changes: changes, NextActions: []presentation.Value{next}}, nil
}
