package presentation

import (
	"bytes"
	"fmt"
	"io"
	"strings"
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
	if err := validateDocument(document); err != nil {
		return err
	}
	leadingFields := 0
	for leadingFields < len(document.nodes) {
		if _, ok := document.nodes[leadingFields].(Field); !ok {
			break
		}
		leadingFields++
	}
	for i, node := range document.nodes {
		if i == leadingFields && leadingFields > 0 {
			dst.WriteByte('\n')
		} else if i > leadingFields {
			dst.WriteByte('\n')
		}
		writeNode(dst, node, 0)
	}
	return nil
}
func writeNode(dst *bytes.Buffer, node Node, depth int) {
	indent := strings.Repeat("  ", depth)
	switch n := node.(type) {
	case Field:
		fmt.Fprintf(dst, "%s%s: %s\n", indent, n.label, n.value.text)
	case Section:
		fmt.Fprintf(dst, "%s%s:\n", indent, n.label)
		for _, child := range n.nodes {
			writeNode(dst, child, depth+1)
		}
	case List:
		fmt.Fprintf(dst, "%s%s:\n", indent, n.label)
		for _, value := range n.values {
			fmt.Fprintf(dst, "%s  %s\n", indent, value.text)
		}
	case RecordGroup:
		fmt.Fprintf(dst, "%s%s:\n", indent, n.label)
		for _, record := range n.records {
			fields := make([]string, len(record.values))
			for i, v := range record.values {
				fields[i] = escapeRecord(v.text)
			}
			fmt.Fprintf(dst, "%s  %s\n", indent, strings.Join(fields, " | "))
		}
	}
}
func escapeRecord(text string) string {
	return strings.NewReplacer("\\", "\\\\", "|", "\\|").Replace(text)
}
