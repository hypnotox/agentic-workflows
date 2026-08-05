package effort

import (
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func unchangedBytes() ([]presentation.Field, error) {
	value, err := presentation.Prose("no")
	if err != nil { // coverage-ignore: fixed refusal fact always validates as prose
		return nil, err
	}
	field, err := presentation.NewField("bytes", value)
	if err != nil { // coverage-ignore: fixed grammar-valid label always validates
		return nil, err
	}
	return []presentation.Field{field}, nil
}

func recoverySteps(actions []RecoveryAction) ([]presentation.Value, error) {
	steps := make([]presentation.Value, 0, len(actions))
	for _, action := range actions {
		step, err := presentation.Literal(action.Text)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, nil
}

// Diagnostic maps a typed effort refusal into the common readable diagnostic.
func (e *refusalError) Diagnostic() (presentation.Diagnostic, error) {
	changed, err := unchangedBytes()
	if err != nil { // coverage-ignore: unchangedBytes constructs fixed valid presentation facts
		return presentation.Diagnostic{}, err
	}
	steps, err := recoverySteps(e.actions)
	if err != nil { // coverage-ignore: every internal refusal action is a fixed valid literal
		return presentation.Diagnostic{}, err
	}
	return presentation.Diagnostic{Condition: e.condition, State: e.state, Cause: e.cause, Changed: changed, Steps: steps}, nil
}

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
	steps, err := recoverySteps(e.actions)
	if err != nil {
		return presentation.Diagnostic{}, err
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
	steps, err := recoverySteps(e.Actions)
	if err != nil {
		return presentation.Diagnostic{}, err
	}
	return presentation.Diagnostic{Condition: "effort finish was interrupted", State: "operation", Changed: changed, Cause: e.Cause.Error(), Steps: steps}, nil
}

// Diagnostic maps a corrupt resident refusal into the common readable diagnostic.
func (e *CorruptError) Diagnostic() (presentation.Diagnostic, error) {
	changed, err := unchangedBytes()
	if err != nil { // coverage-ignore: unchangedBytes constructs fixed valid presentation facts
		return presentation.Diagnostic{}, err
	}
	steps, err := recoverySteps([]RecoveryAction{{Text: "preserve the resident and inspect it for manual cleanup"}})
	if err != nil { // coverage-ignore: fixed recovery literal is presentation-valid
		return presentation.Diagnostic{}, err
	}
	return presentation.Diagnostic{Condition: "effort resident is unusable", State: "resident", Cause: fmt.Sprintf("%s: %v", e.Path, e.Err), Changed: changed, Steps: steps}, nil
}

// Diagnostic maps an invalid memory refusal into the common readable diagnostic.
func (e *InvalidMemoryError) Diagnostic() (presentation.Diagnostic, error) {
	changed, err := unchangedBytes()
	if err != nil { // coverage-ignore: unchangedBytes constructs fixed valid presentation facts
		return presentation.Diagnostic{}, err
	}
	steps, err := recoverySteps([]RecoveryAction{{Text: e.NextAction}})
	if err != nil { // coverage-ignore: service-generated NextAction is a bounded valid effort command
		return presentation.Diagnostic{}, err
	}
	return presentation.Diagnostic{Condition: fmt.Sprintf("memory for effort %q cannot be updated", e.Slug), State: "memory", Cause: e.Err.Error(), Changed: changed, Steps: steps}, nil
}

// Detail maps one resident record into its ordered readable facts.
func (r Record) Detail() (presentation.Detail, error) {
	fields, err := r.presentationFields("slug")
	if err != nil {
		return presentation.Detail{}, err
	}
	return presentation.Detail{Fields: fields}, nil
}

func (r Record) presentationFields(slugLabel string) ([]presentation.Field, error) {
	fields := make([]presentation.Field, 0, 3)
	for _, fact := range []struct {
		label   string
		value   string
		literal bool
	}{{slugLabel, r.Slug, false}, {"title", r.Title, false}, {"memory", r.MemoryPath, true}} {
		var value presentation.Value
		var err error
		if fact.literal {
			value, err = presentation.Literal(fact.value)
		} else {
			value, err = presentation.Prose(fact.value)
		}
		if err != nil {
			return nil, err
		}
		field, err := presentation.NewField(fact.label, value)
		if err != nil { // coverage-ignore: fixed grammar-valid labels and validated values form valid fields
			return nil, err
		}
		fields = append(fields, field)
	}
	return fields, nil
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
	identity, err := r.presentationFields("effort")
	if err != nil {
		return presentation.Mutation{}, err
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
