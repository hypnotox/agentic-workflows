package git_test

import (
	"os"
	"path/filepath"
	"testing"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

// The topology entrypoints answer "which checkouts does this repository have"
// and "which of them owns resident state". Their contract is pinned here on real
// registered worktrees, independent of which backend serves them. These cases
// are serial: they build native repositories and read process state.

// TestListWorktreeRegistrationsReportsEveryRegisteredCheckout proves the list is
// repository-wide rather than checkout-local: the same registrations come back
// whichever checkout asks, each with an absolute path and its own HEAD shape.
func TestListWorktreeRegistrationsReportsEveryRegisteredCheckout(t *testing.T) {
	base := filepath.Join(t.TempDir(), "registration fixtures")
	primary := filepath.Join(base, "primary checkout")
	initNativeRepo(t, primary)
	branched := filepath.Join(base, "branched checkout")
	runGit(t, "-C", primary, "worktree", "add", "-b", "side", branched)
	detached := filepath.Join(base, "detached checkout")
	runGit(t, "-C", primary, "worktree", "add", "--detach", detached, "HEAD")

	// The primary's branch name comes from git's own default, which varies by
	// installation, so the expectation is read back rather than hardcoded.
	primaryBranch := trimGitOutputLine(runGit(t, "-C", primary, "symbolic-ref", "HEAD"))
	// The entrypoint is called here directly, not only through the helper below,
	// so this suite visibly exercises what it is registered against: a suite that
	// reaches its entrypoint only through a helper reads as unrelated to it.
	direct, err := awfgit.ListWorktreeRegistrations(testContext(t), primary)
	if err != nil {
		t.Fatal(err)
	}
	if len(direct) != 3 {
		t.Fatalf("ListWorktreeRegistrations returned %d registrations, want the primary and its two linked checkouts", len(direct))
	}
	fromPrimary := registrationsByPath(t, primary)
	fromLinked := registrationsByPath(t, detached)
	if len(fromPrimary) != 3 {
		t.Fatalf("registrations from the primary = %d, want the primary and its two linked checkouts", len(fromPrimary))
	}
	for path, want := range map[string]awfgit.WorktreeRegistration{
		cleanAbsolute(t, primary):  {Branch: primaryBranch},
		cleanAbsolute(t, branched): {Branch: "refs/heads/side"},
		cleanAbsolute(t, detached): {Detached: true},
	} {
		got, ok := fromPrimary[path]
		if !ok {
			t.Fatalf("registration for %q missing from %v", path, keys(fromPrimary))
		}
		if got.Branch != want.Branch || got.Detached != want.Detached {
			t.Fatalf("registration for %q = branch %q detached %v, want branch %q detached %v", path, got.Branch, got.Detached, want.Branch, want.Detached)
		}
		if got.HEAD == "" || got.Bare || got.Prunable {
			t.Fatalf("registration for %q = %#v, want a live non-bare checkout with a HEAD", path, got)
		}
		if !filepath.IsAbs(got.Path) {
			t.Fatalf("registration path %q is not absolute", got.Path)
		}
		if linked, ok := fromLinked[path]; !ok || linked != got {
			t.Fatalf("registration for %q seen from the linked checkout = %#v, want the same record %#v", path, linked, got)
		}
	}
}

// TestListWorktreeRegistrationsReportsRemovedCheckoutAsPrunable proves the list
// reports what Git has registered rather than what the filesystem still holds: a
// checkout whose directory is gone stays in the answer, marked prunable, instead
// of silently disappearing.
func TestListWorktreeRegistrationsReportsRemovedCheckoutAsPrunable(t *testing.T) {
	base := filepath.Join(t.TempDir(), "prunable fixtures")
	primary := filepath.Join(base, "primary checkout")
	initNativeRepo(t, primary)
	removed := filepath.Join(base, "removed checkout")
	runGit(t, "-C", primary, "worktree", "add", "--detach", removed, "HEAD")
	if err := os.RemoveAll(removed); err != nil {
		t.Fatal(err)
	}

	registrations := registrationsByPath(t, primary)
	got, ok := registrations[cleanAbsolute(t, removed)]
	if !ok {
		t.Fatalf("removed checkout dropped from %v", keys(registrations))
	}
	if !got.Prunable {
		t.Fatalf("registration for the removed checkout = %#v, want it marked prunable", got)
	}
}

// TestControlRootsAgreeWithRegisteredTopology proves the two topology
// entrypoints answer from the same repository: every checkout that resolves
// control roots names a primary that the registration list also carries, and the
// resident root hangs off that primary rather than off the invoking checkout.
func TestControlRootsAgreeWithRegisteredTopology(t *testing.T) {
	base := filepath.Join(t.TempDir(), "agreement fixtures")
	primary := filepath.Join(base, "primary checkout")
	initNativeRepo(t, primary)
	linked := filepath.Join(base, "linked checkout")
	runGit(t, "-C", primary, "worktree", "add", "--detach", linked, "HEAD")

	for _, invoking := range []string{primary, linked} {
		roots, err := awfgit.ResolveControlRoots(testContext(t), invoking)
		if err != nil {
			t.Fatalf("resolve control roots from %q: %v", invoking, err)
		}
		if roots.InvokingRoot != cleanAbsolute(t, invoking) {
			t.Fatalf("invoking root from %q = %q", invoking, roots.InvokingRoot)
		}
		if roots.PrimaryRoot != cleanAbsolute(t, primary) {
			t.Fatalf("primary root from %q = %q, want %q", invoking, roots.PrimaryRoot, cleanAbsolute(t, primary))
		}
		if _, ok := registrationsByPath(t, invoking)[roots.PrimaryRoot]; !ok {
			t.Fatalf("primary root %q is not a registered checkout", roots.PrimaryRoot)
		}
		resident, err := roots.ResidentRoot(awfgit.ResidentEfforts)
		if err != nil {
			t.Fatalf("resolve efforts resident root from %q: %v", invoking, err)
		}
		if want := filepath.Join(cleanAbsolute(t, primary), ".awf", "efforts"); resident != want {
			t.Fatalf("resident root from %q = %q, want %q", invoking, resident, want)
		}
	}
}

func registrationsByPath(t *testing.T, invoking string) map[string]awfgit.WorktreeRegistration {
	t.Helper()
	list, err := awfgit.ListWorktreeRegistrations(testContext(t), invoking)
	if err != nil {
		t.Fatalf("list worktree registrations from %q: %v", invoking, err)
	}
	byPath := map[string]awfgit.WorktreeRegistration{}
	for _, registration := range list {
		if _, duplicate := byPath[registration.Path]; duplicate {
			t.Fatalf("registration path %q listed twice", registration.Path)
		}
		byPath[registration.Path] = registration
	}
	return byPath
}

func keys(byPath map[string]awfgit.WorktreeRegistration) []string {
	var paths []string
	for path := range byPath {
		paths = append(paths, path)
	}
	return paths
}
