package snapshot_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	gogit "github.com/go-git/go-git/v5"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// TestCommitTree captures a committed tree: regular and executable files with
// their mode preserved, symlinks skipped, deterministic path order, and byte
// ownership. It reads only committed content, never the mutated working tree.
func TestCommitTree(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	writeStage(t, wt, dir, "b.txt", "bee\n", 0o644)
	writeStage(t, wt, dir, "a/exec.sh", "run\n", 0o755)
	if err := os.Symlink("b.txt", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add("link"); err != nil {
		t.Fatal(err)
	}
	head, err := wt.Commit("c", &gogit.CommitOptions{Author: gitfixture.Sig, Committer: gitfixture.Sig})
	if err != nil {
		t.Fatal(err)
	}
	// Mutate the working tree after committing: CommitTree must ignore it.
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("EDITED"), 0o644); err != nil {
		t.Fatal(err)
	}

	tree, err := snapshot.CommitTree(dir, head.String())
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
	if _, err := snapshot.CommitTree(t.TempDir(), "HEAD"); err == nil {
		t.Fatal("expected an error outside a repository")
	}
}

// TestCommitTreeBadRevision wraps the revision-resolution failure.
func TestCommitTreeBadRevision(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"a.txt": "a"})
	if _, err := snapshot.CommitTree(dir, "does-not-exist"); err == nil {
		t.Fatal("expected an error for an unresolvable revision")
	} else if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected error kind: %v", err)
	}
}
