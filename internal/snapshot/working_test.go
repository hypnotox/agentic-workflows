package snapshot_test

import (
	"os"
	"path/filepath"
	"testing"

	gogit "github.com/go-git/go-git/v5"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// TestWorkingTree captures the working universe: tracked-and-present regular
// files (reading current working bytes), an executable's mode, and nonignored
// untracked files; it skips symlinks (not followed), a staged deletion, and
// ignored files. It also confirms byte ownership.
func TestWorkingTree(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	// Committed set: a regular file, an executable, an ignored pattern, a
	// to-be-deleted file, and .gitignore itself.
	writeStage(t, wt, dir, "tracked.txt", "orig\n", 0o644)
	writeStage(t, wt, dir, "run.sh", "run\n", 0o755)
	writeStage(t, wt, dir, "gone.txt", "gone\n", 0o644)
	writeStage(t, wt, dir, ".gitignore", "ignored.txt\n", 0o644)
	if _, err := wt.Commit("base", &gogit.CommitOptions{Author: gitfixture.Sig, Committer: gitfixture.Sig}); err != nil {
		t.Fatal(err)
	}
	// Working-tree mutations after the commit.
	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("fresh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("tracked.txt", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "gone.txt")); err != nil {
		t.Fatal(err)
	}

	tree, err := snapshot.WorkingTree(dir)
	if err != nil {
		t.Fatalf("WorkingTree: %v", err)
	}
	if f, ok := tree.Lookup("tracked.txt"); !ok || string(f.Bytes) != "edited\n" || f.Mode != snapshot.Regular {
		t.Errorf("tracked.txt = %q, %d, %v; want current working bytes", f.Bytes, f.Mode, ok)
	}
	if f, ok := tree.Lookup("run.sh"); !ok || f.Mode != snapshot.Executable {
		t.Errorf("run.sh mode = %d, %v; want Executable", f.Mode, ok)
	}
	if f, ok := tree.Lookup("untracked.txt"); !ok || string(f.Bytes) != "fresh\n" {
		t.Errorf("untracked.txt = %q, %v; want included", f.Bytes, ok)
	}
	for _, absent := range []string{"gone.txt", "ignored.txt", "link"} {
		if _, ok := tree.Lookup(absent); ok {
			t.Errorf("%s should be absent from the working Tree", absent)
		}
	}
	if f, _ := tree.Lookup("tracked.txt"); len(f.Bytes) > 0 {
		f.Bytes[0] = 'X'
		if again, _ := tree.Lookup("tracked.txt"); again.Bytes[0] == 'X' {
			t.Errorf("Lookup result aliases the Tree")
		}
	}
}

// TestWorkingTreeOutsideRepo wraps git.WorkingPaths' open-repo failure.
func TestWorkingTreeOutsideRepo(t *testing.T) {
	if _, err := snapshot.WorkingTree(t.TempDir()); err == nil {
		t.Fatal("expected an error outside a repository")
	}
}
