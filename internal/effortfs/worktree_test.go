package effortfs

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAddWorktreeCreatesFlatSiblingCheckouts(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	root := t.TempDir()
	runGit(t, root, "init", "--quiet")
	runGit(t, root, "-c", "user.name=awf", "-c", "user.email=awf@example.invalid", "commit", "--quiet", "--allow-empty", "--message", "initial")
	memoryRelative, err := New(root, "ship-it")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	worktreesRoot := filepath.Join(root, ".awf", "worktrees")
	for _, checkout := range []struct {
		suffix string
		name   string
		branch string
	}{
		{suffix: "", name: "ship-it.default", branch: "awf/ship-it/default"},
		{suffix: "tests", name: "ship-it.tests", branch: "awf/ship-it/tests"},
	} {
		worktree, err := AddWorktree(root, "ship-it", checkout.suffix)
		if err != nil {
			t.Fatalf("AddWorktree(%q) error = %v", checkout.suffix, err)
		}
		if want := filepath.Join(worktreesRoot, checkout.name); worktree.Path != want {
			t.Fatalf("AddWorktree(%q) path = %q, want %q", checkout.suffix, worktree.Path, want)
		}
		if worktree.Branch != checkout.branch {
			t.Fatalf("AddWorktree(%q) branch = %q, want %q", checkout.suffix, worktree.Branch, checkout.branch)
		}
		if want := filepath.Join(root, memoryRelative); worktree.MemoryPath != want {
			t.Fatalf("AddWorktree(%q) memory = %q, want %q", checkout.suffix, worktree.MemoryPath, want)
		}
		if _, err := os.Lstat(filepath.Join(worktree.Path, ".git")); err != nil {
			t.Fatalf("AddWorktree(%q) checkout: %v", checkout.suffix, err)
		}

		if _, err := AddWorktree(root, "ship-it", checkout.suffix); err == nil {
			t.Fatalf("repeated AddWorktree(%q) error = nil, want collision", checkout.suffix)
		}
	}

	// Native removal must leave nothing behind for an effort.
	for _, name := range []string{"ship-it.default", "ship-it.tests"} {
		runGit(t, root, "worktree", "remove", filepath.Join(worktreesRoot, name))
	}
	entries, err := os.ReadDir(worktreesRoot)
	if err != nil {
		t.Fatalf("read worktrees directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("worktrees directory after removal has %d entries, want none", len(entries))
	}
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	if _, err := git(root, args...); err != nil {
		t.Fatal(err)
	}
}
