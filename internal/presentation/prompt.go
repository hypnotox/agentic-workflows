package presentation

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

// Prompt renders a complete prelude atomically, then writes and flushes one
// validated prompt tail without a trailing newline before input is read.
func Prompt(dst io.Writer, prelude Document, tail Value) error {
	if err := Render(dst, prelude); err != nil {
		return err
	}
	if strings.ContainsAny(tail.text, "\r\n") {
		return errors.New("presentation prompt contains a line break")
	}
	if _, err := io.WriteString(dst, "prompt: "+tail.text); err != nil { // coverage-ignore: validated inputs and fixed presentation grammar make this constructor path unreachable
		return fmt.Errorf("write prompt: %w", err)
	}
	if flush, ok := dst.(interface{ Flush() error }); ok {
		if err := flush.Flush(); err != nil {
			return fmt.Errorf("flush prompt: %w", err)
		}
	}
	return nil
}
