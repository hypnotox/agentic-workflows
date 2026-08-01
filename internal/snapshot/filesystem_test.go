package snapshot

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestFilesystemTree(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dir", "regular.txt"), []byte("regular"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "run"), []byte("exec"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "hidden"), []byte("git"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("dir/regular.txt", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(root, "pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	tree, err := FilesystemTree(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for path, mode := range map[string]Mode{"dir/regular.txt": Regular, "run": Executable, "link": Symlink} {
		file, ok := tree.Lookup(path)
		if !ok || file.Mode != mode {
			t.Errorf("%s = (%v, %v), want mode %v", path, file, ok, mode)
		}
	}
	if _, ok := tree.Lookup(".git/hidden"); ok {
		t.Fatal("filesystem snapshot included Git metadata")
	}
	if _, ok := tree.Lookup("pipe"); ok {
		t.Fatal("filesystem snapshot included a non-file entry")
	}
}

func TestFilesystemTreeErrors(t *testing.T) {
	if _, err := FilesystemTree(context.Background(), filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing root accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := FilesystemTree(ctx, t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled context error = %v", err)
	}
}
