package publisher

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/generatedcheck"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

type generatedEntriesErrorReader struct {
	ProjectTreeReader
	err error
}

func (r generatedEntriesErrorReader) Entries(string) ([]generatedcheck.TreeEntry, error) {
	return nil, r.err
}

type generatedPathsErrorReader struct {
	ProjectTreeReader
	err error
}

func (r generatedPathsErrorReader) Paths(string) ([]string, error) { return nil, r.err }

func TestFilesystemReaderEntries(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, ".awf", "example", "file")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := (filesystemProjectReader{root: root}).Entries(".awf/")
	if err != nil || len(entries) != 3 || !entries[0].Directory || entries[2].Path != ".awf/example/file" {
		t.Fatalf("entries=%#v err=%v", entries, err)
	}
	entries, err = (filesystemProjectReader{root: t.TempDir()}).Entries(".awf/")
	if err != nil || len(entries) != 0 {
		t.Fatalf("absent entries=%#v err=%v", entries, err)
	}
}

func TestFilesystemReaderEntriesErrors(t *testing.T) {
	for _, tc := range []struct{ root, prefix string }{{t.TempDir(), "invalid\x00prefix"}, {"invalid\x00root", ""}} {
		_, err := (filesystemProjectReader{root: tc.root}).Entries(tc.prefix)
		if err == nil {
			t.Fatal("Entries error = nil")
		}
		var pathErr *fs.PathError
		if !errors.As(err, &pathErr) {
			t.Fatalf("Entries error identity = %T, want *fs.PathError: %v", err, err)
		}
	}
}

func TestGeneratedSemanticsPropagatesPreparedTreeErrors(t *testing.T) {
	state, err := Open(context.Background(), scaffold(t, sampleYAML))
	if err != nil {
		t.Fatal(err)
	}
	inputs := renderInputsForTest(state)
	sentinel := errors.New("generated entries unavailable")
	inputs.read = generatedEntriesErrorReader{ProjectTreeReader: inputs.read, err: sentinel}
	if _, err := (&Publisher{inputs: inputs}).Prepare(); !errors.Is(err, sentinel) {
		t.Fatalf("Prepare error = %v, want generated entries sentinel", err)
	}

	inputs = renderInputsForTest(state)
	sentinel = errors.New("generated paths unavailable")
	inputs.read = generatedPathsErrorReader{ProjectTreeReader: inputs.read, err: sentinel}
	if _, err := generatedSemantics(inputs, topic.Corpus{}); !errors.Is(err, sentinel) {
		t.Fatalf("generatedSemantics error = %v, want paths sentinel", err)
	}
}

func TestGeneratedSemanticClosedKinds(t *testing.T) {
	c := &catalog.Catalog{}
	if artifactNames(c, "unknown") != nil || artifactSections(c, "unknown", "") != nil {
		t.Fatal("unknown generated semantic kind was accepted")
	}
	projected := map[string]bool{}
	for _, name := range artifactNames(catalog.Standard, "docs") {
		projected[name] = true
	}
	for _, singleton := range catalog.SingletonKindsFor(catalog.Standard) {
		if _, shared := catalog.Standard.Docs[singleton]; shared && !projected[singleton] {
			t.Errorf("singleton-backed docs artifact %q omitted from closed-tree projection", singleton)
		}
	}
}
