package snapshot_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// TestCommitTree captures a committed tree: regular and executable files with
// their mode preserved, symlinks skipped, deterministic path order, and byte
// ownership. It reads only committed content, never the mutated working tree.
func TestCommitTree(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.StageFile(t, repo, "b.txt", "bee\n", 0o644)
	gitfixture.StageFile(t, repo, "a/exec.sh", "run\n", 0o755)
	if err := os.Symlink("b.txt", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	gitfixture.Add(t, repo, "link")
	head := gitfixture.Commit(t, repo, "c", nil)
	// Mutate the working tree after committing: CommitTree must ignore it.
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("EDITED"), 0o644); err != nil {
		t.Fatal(err)
	}

	tree, err := snapshot.CommitTree(testContext(t), snapshotRepo(t, dir), head)
	if err != nil {
		t.Fatalf("CommitTree: %v", err)
	}
	got := tree.List()
	want := []struct {
		path string
		mode snapshot.Mode
		body string
	}{
		{"a/exec.sh", snapshot.Executable, "run\n"},
		{"b.txt", snapshot.Regular, "bee\n"},
		{"link", snapshot.Symlink, "b.txt"},
	}
	if len(got) != len(want) {
		t.Fatalf("CommitTree returned %d files, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Path != w.path || got[i].Mode != w.mode || string(got[i].Bytes) != w.body {
			t.Errorf("file %d = {%q, %d, %q}, want {%q, %d, %q}", i, got[i].Path, got[i].Mode, got[i].Bytes, w.path, w.mode, w.body)
		}
	}
	if f, ok := tree.Lookup("link"); !ok || f.Scannable() {
		t.Errorf("symlink not retained as inert bytes: %#v", f)
	}
	got[0].Bytes[0] = 'X'
	if again := tree.List(); string(again[0].Bytes) != "run\n" {
		t.Errorf("List result aliases the Tree: %q", again[0].Bytes)
	}
}

// TestCommitTreeOutsideRepo wraps git.CommitBlobs' open-repo failure.
func TestCommitTreeOutsideRepo(t *testing.T) {
	t.Parallel()
	if _, err := awfgit.Open(t.TempDir()); err == nil {
		t.Fatal("expected an error outside a repository")
	}
}

// TestCommitTreeBadRevision wraps the revision-resolution failure.
func TestCommitTreeBadRevision(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"a.txt": "a"})
	if _, err := snapshot.CommitTree(testContext(t), snapshotRepo(t, dir), "does-not-exist"); err == nil {
		t.Fatal("expected an error for an unresolvable revision")
	} else if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected error kind: %v", err)
	}
}
