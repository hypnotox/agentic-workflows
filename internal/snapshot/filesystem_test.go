package snapshot

import (
	"context"
	"errors"
	"io/fs"
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
	if err := os.MkdirAll(filepath.Join(root, "nested-dir", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "nested-file"), 0o755); err != nil {
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
	if err := os.WriteFile(filepath.Join(root, "nested-dir", "hidden"), []byte("nested"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested-file", ".git"), []byte("gitdir: elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested-file", "hidden"), []byte("nested"), 0o644); err != nil {
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
	for _, hidden := range []string{".git/hidden", "nested-dir/hidden", "nested-file/hidden"} {
		if _, ok := tree.Lookup(hidden); ok {
			t.Fatalf("filesystem snapshot included checkout content %s", hidden)
		}
	}
	if _, ok := tree.Lookup("pipe"); ok {
		t.Fatal("filesystem snapshot included a non-file entry")
	}
}

func TestFilesystemTreeErrors(t *testing.T) {
	boom := errors.New("boom")
	if _, err := FilesystemTree(context.Background(), filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing root accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := FilesystemTree(ctx, t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled context error = %v", err)
	}

	fileRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(fileRoot, "file"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	symlinkRoot := t.TempDir()
	if err := os.Symlink("target", filepath.Join(symlinkRoot, "link")); err != nil {
		t.Fatal(err)
	}
	dirRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(dirRoot, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		root string
		edit func(*filesystemOps)
	}{
		{"walk callback", fileRoot, func(ops *filesystemOps) {
			ops.walkDir = func(_ string, fn fs.WalkDirFunc) error { return fn(fileRoot, nil, boom) }
		}},
		{"relative path", fileRoot, func(ops *filesystemOps) { ops.rel = func(string, string) (string, error) { return "", boom } }},
		{"directory marker", dirRoot, func(ops *filesystemOps) { ops.lstat = func(string) (os.FileInfo, error) { return nil, boom } }},
		{"entry info", fileRoot, func(ops *filesystemOps) { ops.info = func(os.DirEntry) (os.FileInfo, error) { return nil, boom } }},
		{"symlink read", symlinkRoot, func(ops *filesystemOps) { ops.readlink = func(string) (string, error) { return "", boom } }},
		{"file read", fileRoot, func(ops *filesystemOps) { ops.readFile = func(string) ([]byte, error) { return nil, boom } }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ops := osFilesystemOps()
			tc.edit(&ops)
			if _, err := filesystemTree(context.Background(), tc.root, ops); !errors.Is(err, boom) {
				t.Fatalf("error = %v, want injected failure", err)
			}
		})
	}
}
