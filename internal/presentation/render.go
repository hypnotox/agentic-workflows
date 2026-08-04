package presentation

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

// Render validates document, renders it completely, then writes it once to dst.
func Render(dst io.Writer, document Document) error {
	var rendered bytes.Buffer
	if err := writeDocument(&rendered, document); err != nil {
		return err
	}
	n, err := dst.Write(rendered.Bytes())
	if err != nil {
		return fmt.Errorf("write presentation: %w", err)
	}
	if n != rendered.Len() {
		return fmt.Errorf("write presentation: %w", io.ErrShortWrite)
	}
	return nil
}

func writeDocument(dst *bytes.Buffer, document Document) error {
	if len(document.fields) == 0 {
		return errors.New("presentation document requires at least one field")
	}
	for _, field := range document.fields {
		if err := validateLabel(field.label); err != nil {
			return err
		}
		if err := field.value.validate(); err != nil {
			return err
		}
		fmt.Fprintf(dst, "%s: %s\n", field.label, field.value.text)
	}
	return nil
}
