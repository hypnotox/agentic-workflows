// Package presentation owns the closed CLI presentation grammar.
package presentation

import (
	"errors"
	"fmt"
)

// Document is a complete CLI presentation containing an ordered root field block.
type Document struct {
	fields []Field
}

// Field is a labeled scalar value in a Document.
type Field struct {
	label string
	value value
}

// NewDocument constructs a Document from its ordered root fields.
func NewDocument(fields ...Field) (Document, error) {
	if len(fields) == 0 {
		return Document{}, errors.New("presentation document requires at least one field")
	}
	copyFields := append([]Field(nil), fields...)
	return Document{fields: copyFields}, nil
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
