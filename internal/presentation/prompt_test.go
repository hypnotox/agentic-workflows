package presentation

import (
	"bytes"
	"errors"
	"testing"
)

type promptWriter struct {
	bytes.Buffer
	flushes  int
	flushErr error
}
type failingPromptWriter struct{}

func (failingPromptWriter) Write([]byte) (int, error) { return 0, errors.New("write") }

func (w *promptWriter) Flush() error { w.flushes++; return w.flushErr }

func TestPrompt(t *testing.T) {
	value, _ := Prose("ready")
	field, _ := NewField("status", value)
	document, _ := NewDocument(field)
	writer := &promptWriter{}
	if err := Prompt(writer, document, value); err != nil {
		t.Fatal(err)
	}
	if got, want := writer.String(), "status: ready\nprompt: ready"; got != want {
		t.Fatalf("Prompt = %q, want %q", got, want)
	}
	if writer.flushes != 1 {
		t.Fatalf("flushes = %d", writer.flushes)
	}
	if err := Prompt(writer, Document{}, value); err == nil {
		t.Fatal("invalid prelude accepted")
	}
	bad := Value{text: "bad\n"}
	if err := Prompt(writer, document, bad); err == nil {
		t.Fatal("newline prompt accepted")
	}
	var plain bytes.Buffer
	if err := Prompt(&plain, document, value); err != nil {
		t.Fatal(err)
	}
	if err := Prompt(failingPromptWriter{}, document, value); err == nil {
		t.Fatal("write error accepted")
	}
	writer.flushErr = errors.New("flush")
	if err := Prompt(writer, document, value); !errors.Is(err, writer.flushErr) {
		t.Fatalf("flush error = %v", err)
	}
}
