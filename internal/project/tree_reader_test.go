package project

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

type snapshotTreeReader struct{ tree *snapshot.Tree }

func (r snapshotTreeReader) ReadFile(path string) ([]byte, bool, error) {
	f, ok := r.tree.Lookup(filepath.ToSlash(path))
	if !ok || !f.Scannable() {
		return nil, false, nil
	}
	return slices.Clone(f.Bytes), true, nil
}

func (r snapshotTreeReader) Paths(prefix string) ([]string, error) {
	var out []string
	prefix = filepath.ToSlash(prefix)
	for _, f := range r.tree.List() {
		if f.Scannable() && strings.HasPrefix(f.Path, prefix) {
			out = append(out, f.Path)
		}
	}
	return out, nil
}

func TestFilesystemProjectReaderEntries(t *testing.T) {
	root := t.TempDir()
	path := root + "/.awf/example/file"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
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

func TestFilesystemProjectReaderPathsErrors(t *testing.T) {
	validRoot := t.TempDir()
	for _, tc := range []struct {
		name, root, prefix, subject string
	}{
		{
			name:    "invalid prefix",
			root:    validRoot,
			prefix:  "invalid\x00prefix",
			subject: "invalid\x00prefix",
		},
		{
			name:    "invalid root",
			root:    "invalid\x00root",
			subject: "project tree",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := (filesystemProjectReader{root: tc.root}).Paths(tc.prefix)
			if err == nil {
				t.Fatal("Paths error = nil")
			}
			if _, entryErr := (filesystemProjectReader{root: tc.root}).Entries(tc.prefix); entryErr == nil {
				t.Fatal("Entries error = nil")
			}
			var pathErr *fs.PathError
			if !errors.As(err, &pathErr) {
				t.Fatalf("Paths error identity = %T, want *fs.PathError: %v", err, err)
			}
			if diagnostic := "enumerate " + tc.subject; !strings.Contains(err.Error(), diagnostic) {
				t.Fatalf("Paths error = %q, want diagnostic subject %q", err, diagnostic)
			}
		})
	}
}
