package effort

import "github.com/hypnotox/agentic-workflows/internal/presentation"

// Diagnostic maps typed effort failures into the common readable diagnostic.
func (e *managedTopologyError) Diagnostic() (presentation.Diagnostic, error) {
	changed := make([]presentation.Field, 0, 2)
	for _, fact := range []struct{ label, value string }{{"active resident", "no"}, {"managed topology", "no"}} {
		value, err := presentation.Prose(fact.value)
		if err != nil { // coverage-ignore: fixed nonempty refusal facts always validate as prose
			return presentation.Diagnostic{}, err
		}
		field, err := presentation.NewField(fact.label, value)
		if err != nil { // coverage-ignore: fixed grammar-valid refusal labels always validate
			return presentation.Diagnostic{}, err
		}
		changed = append(changed, field)
	}
	steps := make([]presentation.Value, 0, len(e.actions))
	for _, action := range e.actions {
		step, err := presentation.Literal(action.Text)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		steps = append(steps, step)
	}
	return presentation.Diagnostic{Condition: "managed topology remains", State: "topology", Changed: changed, Steps: steps}, nil
}

// Diagnostic maps a partial finish without embedding recovery prose in Cause.
func (e *PartialFinishError) Diagnostic() (presentation.Diagnostic, error) {
	changed := make([]presentation.Field, 0, 2)
	for _, fact := range []struct {
		label string
		value bool
	}{{"active resident", e.Result.Renamed}, {"finishing cleanup", e.Result.Cleaned}} {
		value, err := presentation.Prose(yesNo(fact.value))
		if err != nil { // coverage-ignore: yesNo always returns a nonempty prose value
			return presentation.Diagnostic{}, err
		}
		field, err := presentation.NewField(fact.label, value)
		if err != nil { // coverage-ignore: fixed grammar-valid finish labels always validate
			return presentation.Diagnostic{}, err
		}
		changed = append(changed, field)
	}
	steps := make([]presentation.Value, 0, len(e.Actions))
	for _, action := range e.Actions {
		step, err := presentation.Literal(action.Text)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		steps = append(steps, step)
	}
	return presentation.Diagnostic{Condition: "effort finish was interrupted", State: "operation", Changed: changed, Cause: e.Cause.Error(), Steps: steps}, nil
}

// Detail maps one resident record into its ordered readable facts.
func (r Record) Detail() (presentation.Detail, error) {
	fields := make([]presentation.Field, 0, 3)
	for _, fact := range []struct{ label, value string }{{"slug", r.Slug}, {"title", r.Title}, {"memory", r.MemoryPath}} {
		value, err := presentation.Prose(fact.value)
		if err != nil { // coverage-ignore: typed result values and fixed presentation grammar are validated before this mapping
			return presentation.Detail{}, err
		}
		field, err := presentation.NewField(fact.label, value)
		if err != nil { // coverage-ignore: typed result values and fixed presentation grammar are validated before this mapping
			return presentation.Detail{}, err
		}
		fields = append(fields, field)
	}
	return presentation.Detail{Fields: fields}, nil
}

// ListDocument maps active efforts in their store-defined slug order.
func ListDocument(records []Record) (presentation.Document, error) {
	if len(records) == 0 {
		value, err := presentation.Prose("none")
		if err != nil { // coverage-ignore: typed result values and fixed presentation grammar are validated before this mapping
			return presentation.Document{}, err
		}
		field, err := presentation.NewField("efforts", value)
		if err != nil { // coverage-ignore: typed result values and fixed presentation grammar are validated before this mapping
			return presentation.Document{}, err
		}
		return presentation.NewDocument(field)
	}
	values := make([]presentation.Value, 0, len(records))
	for _, record := range records {
		value, err := presentation.Prose(record.Slug + ": " + record.Title)
		if err != nil { // coverage-ignore: typed result values and fixed presentation grammar are validated before this mapping
			return presentation.Document{}, err
		}
		values = append(values, value)
	}
	list, err := presentation.NewList("efforts", values...)
	if err != nil { // coverage-ignore: typed result values and fixed presentation grammar are validated before this mapping
		return presentation.Document{}, err
	}
	section, err := presentation.NewSection("effort list", list)
	if err != nil { // coverage-ignore: the fixed section label and validated list form a valid root section
		return presentation.Document{}, err
	}
	return presentation.NewDocument(section)
}

// NewEffortMutation composes effort-owned identity with worktree-owned
// creation facts. The caller supplies a mutation already mapped by worktree.
func (r Record) NewEffortMutation(mutation presentation.Mutation) (presentation.Mutation, error) {
	identity := make([]presentation.Field, 0, 3)
	for _, fact := range []struct{ label, value string }{{"effort", r.Slug}, {"title", r.Title}, {"memory", r.MemoryPath}} {
		value, err := presentation.Prose(fact.value)
		if err != nil {
			return presentation.Mutation{}, err
		}
		field, err := presentation.NewField(fact.label, value)
		if err != nil { // coverage-ignore: NewEffortMutation owns fixed grammar-valid labels and Prose returned a validated value
			return presentation.Mutation{}, err
		}
		identity = append(identity, field)
	}
	mutation.Identity = append(identity, mutation.Identity...)
	return mutation, nil
}

// FinishMutation maps a completed restartable finish into its effort-owned
// mutation identity, changed axes, and continuation action.
func (r FinishResult) FinishMutation(slug string) (presentation.Mutation, error) {
	value, err := presentation.Prose(slug)
	if err != nil { // coverage-ignore: typed result values and fixed presentation grammar are validated before this mapping
		return presentation.Mutation{}, err
	}
	identity, err := presentation.NewField("effort", value)
	if err != nil { // coverage-ignore: typed result values and fixed presentation grammar are validated before this mapping
		return presentation.Mutation{}, err
	}
	changed := make([]presentation.Value, 0, 2)
	for _, axis := range []struct {
		label string
		value bool
	}{{"active resident", r.Renamed}, {"finishing cleanup", r.Cleaned}} {
		if !axis.value {
			continue
		}
		item, err := presentation.Prose(axis.label)
		if err != nil { // coverage-ignore: typed result values and fixed presentation grammar are validated before this mapping
			return presentation.Mutation{}, err
		}
		changed = append(changed, item)
	}
	next, err := presentation.Prose("continue without this finished effort")
	if err != nil { // coverage-ignore: typed result values and fixed presentation grammar are validated before this mapping
		return presentation.Mutation{}, err
	}
	mutation := presentation.Mutation{Status: "completed", Identity: []presentation.Field{identity}, NextActions: []presentation.Value{next}}
	if len(changed) > 0 {
		mutation.Changes = []presentation.MutationChange{{Label: "completed", Values: changed}}
	}
	return mutation, nil
}
