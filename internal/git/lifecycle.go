package git

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// This file holds the handle's checkout-lifecycle surface: the registration,
// branch, revision, and merge operations that create, inspect, and retire a
// managed worktree. They are native-Git operations by nature (go-git has no
// worktree registration model), so each one runs through the package runner and
// inherits its isolation, deadline requirement, and stderr-carrying errors.

// WorktreeAdd registers a new checkout at path on a newly created branch
// starting at base. It is the one entrypoint that creates a branch: the
// creation is inseparable from the registration Git performs with it.
func (r *Repo) WorktreeAdd(ctx context.Context, path, branch, base string) error {
	_, err := r.runner.run(ctx, "worktree", "add", "-b", branch, path, base)
	return err
}

// WorktreeRemove retires the registered checkout at path.
func (r *Repo) WorktreeRemove(ctx context.Context, path string) error {
	_, err := r.runner.run(ctx, "worktree", "remove", path)
	return err
}

// WorktreePrune drops registrations whose checkout is already gone. The
// immediate expiry is deliberate: a caller prunes only after proving the
// specific path absent, so honouring a grace period would leave the proven
// registration behind.
func (r *Repo) WorktreePrune(ctx context.Context) error {
	_, err := r.runner.run(ctx, "worktree", "prune", "--expire", "now")
	return err
}

// WorktreeList returns the repository's registrations as seen from this handle.
func (r *Repo) WorktreeList(ctx context.Context) ([]WorktreeRegistration, error) {
	return listWorktreeRegistrations(ctx, r.runner, r.root)
}

// BranchExists reports whether the local branch name exists. Absence is an
// answer, not a failure.
func (r *Repo) BranchExists(ctx context.Context, name string) (bool, error) {
	return r.runner.probe(ctx, "show-ref", "--verify", "--quiet", "refs/heads/"+name)
}

// BranchDelete deletes the local branch name, refusing when it is unmerged.
// The safe form is the only one offered: a caller that wants to discard
// unmerged work must do it explicitly with native Git rather than through awf.
func (r *Repo) BranchDelete(ctx context.Context, name string) error {
	_, err := r.runner.run(ctx, "branch", "-d", name)
	return err
}

// Ancestor reports whether older is an ancestor of newer. Unrelated histories
// answer false rather than failing, which is what makes this usable as the
// merged-ness test before a destructive operation.
func (r *Repo) Ancestor(ctx context.Context, older, newer string) (bool, error) {
	return r.runner.probe(ctx, "merge-base", "--is-ancestor", older, newer)
}

// ValidateRefName reports whether name is well formed as a branch name, which
// is the question every caller here asks. It validates the full refs/heads/
// form rather than passing --branch, and the difference is not cosmetic: the
// bare name form rejects a one-level name like "main" that is a perfectly valid
// branch, while --branch answers an invalid name with exit 128 instead of the
// exit 1 that means "no", so a malformed name would surface as a fault rather
// than a negative. Qualifying the name asks the branch question and keeps the
// exit-0/1 contract the probe helper depends on.
func (r *Repo) ValidateRefName(ctx context.Context, name string) (bool, error) {
	return r.runner.probe(ctx, "check-ref-format", "refs/heads/"+name)
}

// ResolveCommit resolves revision to the full object ID of the commit it names,
// failing when revision names no commit. The response is length-checked because
// a caller compares the returned identity against another: a truncated or
// abbreviated answer would compare unequal against a full one and be read as
// history having moved.
func (r *Repo) ResolveCommit(ctx context.Context, revision string) (string, error) {
	out, err := r.runner.run(ctx, "rev-parse", "--verify", revision+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", revision, err)
	}
	value := strings.TrimSuffix(string(out), "\n")
	if len(value) != 40 && len(value) != 64 {
		return "", fmt.Errorf("resolve %q: invalid object ID", revision)
	}
	return value, nil
}

// CurrentBranch returns the short name of the branch HEAD points at, or the
// empty string when HEAD is detached. A detached HEAD is a state of the
// repository, not a fault, so it is reported rather than raised.
func (r *Repo) CurrentBranch(ctx context.Context) (string, error) {
	out, err := r.runner.run(ctx, "symbolic-ref", "-q", "--short", "HEAD")
	if err != nil {
		var command *CommandError
		if errors.As(err, &command) && command.ExitCode == 1 {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// GitPath resolves a repository control file name (MERGE_HEAD, rebase-merge,
// and kin) to an absolute path. Git answers relative to the checkout for a
// primary worktree and absolutely for a linked one, and the worktree-private
// files live in the linked checkout's own Git directory rather than the common
// one, so resolution belongs to Git and the absolute form belongs here.
func (r *Repo) GitPath(ctx context.Context, name string) (string, error) {
	out, err := r.runner.run(ctx, "rev-parse", "--git-path", name)
	if err != nil {
		return "", err
	}
	candidate := strings.TrimSpace(string(out))
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(r.root, candidate)
	}
	return candidate, nil
}

// MergeFastForward advances HEAD to rev, refusing anything that would create a
// commit. It is the integration path for a branch that is strictly ahead.
func (r *Repo) MergeFastForward(ctx context.Context, rev string) error {
	_, err := r.runner.run(ctx, "merge", "--ff-only", rev)
	return err
}

// MergeNoCommit merges rev into HEAD and stops before committing, leaving the
// result (or the conflict) visible in the working tree. Divergent integration
// deliberately hands the outcome back to a person instead of committing it.
func (r *Repo) MergeNoCommit(ctx context.Context, rev string) error {
	_, err := r.runner.run(ctx, "merge", "--no-ff", "--no-commit", rev)
	return err
}
