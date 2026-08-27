//go:build !windows

package publisher

import (
	"net"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestFilesystemProjectReaderExcludesUnsupportedEntries(t *testing.T) {
	root := t.TempDir()
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

	reader := filesystemProjectReader{root: root}
	paths, err := reader.Paths("")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(paths, []string{"regular.md"}) {
		t.Fatalf("scannable project paths = %#v, want regular files only", paths)
	}
	entries, err := reader.Entries("")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Path == ".codegraph/daemon.sock" {
			t.Fatalf("unsupported project entry entered readable inventory: %#v", entries)
		}
	}
}
