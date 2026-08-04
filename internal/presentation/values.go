package presentation

import (
	"errors"
	"strings"
	"unicode"
)

// value is a validated scalar value for a Field.
type value struct {
	text string
}

// Prose constructs a Value after trimming and collapsing Unicode whitespace to
// one ASCII space.
func Prose(text string) (value, error) {
	return newValue(strings.Join(strings.Fields(text), " "))
}

// Literal constructs a Value that preserves its horizontal spaces.
func Literal(text string) (value, error) {
	return newValue(text)
}

func newValue(text string) (value, error) {
	result := value{text: text}
	if err := result.validate(); err != nil {
		return value{}, err
	}
	return result, nil
}

func (v value) validate() error {
	if v.text == "" {
		return errors.New("presentation value is empty")
	}
	if strings.ContainsAny(v.text, "\r\n") {
		return errors.New("presentation value contains a line break")
	}
	for _, r := range v.text {
		if unicode.IsSpace(r) && r != ' ' && r != '\t' {
			return errors.New("presentation value contains unsupported whitespace")
		}
	}
	return nil
}
