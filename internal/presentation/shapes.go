package presentation

import (
	"errors"
	"fmt"
)

// ReportCategory is one semantic category of same-schema findings in a report.
type ReportCategory struct {
	Label   string
	Schema  []string
	Records []Record
}

// Collection is a representation-only ordered set of semantic record
// categories. Its owner supplies category labels, schemas, records, and order.
type Collection struct {
	Status     string
	Categories []CollectionCategory
}

// CollectionCategory is one same-schema category in a Collection.
type CollectionCategory struct {
	Label   string
	Schema  []string
	Records []Record
	Values  []Value
}

// Document lowers a collection into the closed presentation tree.
func (c Collection) Document() (Document, error) {
	statusValue, err := Prose(c.Status)
	if err != nil {
		return Document{}, err
	}
	status, err := NewField("status", statusValue)
	if err != nil {
		return Document{}, err
	}
	categories := make([]Node, 0, len(c.Categories))
	for _, category := range c.Categories {
		if len(category.Values) > 0 {
			if len(category.Schema) > 0 || len(category.Records) > 0 {
				return Document{}, errors.New("collection category cannot mix values with schema or records")
			}
			list, err := NewList(category.Label, category.Values...)
			if err != nil {
				return Document{}, err
			}
			categories = append(categories, list)
			continue
		}
		group, err := NewRecordGroup(category.Label, category.Schema, category.Records...)
		if err != nil {
			return Document{}, err
		}
		categories = append(categories, group)
	}
	if len(categories) == 0 {
		return NewDocument(status)
	}
	section, err := NewSection("collection", categories...)
	if err != nil {
		return Document{}, err
	}
	return NewDocument(status, section)
}

// Report is a representation-only complete check or audit result. Its owner
// supplies the status, context, summary, categories, schemas, and ordering.
type Report struct {
	Status     string
	Context    []Field
	Summary    []Field
	Categories []ReportCategory
}

// Document lowers a report into the closed presentation tree.
func (r Report) Document() (Document, error) {
	statusValue, err := Prose(r.Status)
	if err != nil {
		return Document{}, err
	}
	status, err := NewField("status", statusValue)
	if err != nil {
		return Document{}, err
	}
	nodes := []Node{status}
	if len(r.Context) > 0 {
		section, err := NewSection("context", fieldsAsNodes(r.Context)...)
		if err != nil {
			return Document{}, err
		}
		nodes = append(nodes, section)
	}
	if len(r.Summary) > 0 {
		section, err := NewSection("summary", fieldsAsNodes(r.Summary)...)
		if err != nil {
			return Document{}, err
		}
		nodes = append(nodes, section)
	}
	if len(r.Categories) > 0 {
		children := make([]Node, 0, len(r.Categories))
		lastCategory := -1
		for _, category := range r.Categories {
			categoryOrder, ok := reportCategoryOrder(category.Label)
			if !ok {
				return Document{}, fmt.Errorf("unknown report category %q", category.Label)
			}
			if categoryOrder <= lastCategory {
				return Document{}, errors.New("report categories must be ordered errors then warnings then information")
			}
			lastCategory = categoryOrder
			group, err := NewRecordGroup(category.Label, category.Schema, category.Records...)
			if err != nil {
				return Document{}, err
			}
			children = append(children, group)
		}
		section, err := NewSection("findings", children...)
		if err != nil {
			return Document{}, err
		}
		nodes = append(nodes, section)
	}
	return NewDocument(nodes...)
}

// Diagnostic is a representation-only actionable failure. Its owner supplies
// all semantic labels, state, changed axes, cause, and ordered remedies.
type Diagnostic struct {
	Condition string
	State     string
	Planned   []Field
	Changed   []Field
	Cause     string
	Steps     []Value
}

// Document lowers a diagnostic into the closed presentation tree.
func (d Diagnostic) Document() (Document, error) {
	conditionValue, err := Prose(d.Condition)
	if err != nil {
		return Document{}, err
	}
	condition, err := NewField("condition", conditionValue)
	if err != nil {
		return Document{}, err
	}
	nodes := []Node{condition}
	if d.State != "" {
		stateValue, err := Prose(d.State)
		if err != nil {
			return Document{}, err
		}
		state, err := NewField("state", stateValue)
		if err != nil {
			return Document{}, err
		}
		nodes = append(nodes, state)
	}
	if d.Cause != "" {
		causeValue, err := Prose(d.Cause)
		if err != nil {
			return Document{}, err
		}
		cause, err := NewField("cause", causeValue)
		if err != nil {
			return Document{}, err
		}
		nodes = append(nodes, cause)
	}
	children := []Node{}
	if len(d.Planned) > 0 {
		planned, err := NewSection("planned", fieldsAsNodes(d.Planned)...)
		if err != nil {
			return Document{}, err
		}
		children = append(children, planned)
	}
	if len(d.Changed) > 0 {
		changed, err := NewSection("changed", fieldsAsNodes(d.Changed)...)
		if err != nil {
			return Document{}, err
		}
		children = append(children, changed)
	}
	if len(d.Steps) > 0 {
		steps, err := NewSteps("steps", d.Steps...)
		if err != nil {
			return Document{}, err
		}
		children = append(children, steps)
	}
	if len(children) > 0 {
		section, err := NewSection("diagnostic", children...)
		if err != nil {
			return Document{}, err
		}
		nodes = append(nodes, section)
	}
	return NewDocument(nodes...)
}

// MutationChange is one owner-named ordered group of mutation facts.
type MutationChange struct {
	Label  string
	Values []Value
}

// Mutation is a representation-only completed operation. Its owner supplies
// status, identity, changed axes, notes, and ordered next actions.
type Mutation struct {
	Status      string
	Identity    []Field
	Changes     []MutationChange
	Notes       []Value
	NextActions []Value
}

// Document lowers a mutation into the closed presentation tree.
func (m Mutation) Document() (Document, error) {
	statusValue, err := Prose(m.Status)
	if err != nil {
		return Document{}, err
	}
	status, err := NewField("status", statusValue)
	if err != nil {
		return Document{}, err
	}
	children := []Node{}
	if len(m.Identity) > 0 {
		identity, err := NewSection("identity", fieldsAsNodes(m.Identity)...)
		if err != nil {
			return Document{}, err
		}
		children = append(children, identity)
	}
	if len(m.Changes) > 0 {
		changeNodes := make([]Node, 0, len(m.Changes))
		for _, change := range m.Changes {
			list, err := NewList(change.Label, change.Values...)
			if err != nil {
				return Document{}, err
			}
			changeNodes = append(changeNodes, list)
		}
		changes, err := NewSection("changes", changeNodes...)
		if err != nil {
			return Document{}, err
		}
		children = append(children, changes)
	}
	if len(m.Notes) > 0 {
		notes, err := NewList("notes", m.Notes...)
		if err != nil {
			return Document{}, err
		}
		children = append(children, notes)
	}
	if len(m.NextActions) > 0 {
		steps, err := NewSteps("next actions", m.NextActions...)
		if err != nil {
			return Document{}, err
		}
		children = append(children, steps)
	}
	if len(children) == 0 {
		return NewDocument(status)
	}
	section, err := NewSection("mutation", children...)
	if err != nil {
		return Document{}, err
	}
	return NewDocument(status, section)
}

func reportCategoryOrder(label string) (int, bool) {
	switch label {
	case "errors":
		return 0, true
	case "warnings":
		return 1, true
	case "information":
		return 2, true
	default:
		return 0, false
	}
}

func fieldsAsNodes(fields []Field) []Node {
	nodes := make([]Node, len(fields))
	for i := range fields {
		nodes[i] = fields[i]
	}
	return nodes
}
