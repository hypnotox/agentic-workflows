package presentation

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func TestPresentationCore(t *testing.T) {
	prose, err := Prose("  one\t\n two\u00a0three  ")
	if err != nil {
		t.Fatal(err)
	}
	literal, err := Literal("one\t  two")
	if err != nil {
		t.Fatal(err)
	}
	field, err := NewField("version", prose)
	if err != nil {
		t.Fatal(err)
	}
	literalField, err := NewField("build-provenance", literal)
	if err != nil {
		t.Fatal(err)
	}
	document, err := NewDocument(field, literalField)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/fields.txt")
	if err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	if err := Render(&got, document); err != nil {
		t.Fatal(err)
	}
	if got.String() != string(want) {
		t.Errorf("Render() = %q, want %q", got.String(), want)
	}

	for _, label := range []string{"version2", "build version", "build-version", "build 2-version"} {
		t.Run("valid label "+label, func(t *testing.T) {
			if _, err := NewField(label, prose); err != nil {
				t.Fatalf("NewField(%q) failed: %v", label, err)
			}
		})
	}

	for _, tc := range []struct {
		name string
		fn   func() error
	}{
		{"empty document", func() error { _, err := NewDocument(); return err }},
		{"empty label", func() error { _, err := NewField("", prose); return err }},
		{"uppercase label", func() error { _, err := NewField("Version", prose); return err }},
		{"repeated space separator", func() error { _, err := NewField("build  version", prose); return err }},
		{"repeated hyphen separator", func() error { _, err := NewField("build--version", prose); return err }},
		{"mixed separator gap", func() error { _, err := NewField("build- version", prose); return err }},
		{"leading label separator", func() error { _, err := NewField(" version", prose); return err }},
		{"trailing label separator", func() error { _, err := NewField("version-", prose); return err }},
		{"empty prose", func() error { _, err := Prose(" \t\n"); return err }},
		{"empty literal", func() error { _, err := Literal(""); return err }},
		{"literal line feed", func() error { _, err := Literal("one\ntwo"); return err }},
		{"literal carriage return", func() error { _, err := Literal("one\rtwo"); return err }},
		{"literal vertical tab", func() error { _, err := Literal("one\vtwo"); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.fn(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	for _, invalid := range []Document{
		{},
		{fields: []Field{{label: "invalid label!", value: prose}}},
		{fields: []Field{{label: "version", value: value{}}}},
	} {
		got.Reset()
		if err := Render(&got, invalid); err == nil {
			t.Fatal("Render(invalid) succeeded")
		}
		if got.Len() != 0 {
			t.Errorf("Render(invalid) wrote %q", got.String())
		}
	}
	if _, err := NewField("version", value{}); err == nil {
		t.Fatal("NewField() accepted an invalid value")
	}

	writer := &countingWriter{err: errors.New("write failed")}
	if err := Render(writer, document); !errors.Is(err, writer.err) {
		t.Errorf("Render() error = %v, want write error", err)
	}
	if writer.writes != 1 {
		t.Errorf("Render() writes = %d, want 1", writer.writes)
	}

	short := &shortWriter{}
	if err := Render(short, document); !errors.Is(err, io.ErrShortWrite) {
		t.Errorf("Render() error = %v, want io.ErrShortWrite", err)
	}
	if short.writes != 1 {
		t.Errorf("Render() short writes = %d, want 1", short.writes)
	}
}

type countingWriter struct {
	err    error
	writes int
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.writes++
	if strings.Contains(string(p), "version: one two three") {
		return 0, w.err
	}
	return len(p), nil
}

type shortWriter struct {
	writes int
}

func (w *shortWriter) Write(p []byte) (int, error) {
	w.writes++
	return len(p) - 1, nil
}
