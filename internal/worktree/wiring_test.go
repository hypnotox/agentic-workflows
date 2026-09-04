package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/effort"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

// openCheckout is the production binding: the Git seam's handle satisfies the
// manager's checkout contract with no adapter between them.
func openCheckout(root string) (Runner, error) { return awfgit.Open(root) }

func openResidentForRoots(roots awfgit.ControlRoots) OpenResident {
	return func(name awfgit.ResidentName) (ResidentHandle, error) {
		root, err := roots.ResidentRoot(name)
		if err != nil {
			return nil, err
		}
		return filesystem.Open(root)
	}
}

// checkoutStub answers exactly like the checkout it wraps except where a test
// sets an override. Every operation is listed so an override is chosen by name
// rather than by matching argument text, which is what makes a fault injected
// here unambiguous about which question failed.
type checkoutStub struct {
	Runner
	worktreeList     func(ctx context.Context) ([]awfgit.WorktreeRegistration, error)
	worktreeAdd      func(ctx context.Context, path, branch, base string) error
	worktreeRemove   func(ctx context.Context, path string) error
	worktreePrune    func(ctx context.Context) error
	branchProbe      func(ctx context.Context, name string) (bool, error)
	branchDelete     func(ctx context.Context, name string) error
	ancestor         func(ctx context.Context, older, newer string) (bool, error)
	mergeBase        func(ctx context.Context, a, b string) (string, error)
	mergeFastForward func(ctx context.Context, rev string) error
	mergeNoCommit    func(ctx context.Context, rev string) error
	resolveCommit    func(ctx context.Context, revision string) (string, error)
	currentBranch    func(ctx context.Context) (string, error)
	changeCounts     func(ctx context.Context) (int, int, error)
	ignoredPaths     func(ctx context.Context) ([]string, error)
	gitPath          func(ctx context.Context, name string) (string, error)
}

func (s *checkoutStub) WorktreeList(ctx context.Context) ([]awfgit.WorktreeRegistration, error) {
	if s.worktreeList != nil {
		return s.worktreeList(ctx)
	}
	return s.Runner.WorktreeList(ctx)
}

func (s *checkoutStub) WorktreeAdd(ctx context.Context, path, branch, base string) error {
	if s.worktreeAdd != nil {
		return s.worktreeAdd(ctx, path, branch, base)
	}
	return s.Runner.WorktreeAdd(ctx, path, branch, base)
}

func (s *checkoutStub) WorktreeRemove(ctx context.Context, path string) error {
	if s.worktreeRemove != nil {
		return s.worktreeRemove(ctx, path)
	}
	return s.Runner.WorktreeRemove(ctx, path)
}

func (s *checkoutStub) WorktreePrune(ctx context.Context) error {
	if s.worktreePrune != nil {
		return s.worktreePrune(ctx)
	}
	return s.Runner.WorktreePrune(ctx)
}

func (s *checkoutStub) BranchExists(ctx context.Context, name string) (bool, error) {
	if s.branchProbe != nil {
		return s.branchProbe(ctx, name)
	}
	return s.Runner.BranchExists(ctx, name)
}

func (s *checkoutStub) BranchDelete(ctx context.Context, name string) error {
	if s.branchDelete != nil {
		return s.branchDelete(ctx, name)
	}
	return s.Runner.BranchDelete(ctx, name)
}

func (s *checkoutStub) Ancestor(ctx context.Context, older, newer string) (bool, error) {
	if s.ancestor != nil {
		return s.ancestor(ctx, older, newer)
	}
	return s.Runner.Ancestor(ctx, older, newer)
}

func (s *checkoutStub) MergeBase(ctx context.Context, a, b string) (string, error) {
	if s.mergeBase != nil {
		return s.mergeBase(ctx, a, b)
	}
	return s.Runner.MergeBase(ctx, a, b)
}

func (s *checkoutStub) MergeFastForward(ctx context.Context, rev string) error {
	if s.mergeFastForward != nil {
		return s.mergeFastForward(ctx, rev)
	}
	return s.Runner.MergeFastForward(ctx, rev)
}

func (s *checkoutStub) MergeNoCommit(ctx context.Context, rev string) error {
	if s.mergeNoCommit != nil {
		return s.mergeNoCommit(ctx, rev)
	}
	return s.Runner.MergeNoCommit(ctx, rev)
}

func (s *checkoutStub) ResolveCommit(ctx context.Context, revision string) (string, error) {
	if s.resolveCommit != nil {
		return s.resolveCommit(ctx, revision)
	}
	return s.Runner.ResolveCommit(ctx, revision)
}

func (s *checkoutStub) CurrentBranch(ctx context.Context) (string, error) {
	if s.currentBranch != nil {
		return s.currentBranch(ctx)
	}
	return s.Runner.CurrentBranch(ctx)
}

func (s *checkoutStub) ChangeCounts(ctx context.Context) (int, int, error) {
	if s.changeCounts != nil {
		return s.changeCounts(ctx)
	}
	return s.Runner.ChangeCounts(ctx)
}

func (s *checkoutStub) IgnoredPaths(ctx context.Context) ([]string, error) {
	if s.ignoredPaths != nil {
		return s.ignoredPaths(ctx)
	}
	return s.Runner.IgnoredPaths(ctx)
}

func (s *checkoutStub) GitPath(ctx context.Context, name string) (string, error) {
	if s.gitPath != nil {
		return s.gitPath(ctx, name)
	}
	return s.Runner.GitPath(ctx, name)
}

// stubOpener opens the real checkout at each root and lets apply override the
// operations that test needs to control, per root.
func stubOpener(apply func(root string, stub *checkoutStub)) OpenCheckout {
	return func(root string) (Runner, error) {
		checkout, err := awfgit.Open(root)
		if err != nil {
			return nil, err
		}
		stub := &checkoutStub{Runner: checkout}
		apply(root, stub)
		return stub, nil
	}
}

// invokingStub overrides only the invoking checkout, leaving a managed checkout
// answering truthfully.
func invokingStub(invoking string, apply func(stub *checkoutStub)) OpenCheckout {
	return stubOpener(func(root string, stub *checkoutStub) {
		if sameWorktreePath(root, invoking) {
			apply(stub)
		}
	})
}

func sameWorktreePath(left, right string) bool {
	return filesystem.NormalizePlatformPath(left) == filesystem.NormalizePlatformPath(right)
}

// failing is the shape every injected fault takes: a named refusal a test can
// assert on without depending on any Git message.
func failing(name string) error { return errors.New("injected " + name) }

// mergedTip and otherTip are the two commits a topology test needs: the one
// MERGE_HEAD names, and any commit that is not it.
const (
	mergedTip = "0000000000000000000000000000000000000001"
	otherTip  = "0000000000000000000000000000000000000002"
)

// markerAt answers an existing path for one Git operation marker and an absent
// path for every other, which is how a test puts a checkout mid-operation.
func markerAt(t *testing.T, present string) func(context.Context, string) (string, error) {
	t.Helper()
	marker := filepath.Join(t.TempDir(), present)
	if err := os.WriteFile(marker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	absent := t.TempDir()
	return func(_ context.Context, name string) (string, error) {
		if name == present {
			return marker, nil
		}
		return filepath.Join(absent, name), nil
	}
}

// mergeHeadAt answers commit for MERGE_HEAD and a different commit for every
// other revision, so a test proves attribution reads the merged tip from
// MERGE_HEAD and not from whatever revision it happens to ask for.
func mergeHeadAt(commit string) func(context.Context, string) (string, error) {
	return func(_ context.Context, revision string) (string, error) {
		if revision == "MERGE_HEAD" {
			return commit, nil
		}
		return otherTip, nil
	}
}

// registrations answers a fixed worktree listing, so a test states the managed
// topology it exercises as data rather than by building one.
func registrations(listed ...awfgit.WorktreeRegistration) func(context.Context) ([]awfgit.WorktreeRegistration, error) {
	return func(context.Context) ([]awfgit.WorktreeRegistration, error) { return listed, nil }
}

// worktreeControlRoots resolves the control roots the composed command would.
func worktreeControlRoots(t *testing.T, root string) awfgit.ControlRoots {
	t.Helper()
	roots, err := awfgit.ResolveControlRoots(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	return roots
}

// newEffortService composes the effort authority the manager owns, wired to the
// same seam handle the production composition uses.
func newEffortService(t *testing.T, roots awfgit.ControlRoots, uuid func() (string, error), fault func(string) error) *effort.Service {
	t.Helper()
	repo, err := awfgit.Open(roots.InvokingRoot)
	if err != nil {
		t.Fatal(err)
	}
	if uuid == nil {
		uuid = effort.RandomUUIDv4
	}
	service, err := effort.Open(roots, effort.Dependencies{
		Clock:                 time.Now,
		UUID:                  uuid,
		Worktrees:             repo.WorktreeList,
		BranchExists:          repo.BranchExists,
		ValidateRef:           repo.ValidateRefName,
		ExpectedArchiveMarker: func() ([]byte, error) { return []byte("marker\n"), nil },
		Fault:                 fault,
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

// managerWith composes a manager over root with the given checkout opener.
func managerWith(t *testing.T, root string, open OpenCheckout) *Manager {
	t.Helper()
	roots := worktreeControlRoots(t, root)
	manager, err := Open(roots, open, openResidentForRoots(roots))
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

// managerRooted composes a manager whose control roots have drifted from the
// checkout it actually reads. The opener deliberately answers with the real
// checkout whatever root it is handed, so the drift lives entirely in what the
// manager believes its roots are - which is what the identity refusals guard.
func managerRooted(t *testing.T, root string, drift func(*awfgit.ControlRoots), apply func(*checkoutStub)) *Manager {
	t.Helper()
	roots := worktreeControlRoots(t, root)
	drifted := roots
	drift(&drifted)
	drifted.InvokingRoot = filesystem.NormalizePlatformPath(drifted.InvokingRoot)
	open := func(string) (Runner, error) {
		checkout, err := awfgit.Open(root)
		if err != nil {
			return nil, err
		}
		stub := &checkoutStub{Runner: checkout}
		if apply != nil {
			apply(stub)
		}
		return stub, nil
	}
	manager, err := Open(drifted, open, openResidentForRoots(drifted))
	if err != nil {
		t.Fatal(err)
	}
	return manager
}
