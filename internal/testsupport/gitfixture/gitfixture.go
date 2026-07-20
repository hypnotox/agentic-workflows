// Package gitfixture provides go-git-backed test fixtures (a fixed commit
// signature, a fresh repo, and a write+commit helper) for awf's test suites
// that need a real git repository. It is kept separate from
// internal/testsupport so a caller that only needs e.g.
// testsupport.WriteFile does not have to pull go-git into its test binary
// (ADR-0044).
package gitfixture

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Sig is the fixed commit signature used by InitRepo/Commit fixtures across
// awf's test suites.
var Sig = &object.Signature{Name: "T", Email: "t@example.com", When: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}

// InitRepo creates a fresh git repository in a new t.TempDir(), returning the
// repository and its root path.
func InitRepo(t *testing.T) (*git.Repository, string) {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil { // coverage-ignore: PlainInit into a fresh empty t.TempDir() fails only on a permission fault a test cannot trigger
		t.Fatalf("init: %v", err)
	}
	return repo, dir
}

// Stage writes the given paths into repo's worktree (rooted at dir, creating any
// parent directories) and adds them to the index without committing, so a test
// can exercise a staged-but-uncommitted index universe distinct from the working
// tree.
func Stage(t *testing.T, repo *git.Repository, dir string, write map[string]string) {
	t.Helper()
	wt, err := repo.Worktree()
	if err != nil { // coverage-ignore: Worktree() on a just-initialized non-bare repo cannot fail
		t.Fatalf("worktree: %v", err)
	}
	for name, content := range write {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil { // coverage-ignore: MkdirAll under a fresh t.TempDir() fails only on a permission fault a test cannot trigger
			t.Fatalf("mkdir %s: %v", name, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil { // coverage-ignore: writing into the repo's own worktree dir fails only on a permission fault a test cannot trigger
			t.Fatalf("write %s: %v", name, err)
		}
		if _, err := wt.Add(name); err != nil { // coverage-ignore: Add of a path just written above cannot fail
			t.Fatalf("add %s: %v", name, err)
		}
	}
}

// Merge creates a merge commit whose tree is the current index, with the given
// parents in order (the first is the first parent), so a fixture can exercise
// first-parent range semantics. It allows an empty commit, so a merge can
// integrate a branch whose tree already matches HEAD.
func Merge(t *testing.T, repo *git.Repository, msg string, parents ...plumbing.Hash) plumbing.Hash {
	t.Helper()
	wt, err := repo.Worktree()
	if err != nil { // coverage-ignore: Worktree() on a just-initialized non-bare repo cannot fail
		t.Fatalf("worktree: %v", err)
	}
	h, err := wt.Commit(msg, &git.CommitOptions{Parents: parents, AllowEmptyCommits: true, Author: Sig, Committer: Sig})
	if err != nil { // coverage-ignore: Commit with valid parents and signature on a healthy worktree cannot fail
		t.Fatalf("merge commit: %v", err)
	}
	return h
}

// Commit writes/removes the given paths in repo's worktree (rooted at dir),
// stages them, and commits with Sig, returning the commit hash.
func Commit(t *testing.T, repo *git.Repository, dir, msg string, write map[string]string, remove ...string) plumbing.Hash {
	t.Helper()
	wt, err := repo.Worktree()
	if err != nil { // coverage-ignore: Worktree() on a just-initialized non-bare repo cannot fail
		t.Fatalf("worktree: %v", err)
	}
	for name, content := range write {
		if werr := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); werr != nil { // coverage-ignore: writing into the repo's own worktree dir fails only on a permission fault a test cannot trigger
			t.Fatalf("write %s: %v", name, werr)
		}
		if _, aerr := wt.Add(name); aerr != nil { // coverage-ignore: Add of a path just written above cannot fail
			t.Fatalf("add %s: %v", name, aerr)
		}
	}
	for _, name := range remove {
		if _, rerr := wt.Remove(name); rerr != nil { // coverage-ignore: Remove of a path the caller asserts is tracked cannot fail in a test fixture
			t.Fatalf("remove %s: %v", name, rerr)
		}
	}
	h, err := wt.Commit(msg, &git.CommitOptions{Author: Sig, Committer: Sig})
	if err != nil { // coverage-ignore: Commit with a valid signature and staged tree cannot fail
		t.Fatalf("commit: %v", err)
	}
	return h
}
