package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	gogit "github.com/go-git/go-git/v5"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// lifecycleGit runs git directly in dir with a fixed identity. The contract
// suite needs a few fixture shapes (an unrelated root history) that the go-git
// fixture lane cannot express; every assertion below still reads the seam.
func lifecycleGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	fixed := append([]string{"-C", dir, "-c", "user.name=T", "-c", "user.email=t@example.com"}, args...)
	out, err := exec.CommandContext(t.Context(), "git", fixed...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func lifecycleRepo(t *testing.T) (*Repo, string) {
	t.Helper()
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"tracked.txt": "base\n"})
	// The seam's isolated environment strips user and system config, so an
	// operation that records a commit needs the identity in the repository's own
	// config - which is where a real checkout carries it too.
	lifecycleGit(t, dir, "config", "user.name", "T")
	lifecycleGit(t, dir, "config", "user.email", "t@example.com")
	return statusRepo(t, dir), dir
}

// TestWorktreeRegistrationRoundTrip pins the managed-checkout lifecycle as one
// story: what add registers is what list reports, what remove retires, and what
// the branch probe then denies.
func TestWorktreeRegistrationRoundTrip(t *testing.T) {
	repo, dir := lifecycleRepo(t)
	ctx := testContext(t)
	managed := filepath.Join(t.TempDir(), "managed")

	if exists, err := repo.BranchExists(ctx, "awf/round-trip"); err != nil || exists {
		t.Fatalf("branch before add exists=%v err=%v", exists, err)
	}
	if err := repo.WorktreeAdd(ctx, managed, "awf/round-trip", "HEAD"); err != nil {
		t.Fatal(err)
	}
	registrations, err := repo.WorktreeList(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, registration := range registrations {
		if registration.Path == filepath.Clean(managed) {
			found = true
			if registration.Branch != "refs/heads/awf/round-trip" || registration.Detached || registration.Bare {
				t.Fatalf("registration = %#v", registration)
			}
		}
	}
	if !found || len(registrations) != 2 {
		t.Fatalf("registrations = %#v, want the primary plus the managed checkout", registrations)
	}
	if exists, err := repo.BranchExists(ctx, "awf/round-trip"); err != nil || !exists {
		t.Fatalf("branch after add exists=%v err=%v", exists, err)
	}

	if err := repo.WorktreeRemove(ctx, managed); err != nil {
		t.Fatal(err)
	}
	registrations, err = repo.WorktreeList(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(registrations) != 1 || registrations[0].Path != filepath.Clean(dir) {
		t.Fatalf("registrations after remove = %#v", registrations)
	}
	if err := repo.BranchDelete(ctx, "awf/round-trip"); err != nil {
		t.Fatal(err)
	}
	if exists, err := repo.BranchExists(ctx, "awf/round-trip"); err != nil || exists {
		t.Fatalf("branch after delete exists=%v err=%v", exists, err)
	}
}

// TestWorktreePruneRetiresAnAbandonedRegistration covers the other retirement
// route: the checkout is already gone and only the registration remains.
func TestWorktreePruneRetiresAnAbandonedRegistration(t *testing.T) {
	repo, _ := lifecycleRepo(t)
	ctx := testContext(t)
	managed := filepath.Join(t.TempDir(), "abandoned")
	if err := repo.WorktreeAdd(ctx, managed, "awf/abandoned", "HEAD"); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(managed); err != nil {
		t.Fatal(err)
	}
	if err := repo.WorktreePrune(ctx); err != nil {
		t.Fatal(err)
	}
	registrations, err := repo.WorktreeList(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(registrations) != 1 {
		t.Fatalf("registrations after prune = %#v, want the primary alone", registrations)
	}
}

// TestAncestorAnswersTheFullTruthTable pins that the merged-ness question a
// destructive operation depends on distinguishes all three real cases, and in
// particular that unrelated histories answer "no" rather than failing.
func TestAncestorAnswersTheFullTruthTable(t *testing.T) {
	repo, dir := lifecycleRepo(t)
	ctx := testContext(t)
	base := lifecycleGit(t, dir, "rev-parse", "HEAD")
	lifecycleGit(t, dir, "commit", "--allow-empty", "-m", "later")
	later := lifecycleGit(t, dir, "rev-parse", "HEAD")
	lifecycleGit(t, dir, "checkout", "--orphan", "unrelated")
	lifecycleGit(t, dir, "commit", "--allow-empty", "-m", "unrelated root")
	unrelated := lifecycleGit(t, dir, "rev-parse", "HEAD")

	for _, test := range []struct {
		name         string
		older, newer string
		want         bool
	}{
		{name: "ancestor", older: base, newer: later, want: true},
		{name: "non-ancestor", older: later, newer: base},
		{name: "unrelated histories", older: unrelated, newer: later},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := repo.Ancestor(ctx, test.older, test.newer)
			if err != nil || got != test.want {
				t.Fatalf("Ancestor = %v, %v; want %v", got, err, test.want)
			}
		})
	}
	if _, err := repo.Ancestor(ctx, "no-such-revision", later); err == nil {
		t.Fatal("an unresolvable revision was answered rather than raised")
	}
}

// TestValidateRefNameAcceptsSlugsAndRejectsMalformedNames pins the probe the
// effort slug minter depends on: the shapes it must accept and the three
// malformed shapes a derived slug could plausibly produce.
func TestValidateRefNameAcceptsSlugsAndRejectsMalformedNames(t *testing.T) {
	repo, _ := lifecycleRepo(t)
	ctx := testContext(t)
	for name, want := range map[string]bool{
		"awf/single-home-and-git-seam": true,
		"awf/..double-dot":             false,
		"awf/with space":               false,
		"awf/trailing/":                false,
	} {
		t.Run(name, func(t *testing.T) {
			got, err := repo.ValidateRefName(ctx, name)
			if err != nil || got != want {
				t.Fatalf("ValidateRefName(%q) = %v, %v; want %v", name, got, err, want)
			}
		})
	}
}

// TestChangeCountsSeparatesEveryDirtyTreeState pins the one cleanliness oracle
// on each edge its two consumers rely on: the audit rule that reports
// uncommitted work and the worktree manager's refusal to mutate a dirty
// checkout.
func TestChangeCountsSeparatesEveryDirtyTreeState(t *testing.T) {
	ctx := testContext(t)
	for _, test := range []struct {
		name               string
		dirty              func(t *testing.T, repo *gitfixtureRepo)
		tracked, untracked int
	}{
		{name: "clean"},
		{
			name: "staged only",
			dirty: func(t *testing.T, f *gitfixtureRepo) {
				gitfixture.Stage(t, f.repo, f.dir, map[string]string{"tracked.txt": "staged\n"})
			},
			tracked: 1,
		},
		{
			name: "unstaged only",
			dirty: func(t *testing.T, f *gitfixtureRepo) {
				if err := os.WriteFile(filepath.Join(f.dir, "tracked.txt"), []byte("unstaged\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			tracked: 1,
		},
		{
			name: "untracked only",
			dirty: func(t *testing.T, f *gitfixtureRepo) {
				if err := os.WriteFile(filepath.Join(f.dir, "loose.txt"), []byte("loose\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			untracked: 1,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo, dir := gitfixture.InitRepo(t)
			gitfixture.Commit(t, repo, dir, "base", map[string]string{"tracked.txt": "base\n"})
			handle := statusRepo(t, dir)
			if test.dirty != nil {
				test.dirty(t, &gitfixtureRepo{repo: repo, dir: dir})
			}
			tracked, untracked, err := handle.ChangeCounts(ctx)
			if err != nil || tracked != test.tracked || untracked != test.untracked {
				t.Fatalf("ChangeCounts = (%d, %d), %v; want (%d, %d)", tracked, untracked, err, test.tracked, test.untracked)
			}
		})
	}
}

// TestRevisionAndBranchReadsReportRepositoryState pins the reads the worktree
// manager compares identities and integration targets with.
func TestRevisionAndBranchReadsReportRepositoryState(t *testing.T) {
	repo, dir := lifecycleRepo(t)
	ctx := testContext(t)
	head := lifecycleGit(t, dir, "rev-parse", "HEAD")
	if got, err := repo.ResolveCommit(ctx, "HEAD"); err != nil || got != head {
		t.Fatalf("ResolveCommit = %q, %v; want %q", got, err, head)
	}
	if _, err := repo.ResolveCommit(ctx, "no-such-revision"); err == nil {
		t.Fatal("unresolvable revision accepted")
	}
	branch, err := repo.CurrentBranch(ctx)
	if err != nil || branch == "" {
		t.Fatalf("CurrentBranch = %q, %v; want the checked-out branch", branch, err)
	}
	lifecycleGit(t, dir, "checkout", "--detach")
	if detached, err := repo.CurrentBranch(ctx); err != nil || detached != "" {
		t.Fatalf("CurrentBranch while detached = %q, %v; want an empty answer", detached, err)
	}
	mergeHead, err := repo.GitPath(ctx, "MERGE_HEAD")
	if err != nil || !filepath.IsAbs(mergeHead) || filepath.Base(mergeHead) != "MERGE_HEAD" {
		t.Fatalf("GitPath = %q, %v; want an absolute MERGE_HEAD path", mergeHead, err)
	}
}

// TestMergeEntrypointsAdvanceAndStageWithoutCommitting pins the two integration
// mutations: the fast-forward that must create no commit of its own, and the
// divergent merge that must stop before committing.
func TestMergeEntrypointsAdvanceAndStageWithoutCommitting(t *testing.T) {
	ctx := testContext(t)
	t.Run("fast forward", func(t *testing.T) {
		repo, dir := lifecycleRepo(t)
		lifecycleGit(t, dir, "checkout", "-b", "ahead")
		lifecycleGit(t, dir, "commit", "--allow-empty", "-m", "ahead")
		tip := lifecycleGit(t, dir, "rev-parse", "HEAD")
		lifecycleGit(t, dir, "checkout", "-")
		if err := repo.MergeFastForward(ctx, "ahead"); err != nil {
			t.Fatal(err)
		}
		if got := lifecycleGit(t, dir, "rev-parse", "HEAD"); got != tip {
			t.Fatalf("HEAD after fast-forward = %q, want the branch tip %q", got, tip)
		}
	})
	t.Run("divergent", func(t *testing.T) {
		repo, dir := lifecycleRepo(t)
		base := lifecycleGit(t, dir, "rev-parse", "HEAD")
		lifecycleGit(t, dir, "checkout", "-b", "sideways")
		lifecycleGit(t, dir, "commit", "--allow-empty", "-m", "sideways")
		lifecycleGit(t, dir, "checkout", "-")
		lifecycleGit(t, dir, "commit", "--allow-empty", "-m", "onwards")
		target := lifecycleGit(t, dir, "rev-parse", "HEAD")
		if got, err := repo.MergeBase(ctx, "HEAD", "sideways"); err != nil || got != base {
			t.Fatalf("MergeBase = %q, %v; want %q", got, err, base)
		}
		if err := repo.MergeNoCommit(ctx, "sideways"); err != nil {
			t.Fatal(err)
		}
		if got := lifecycleGit(t, dir, "rev-parse", "HEAD"); got != target {
			t.Fatalf("HEAD after --no-commit merge = %q, want the unchanged target %q", got, target)
		}
		mergeHead, err := repo.GitPath(ctx, "MERGE_HEAD")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(mergeHead); err != nil {
			t.Fatalf("merge state is not visible: %v", err)
		}
	})
}

// TestLifecycleReadsRefuseAnUnusableRepositoryAndResponse pins the failure half
// of the reads whose success half the fixtures above cover: an operation
// against a path that carries no repository raises, and a revision response
// that is not a full object ID is refused rather than compared.
func TestLifecycleReadsRefuseAnUnusableRepositoryAndResponse(t *testing.T) {
	ctx := testContext(t)
	// The handle is built directly here because Open deliberately refuses a
	// non-repository, and these are exactly the per-operation failures a caller
	// sees when a checkout stops being one underneath an opened handle.
	absent := &Repo{root: t.TempDir(), runner: newRunner(t.TempDir())}
	if _, err := absent.CurrentBranch(ctx); err == nil {
		t.Fatal("CurrentBranch outside a repository was answered rather than raised")
	}
	if _, err := absent.GitPath(ctx, "MERGE_HEAD"); err == nil {
		t.Fatal("GitPath outside a repository was answered rather than raised")
	}

	if runtime.GOOS == "windows" {
		t.Skip("the truncated-response fixture requires a POSIX script")
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\necho short\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	truncated := &Repo{root: bin, runner: newRunner(bin)}
	if _, err := truncated.ResolveCommit(ctx, "HEAD"); err == nil || !strings.Contains(err.Error(), "invalid object ID") {
		t.Fatalf("truncated revision response error = %v, want an invalid-object-ID refusal", err)
	}
}

// gitfixtureRepo carries the two fixture handles a dirty-state setup needs.
type gitfixtureRepo struct {
	repo *gogit.Repository
	dir  string
}
