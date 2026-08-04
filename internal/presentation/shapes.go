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

func fieldsAsNodes(fields []Field) []Node {
	nodes := make([]Node, len(fields))
	for i := range fields {
		nodes[i] = fields[i]
	}
	return nodes
}
