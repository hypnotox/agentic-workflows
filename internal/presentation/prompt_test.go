package presentation

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

type promptWriter struct {
	bytes.Buffer
	flushes  int
	flushErr error
}
type failingPromptWriter struct{}
type failSecondWrite struct{ writes int }
type shortPromptWriter struct {
	writes  int
	shortAt int
}

func (failingPromptWriter) Write([]byte) (int, error) { return 0, errors.New("write") }
func (w *failSecondWrite) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == 2 {
		return 0, errors.New("tail write")
	}
	return len(p), nil
}

func (w *shortPromptWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == w.shortAt {
		return len(p) - 1, nil
	}
	return len(p), nil
}

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
	invalidPrelude := &promptWriter{}
	if err := Prompt(invalidPrelude, Document{}, value); err == nil {
		t.Fatal("invalid prelude accepted")
	}
	if invalidPrelude.Len() != 0 || invalidPrelude.flushes != 0 {
		t.Fatalf("invalid prelude leaked %q or flushed %d times", invalidPrelude.String(), invalidPrelude.flushes)
	}
	for _, bad := range []Value{{}, {text: "bad\n"}} {
		invalidTail := &promptWriter{}
		if err := Prompt(invalidTail, document, bad); err == nil {
			t.Fatal("invalid prompt tail accepted")
		}
		if invalidTail.Len() != 0 || invalidTail.flushes != 0 {
			t.Fatalf("invalid tail leaked %q or flushed %d times", invalidTail.String(), invalidTail.flushes)
		}
	}
	var plain bytes.Buffer
	if err := Prompt(&plain, document, value); err != nil {
		t.Fatal(err)
	}
	if err := Prompt(failingPromptWriter{}, document, value); err == nil {
		t.Fatal("write error accepted")
	}
	if err := Prompt(&failSecondWrite{}, document, value); err == nil {
		t.Fatal("tail write error accepted")
	}
	for _, shortAt := range []int{1, 2} {
		if err := Prompt(&shortPromptWriter{shortAt: shortAt}, document, value); !errors.Is(err, io.ErrShortWrite) {
			t.Errorf("short write %d error = %v", shortAt, err)
		}
	}
	writer.flushErr = errors.New("flush")
	if err := Prompt(writer, document, value); !errors.Is(err, writer.flushErr) {
		t.Fatalf("flush error = %v", err)
	}
}
