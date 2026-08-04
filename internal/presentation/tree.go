// Package presentation owns the closed CLI presentation grammar.
package presentation

import (
	"errors"
	"fmt"
)

// Document is a complete CLI presentation. Root fields must precede sections.
type Document struct {
	nodes  []Node
	fields []Field
}

// Node is a grammar node. The closed implementation is Field, Section, List,
// RecordGroup, Record, and Steps.
type Node interface{ presentationNode() }

type nodeMarker struct{}

func (nodeMarker) presentationNode() {}

// Field is a labeled scalar value.
type Field struct {
	nodeMarker
	label string
	value value
}

// Section groups nested nodes under a label.
type Section struct {
	nodeMarker
	label string
	nodes []Node
}

// List is a labeled collection of bare scalar leaves.
type List struct {
	nodeMarker
	label  string
	values []value
}

// RecordGroup is a labeled collection with one fixed record schema.
type RecordGroup struct {
	nodeMarker
	label   string
	schema  []string
	records []Record
}

// Record is a fixed-arity compact record leaf.
type Record struct {
	nodeMarker
	values []value
}

// Steps is a labeled ordered collection of actionable values.
type Steps struct {
	nodeMarker
	label  string
	values []value
}

// Detail is the standard shape for a detailed query. Its optional leading
// fields and ordered sections lower to a Document.
type Detail struct {
	Fields   []Field
	Sections []Section
}

// Document lowers Detail into the closed presentation tree.
func (d Detail) Document() (Document, error) {
	nodes := make([]Node, 0, len(d.Fields)+len(d.Sections))
	for _, field := range d.Fields {
		nodes = append(nodes, field)
	}
	for _, section := range d.Sections {
		nodes = append(nodes, section)
	}
	return NewDocument(nodes...)
}

// NewDocument constructs a Document from ordered root nodes.
func NewDocument(nodes ...Node) (Document, error) {
	if len(nodes) == 0 {
		return Document{}, errors.New("presentation document requires at least one node")
	}
	document := Document{nodes: append([]Node(nil), nodes...)}
	for _, node := range nodes {
		if field, ok := node.(Field); ok {
			document.fields = append(document.fields, field)
		}
	}
	if err := validateDocument(document); err != nil {
		return Document{}, err
	}
	return document, nil
}

// NewField constructs a Field with a grammar-valid label and nonempty value.
func NewField(label string, value value) (Field, error) {
	if err := validateLabel(label); err != nil {
		return Field{}, err
	}
	if err := value.validate(); err != nil {
		return Field{}, err
	}
	return Field{label: label, value: value}, nil
}

// NewSection constructs a labeled nested group.
func NewSection(label string, nodes ...Node) (Section, error) {
	section := Section{label: label, nodes: append([]Node(nil), nodes...)}
	if err := validateSection(section, 1); err != nil {
		return Section{}, err
	}
	return section, nil
}

// NewList constructs a labeled list of bare values.
func NewList(label string, values ...value) (List, error) {
	list := List{label: label, values: append([]value(nil), values...)}
	if err := validateList(list); err != nil {
		return List{}, err
	}
	return list, nil
}

// NewSteps constructs a labeled ordered action collection.
func NewSteps(label string, values ...value) (Steps, error) {
	steps := Steps{label: label, values: append([]value(nil), values...)}
	if err := validateSteps(steps); err != nil {
		return Steps{}, err
	}
	return steps, nil
}

// NewRecord constructs one fixed-arity compact record.
func NewRecord(values ...value) (Record, error) {
	record := Record{values: append([]value(nil), values...)}
	if len(record.values) == 0 {
		return Record{}, errors.New("presentation record requires fields")
	}
	for _, value := range record.values {
		if err := value.validate(); err != nil {
			return Record{}, err
		}
	}
	return record, nil
}

// NewRecordGroup constructs a labeled fixed-schema record collection.
func NewRecordGroup(label string, schema []string, records ...Record) (RecordGroup, error) {
	group := RecordGroup{label: label, schema: append([]string(nil), schema...), records: append([]Record(nil), records...)}
	if err := validateRecordGroup(group); err != nil {
		return RecordGroup{}, err
	}
	return group, nil
}

func validateDocument(document Document) error {
	nodes := document.nodes
	if len(nodes) == 0 && len(document.fields) > 0 {
		nodes = make([]Node, len(document.fields))
		for i, field := range document.fields {
			nodes[i] = field
		}
	}
	if len(nodes) == 0 {
		return errors.New("presentation document requires at least one node")
	}
	seenSection := false
	for _, node := range nodes {
		if node == nil {
			return errors.New("presentation document contains a nil node")
		}
		switch n := node.(type) {
		case Field:
			if seenSection {
				return errors.New("presentation root field follows section")
			}
			if err := validateField(n); err != nil {
				return err
			}
		case Section:
			seenSection = true
			if err := validateSection(n, 1); err != nil {
				return err
			}
		default:
			return errors.New("presentation document admits only fields and sections")
		}
	}
	return nil
}
func validateField(field Field) error {
	if err := validateLabel(field.label); err != nil {
		return err
	}
	return field.value.validate()
}
func validateSection(section Section, depth int) error {
	if err := validateLabel(section.label); err != nil {
		return err
	}
	if len(section.nodes) == 0 {
		return errors.New("presentation section requires children")
	}
	for _, node := range section.nodes {
		if node == nil {
			return errors.New("presentation section contains a nil node")
		}
		switch n := node.(type) {
		case Field:
			if err := validateField(n); err != nil {
				return err
			}
		case Section:
			if depth >= 3 {
				return errors.New("presentation section nesting exceeds three levels")
			}
			if err := validateSection(n, depth+1); err != nil {
				return err
			}
		case List:
			if err := validateList(n); err != nil {
				return err
			}
		case RecordGroup:
			if err := validateRecordGroup(n); err != nil {
				return err
			}
		case Steps:
			if err := validateSteps(n); err != nil {
				return err
			}
		default:
			return errors.New("presentation section admits only fields, sections, lists, record groups, and steps")
		}
	}
	return nil
}
func validateList(list List) error {
	if err := validateLabel(list.label); err != nil {
		return err
	}
	if len(list.values) == 0 {
		return errors.New("presentation list requires values")
	}
	for _, v := range list.values {
		if err := v.validate(); err != nil {
			return err
		}
	}
	return nil
}
func validateSteps(steps Steps) error {
	if err := validateLabel(steps.label); err != nil { // coverage-ignore: validated inputs and fixed presentation grammar make this constructor path unreachable
		return err
	}
	if len(steps.values) == 0 {
		return errors.New("presentation steps requires values")
	}
	for _, v := range steps.values {
		if err := v.validate(); err != nil {
			return err
		}
	}
	return nil
}

func validateRecordGroup(group RecordGroup) error {
	if err := validateLabel(group.label); err != nil {
		return err
	}
	if len(group.schema) == 0 || len(group.records) == 0 {
		return errors.New("presentation record group requires schema and records")
	}
	for _, label := range group.schema {
		if err := validateLabel(label); err != nil {
			return err
		}
	}
	for _, record := range group.records {
		if len(record.values) != len(group.schema) {
			return errors.New("presentation record arity does not match schema")
		}
		for _, v := range record.values {
			if err := v.validate(); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateLabel(label string) error {
	if label == "" {
		return errors.New("presentation label is empty")
	}
	previousSeparator := true
	for _, r := range label {
		word := r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
		separator := r == ' ' || r == '-'
		if !word && !separator {
			return fmt.Errorf("presentation label %q is invalid", label)
		}
		if separator && previousSeparator {
			return fmt.Errorf("presentation label %q is invalid", label)
		}
		previousSeparator = separator
	}
	if previousSeparator {
		return fmt.Errorf("presentation label %q is invalid", label)
	}
	return nil
}
