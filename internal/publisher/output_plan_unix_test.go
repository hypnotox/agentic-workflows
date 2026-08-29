//go:build !windows

package publisher

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestFilesystemProjectReaderReadLinesIsBounded(t *testing.T) {
	root := testsupport.ShortTempDir(t)
	path := filepath.Join(root, "source.go")
	if err := os.WriteFile(path, []byte("first\nsecond\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reader := filesystemProjectReader{root: root}
	var lines []string
	found, err := reader.ReadLines("source.go", 64, func(line string) error {
		lines = append(lines, line)
		return nil
	})
	if err != nil || !found || !slices.Equal(lines, []string{"first", "second"}) {
		t.Fatalf("ReadLines = found %t, lines %#v, error %v", found, lines, err)
	}
	if _, err := reader.ReadLines("source.go", 3, func(string) error { return nil }); err == nil {
		t.Fatal("ReadLines accepted a line beyond its scanner bound")
	}
	visitErr := errors.New("visit failed")
	if _, err := reader.ReadLines("source.go", 64, func(string) error { return visitErr }); !errors.Is(err, visitErr) {
		t.Fatalf("ReadLines visitor error = %v, want %v", err, visitErr)
	}
}

func TestFilesystemProjectReaderReadLinesAcceptsExactMarkerBoundary(t *testing.T) {
	const limit = 4 << 20
	root := testsupport.ShortTempDir(t)
	path := filepath.Join(root, "source.go")
	reader := filesystemProjectReader{root: root}
	for _, tc := range []struct {
		name    string
		length  int
		wantErr bool
	}{
		{name: "exact", length: limit},
		{name: "over", length: limit + 1, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, append([]byte(strings.Repeat("x", tc.length)), '\n'), 0o644); err != nil {
				t.Fatal(err)
			}
			visits := 0
			_, err := reader.ReadLines("source.go", limit, func(line string) error {
				visits++
				if len(line) != tc.length {
					t.Fatalf("line length = %d, want %d", len(line), tc.length)
				}
				return nil
			})
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "line exceeds 4194304 bytes") || visits != 0 {
					t.Fatalf("over-bound read visits=%d error=%v", visits, err)
				}
				return
			}
			if err != nil || visits != 1 {
				t.Fatalf("exact-bound read visits=%d error=%v", visits, err)
			}
		})
	}
}

func TestFilesystemProjectReaderExcludesUnsupportedEntries(t *testing.T) {
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
