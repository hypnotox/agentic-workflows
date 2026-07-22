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
	writeStage(t, wt, dir, "recreated.txt", "old\n", 0o644)
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
	if _, err := wt.Remove("recreated.txt"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "recreated.txt"), []byte("new\n"), 0o644); err != nil {
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
	if f, ok := tree.Lookup("recreated.txt"); !ok || string(f.Bytes) != "new\n" {
		t.Errorf("recreated.txt = %q, %v; want recreated working bytes", f.Bytes, ok)
	}
	for _, absent := range []string{"gone.txt", "ignored.txt"} {
		if _, ok := tree.Lookup(absent); ok {
			t.Errorf("%s should be absent from the working Tree", absent)
		}
	}
	if f, ok := tree.Lookup("link"); !ok || f.Mode != snapshot.Symlink || string(f.Bytes) != "tracked.txt" || f.Scannable() {
		t.Errorf("link = %#v, %v; want inert symlink target", f, ok)
	}
	if f, _ := tree.Lookup("tracked.txt"); len(f.Bytes) > 0 {
		f.Bytes[0] = 'X'
		if again, _ := tree.Lookup("tracked.txt"); again.Bytes[0] == 'X' {
			t.Errorf("Lookup result aliases the Tree")
		}
	}
	if err := os.Chmod(filepath.Join(dir, "tracked.txt"), 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(filepath.Join(dir, "tracked.txt"), 0o644); err != nil {
			t.Errorf("restore tracked.txt permissions: %v", err)
		}
	})
	if _, err := snapshot.WorkingTree(dir); err == nil {
		t.Fatal("expected unreadable working file to fail the snapshot")
	}
}

func TestWorkingTreeExcludesIgnoredMetricsDescendants(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	writeStage(t, wt, dir, ".awf/metrics/.gitignore", "*\n!.gitignore\n", 0o644)
	if _, err := wt.Commit("metrics ignore", &gogit.CommitOptions{Author: gitfixture.Sig, Committer: gitfixture.Sig}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".awf", "metrics", "efforts", "e", "sessions", "s.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("resident\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tree, err := snapshot.WorkingTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := tree.Lookup(".awf/metrics/.gitignore"); !ok {
		t.Fatal("governed metrics ignore missing from snapshot")
	}
	if _, ok := tree.Lookup(".awf/metrics/efforts/e/sessions/s.jsonl"); ok {
		t.Fatal("ignored resident metrics descendant entered snapshot")
	}
}

// invariant: tooling/cli:init-unborn-head-supported
func TestWorkingTreeUnborn(t *testing.T) {
	_, dir := gitfixture.InitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "eligible.txt"), []byte("working\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tree, err := snapshot.WorkingTree(dir)
	if err != nil {
		t.Fatalf("WorkingTree on unborn HEAD: %v", err)
	}
	if file, ok := tree.Lookup("eligible.txt"); !ok || string(file.Bytes) != "working\n" {
		t.Fatalf("eligible.txt = %q, %v; want unborn worktree bytes", file.Bytes, ok)
	}

	t.Run("outside-repository", func(t *testing.T) {
		if _, err := snapshot.WorkingTree(t.TempDir()); err == nil {
			t.Fatal("expected an error outside a repository")
		}
	})

	t.Run("corrupt-reference", func(t *testing.T) {
		_, dir := gitfixture.InitRepo(t)
		if err := os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("not a reference\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := snapshot.WorkingTree(dir); err == nil {
			t.Fatal("corrupt HEAD accepted as unborn")
		}
	})

	t.Run("dangling-reference", func(t *testing.T) {
		_, dir := gitfixture.InitRepo(t)
		if err := os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("0123456789012345678901234567890123456789\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := snapshot.WorkingTree(dir); err == nil {
			t.Fatal("dangling HEAD accepted as unborn")
		}
	})

	t.Run("missing-object", func(t *testing.T) {
		repo, dir := gitfixture.InitRepo(t)
		head := gitfixture.Commit(t, repo, dir, "base", map[string]string{"tracked.txt": "tracked\n"})
		commit, err := repo.CommitObject(head)
		if err != nil {
			t.Fatal(err)
		}
		treeObject := filepath.Join(dir, ".git", "objects", commit.TreeHash.String()[:2], commit.TreeHash.String()[2:])
		if err := os.Remove(treeObject); err != nil {
			t.Fatal(err)
		}
		if _, err := snapshot.WorkingTree(dir); err == nil {
			t.Fatal("missing committed tree object accepted as unborn")
		}
	})
}
