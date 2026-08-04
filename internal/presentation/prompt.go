package presentation

import (
	"bytes"
	"fmt"
	"io"
)

// Prompt renders a complete prelude atomically, then writes and flushes one
// validated prompt tail without a trailing newline before input is read.
func Prompt(dst io.Writer, prelude Document, tail Value) error {
	// Validate and render the complete prelude before touching dst. Tail is a
	// Value rather than a string so its zero value and line breaks must be
	// rejected by the same grammar before the prelude can leak.
	var rendered bytes.Buffer
	if err := writeDocument(&rendered, prelude); err != nil {
		return err
	}
	if err := tail.validate(); err != nil {
		return fmt.Errorf("presentation prompt tail: %w", err)
	}
	if _, err := dst.Write(rendered.Bytes()); err != nil {
		return fmt.Errorf("write presentation: %w", err)
	}
	if _, err := io.WriteString(dst, "prompt: "+tail.text); err != nil {
		return fmt.Errorf("write prompt: %w", err)
	}
	if flush, ok := dst.(interface{ Flush() error }); ok {
		if err := flush.Flush(); err != nil {
			return fmt.Errorf("flush prompt: %w", err)
		}
	}
	return nil
}
