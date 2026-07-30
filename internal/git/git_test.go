package git_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	indexformat "github.com/go-git/go-git/v5/plumbing/format/index"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func TestMain(m *testing.M) { os.Exit(testsupport.RunIsolated(m, "awf-git-test-home")) }

func TestRepoMethodsReturnPreCancelledContext(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"tracked.txt": "tracked"})
	ctx, cancel := context.WithCancel(testsupport.Context(t))
	cancel()
	for name, call := range map[string]func() error{
		"changed":  func() error { _, err := gitRepo(t, dir).ChangedPaths(ctx, true, ""); return err },
		"head":     func() error { _, err := gitRepo(t, dir).HeadExists(ctx); return err },
		"hash":     func() error { _, err := gitRepo(t, dir).HeadHash(ctx); return err },
		"branches": func() error { _, err := gitRepo(t, dir).Branches(ctx); return err },
		"working":  func() error { _, err := gitRepo(t, dir).WorkingPaths(ctx); return err },
		"index":    func() error { _, err := gitRepo(t, dir).IndexBlobs(ctx); return err },
		"commit":   func() error { _, err := gitRepo(t, dir).CommitBlobs(ctx, "HEAD"); return err },
		"range":    func() error { _, _, err := gitRepo(t, dir).RangeBlobs(ctx, "HEAD"); return err },
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, context.Canceled) {
				t.Fatalf("pre-cancelled error = %v, want context.Canceled", err)
			}
		})
	}
}

type cancelOnErrCall struct {
	context.Context
	remaining int
}

func (c *cancelOnErrCall) Err() error {
	c.remaining--
	if c.remaining <= 0 {
		return context.Canceled
	}
	return nil
}

func TestRepoMethodsObserveCancellationDuringIteration(t *testing.T) {
	fixture, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, fixture, dir, "base", map[string]string{"tracked.txt": "base"})
	gitfixture.Commit(t, fixture, dir, "changed", map[string]string{"tracked.txt": "changed"})
	gitfixture.Stage(t, fixture, dir, map[string]string{"staged.txt": "staged"})
	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("untracked"), 0o644); err != nil {
		t.Fatal(err)
	}

	assertCanceled := func(name string, errCall int, call func(context.Context) error) {
		t.Helper()
		ctx := &cancelOnErrCall{Context: testsupport.Context(t), remaining: errCall}
		if err := call(ctx); !errors.Is(err, context.Canceled) {
			t.Errorf("%s cancellation = %v, want context.Canceled", name, err)
		}
	}
	repo := gitRepo(t, dir)
	assertCanceled("staged changes", 2, func(ctx context.Context) error {
		_, err := repo.ChangedPaths(ctx, true, "")
		return err
	})
	assertCanceled("range changes", 2, func(ctx context.Context) error {
		_, err := repo.ChangedPaths(ctx, false, "HEAD~1..HEAD")
		return err
	})
	assertCanceled("branches", 2, func(ctx context.Context) error {
		_, err := repo.Branches(ctx)
		return err
	})
	assertCanceled("working tree", 2, func(ctx context.Context) error {
		_, err := repo.WorkingPaths(ctx)
		return err
	})
	assertCanceled("working status", 3, func(ctx context.Context) error {
		_, err := repo.WorkingPaths(ctx)
		return err
	})
	assertCanceled("index", 2, func(ctx context.Context) error {
		_, err := repo.IndexBlobs(ctx)
		return err
	})
	assertCanceled("commit blobs", 2, func(ctx context.Context) error {
		_, err := repo.CommitBlobs(ctx, "HEAD")
		return err
	})
}

func TestWorkingPaths(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"src/a.txt": "a", "gone.txt": "gone", ".gitignore": "ignored.txt\n"})
	if err := os.Remove(filepath.Join(dir, "gone.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	paths, err := gitRepo(t, dir).WorkingPaths(testsupport.Context(t))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(paths, ",")
	if strings.Contains(joined, "gone.txt") || strings.Contains(joined, "ignored.txt") || !strings.Contains(joined, "new.txt") || !strings.Contains(joined, "src/a.txt") {
		t.Fatalf("working paths: %v", paths)
	}
}

// TestWorktreeStatusInjectsGlobalExcludes fails if a production file calls
// go-git's Worktree Status() without referencing GlobalExcludePatterns, so a
// new status-derived path universe cannot silently revert to go-git's
// repo-local-only ignore semantics (docs/pitfalls.md: "go-git status ignores
// the global and system gitignore"; the limitation bit audit first and
// WorkingPaths second). A future staged-only status consumer that never reads
// untracked entries earns an explicit path exemption here instead.
func TestWorkingPathsExcludesNestedFileBelowIgnoredManagedWorktreeRoot(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{".gitignore": ".awf/worktrees/\n", "tracked.txt": "tracked"})
	managed := filepath.Join(dir, ".awf", "worktrees", "managed")
	cmd := exec.CommandContext(testsupport.Context(t), "git", "-C", dir, "worktree", "add", "-b", "managed", managed, "HEAD")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("add managed worktree: %v: %s", err, out)
	}
	paths, err := gitRepo(t, dir).WorkingPaths(testsupport.Context(t))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(paths, ","); got != ".gitignore,tracked.txt" {
		t.Fatalf("working paths below ignored managed root = %q, want committed paths only", got)
	}
}

func TestWorkingPathsHonorsGlobalExcludes(t *testing.T) {
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
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"tracked.txt": "tracked"})
	if err := os.WriteFile(filepath.Join(dir, "globally-ignored.txt"), []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kept.txt"), []byte("kept"), 0o644); err != nil {
		t.Fatal(err)
	}
	paths, err := gitRepo(t, dir).WorkingPaths(testsupport.Context(t))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(paths, ","); got != "kept.txt,tracked.txt" {
		t.Fatalf("working paths with global excludesfile = %q, want %q", got, "kept.txt,tracked.txt")
	}
}

func TestOpenContainingStopsAtMalformedCandidateAndHidesBackendErrors(t *testing.T) {
	root := t.TempDir()
	for _, start := range []string{root, filepath.Join(root, "nested")} {
		t.Run(filepath.Base(start), func(t *testing.T) {
			if err := os.MkdirAll(start, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".git"), []byte("not a gitdir pointer"), 0o600); err != nil {
				t.Fatal(err)
			}
			_, _, err := awfgit.OpenContaining(start)
			if err == nil || errors.Is(err, awfgit.ErrNotARepository) {
				t.Fatalf("malformed open = %v, want malformed error", err)
			}
			if errors.Is(err, gogit.ErrRepositoryNotExists) {
				t.Fatalf("malformed open leaked go-git sentinel: %v", err)
			}
		})
	}
}

func TestWorkingPathsFindsContainingMonorepo(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, "nested", ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitfixture.Commit(t, repo, dir, "base", map[string]string{
		"nested/.awf/config.yaml": "prefix: nested\n",
		"nested/tracked.txt":      "tracked",
		"outside.txt":             "outside",
	})
	if err := os.WriteFile(filepath.Join(dir, "nested", "new.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "outside-new.txt"), []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	paths, err := gitRepo(t, filepath.Join(dir, "nested")).WorkingPaths(testsupport.Context(t))
	if err != nil {
		t.Fatal(err)
	}
	want := ".awf/config.yaml,new.txt,tracked.txt"
	if got := strings.Join(paths, ","); got != want {
		t.Fatalf("nested project paths = %q, want %q", got, want)
	}
	if exists, err := gitRepo(t, filepath.Join(dir, "nested")).HeadExists(testsupport.Context(t)); err != nil || !exists {
		t.Fatalf("nested HeadExists = %v, %v", exists, err)
	}
	if hash, err := gitRepo(t, filepath.Join(dir, "nested")).HeadHash(testsupport.Context(t)); err != nil || hash == "" {
		t.Fatalf("nested HeadHash = %q, %v", hash, err)
	}
	for name, load := range map[string]func() ([]awfgit.IndexBlob, error){
		"index": func() ([]awfgit.IndexBlob, error) {
			return gitRepo(t, filepath.Join(dir, "nested")).IndexBlobs(testsupport.Context(t))
		},
		"commit": func() ([]awfgit.IndexBlob, error) {
			return gitRepo(t, filepath.Join(dir, "nested")).CommitBlobs(testsupport.Context(t), "HEAD")
		},
	} {
		blobs, err := load()
		if err != nil {
			t.Fatalf("nested %s blobs: %v", name, err)
		}
		var got []string
		for _, b := range blobs {
			got = append(got, b.Path)
		}
		if joined := strings.Join(got, ","); joined != ".awf/config.yaml,tracked.txt" {
			t.Fatalf("nested %s blobs = %q", name, joined)
		}
	}
	before, after, err := gitRepo(t, filepath.Join(dir, "nested")).RangeBlobs(testsupport.Context(t), "HEAD")
	if err != nil || before != nil || len(after) != 2 {
		t.Fatalf("nested range blobs: before=%v after=%v err=%v", before, after, err)
	}
}

func TestWorkingPathsUnborn(t *testing.T) {
	_, dir := gitfixture.InitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "eligible.txt"), []byte("working\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("ignored\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	paths, err := gitRepo(t, dir).WorkingPaths(testsupport.Context(t))
	if err != nil {
		t.Fatalf("WorkingPaths on unborn HEAD: %v", err)
	}
	if got := strings.Join(paths, ","); got != ".gitignore,eligible.txt" {
		t.Fatalf("unborn working paths = %q, want %q", got, ".gitignore,eligible.txt")
	}
}

func TestWorkingPathsUnbornErrorControls(t *testing.T) {
	t.Run("outside-repository", func(t *testing.T) {
		if _, err := awfgit.Open(t.TempDir()); err == nil {
			t.Fatal("non-repository accepted")
		}
	})

	t.Run("corrupt-head-store", func(t *testing.T) {
		_, dir := gitfixture.InitRepo(t)
		headPath := filepath.Join(dir, ".git", "HEAD")
		if err := os.Remove(headPath); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(headPath, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := awfgit.Open(dir); err == nil {
			t.Fatal("unreadable HEAD accepted as unborn")
		}
	})

	t.Run("dangling-reference", func(t *testing.T) {
		_, dir := gitfixture.InitRepo(t)
		if err := os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("0123456789012345678901234567890123456789\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := gitRepo(t, dir).WorkingPaths(testsupport.Context(t)); err == nil {
			t.Fatal("dangling HEAD accepted as unborn")
		}
	})

	t.Run("missing-commit-object", func(t *testing.T) {
		repo, dir := gitfixture.InitRepo(t)
		head := gitfixture.Commit(t, repo, dir, "base", map[string]string{"tracked.txt": "tracked\n"})
		commitObject := filepath.Join(dir, ".git", "objects", head.String()[:2], head.String()[2:])
		if err := os.Remove(commitObject); err != nil {
			t.Fatal(err)
		}
		_, err := gitRepo(t, dir).WorkingPaths(testsupport.Context(t))
		wantContext := "resolve working paths HEAD commit " + head.String() + ": "
		if err == nil || !strings.HasPrefix(err.Error(), wantContext) {
			t.Fatalf("missing commit error = %v, want prefix %q", err, wantContext)
		}
	})

	t.Run("missing-tree-object", func(t *testing.T) {
		repo, dir := gitfixture.InitRepo(t)
		head := gitfixture.Commit(t, repo, dir, "base", map[string]string{"tracked.txt": "tracked\n"})
		commit, err := repo.CommitObject(head)
		if err != nil {
			t.Fatal(err)
		}
		treeHash := commit.TreeHash.String()
		treeObject := filepath.Join(dir, ".git", "objects", treeHash[:2], treeHash[2:])
		if err := os.Remove(treeObject); err != nil {
			t.Fatal(err)
		}
		_, err = gitRepo(t, dir).WorkingPaths(testsupport.Context(t))
		wantContext := "resolve working paths HEAD tree " + treeHash + ": "
		if err == nil || !strings.HasPrefix(err.Error(), wantContext) {
			t.Fatalf("missing tree error = %v, want prefix %q", err, wantContext)
		}
	})
}

func TestHeadExists(t *testing.T) {
	_, unborn := gitfixture.InitRepo(t)
	if has, err := gitRepo(t, unborn).HeadExists(testsupport.Context(t)); err != nil || has {
		t.Fatalf("unborn HEAD: has=%v err=%v; want false, nil", has, err)
	}
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"a.txt": "a"})
	if has, err := gitRepo(t, dir).HeadExists(testsupport.Context(t)); err != nil || !has {
		t.Fatalf("born HEAD: has=%v err=%v; want true, nil", has, err)
	}
	if _, err := awfgit.Open(t.TempDir()); err == nil {
		t.Fatal("non-repository accepted")
	}
}

func TestHeadExistsRejectsBrokenSymbolicChains(t *testing.T) {
	for name, refs := range map[string]map[string]string{
		"existing-symbolic-ref-to-missing-ref": {
			"HEAD":             "ref: refs/heads/alias\n",
			"refs/heads/alias": "ref: refs/heads/missing\n",
		},
		"cyclic-chain": {
			"HEAD":           "ref: refs/heads/one\n",
			"refs/heads/one": "ref: refs/heads/two\n",
			"refs/heads/two": "ref: refs/heads/one\n",
		},
		"corrupt-chain": {
			"HEAD":              "ref: refs/heads/broken\n",
			"refs/heads/broken": "not a reference\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, dir := gitfixture.InitRepo(t)
			for ref, content := range refs {
				path := filepath.Join(dir, ".git", filepath.FromSlash(ref))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if has, err := gitRepo(t, dir).HeadExists(testsupport.Context(t)); err == nil {
				t.Fatalf("HeadExists accepted broken chain: has=%v", has)
			}
			if paths, err := gitRepo(t, dir).WorkingPaths(testsupport.Context(t)); err == nil {
				t.Fatalf("WorkingPaths accepted broken chain: paths=%v", paths)
			}
		})
	}
}

func TestHeadHash(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"a.txt": "a"})
	if h, err := gitRepo(t, dir).HeadHash(testsupport.Context(t)); err != nil || h == "" {
		t.Fatalf("born HEAD: hash=%q err=%v; want a hash, nil", h, err)
	}
	// An unborn HEAD (a repo with no commits) surfaces the resolve error.
	_, unborn := gitfixture.InitRepo(t)
	if _, err := gitRepo(t, unborn).HeadHash(testsupport.Context(t)); err == nil {
		t.Fatal("unborn HEAD must surface a resolve error")
	} else if errors.Is(err, plumbing.ErrReferenceNotFound) {
		t.Fatalf("HeadHash leaked go-git reference identity: %v", err)
	}
	// A non-repository fails to open.
	if _, err := awfgit.Open(t.TempDir()); err == nil {
		t.Fatal("non-repository accepted")
	}
}

func TestChangeCountsDoesNotLeakExecErrors(t *testing.T) {
	_, dir := gitfixture.InitRepo(t)
	repo := gitRepo(t, dir)
	t.Setenv("PATH", t.TempDir())
	_, _, err := repo.ChangeCounts(testsupport.Context(t))
	if err == nil {
		t.Fatal("ChangeCounts succeeded without a git binary")
	}
	if errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("ChangeCounts leaked exec.ErrNotFound: %v", err)
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		t.Fatalf("ChangeCounts leaked *exec.ExitError: %v", err)
	}
}

func TestChangedPathsRange(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "one", map[string]string{"a.txt": "a"})
	// Modify a.txt (From.Name is set) and add b.txt (From.Name empty) so both
	// sides of the change are exercised.
	gitfixture.Commit(t, repo, dir, "two", map[string]string{"a.txt": "aa", "b.txt": "b"})

	got, err := gitRepo(t, dir).ChangedPaths(testsupport.Context(t), false, "HEAD~1..HEAD")
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

	got, err := gitRepo(t, dir).ChangedPaths(testsupport.Context(t), true, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "staged.txt" {
		t.Errorf("staged: got %v want [staged.txt] (untracked excluded)", got)
	}
}

func TestChangedPathsNestedAdopter(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	base := gitfixture.Commit(t, repo, dir, "base", map[string]string{
		"nested/inside.txt": "old\n",
		"outside.txt":       "old\n",
	})
	gitfixture.Commit(t, repo, dir, "range changes", map[string]string{
		"nested/inside.txt": "new\n",
		"nested/added.txt":  "new\n",
		"outside.txt":       "new\n",
	})
	root := filepath.Join(dir, "nested")
	got, err := gitRepo(t, root).ChangedPaths(testsupport.Context(t), false, base.String()+"..HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if joined := strings.Join(got, ","); joined != "added.txt,inside.txt" {
		t.Fatalf("nested range paths = %q, want %q", joined, "added.txt,inside.txt")
	}

	gitfixture.Stage(t, repo, dir, map[string]string{
		"nested/staged.txt":  "staged\n",
		"outside-staged.txt": "outside\n",
	})
	got, err = gitRepo(t, root).ChangedPaths(testsupport.Context(t), true, "")
	if err != nil {
		t.Fatal(err)
	}
	if joined := strings.Join(got, ","); joined != "staged.txt" {
		t.Fatalf("nested staged paths = %q, want staged.txt", joined)
	}
}

func TestChangedPathsNothingStaged(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"a.txt": "a"})
	got, err := gitRepo(t, dir).ChangedPaths(testsupport.Context(t), true, "")
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

	if _, err := gitRepo(t, dir).ChangedPaths(testsupport.Context(t), false, "no-separator"); err == nil {
		t.Error("expected a malformed-range error")
	}
	if _, err := gitRepo(t, dir).ChangedPaths(testsupport.Context(t), false, "does-not-exist..HEAD"); err == nil {
		t.Error("expected an unresolvable-revision error (from side)")
	}
	if _, err := gitRepo(t, dir).ChangedPaths(testsupport.Context(t), false, "HEAD..does-not-exist"); err == nil {
		t.Error("expected an unresolvable-revision error (to side)")
	}
	if _, err := awfgit.Open(t.TempDir()); err == nil {
		t.Error("expected an open-repo error outside a repository")
	}
}

// OpenRepo resolves a normal repository and reports the canonical
// not-a-repository error outside one.
func TestOpenRepo(t *testing.T) {
	_, dir := gitfixture.InitRepo(t)
	if _, err := awfgit.Open(dir); err != nil {
		t.Fatalf("open a fresh repo: %v", err)
	}
	if _, err := awfgit.Open(t.TempDir()); !errors.Is(err, awfgit.ErrNotARepository) {
		t.Errorf("non-repo: got %v want ErrRepositoryNotExists", err)
	}
}

// A syntactically invalid .git/config (not merely a missing one, which the
// storage tolerates) makes the underlying storer's Config() fail, which
// noExtensionsStorer.Config must propagate rather than swallow.
// TestOpenToleratesWorktreeConfig pins the tolerant-open regression: a
// repository may carry this extension after native worktree operations, and
// opening its seam handle must not expose go-git's rejection to consumers.
func TestOpenToleratesWorktreeConfig(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"a.txt": "a"})
	configPath := filepath.Join(dir, ".git", "config")
	file, err := os.OpenFile(configPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("\n[extensions]\n\tworktreeConfig = true\n"); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := awfgit.Open(dir); err != nil {
		t.Fatalf("Open worktreeConfig repository: %v", err)
	}
}

func TestOpenRepoMalformedConfig(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"go.mod": "module x\n"})
	if err := os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("[core\nbroken = = =\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := awfgit.Open(dir)
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
			r, err := awfgit.Open(wtRoot)
			if err != nil {
				t.Fatalf("open linked worktree: %v", err)
			}
			if _, err := r.HeadHash(testsupport.Context(t)); err != nil {
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
		if _, err := awfgit.Open(dir); err != nil {
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
	if _, err := awfgit.Open(dir); err == nil || !strings.Contains(err.Error(), "gitdir:") {
		t.Fatalf("want a gitdir-pointer parse error, got: %v", err)
	}

	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions")
	}
	unreadable := t.TempDir()
	if err := os.WriteFile(filepath.Join(unreadable, ".git"), []byte("gitdir: nowhere\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := awfgit.Open(unreadable); err == nil {
		t.Error("expected a read error on an unreadable .git pointer file")
	}
}

// TestObjectReadContracts pins the handle's staged, committed, and transition
// object views across rename and deletion edges. These are semantic snapshots:
// names present before the transition must disappear afterwards, while the
// renamed blob remains readable in both the index and commit universes.
func TestObjectReadContracts(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	base := gitfixture.Commit(t, repo, dir, "base", map[string]string{"old.txt": "old", "gone.txt": "gone"})
	if err := os.Rename(filepath.Join(dir, "old.txt"), filepath.Join(dir, "renamed.txt")); err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Remove("old.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Remove("gone.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add("renamed.txt"); err != nil {
		t.Fatal(err)
	}
	handle := gitRepo(t, dir)
	indexed, err := handle.IndexBlobs(testsupport.Context(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(indexed) != 1 || indexed[0].Path != "renamed.txt" || string(indexed[0].Bytes) != "old" {
		t.Fatalf("staged blobs = %#v", indexed)
	}
	head, err := wt.Commit("rename and delete", &gogit.CommitOptions{Author: gitfixture.Sig, Committer: gitfixture.Sig})
	if err != nil {
		t.Fatal(err)
	}
	committed, err := handle.CommitBlobs(testsupport.Context(t), head.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(committed) != 1 || committed[0].Path != "renamed.txt" {
		t.Fatalf("committed blobs = %#v", committed)
	}
	before, after, err := handle.RangeBlobs(testsupport.Context(t), head.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 2 || len(after) != 1 || before[0].Path != "gone.txt" || before[1].Path != "old.txt" || after[0].Path != "renamed.txt" {
		t.Fatalf("range blobs = before %#v after %#v", before, after)
	}
	if base.String() == head.String() {
		t.Fatal("fixture transition did not create a new commit")
	}
}

func TestIndexBlobs(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"base.txt": "base"})
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"ordinary.txt": "ordinary bytes\n", "executable.sh": "executable bytes\n"} {
		mode := os.FileMode(0o644)
		if name == "executable.sh" {
			mode = 0o755
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), mode); err != nil {
			t.Fatal(err)
		}
		if _, err := wt.Add(name); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("ordinary.txt", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add("link"); err != nil {
		t.Fatal(err)
	}
	idx, err := repo.Storer.Index()
	if err != nil {
		t.Fatal(err)
	}
	idx.Entries = append(idx.Entries, &indexformat.Entry{Name: "submodule", Mode: filemode.Submodule, Hash: plumbing.NewHash("0123456789012345678901234567890123456789")})
	if err := repo.Storer.SetIndex(idx); err != nil {
		t.Fatal(err)
	}

	got, err := gitRepo(t, dir).IndexBlobs(testsupport.Context(t))
	if err != nil {
		t.Fatalf("IndexBlobs: %v", err)
	}
	if len(got) != 4 { // base, two ordinary/executable files, and inert symlink
		t.Fatalf("IndexBlobs returned %d blobs, want 4: %+v", len(got), got)
	}
	for _, want := range []struct {
		path, bytes string
		mode        awfgit.BlobMode
	}{{"base.txt", "base", awfgit.BlobRegular}, {"executable.sh", "executable bytes\n", awfgit.BlobExecutable}, {"link", "ordinary.txt", awfgit.BlobSymlink}, {"ordinary.txt", "ordinary bytes\n", awfgit.BlobRegular}} {
		found := false
		for _, blob := range got {
			if blob.Path == want.path && string(blob.Bytes) == want.bytes && blob.Mode == want.mode {
				found = true
			}
		}
		if !found {
			t.Errorf("missing exact staged blob %q (%q, mode=%v): %+v", want.path, want.bytes, want.mode, got)
		}
	}

	idx, err = repo.Storer.Index()
	if err != nil {
		t.Fatal(err)
	}
	idx.Entries = append(idx.Entries, &indexformat.Entry{Name: "conflict.md", Mode: filemode.Regular, Stage: indexformat.OurMode})
	if err := repo.Storer.SetIndex(idx); err != nil {
		t.Fatal(err)
	}
	if _, err := gitRepo(t, dir).IndexBlobs(testsupport.Context(t)); !errors.Is(err, awfgit.ErrIndexUnmerged) {
		t.Fatalf("unmerged index: got %v, want ErrIndexUnmerged", err)
	}
}

func TestIndexBlobsErrors(t *testing.T) {
	if _, err := awfgit.Open(t.TempDir()); !errors.Is(err, awfgit.ErrNotARepository) {
		t.Fatalf("outside repository: got %v", err)
	}

	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"a.txt": "a"})
	if err := os.WriteFile(filepath.Join(dir, ".git", "index"), []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := gitRepo(t, dir).IndexBlobs(testsupport.Context(t)); err == nil || !strings.Contains(err.Error(), "read index") {
		t.Fatalf("corrupt index: got %v", err)
	}

	repo, dir = gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"a.txt": "a"})
	idx, err := repo.Storer.Index()
	if err != nil {
		t.Fatal(err)
	}
	idx.Entries = append(idx.Entries, &indexformat.Entry{Name: "empty.md", Mode: filemode.Regular})
	if err := repo.Storer.SetIndex(idx); err != nil {
		t.Fatal(err)
	}
	if _, err := gitRepo(t, dir).IndexBlobs(testsupport.Context(t)); !errors.Is(err, awfgit.ErrIndexBlob) {
		t.Fatalf("content-less entry: got %v, want ErrIndexBlob", err)
	}
}
