package git_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gogit "github.com/go-git/go-git/v5"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func TestMain(m *testing.M) { os.Exit(testsupport.RunIsolated(m, "awf-git-test-home")) }

func TestChangedPathsRange(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "one", map[string]string{"a.txt": "a"})
	// Modify a.txt (From.Name is set) and add b.txt (From.Name empty) so both
	// sides of the change are exercised.
	gitfixture.Commit(t, repo, dir, "two", map[string]string{"a.txt": "aa", "b.txt": "b"})

	got, err := awfgit.ChangedPaths(dir, false, "HEAD~1..HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "a.txt,b.txt" {
		t.Errorf("range: got %v want [a.txt b.txt]", got)
	}
}

func TestChangedPathsStaged(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"a.txt": "a"})

	// Stage a new file without committing; leave a second file untracked.
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("s"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add("staged.txt"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("u"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := awfgit.ChangedPaths(dir, true, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "staged.txt" {
		t.Errorf("staged: got %v want [staged.txt] (untracked excluded)", got)
	}
}

func TestChangedPathsNothingStaged(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"a.txt": "a"})
	got, err := awfgit.ChangedPaths(dir, true, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("clean tree: got %v want none", got)
	}
}

func TestChangedPathsErrors(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"a.txt": "a"})

	if _, err := awfgit.ChangedPaths(dir, false, "no-separator"); err == nil {
		t.Error("expected a malformed-range error")
	}
	if _, err := awfgit.ChangedPaths(dir, false, "does-not-exist..HEAD"); err == nil {
		t.Error("expected an unresolvable-revision error (from side)")
	}
	if _, err := awfgit.ChangedPaths(dir, false, "HEAD..does-not-exist"); err == nil {
		t.Error("expected an unresolvable-revision error (to side)")
	}
	if _, err := awfgit.ChangedPaths(t.TempDir(), false, "a..b"); err == nil {
		t.Error("expected an open-repo error outside a repository")
	}
}

// OpenRepo resolves a normal repository and reports the canonical
// not-a-repository error outside one.
func TestOpenRepo(t *testing.T) {
	_, dir := gitfixture.InitRepo(t)
	if _, err := awfgit.OpenRepo(dir); err != nil {
		t.Fatalf("open a fresh repo: %v", err)
	}
	if _, err := awfgit.OpenRepo(t.TempDir()); !errors.Is(err, gogit.ErrRepositoryNotExists) {
		t.Errorf("non-repo: got %v want ErrRepositoryNotExists", err)
	}
}

// A syntactically invalid .git/config (not merely a missing one, which the
// storage tolerates) makes the underlying storer's Config() fail, which
// noExtensionsStorer.Config must propagate rather than swallow.
func TestOpenRepoMalformedConfig(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"go.mod": "module x\n"})
	if err := os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("[core\nbroken = = =\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := awfgit.OpenRepo(dir)
	if err == nil {
		_, err = r.Config()
	}
	if err == nil {
		t.Fatal("expected a malformed .git/config error to propagate")
	}
}

// linkedWorktree hand-crafts the on-disk layout `git worktree add` produces for
// repo rooted at mainDir: a worktree-private gitdir under .git/worktrees/<name>
// holding HEAD/commondir/gitdir plus a copy of the index, and a `gitdir:`
// pointer file at the new root. go-git cannot create linked worktrees, so the
// fixture writes exactly the files git itself would.
func linkedWorktree(t *testing.T, mainDir, name, head, commondir string) string {
	t.Helper()
	wtRoot := t.TempDir()
	gitdir := filepath.Join(mainDir, ".git", "worktrees", name)
	if err := os.MkdirAll(gitdir, 0o755); err != nil {
		t.Fatalf("mkdir gitdir: %v", err)
	}
	idx, err := os.ReadFile(filepath.Join(mainDir, ".git", "index"))
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	for path, content := range map[string][]byte{
		filepath.Join(wtRoot, ".git"):      []byte("gitdir: " + gitdir + "\n"),
		filepath.Join(gitdir, "commondir"): []byte(commondir + "\n"),
		filepath.Join(gitdir, "gitdir"):    []byte(filepath.Join(wtRoot, ".git") + "\n"),
		filepath.Join(gitdir, "HEAD"):      []byte(head + "\n"),
		filepath.Join(gitdir, "index"):     idx,
	} {
		if werr := os.WriteFile(path, content, 0o644); werr != nil {
			t.Fatalf("write %s: %v", path, werr)
		}
	}
	return wtRoot
}

// OpenRepo must resolve a linked worktree root, where .git is a `gitdir:`
// pointer file rather than a directory (both commondir spellings and both HEAD
// forms git may write), and a relative pointer to a self-contained gitdir
// without a commondir (the submodule layout).
func TestOpenRepoGitfileLayouts(t *testing.T) {
	repo, mainDir := gitfixture.InitRepo(t)
	head := gitfixture.Commit(t, repo, mainDir, "base", map[string]string{"go.mod": "module x\n"})

	for name, tc := range map[string]struct{ head, commondir string }{
		"relative-commondir-symbolic-head": {"ref: refs/heads/master", "../.."},
		"absolute-commondir-detached-head": {head.String(), filepath.Join(mainDir, ".git")},
	} {
		t.Run(name, func(t *testing.T) {
			wtRoot := linkedWorktree(t, mainDir, name, tc.head, tc.commondir)
			r, err := awfgit.OpenRepo(wtRoot)
			if err != nil {
				t.Fatalf("open linked worktree: %v", err)
			}
			if _, err := r.Head(); err != nil {
				t.Fatalf("resolve HEAD in linked worktree: %v", err)
			}
		})
	}

	t.Run("relative-gitfile-without-commondir", func(t *testing.T) {
		sub, dir := gitfixture.InitRepo(t)
		gitfixture.Commit(t, sub, dir, "x", map[string]string{"a.txt": "a"})
		if err := os.Rename(filepath.Join(dir, ".git"), filepath.Join(dir, ".realgit")); err != nil {
			t.Fatalf("rename: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: .realgit\n"), 0o644); err != nil {
			t.Fatalf("write pointer: %v", err)
		}
		if _, err := awfgit.OpenRepo(dir); err != nil {
			t.Fatalf("open via relative gitdir pointer: %v", err)
		}
	})
}

// A .git file that is not a gitdir pointer is a hard, named error; an unreadable
// pointer file propagates its read error rather than silently falling through.
func TestOpenRepoMalformedGitfile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("not a pointer\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := awfgit.OpenRepo(dir); err == nil || !strings.Contains(err.Error(), "gitdir:") {
		t.Fatalf("want a gitdir-pointer parse error, got: %v", err)
	}

	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions")
	}
	unreadable := t.TempDir()
	if err := os.WriteFile(filepath.Join(unreadable, ".git"), []byte("gitdir: nowhere\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := awfgit.OpenRepo(unreadable); err == nil {
		t.Error("expected a read error on an unreadable .git pointer file")
	}
}
