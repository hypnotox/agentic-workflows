package effort

import (
	"errors"
	"fmt"
	"strconv"

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

// MemoryDocument maps every protocol-neutral memory fact to ordinary readable text.
func (r MemoryOperationResult) MemoryDocument() (presentation.Document, error) {
	fields := []presentation.Field{}
	add := func(label, text string, prose bool) error {
		var value presentation.Value
		var err error
		if prose {
			value, err = presentation.Prose(text)
		} else {
			value, err = presentation.Literal(text)
		}
		if err != nil {
			return err
		}
		field, err := presentation.NewField(label, value)
		if err != nil { // coverage-ignore: every caller supplies a fixed grammar-valid label
			return err
		}
		fields = append(fields, field)
		return nil
	}
	if r.Outcome != nil {
		if err := add("condition", r.Outcome.Condition, true); err != nil {
			return presentation.Document{}, err
		}
		if err := add("state", r.Outcome.Category, true); err != nil {
			return presentation.Document{}, err
		}
		if r.Outcome.Cause != "" {
			if err := add("cause", r.Outcome.Cause, true); err != nil { // coverage-ignore: a nonempty mechanism cause always normalizes to valid prose
				return presentation.Document{}, err
			}
		}
		if err := add("changed memory", yesNo(r.Outcome.ChangedMemory), false); err != nil { // coverage-ignore: fixed label and yes/no literal are constructor-valid
			return presentation.Document{}, err
		}
		for _, fact := range memoryRefusalPresentationFacts(r) {
			if err := add(fact.label, fact.value, false); err != nil { // coverage-ignore: fixed labels and integer literals are constructor-valid
				return presentation.Document{}, err
			}
		}
		steps, err := recoverySteps(r.Outcome.NextActions)
		if err != nil {
			return presentation.Document{}, err
		}
		nodes := fieldsAsNodesForEffort(fields)
		if len(steps) > 0 {
			stepNode, err := presentation.NewSteps("steps", steps...)
			if err != nil { // coverage-ignore: recoverySteps validated every action
				return presentation.Document{}, err
			}
			diagnostic, err := presentation.NewSection("diagnostic", stepNode)
			if err != nil { // coverage-ignore: fixed labels and validated children are constructor-valid
				return presentation.Document{}, err
			}
			nodes = append(nodes, diagnostic)
		}
		return presentation.NewDocument(nodes...)
	}
	if err := add("status", string(r.Condition), true); err != nil {
		return presentation.Document{}, err
	}
	if r.Memory == nil {
		return presentation.Document{}, errors.New("memory success requires metadata")
	}
	for _, fact := range []struct{ label, value string }{{"effort", r.Memory.Effort}, {"phase", r.Memory.Phase}, {"next", r.Memory.Next}, {"updated", r.Memory.Updated}} {
		if err := add(fact.label, fact.value, false); err != nil {
			return presentation.Document{}, err
		}
	}
	if r.Condition == MemoryRead {
		if r.Range == nil {
			return presentation.Document{}, errors.New("memory read requires range")
		}
		next := "null"
		if r.Range.NextOffset != nil {
			next = strconv.Itoa(*r.Range.NextOffset)
		}
		for _, fact := range []struct{ label, value string }{
			{"start line", strconv.Itoa(r.Range.StartLine)},
			{"end line", strconv.Itoa(r.Range.EndLine)},
			{"total lines", strconv.Itoa(r.Range.TotalLines)},
			{"next offset", next},
			{"truncated by", r.Range.TruncatedBy},
			{"content", strconv.Quote(r.Content)},
		} {
			if err := add(fact.label, fact.value, false); err != nil { // coverage-ignore: fixed labels and quoted or typed range literals are constructor-valid
				return presentation.Document{}, err
			}
		}
	}
	if r.Condition == MemoryEdited {
		if r.Diff == nil {
			return presentation.Document{}, errors.New("memory edit requires diff")
		}
		first := "null"
		if r.Diff.FirstChangedLine != nil {
			first = strconv.Itoa(*r.Diff.FirstChangedLine)
		}
		for _, fact := range []struct{ label, value string }{
			{"replacements", strconv.Itoa(r.ReplacementCount)},
			{"diff", strconv.Quote(r.Diff.Text)},
			{"first changed line", first},
			{"diff truncated", yesNo(r.Diff.Truncated)},
		} {
			if err := add(fact.label, fact.value, false); err != nil { // coverage-ignore: fixed labels and quoted or typed diff literals are constructor-valid
				return presentation.Document{}, err
			}
		}
	}
	return presentation.NewDocument(fieldsAsNodesForEffort(fields)...)
}

type memoryPresentationFact struct{ label, value string }

func memoryRefusalPresentationFacts(r MemoryOperationResult) []memoryPresentationFact {
	facts := []memoryPresentationFact{}
	if r.Offset != nil {
		facts = append(facts, memoryPresentationFact{"offset", strconv.Itoa(r.Offset.Offset)}, memoryPresentationFact{"total lines", strconv.Itoa(r.Offset.TotalLines)})
	}
	if r.Edit != nil {
		facts = append(facts, memoryPresentationFact{"edit index", strconv.Itoa(r.Edit.Index)})
		if r.Edit.Occurrences > 0 {
			facts = append(facts, memoryPresentationFact{"occurrences", strconv.Itoa(r.Edit.Occurrences)})
		}
	}
	if r.Overlap != nil {
		facts = append(facts, memoryPresentationFact{"first edit index", strconv.Itoa(r.Overlap.FirstIndex)}, memoryPresentationFact{"second edit index", strconv.Itoa(r.Overlap.SecondIndex)})
	}
	if r.Size != nil {
		facts = append(facts, memoryPresentationFact{"bytes", strconv.Itoa(r.Size.Bytes)}, memoryPresentationFact{"maximum bytes", strconv.Itoa(r.Size.MaxBytes)})
	}
	return facts
}

func fieldsAsNodesForEffort(fields []presentation.Field) []presentation.Node {
	nodes := make([]presentation.Node, len(fields))
	for i := range fields {
		nodes[i] = fields[i]
	}
	return nodes
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
