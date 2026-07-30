package git_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestMergeInProgressPrimaryCheckout(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".git", "HEAD"), "ref: refs/heads/main\n")

	got, err := git.MergeInProgress(root)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Error("a checkout with no MERGE_HEAD must report no merge")
	}

	testsupport.WriteFile(t, filepath.Join(root, ".git", "MERGE_HEAD"), "deadbeef\n")
	got, err = git.MergeInProgress(root)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("MERGE_HEAD present must report a merge in progress")
	}
}

// A project root below the checkout root resolves upward, which is the shape an
// adopter tree nested in a monorepo takes.
func TestMergeInProgressFromNestedRoot(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".git", "MERGE_HEAD"), "deadbeef\n")
	nested := filepath.Join(root, "projects", "adopter")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := git.MergeInProgress(nested)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("a nested project root must see the containing checkout's merge state")
	}
}

// MERGE_HEAD is worktree-private, so a linked worktree's `.git` pointer must
// resolve to that worktree's own gitdir and not to the shared common dir.
func TestMergeInProgressLinkedWorktree(t *testing.T) {
	root := t.TempDir()
	common := filepath.Join(root, "repo", ".git")
	private := filepath.Join(common, "worktrees", "wt")
	testsupport.WriteFile(t, filepath.Join(common, "HEAD"), "ref: refs/heads/main\n")
	testsupport.WriteFile(t, filepath.Join(private, "HEAD"), "ref: refs/heads/wt\n")

	linked := filepath.Join(root, "wt")
	testsupport.WriteFile(t, filepath.Join(linked, ".git"), "gitdir: "+private+"\n")

	got, err := git.MergeInProgress(linked)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Error("no MERGE_HEAD anywhere must report no merge")
	}

	// A merge in the COMMON dir must not be seen from the linked worktree.
	testsupport.WriteFile(t, filepath.Join(common, "MERGE_HEAD"), "deadbeef\n")
	got, err = git.MergeInProgress(linked)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Error("MERGE_HEAD in the common dir is another worktree's merge, not this one's")
	}

	testsupport.WriteFile(t, filepath.Join(private, "MERGE_HEAD"), "deadbeef\n")
	got, err = git.MergeInProgress(linked)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("MERGE_HEAD in the worktree's own gitdir must report a merge")
	}
}

func TestMergeInProgressNoCheckout(t *testing.T) {
	if _, err := git.MergeInProgress(t.TempDir()); err == nil {
		t.Fatal("a path in no checkout must report an error, not a bare false")
	}
}

func TestMergeInProgressMalformedPointer(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".git"), "not a gitdir pointer\n")
	if _, err := git.MergeInProgress(root); err == nil {
		t.Fatal("a malformed .git pointer must surface, not be masked by walking upward")
	}
}
