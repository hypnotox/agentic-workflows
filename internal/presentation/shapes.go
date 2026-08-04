package presentation

// Diagnostic is a representation-only actionable failure. Its owner supplies
// all semantic labels, state, changed axes, cause, and ordered remedies.
type Diagnostic struct {
	Condition string
	State     string
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
	if err != nil { // coverage-ignore: validated inputs and fixed presentation grammar make this constructor path unreachable
		return Document{}, err
	}
	nodes := []Node{condition}
	if d.State != "" {
		stateValue, err := Prose(d.State)
		if err != nil {
			return Document{}, err
		}
		state, err := NewField("state", stateValue)
		if err != nil { // coverage-ignore: validated inputs and fixed presentation grammar make this constructor path unreachable
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
		if err != nil { // coverage-ignore: validated inputs and fixed presentation grammar make this constructor path unreachable
			return Document{}, err
		}
		nodes = append(nodes, cause)
	}
	children := []Node{}
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
		if err != nil { // coverage-ignore: validated inputs and fixed presentation grammar make this constructor path unreachable
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
	if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
		return Document{}, err
	}
	status, err := NewField("status", statusValue)
	if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
		return Document{}, err
	}
	children := []Node{}
	if len(m.Identity) > 0 {
		identity, err := NewSection("identity", fieldsAsNodes(m.Identity)...)
		if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
			return Document{}, err
		}
		children = append(children, identity)
	}
	if len(m.Changes) > 0 {
		changeNodes := make([]Node, 0, len(m.Changes))
		for _, change := range m.Changes {
			list, err := NewList(change.Label, change.Values...)
			if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
				return Document{}, err
			}
			changeNodes = append(changeNodes, list)
		}
		changes, err := NewSection("changes", changeNodes...)
		if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
			return Document{}, err
		}
		children = append(children, changes)
	}
	if len(m.Notes) > 0 {
		notes, err := NewList("notes", m.Notes...)
		if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
			return Document{}, err
		}
		children = append(children, notes)
	}
	if len(m.NextActions) > 0 {
		steps, err := NewSteps("next actions", m.NextActions...)
		if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
			return Document{}, err
		}
		children = append(children, steps)
	}
	if len(children) == 0 {
		return NewDocument(status)
	}
	section, err := NewSection("mutation", children...)
	if err != nil { // coverage-ignore: typed results and fixed presentation grammar make this mapping failure unreachable
		return Document{}, err
	}
	return NewDocument(status, section)
}

func fieldsAsNodes(fields []Field) []Node {
	nodes := make([]Node, len(fields))
	for i := range fields {
		nodes[i] = fields[i]
	}
	return nodes
}
