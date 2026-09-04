package git

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func TestVisibleWorkingStatusDropsOnlyIgnoredUntrackedPaths(t *testing.T) {
	matcher := gitignore.NewMatcher([]gitignore.Pattern{gitignore.ParsePattern(".awf/worktrees/", nil)})
	status := gogit.Status{
		".awf/worktrees/managed/.gitignore": {Worktree: gogit.Untracked},
		"tracked.txt":                       {Worktree: gogit.Modified},
		"untracked.txt":                     {Worktree: gogit.Untracked},
	}
	visible := visibleWorkingStatus(status, matcher)
	if _, ok := visible[".awf/worktrees/managed/.gitignore"]; ok {
		t.Fatal("ignored untracked descendant remained visible")
	}
	if _, ok := visible["tracked.txt"]; !ok {
		t.Fatal("tracked modification was suppressed")
	}
	if _, ok := visible["untracked.txt"]; !ok {
		t.Fatal("ordinary untracked path was suppressed")
	}
}

func TestParseWorktreeStatus(t *testing.T) {
	tracked, untracked, err := parseWorktreeStatus([]byte("\x00? loose.txt\x001 tracked.txt\x00u conflicted.txt\x002 renamed.txt\x00old.txt\x00"))
	if err != nil {
		t.Fatal(err)
	}
	if tracked != 3 || untracked != 1 {
		t.Fatalf("counts = (%d, %d), want (3, 1)", tracked, untracked)
	}
}

func TestParseWorktreeStatusRejectsMalformedRecords(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "unterminated", body: "? loose.txt"},
		{name: "rename missing original", body: "2 renamed.txt\x00"},
		{name: "unknown type", body: "! ignored.txt\x00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := parseWorktreeStatus([]byte(tc.body)); err == nil {
				t.Fatal("parseWorktreeStatus unexpectedly succeeded")
			}
		})
	}
}

func TestWorktreeChangeCountsDoesNotRefreshIndex(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"tracked.txt": "tracked"})
	indexPath := filepath.Join(dir, ".git", "index")
	before, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "tracked.txt"), future, future); err != nil {
		t.Fatal(err)
	}

	tracked, untracked, err := statusRepo(t, dir).ChangeCounts(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if tracked != 0 || untracked != 0 {
		t.Fatalf("counts after metadata-only change = (%d, %d), want (0, 0)", tracked, untracked)
	}
	after, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("WorktreeChangeCounts refreshed the Git index")
	}
}

// TestChangeCountsHonorsGlobalExcludes pins the native cleanliness oracle to
// Git's real ignore universe. The runner isolates all other ambient state, but
// replays this one effective setting so an ignored global file is not reported
// as untracked dirt.
func TestChangeCountsHonorsGlobalExcludes(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	excludes := filepath.Join(home, "global-ignore")
	if err := os.WriteFile(excludes, []byte("globally-ignored.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitconfig := filepath.Join(home, ".gitconfig")
	if err := os.WriteFile(gitconfig, []byte("[core]\n\texcludesfile = "+excludes+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Remove(gitconfig)
		_ = os.Remove(excludes)
	})
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"tracked.txt": "tracked"})
	if err := os.WriteFile(filepath.Join(dir, "globally-ignored.txt"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kept.txt"), []byte("kept"), 0o644); err != nil {
		t.Fatal(err)
	}
	tracked, untracked, err := statusRepo(t, dir).ChangeCounts(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if tracked != 0 || untracked != 1 {
		t.Fatalf("counts with global excludesfile = (%d, %d), want (0, 1)", tracked, untracked)
	}
}

// invariant: tooling/git-access:single-cleanliness-oracle (TestChangeCountsAndIgnoredPathsUseGitIgnoreSemantics)
func TestChangeCountsAndIgnoredPathsUseGitIgnoreSemantics(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{".gitignore": "ignored.txt\n", "tracked.txt": "tracked"})
	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "visible.txt"), []byte("visible"), 0o644); err != nil {
		t.Fatal(err)
	}
	handle := statusRepo(t, dir)
	tracked, untracked, err := handle.ChangeCounts(testContext(t))
	if err != nil || tracked != 0 || untracked != 1 {
		t.Fatalf("counts=(%d,%d), %v", tracked, untracked, err)
	}
	ignored, err := handle.IgnoredPaths(testContext(t))
	if err != nil || len(ignored) != 1 || ignored[0] != "ignored.txt" {
		t.Fatalf("ignored paths=%#v, %v", ignored, err)
	}
}

func TestWorktreeChangeCountsRejectsNonRepository(t *testing.T) {
	if _, err := Open(t.TempDir()); err == nil {
		t.Fatal("WorktreeChangeCounts unexpectedly succeeded outside a repository")
	}
}

// TestWorktreeChangeCountsIgnoresInheritedGitEnvironment pins that the
// cleanliness oracle reads the worktree it was asked about. Run unisolated, an
// inherited GIT_DIR selects a different repository, so the audit's clean-tree
// verdict would describe whichever repository the environment happened to name.
func TestWorktreeChangeCountsIgnoresInheritedGitEnvironment(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"tracked.txt": "tracked"})
	if err := os.WriteFile(filepath.Join(dir, "loose.txt"), []byte("loose"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A second, entirely clean repository whose .git the environment points at.
	otherRepo := gitfixture.InitRepo(t)
	other := otherRepo.Root()
	gitfixture.Commit(t, otherRepo, "base", map[string]string{"only.txt": "only"})
	t.Setenv("GIT_DIR", filepath.Join(other, ".git"))
	t.Setenv("GIT_WORK_TREE", other)

	tracked, untracked, err := statusRepo(t, dir).ChangeCounts(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if tracked != 0 || untracked != 1 {
		t.Fatalf("counts = (%d, %d), want (0, 1) from the requested worktree, not the inherited one", tracked, untracked)
	}
}
