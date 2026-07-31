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

// TestIndexTree captures the stage-0 index: ordinary and executable files with
// their mode preserved, symlinks and gitlinks skipped, deletions and unstaged
// files absent, and a deterministic path order. It also confirms the Tree owns
// its bytes.
func TestIndexTree(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"base.txt": "base", "gone.txt": "gone"})
	// Stage an ordinary and an executable file, an untracked-but-unstaged file
	// (must stay absent), a symlink (gitlink-free, no regular content), and a
	// staged deletion of gone.txt.
	gitfixture.StageFile(t, repo, "ordinary.txt", "ordinary\n", 0o644)
	gitfixture.StageFile(t, repo, "sub/exec.sh", "exec\n", 0o755)
	if err := os.WriteFile(filepath.Join(dir, "unstaged.txt"), []byte("nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("ordinary.txt", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	gitfixture.Add(t, repo, "link")
	gitfixture.StageRemoval(t, repo, "gone.txt")
	// A gitlink (submodule) index entry has no regular content.
	gitfixture.StageGitlink(t, repo, "submodule")

	tree, err := snapshot.IndexTree(testContext(t), snapshotRepo(t, dir))
	if err != nil {
		t.Fatalf("IndexTree: %v", err)
	}
	got := tree.List()
	want := []struct {
		path string
		mode snapshot.Mode
		body string
	}{
		{"base.txt", snapshot.Regular, "base"},
		{"link", snapshot.Symlink, "ordinary.txt"},
		{"ordinary.txt", snapshot.Regular, "ordinary\n"},
		{"sub/exec.sh", snapshot.Executable, "exec\n"},
	}
	if len(got) != len(want) {
		t.Fatalf("IndexTree returned %d files, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Path != w.path || got[i].Mode != w.mode || string(got[i].Bytes) != w.body {
			t.Errorf("file %d = {%q, %d, %q}, want {%q, %d, %q}", i, got[i].Path, got[i].Mode, got[i].Bytes, w.path, w.mode, w.body)
		}
	}
	// The Tree owns its bytes: mutating a List result cannot change a re-read.
	got[0].Bytes[0] = 'X'
	if again := tree.List(); string(again[0].Bytes) != "base" {
		t.Errorf("List result aliases the Tree: %q", again[0].Bytes)
	}
	if _, ok := tree.Lookup("unstaged.txt"); ok {
		t.Errorf("unstaged file leaked into the index Tree")
	}
	if _, ok := tree.Lookup("gone.txt"); ok {
		t.Errorf("staged deletion left gone.txt in the index Tree")
	}
}

// TestIndexTreeUnmerged rejects an index with a conflicted entry.
func TestIndexTreeUnmerged(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"a.txt": "a"})
	gitfixture.StageUnmerged(t, repo, "conflict.md")
	if _, err := snapshot.IndexTree(testContext(t), snapshotRepo(t, dir)); !errors.Is(err, awfgit.ErrIndexUnmerged) {
		t.Fatalf("unmerged index: got %v, want ErrIndexUnmerged", err)
	}
}

// TestIndexTreeOutsideRepo wraps git.IndexBlobs' open-repo failure.
func TestIndexTreeOutsideRepo(t *testing.T) {
	t.Parallel()
	if _, err := awfgit.Open(t.TempDir()); err == nil || !errors.Is(err, awfgit.ErrNotARepository) {
		t.Fatalf("outside repository: got %v", err)
	}
}

// TestIndexTreeLinkedWorktree resolves a linked worktree's own index, proving
// the OpenRepo-based reader works from a `git worktree add` root.
func TestIndexTreeLinkedWorktree(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	mainDir := repo.Root()
	head := gitfixture.Commit(t, repo, "base", map[string]string{"a.txt": "a"})
	wtRoot := t.TempDir()
	gitdir := filepath.Join(mainDir, ".git", "worktrees", "wt")
	if err := os.MkdirAll(gitdir, 0o755); err != nil {
		t.Fatal(err)
	}
	idx, err := os.ReadFile(filepath.Join(mainDir, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string][]byte{
		filepath.Join(wtRoot, ".git"):      []byte("gitdir: " + gitdir + "\n"),
		filepath.Join(gitdir, "commondir"): []byte(filepath.Join(mainDir, ".git") + "\n"),
		filepath.Join(gitdir, "gitdir"):    []byte(filepath.Join(wtRoot, ".git") + "\n"),
		filepath.Join(gitdir, "HEAD"):      []byte(head + "\n"),
		filepath.Join(gitdir, "index"):     idx,
	} {
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tree, err := snapshot.IndexTree(testContext(t), snapshotRepo(t, wtRoot))
	if err != nil {
		t.Fatalf("IndexTree from linked worktree: %v", err)
	}
	if f, ok := tree.Lookup("a.txt"); !ok || string(f.Bytes) != "a" {
		t.Fatalf("linked worktree index missing a.txt: %q, %v", f.Bytes, ok)
	}
}
