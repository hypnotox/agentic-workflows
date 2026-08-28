//go:build !windows

package project

import (
	"net"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestFilesystemProjectReaderPathsExcludeUnsupportedEntries(t *testing.T) {
	root := testsupport.ShortTempDir(t)
	if err := os.WriteFile(filepath.Join(root, "regular.md"), []byte("regular\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	socketPath := filepath.Join(root, ".codegraph", "daemon.sock")
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	paths, err := (filesystemProjectReader{root: root}).Paths("")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(paths, []string{"regular.md"}) {
		t.Fatalf("scannable project paths = %#v, want regular files only", paths)
	}
	entries, err := (filesystemProjectReader{root: root}).Entries("")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Path == ".codegraph/daemon.sock" {
			t.Fatalf("unsupported project entry entered readable inventory: %#v", entries)
		}
	}
}
