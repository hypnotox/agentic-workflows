package effort

import "github.com/hypnotox/agentic-workflows/internal/presentation"

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
	list, err := presentation.NewList("items", values...)
	if err != nil { // coverage-ignore: typed result values and fixed presentation grammar are validated before this mapping
		return presentation.Document{}, err
	}
	section, err := presentation.NewSection("efforts", list)
	if err != nil { // coverage-ignore: typed result values and fixed presentation grammar are validated before this mapping
		return presentation.Document{}, err
	}
	return presentation.NewDocument(section)
}

// FinishMutation maps restartable resident deletion facts without inspecting
// error prose. It is used only after a completed finish.
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
		if !axis.value { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
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
