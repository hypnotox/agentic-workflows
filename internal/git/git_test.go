package git_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	indexformat "github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/plumbing/object"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func TestMain(m *testing.M) { os.Exit(testsupport.RunIsolated(m)) }

func TestRepoMethodsReturnPreCancelledContext(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"tracked.txt": "tracked"})
	ctx, cancel := context.WithCancel(testsupport.Context(t))
	cancel()
	for name, call := range map[string]func() error{
		"changed":        func() error { _, err := gitRepo(t, dir).ChangedPaths(ctx, true, ""); return err },
		"head":           func() error { _, err := gitRepo(t, dir).HeadExists(ctx); return err },
		"branches":       func() error { _, err := gitRepo(t, dir).Branches(ctx); return err },
		"working":        func() error { _, err := gitRepo(t, dir).WorkingPaths(ctx); return err },
		"index":          func() error { _, err := gitRepo(t, dir).IndexBlobs(ctx); return err },
		"index paths":    func() error { _, err := gitRepo(t, dir).IndexPaths(ctx); return err },
		"commit":         func() error { _, err := gitRepo(t, dir).CommitBlobs(ctx, "HEAD"); return err },
		"commit entries": func() error { _, err := gitRepo(t, dir).CommitEntries(ctx, "HEAD"); return err },
		"commit selected blobs": func() error {
			_, err := gitRepo(t, dir).CommitBlobsAt(ctx, "HEAD", []string{"tracked.txt"})
			return err
		},
		"range":        func() error { _, _, err := gitRepo(t, dir).RangeBlobs(ctx, "HEAD"); return err },
		"first-parent": func() error { _, err := gitRepo(t, dir).FirstParentChangedPaths(ctx, "HEAD"); return err },
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
	fixture := gitfixture.InitRepo(t)
	dir := fixture.Root()
	gitfixture.Commit(t, fixture, "base", map[string]string{"also.txt": "also", "tracked.txt": "base"})
	gitfixture.Commit(t, fixture, "changed", map[string]string{"tracked.txt": "changed"})
	gitfixture.Stage(t, fixture, map[string]string{"staged.txt": "staged"})
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
	assertCanceled("working status", 4, func(ctx context.Context) error {
		_, err := repo.WorkingPaths(ctx)
		return err
	})
	assertCanceled("index", 2, func(ctx context.Context) error {
		_, err := repo.IndexBlobs(ctx)
		return err
	})
	assertCanceled("index paths", 2, func(ctx context.Context) error {
		_, err := repo.IndexPaths(ctx)
		return err
	})
	assertCanceled("commit blobs", 2, func(ctx context.Context) error {
		_, err := repo.CommitBlobs(ctx, "HEAD")
		return err
	})
	assertCanceled("first-parent paths", 2, func(ctx context.Context) error {
		_, err := repo.FirstParentChangedPaths(ctx, "HEAD")
		return err
	})
	assertCanceled("first-parent diff paths", 6, func(ctx context.Context) error {
		_, err := repo.FirstParentChangedPaths(ctx, "HEAD")
		return err
	})
	assertCanceled("commit entries", 2, func(ctx context.Context) error {
		_, err := repo.CommitEntries(ctx, "HEAD")
		return err
	})
	assertCanceled("selected commit blobs", 5, func(ctx context.Context) error {
		_, err := repo.CommitBlobsAt(ctx, "HEAD", []string{"also.txt", "tracked.txt"})
		return err
	})
	assertCanceled("selected commit blob validation", 2, func(ctx context.Context) error {
		_, err := repo.CommitBlobsAt(ctx, "HEAD", []string{"tracked.txt"})
		return err
	})

	nestedFixture := gitfixture.InitRepo(t)
	gitfixture.Commit(t, nestedFixture, "base", map[string]string{"nested/inside.txt": "inside", "root.txt": "root"})
	gitfixture.Commit(t, nestedFixture, "head", map[string]string{"root.txt": "changed"})
	nestedRepo := gitRepo(t, nestedFixture.Root())
	assertCanceled("first-parent recursive paths", 3, func(ctx context.Context) error {
		_, err := nestedRepo.FirstParentChangedPaths(ctx, "HEAD")
		return err
	})
	nestedHandle := gitRepo(t, filepath.Join(nestedFixture.Root(), "nested"))
	assertCanceled("commit entries nested prefix", 2, func(ctx context.Context) error {
		_, err := nestedHandle.CommitEntries(ctx, "HEAD")
		return err
	})
}

func TestWorkingPaths(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitfixture.Commit(t, repo, "base", map[string]string{"src/a.txt": "a", "gone.txt": "gone", ".gitignore": "ignored.txt\n"})
	if err := os.Remove(filepath.Join(dir, "gone.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	handle := gitRepo(t, dir)
	paths, err := handle.WorkingPaths(testsupport.Context(t))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(paths, ",")
	if strings.Contains(joined, "gone.txt") || strings.Contains(joined, "ignored.txt") || !strings.Contains(joined, "new.txt") || !strings.Contains(joined, "src/a.txt") {
		t.Fatalf("working paths: %v", paths)
	}
	// Root is the anchor those repository-relative paths join back onto, so it
	// is pinned beside them rather than alone: a consumer that resolves
	// "src/a.txt" to a file on disk is relying on both answers agreeing.
	if got := handle.Root(); got != dir {
		t.Fatalf("Root = %q, want the anchored checkout %q", got, dir)
	}
	if _, err := os.Stat(filepath.Join(handle.Root(), "src", "a.txt")); err != nil {
		t.Fatalf("joining a reported path onto Root did not reach the file: %v", err)
	}
}

func TestWorkingPathsExcludesNestedFileBelowIgnoredManagedWorktreeRoot(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{".gitignore": ".awf/worktrees/\n", "tracked.txt": "tracked"})
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
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"tracked.txt": "tracked"})
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
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	if err := os.MkdirAll(filepath.Join(dir, "nested", ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitfixture.Commit(t, repo, "base", map[string]string{
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
	dir := gitfixture.InitRepo(t).Root()
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
		dir := gitfixture.InitRepo(t).Root()
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
		dir := gitfixture.InitRepo(t).Root()
		if err := os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("0123456789012345678901234567890123456789\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := gitRepo(t, dir).WorkingPaths(testsupport.Context(t)); err == nil {
			t.Fatal("dangling HEAD accepted as unborn")
		}
	})

	t.Run("missing-commit-object", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		dir := repo.Root()
		head := gitfixture.Commit(t, repo, "base", map[string]string{"tracked.txt": "tracked\n"})
		commitObject := filepath.Join(dir, ".git", "objects", head[:2], head[2:])
		if err := os.Remove(commitObject); err != nil {
			t.Fatal(err)
		}
		_, err := gitRepo(t, dir).WorkingPaths(testsupport.Context(t))
		wantContext := "resolve working paths HEAD commit " + head + ": "
		if err == nil || !strings.HasPrefix(err.Error(), wantContext) {
			t.Fatalf("missing commit error = %v, want prefix %q", err, wantContext)
		}
	})

	t.Run("missing-tree-object", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		dir := repo.Root()
		head := gitfixture.Commit(t, repo, "base", map[string]string{"tracked.txt": "tracked\n"})
		treeHash := gitfixture.TreeHash(t, repo, head)
		treeObject := filepath.Join(dir, ".git", "objects", treeHash[:2], treeHash[2:])
		if err := os.Remove(treeObject); err != nil {
			t.Fatal(err)
		}
		_, err := gitRepo(t, dir).WorkingPaths(testsupport.Context(t))
		wantContext := "resolve working paths HEAD tree " + treeHash + ": "
		if err == nil || !strings.HasPrefix(err.Error(), wantContext) {
			t.Fatalf("missing tree error = %v, want prefix %q", err, wantContext)
		}
	})
}

func TestHeadExists(t *testing.T) {
	unborn := gitfixture.InitRepo(t).Root()
	if has, err := gitRepo(t, unborn).HeadExists(testsupport.Context(t)); err != nil || has {
		t.Fatalf("unborn HEAD: has=%v err=%v; want false, nil", has, err)
	}
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"a.txt": "a"})
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
			dir := gitfixture.InitRepo(t).Root()
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

func TestChangeCountsExposesOnlySeamAndContextErrors(t *testing.T) {
	dir := gitfixture.InitRepo(t).Root()
	repo := gitRepo(t, dir)
	assertNoExecError := func(t *testing.T, err error) {
		t.Helper()
		if errors.Is(err, exec.ErrNotFound) {
			t.Fatalf("ChangeCounts leaked exec.ErrNotFound: %v", err)
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			t.Fatalf("ChangeCounts leaked *exec.ExitError: %v", err)
		}
	}

	t.Run("missing binary", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		_, _, err := repo.ChangeCounts(testsupport.Context(t))
		if err == nil {
			t.Fatal("ChangeCounts succeeded without a git binary")
		}
		assertNoExecError(t, err)
	})

	if runtime.GOOS == "windows" {
		return
	}
	t.Run("non-zero exit", func(t *testing.T) {
		bin := t.TempDir()
		if err := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\necho seam-failure >&2\nexit 7\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		_, _, err := repo.ChangeCounts(testsupport.Context(t))
		var command *awfgit.CommandError
		if !errors.As(err, &command) || command.ExitCode != 7 {
			t.Fatalf("ChangeCounts command error = %T %v", err, err)
		}
		assertNoExecError(t, err)
	})

	t.Run("deadline", func(t *testing.T) {
		bin := t.TempDir()
		script := "#!/bin/sh\nif [ \"$3\" = config ]; then exit 1; fi\nexec sleep 30\n"
		if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		ctx, cancel := context.WithTimeout(testsupport.Context(t), 50*time.Millisecond)
		defer cancel()
		_, _, err := repo.ChangeCounts(ctx)
		var command *awfgit.CommandError
		if !errors.As(err, &command) || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("ChangeCounts deadline error = %T %v", err, err)
		}
		assertNoExecError(t, err)
	})
}

func TestChangedPathsRange(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "one", map[string]string{"a.txt": "a"})
	// Modify a.txt (From.Name is set) and add b.txt (From.Name empty) so both
	// sides of the change are exercised.
	gitfixture.Commit(t, repo, "two", map[string]string{"a.txt": "aa", "b.txt": "b"})

	got, err := gitRepo(t, dir).ChangedPaths(testsupport.Context(t), false, "HEAD~1..HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "a.txt,b.txt" {
		t.Errorf("range: got %v want [a.txt b.txt]", got)
	}
}

func TestChangedPathsStaged(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"a.txt": "a"})

	// Stage a new file without committing; leave a second file untracked.
	gitfixture.Stage(t, repo, map[string]string{"staged.txt": "s"})
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
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	base := gitfixture.Commit(t, repo, "base", map[string]string{
		"nested/inside.txt": "old\n",
		"outside.txt":       "old\n",
	})
	gitfixture.Commit(t, repo, "range changes", map[string]string{
		"nested/inside.txt": "new\n",
		"nested/added.txt":  "new\n",
		"outside.txt":       "new\n",
	})
	root := filepath.Join(dir, "nested")
	got, err := gitRepo(t, root).ChangedPaths(testsupport.Context(t), false, base+"..HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if joined := strings.Join(got, ","); joined != "added.txt,inside.txt" {
		t.Fatalf("nested range paths = %q, want %q", joined, "added.txt,inside.txt")
	}

	gitfixture.Stage(t, repo, map[string]string{
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
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"a.txt": "a"})
	got, err := gitRepo(t, dir).ChangedPaths(testsupport.Context(t), true, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("clean tree: got %v want none", got)
	}
}

func TestChangedPathsErrors(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"a.txt": "a"})

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
	dir := gitfixture.InitRepo(t).Root()
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
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"a.txt": "a"})
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
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"go.mod": "module x\n"})
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
	repo := gitfixture.InitRepo(t)
	mainDir := repo.Root()
	head := gitfixture.Commit(t, repo, "base", map[string]string{"go.mod": "module x\n"})

	for name, tc := range map[string]struct{ head, commondir string }{
		"relative-commondir-symbolic-head": {"ref: refs/heads/master", "../.."},
		"absolute-commondir-detached-head": {head, filepath.Join(mainDir, ".git")},
	} {
		t.Run(name, func(t *testing.T) {
			wtRoot := linkedWorktree(t, mainDir, name, tc.head, tc.commondir)
			r, err := awfgit.Open(wtRoot)
			if err != nil {
				t.Fatalf("open linked worktree: %v", err)
			}
			if exists, err := r.HeadExists(testsupport.Context(t)); err != nil || !exists {
				t.Fatalf("resolve HEAD in linked worktree: exists=%v err=%v", exists, err)
			}
		})
	}

	t.Run("relative-gitfile-without-commondir", func(t *testing.T) {
		sub := gitfixture.InitRepo(t)
		dir := sub.Root()
		gitfixture.Commit(t, sub, "x", map[string]string{"a.txt": "a"})
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
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	base := gitfixture.Commit(t, repo, "base", map[string]string{"old.txt": "old", "gone.txt": "gone"})
	if err := os.Rename(filepath.Join(dir, "old.txt"), filepath.Join(dir, "renamed.txt")); err != nil {
		t.Fatal(err)
	}
	gitfixture.StageRemoval(t, repo, "old.txt", "gone.txt")
	gitfixture.Add(t, repo, "renamed.txt")
	handle := gitRepo(t, dir)
	indexed, err := handle.IndexBlobs(testsupport.Context(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(indexed) != 1 || indexed[0].Path != "renamed.txt" || string(indexed[0].Bytes) != "old" {
		t.Fatalf("staged blobs = %#v", indexed)
	}
	head := gitfixture.Commit(t, repo, "rename and delete", nil)
	committed, err := handle.CommitBlobs(testsupport.Context(t), head)
	if err != nil {
		t.Fatal(err)
	}
	if len(committed) != 1 || committed[0].Path != "renamed.txt" {
		t.Fatalf("committed blobs = %#v", committed)
	}
	before, after, err := handle.RangeBlobs(testsupport.Context(t), head)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 2 || len(after) != 1 || before[0].Path != "gone.txt" || before[1].Path != "old.txt" || after[0].Path != "renamed.txt" {
		t.Fatalf("range blobs = before %#v after %#v", before, after)
	}
	if base == head {
		t.Fatal("fixture transition did not create a new commit")
	}
	rootBefore, rootAfter, err := handle.RangeBlobs(testsupport.Context(t), base)
	if err != nil || len(rootBefore) != 0 || len(rootAfter) != 2 {
		t.Fatalf("root range blobs = before %#v after %#v, err=%v", rootBefore, rootAfter, err)
	}
	if _, _, err := handle.RangeBlobs(testsupport.Context(t), "missing"); err == nil {
		t.Fatal("missing range revision succeeded")
	}
	backend, err := gogit.PlainOpen(repo.Root())
	if err != nil {
		t.Fatal(err)
	}
	baseCommit, err := backend.CommitObject(plumbing.NewHash(base))
	if err != nil {
		t.Fatal(err)
	}
	baseTree, err := baseCommit.Tree()
	if err != nil {
		t.Fatal(err)
	}
	blob, err := baseTree.File("old.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := handle.RangeBlobs(testsupport.Context(t), blob.Hash.String()); err == nil {
		t.Fatal("resolvable blob accepted as a range revision")
	}
	merge := gitfixture.Graft(t, repo, "merge", base, head, base)
	mergeBefore, mergeAfter, err := handle.RangeBlobs(testsupport.Context(t), merge)
	if err != nil || len(mergeBefore) != 1 || mergeBefore[0].Path != "renamed.txt" || len(mergeAfter) != 2 || mergeAfter[0].Path != "gone.txt" {
		t.Fatalf("merge range blobs = before %#v after %#v, err=%v", mergeBefore, mergeAfter, err)
	}
}

func TestCommitEvidenceReads(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	base := gitfixture.Commit(t, repo, "base", map[string]string{"a.txt": "a"})
	head := gitfixture.Commit(t, repo, "feat: subject\n\nbody\n", map[string]string{"b.txt": "b"})
	handle := gitRepo(t, repo.Root())
	parents, err := handle.CommitParents(testsupport.Context(t), head)
	if err != nil {
		t.Fatal(err)
	}
	if len(parents) != 1 || parents[0] != base {
		t.Fatalf("CommitParents = %q, want [%s]", parents, base)
	}
	message, err := handle.CommitMessage(testsupport.Context(t), head)
	if err != nil {
		t.Fatal(err)
	}
	if message != "feat: subject\n\nbody\n" {
		t.Fatalf("CommitMessage = %q", message)
	}
	if _, err := handle.CommitParents(testsupport.Context(t), "missing"); err == nil {
		t.Fatal("missing revision succeeded")
	}
	if _, err := handle.CommitMessage(testsupport.Context(t), "missing"); err == nil {
		t.Fatal("missing message revision succeeded")
	}
	backend, err := gogit.PlainOpen(repo.Root())
	if err != nil {
		t.Fatal(err)
	}
	commit, err := backend.CommitObject(plumbing.NewHash(head))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := commit.Tree()
	if err != nil {
		t.Fatal(err)
	}
	file, err := tree.File("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handle.CommitMessage(testsupport.Context(t), file.Hash.String()); err == nil {
		t.Fatal("resolvable blob accepted as a commit")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := handle.CommitParents(canceled, head); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled CommitParents = %v", err)
	}
	if _, err := handle.CommitMessage(canceled, head); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled CommitMessage = %v", err)
	}
}

// invariant: tooling/audit-and-snapshots:sparse-snapshot-explicit-selection (TestCommitEntriesAndBlobsAtContracts)
func TestCommitEntriesAndBlobsAtContracts(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.StageFile(t, repo, "nested/regular.txt", "regular bytes\n", 0o644)
	gitfixture.StageFile(t, repo, "nested/executable.sh", "executable bytes\n", 0o755)
	if err := os.Symlink("regular.txt", filepath.Join(dir, "nested", "link")); err != nil {
		t.Fatal(err)
	}
	gitfixture.Add(t, repo, "nested/link")
	gitfixture.StageGitlink(t, repo, "nested/submodule")
	head := gitfixture.Commit(t, repo, "entries", nil)
	handle := gitRepo(t, dir)

	entries, err := handle.CommitEntries(testsupport.Context(t), head)
	if err != nil {
		t.Fatalf("CommitEntries: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("CommitEntries = %#v, want three non-gitlink entries", entries)
	}
	for i, want := range []struct {
		path string
		mode awfgit.BlobMode
	}{
		{"nested/executable.sh", awfgit.BlobExecutable},
		{"nested/link", awfgit.BlobSymlink},
		{"nested/regular.txt", awfgit.BlobRegular},
	} {
		if entries[i].Path != want.path || entries[i].Mode != want.mode {
			t.Fatalf("entry[%d] = %#v, want %q mode %v", i, entries[i], want.path, want.mode)
		}
	}

	blobs, err := handle.CommitBlobsAt(testsupport.Context(t), head, []string{"nested/regular.txt", "nested/link", "nested/executable.sh"})
	if err != nil {
		t.Fatalf("CommitBlobsAt: %v", err)
	}
	for i, want := range []struct {
		path, bytes string
		mode        awfgit.BlobMode
	}{
		{"nested/executable.sh", "executable bytes\n", awfgit.BlobExecutable},
		{"nested/link", "regular.txt", awfgit.BlobSymlink},
		{"nested/regular.txt", "regular bytes\n", awfgit.BlobRegular},
	} {
		if blobs[i].Path != want.path || string(blobs[i].Bytes) != want.bytes || blobs[i].Mode != want.mode {
			t.Fatalf("blob[%d] = %#v, want %q %q mode %v", i, blobs[i], want.path, want.bytes, want.mode)
		}
	}
	blobs[0].Bytes[0] = 'X'
	if reread, err := handle.CommitBlobsAt(testsupport.Context(t), head, []string{"nested/executable.sh"}); err != nil || string(reread[0].Bytes) != "executable bytes\n" {
		t.Fatalf("selected bytes were not owned: %#v, %v", reread, err)
	}
	nested := gitRepo(t, filepath.Join(dir, "nested"))
	if rerooted, err := nested.CommitEntries(testsupport.Context(t), head); err != nil || len(rerooted) != 3 || rerooted[0].Path != "executable.sh" || rerooted[2].Path != "regular.txt" {
		t.Fatalf("nested entries = %#v, %v", rerooted, err)
	}
	if rerooted, err := nested.CommitBlobsAt(testsupport.Context(t), head, []string{"regular.txt"}); err != nil || len(rerooted) != 1 || string(rerooted[0].Bytes) != "regular bytes\n" {
		t.Fatalf("nested selected blob = %#v, %v", rerooted, err)
	}
	if empty, err := handle.CommitBlobsAt(testsupport.Context(t), head, nil); err != nil || len(empty) != 0 {
		t.Fatalf("empty selection = %#v, %v", empty, err)
	}

	for name, paths := range map[string][]string{
		"missing":               {"missing.txt"},
		"duplicate":             {"nested/regular.txt", "nested/regular.txt"},
		"unsafe parent":         {"../outside.txt"},
		"unsafe noncanonical":   {"nested/../nested/regular.txt"},
		"gitlink":               {"nested/submodule"},
		"directory":             {"nested"},
		"outside nested handle": {"outside.txt"},
	} {
		t.Run(name, func(t *testing.T) {
			rooted := handle
			if name == "outside nested handle" {
				rooted = gitRepo(t, filepath.Join(dir, "nested"))
			}
			if _, err := rooted.CommitBlobsAt(testsupport.Context(t), head, paths); err == nil {
				t.Fatalf("CommitBlobsAt(%q) accepted %q", head, paths)
			}
		})
	}

	backend, err := gogit.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	commit, err := backend.CommitObject(plumbing.NewHash(head))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := commit.Tree()
	if err != nil {
		t.Fatal(err)
	}
	file, err := tree.File("nested/regular.txt")
	if err != nil {
		t.Fatal(err)
	}
	unsupportedTree := &object.Tree{Entries: []object.TreeEntry{{Name: "legacy.txt", Mode: filemode.Empty, Hash: file.Hash}}}
	encodedTree := backend.Storer.NewEncodedObject()
	if err := unsupportedTree.Encode(encodedTree); err != nil {
		t.Fatal(err)
	}
	unsupportedTreeHash, err := backend.Storer.SetEncodedObject(encodedTree)
	if err != nil {
		t.Fatal(err)
	}
	unsupportedCommit := &object.Commit{Author: *gitfixture.Sig, Committer: *gitfixture.Sig, Message: "unsupported mode\n", TreeHash: unsupportedTreeHash}
	encodedCommit := backend.Storer.NewEncodedObject()
	if err := unsupportedCommit.Encode(encodedCommit); err != nil {
		t.Fatal(err)
	}
	unsupportedCommitHash, err := backend.Storer.SetEncodedObject(encodedCommit)
	if err != nil {
		t.Fatal(err)
	}
	if entries, err := handle.CommitEntries(testsupport.Context(t), unsupportedCommitHash.String()); err != nil || len(entries) != 0 {
		t.Fatalf("unsupported entries = %#v, %v", entries, err)
	}
	if _, err := handle.CommitBlobsAt(testsupport.Context(t), unsupportedCommitHash.String(), []string{"legacy.txt"}); err == nil {
		t.Fatal("unsupported entry accepted")
	}

	// A nonexistent adopted-project subdirectory has no committed entries yet.
	if entries, err := gitRepo(t, filepath.Join(dir, "missing-project")).CommitEntries(testsupport.Context(t), head); err != nil || len(entries) != 0 {
		t.Fatalf("missing project entries = %#v, %v", entries, err)
	}

	// Tree metadata can name a missing descendant tree in a damaged object
	// store. The walker must surface both the descendant read failure and its
	// propagation through the parent walk.
	storeTree := func(tree *object.Tree) plumbing.Hash {
		t.Helper()
		encoded := backend.Storer.NewEncodedObject()
		if err := tree.Encode(encoded); err != nil {
			t.Fatal(err)
		}
		hash, err := backend.Storer.SetEncodedObject(encoded)
		if err != nil {
			t.Fatal(err)
		}
		return hash
	}
	// Write raw tree bytes because object.Tree.Encode correctly refuses the
	// malformed name. The committed object must still fail closed at the
	// treeEntries project-path boundary.
	malformedEncoded := backend.Storer.NewEncodedObject()
	malformedEncoded.SetType(plumbing.TreeObject)
	writer, err := malformedEncoded.Writer()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(append([]byte("100644 ..\x00"), file.Hash[:]...)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	malformedTreeHash, err := backend.Storer.SetEncodedObject(malformedEncoded)
	if err != nil {
		t.Fatal(err)
	}
	malformedCommit := &object.Commit{Author: *gitfixture.Sig, Committer: *gitfixture.Sig, Message: "malformed tree\n", TreeHash: malformedTreeHash}
	malformedCommitEncoded := backend.Storer.NewEncodedObject()
	if err := malformedCommit.Encode(malformedCommitEncoded); err != nil {
		t.Fatal(err)
	}
	malformedCommitHash, err := backend.Storer.SetEncodedObject(malformedCommitEncoded)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handle.CommitEntries(testsupport.Context(t), malformedCommitHash.String()); err == nil {
		t.Fatal("unsafe encoded tree path accepted")
	}

	missingTree := plumbing.NewHash(strings.Repeat("f", 40))
	missingPrefixTree := storeTree(&object.Tree{Entries: []object.TreeEntry{{Name: "missing-project", Mode: filemode.Dir, Hash: missingTree}}})
	missingPrefixCommit := &object.Commit{Author: *gitfixture.Sig, Committer: *gitfixture.Sig, Message: "missing project tree\n", TreeHash: missingPrefixTree}
	missingPrefixEncoded := backend.Storer.NewEncodedObject()
	if err := missingPrefixCommit.Encode(missingPrefixEncoded); err != nil {
		t.Fatal(err)
	}
	missingPrefixHash, err := backend.Storer.SetEncodedObject(missingPrefixEncoded)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gitRepo(t, filepath.Join(dir, "missing-project")).CommitEntries(testsupport.Context(t), missingPrefixHash.String()); err == nil {
		t.Fatal("missing project tree object accepted")
	}

	childTree := storeTree(&object.Tree{Entries: []object.TreeEntry{{Name: "missing", Mode: filemode.Dir, Hash: missingTree}}})
	outerTree := storeTree(&object.Tree{Entries: []object.TreeEntry{{Name: "child", Mode: filemode.Dir, Hash: childTree}}})
	brokenCommit := &object.Commit{Author: *gitfixture.Sig, Committer: *gitfixture.Sig, Message: "broken tree\n", TreeHash: outerTree}
	brokenEncoded := backend.Storer.NewEncodedObject()
	if err := brokenCommit.Encode(brokenEncoded); err != nil {
		t.Fatal(err)
	}
	brokenHash, err := backend.Storer.SetEncodedObject(brokenEncoded)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handle.CommitEntries(testsupport.Context(t), brokenHash.String()); err == nil {
		t.Fatal("missing descendant tree accepted")
	}

	for name, call := range map[string]func(string) error{
		"entries": func(rev string) error {
			_, err := handle.CommitEntries(testsupport.Context(t), rev)
			return err
		},
		"selected blobs": func(rev string) error {
			_, err := handle.CommitBlobsAt(testsupport.Context(t), rev, []string{"nested/regular.txt"})
			return err
		},
	} {
		t.Run(name+" missing revision", func(t *testing.T) {
			if err := call("missing"); err == nil {
				t.Fatal("missing revision accepted")
			}
		})
	}

	// The sparse-read contract remains observable with loose objects: tree
	// metadata is readable after removing an unrelated blob, but selecting that
	// blob later fails.
	sparseRepo := gitfixture.InitRepo(t)
	sparseDir := sparseRepo.Root()
	sparseHead := gitfixture.Commit(t, sparseRepo, "two blobs", map[string]string{"selected.txt": "selected", "unselected.txt": "unselected"})
	sparseBackend, err := gogit.PlainOpen(sparseDir)
	if err != nil {
		t.Fatal(err)
	}
	sparseCommit, err := sparseBackend.CommitObject(plumbing.NewHash(sparseHead))
	if err != nil {
		t.Fatal(err)
	}
	sparseTree, err := sparseCommit.Tree()
	if err != nil {
		t.Fatal(err)
	}
	unselected, err := sparseTree.File("unselected.txt")
	if err != nil {
		t.Fatal(err)
	}
	objectPath := filepath.Join(sparseDir, ".git", "objects", unselected.Hash.String()[:2], unselected.Hash.String()[2:])
	if err := os.Remove(objectPath); err != nil {
		t.Fatal(err)
	}

	sparseHandle := gitRepo(t, sparseDir)
	if entries, err := sparseHandle.CommitEntries(testsupport.Context(t), sparseHead); err != nil || len(entries) != 2 {
		t.Fatalf("entries after removing unselected object = %#v, %v", entries, err)
	}
	if blobs, err := sparseHandle.CommitBlobsAt(testsupport.Context(t), sparseHead, []string{"selected.txt"}); err != nil || len(blobs) != 1 || string(blobs[0].Bytes) != "selected" {
		t.Fatalf("selected blob after removing unselected object = %#v, %v", blobs, err)
	}
	if _, err := sparseHandle.CommitBlobsAt(testsupport.Context(t), sparseHead, []string{"unselected.txt"}); err == nil {
		t.Fatal("removed selected blob accepted")
	}

	sparseTreeHash := gitfixture.TreeHash(t, sparseRepo, sparseHead)
	if err := os.Remove(filepath.Join(sparseDir, ".git", "objects", sparseTreeHash[:2], sparseTreeHash[2:])); err != nil {
		t.Fatal(err)
	}
	for name, call := range map[string]func() error{
		"entries": func() error {
			_, err := sparseHandle.CommitEntries(testsupport.Context(t), sparseHead)
			return err
		},
		"selected blobs": func() error {
			_, err := sparseHandle.CommitBlobsAt(testsupport.Context(t), sparseHead, []string{"selected.txt"})
			return err
		},
	} {
		t.Run(name+" corrupt revision", func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("corrupt revision accepted")
			}
		})
	}
}

func TestIndexPaths(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"tracked.txt": "tracked\n"})
	gitfixture.StageFile(t, repo, "ordinary.txt", "ordinary\n", 0o644)
	gitfixture.StageFile(t, repo, "executable.sh", "executable\n", 0o755)
	if err := os.Symlink("ordinary.txt", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	gitfixture.Add(t, repo, "link")
	gitfixture.StageUnmerged(t, repo, "conflict.txt")
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("ignored\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rootHandle := gitRepo(t, dir)
	if rootHandle.IsNested() {
		t.Fatal("repository-root handle IsNested = true")
	}
	paths, err := rootHandle.IndexPaths(testsupport.Context(t))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(paths, ","), "conflict.txt,executable.sh,link,ordinary.txt,tracked.txt"; got != want {
		t.Fatalf("IndexPaths = %q, want %q", got, want)
	}

	gitfixture.Add(t, repo, ".gitignore")
	gitfixture.Add(t, repo, "ignored.txt")
	paths, err = gitRepo(t, dir).IndexPaths(testsupport.Context(t))
	if err != nil || !slices.Contains(paths, "ignored.txt") {
		t.Fatalf("tracked ignored IndexPaths = %v, %v", paths, err)
	}

	gitfixture.Stage(t, repo, map[string]string{"nested/inside.txt": "inside\n"})
	nested := gitRepo(t, filepath.Join(dir, "nested"))
	if !nested.IsNested() {
		t.Fatal("nested handle IsNested = false")
	}
	if paths, err := nested.IndexPaths(testsupport.Context(t)); err != nil || !slices.Equal(paths, []string{"inside.txt"}) {
		t.Fatalf("nested IndexPaths = %v, %v, want rerooted inside.txt only", paths, err)
	}
	ctx, cancel := context.WithCancel(testsupport.Context(t))
	cancel()
	if _, err := rootHandle.IndexPaths(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled IndexPaths = %v", err)
	}
}

func TestIndexBlobs(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"base.txt": "base"})
	for name, content := range map[string]string{"ordinary.txt": "ordinary bytes\n", "executable.sh": "executable bytes\n"} {
		mode := os.FileMode(0o644)
		if name == "executable.sh" {
			mode = 0o755
		}
		gitfixture.StageFile(t, repo, name, content, mode)
	}
	if err := os.Symlink("ordinary.txt", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	gitfixture.Add(t, repo, "link")
	gitfixture.StageGitlink(t, repo, "submodule")

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

	gitfixture.StageUnmerged(t, repo, "conflict.md")
	if _, err := gitRepo(t, dir).IndexBlobs(testsupport.Context(t)); !errors.Is(err, awfgit.ErrIndexUnmerged) {
		t.Fatalf("unmerged index: got %v, want ErrIndexUnmerged", err)
	}
}

func TestIndexBlobsErrors(t *testing.T) {
	if _, err := awfgit.Open(t.TempDir()); !errors.Is(err, awfgit.ErrNotARepository) {
		t.Fatalf("outside repository: got %v", err)
	}

	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"a.txt": "a"})
	if err := os.WriteFile(filepath.Join(dir, ".git", "index"), []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := gitRepo(t, dir).IndexPaths(testsupport.Context(t)); err == nil || !strings.Contains(err.Error(), "read index") {
		t.Fatalf("corrupt index paths: got %v", err)
	}
	if _, err := gitRepo(t, dir).IndexBlobs(testsupport.Context(t)); err == nil || !strings.Contains(err.Error(), "read index") {
		t.Fatalf("corrupt index: got %v", err)
	}

	repo = gitfixture.InitRepo(t)
	dir = repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"a.txt": "a"})
	stageContentlessEntry(t, dir, "empty.md")
	if _, err := gitRepo(t, dir).IndexBlobs(testsupport.Context(t)); !errors.Is(err, awfgit.ErrIndexBlob) {
		t.Fatalf("content-less entry: got %v, want ErrIndexBlob", err)
	}
}

// stageContentlessEntry appends an index entry whose hash names no object, the
// storage-level corruption that drives IndexBlobs' blob-read failure. It stays
// here rather than in gitfixture because it is a fault this reader alone must
// answer for, not a repository state a fixture consumer builds.
func stageContentlessEntry(t *testing.T, dir, name string) {
	t.Helper()
	repo, err := gogit.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := repo.Storer.Index()
	if err != nil {
		t.Fatal(err)
	}
	idx.Entries = append(idx.Entries, &indexformat.Entry{Name: name, Mode: filemode.Regular})
	if err := repo.Storer.SetIndex(idx); err != nil {
		t.Fatal(err)
	}
}
