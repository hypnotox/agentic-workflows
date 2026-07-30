package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/effort"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

type Options struct {
	Runner Runner
}

// Result is the line-oriented mutation protocol plus structured facts
// for orchestration; String() remains the only text surface.
type Result struct {
	Condition       string
	ChangedTopology bool
	NextAction      string
	Path            string
	Branch          string
}

func (r Result) String() string {
	return fmt.Sprintf("%s; changed topology: %s; next action: %s", r.Condition, yesNo(r.ChangedTopology), r.NextAction)
}

type Manager struct {
	ctx     context.Context
	roots   awfgit.ControlRoots
	efforts *effort.Service
	run     Runner
}

func Open(ctx context.Context, invoking string, options Options) (*Manager, error) {
	roots, err := awfgit.ResolveControlRoots(ctx, invoking)
	if err != nil {
		return nil, err
	}
	run := options.Runner
	if run == nil {
		run = nativeRunner
	}
	service, err := effort.Open(ctx, invoking, effort.Options{})
	if err != nil { // coverage-ignore: the same control-root proof just succeeded; a second resolution failure requires a concurrent repository-identity race
		return nil, err
	}
	return &Manager{ctx: ctx, roots: roots, efforts: service, run: run}, nil
}

func (m *Manager) managed(slug string) (string, error) {
	root, err := m.roots.ResidentRoot(awfgit.ResidentWorktrees)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, slug), nil
}

func branch(slug string) string { return "awf/" + slug }

func (m *Manager) validateManagedTarget(path string) error {
	if _, err := m.roots.ResidentRoot(awfgit.ResidentWorktrees); err != nil {
		return err
	}
	if err := safeManagedPath(path); err != nil {
		return err
	}
	roots, err := awfgit.ResolveControlRoots(m.ctx, path)
	if err != nil {
		return &awfgit.HardSafetyError{Category: "repository-identity", Path: path, Err: err}
	}
	if filepath.Clean(roots.CommonDir) != filepath.Clean(m.roots.CommonDir) || filepath.Clean(roots.InvokingRoot) != filepath.Clean(path) {
		return &awfgit.HardSafetyError{Category: "repository-identity", Path: path, Err: errors.New("managed checkout belongs to a different repository identity")}
	}
	return nil
}

func (m *Manager) validateLiveInvokingCheckout() error {
	if err := safeManagedPath(m.roots.InvokingRoot); err != nil {
		return &awfgit.HardSafetyError{Category: "repository-identity", Path: m.roots.InvokingRoot, Err: err}
	}
	live, err := awfgit.ResolveControlRoots(m.ctx, m.roots.InvokingRoot)
	if err != nil {
		return &awfgit.HardSafetyError{Category: "repository-identity", Path: m.roots.InvokingRoot, Err: err}
	}
	if filepath.Clean(live.InvokingRoot) != filepath.Clean(m.roots.InvokingRoot) || filepath.Clean(live.CommonDir) != filepath.Clean(m.roots.CommonDir) || filepath.Clean(live.PrimaryRoot) != filepath.Clean(m.roots.PrimaryRoot) {
		return &awfgit.HardSafetyError{Category: "repository-identity", Path: m.roots.InvokingRoot, Err: errors.New("invoking checkout identity changed")}
	}
	return nil
}

func (m *Manager) operationFree(path string) error {
	for _, name := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "rebase-merge", "rebase-apply"} {
		out, err := m.run(m.ctx, path, "rev-parse", "--git-path", name)
		if err != nil {
			return err
		}
		candidate := strings.TrimSpace(string(out))
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(path, candidate)
		}
		if _, err = os.Lstat(candidate); err == nil {
			return refusal("operation", "checkout has an in-progress Git operation", false, "finish or abort the native Git operation, then retry")
		} else if !errors.Is(err, os.ErrNotExist) { // coverage-ignore: local lstat reports an inode or os.ErrNotExist absent a kernel fault
			return err
		}
	}
	return nil
}

func (m *Manager) Add(slug, base string) (Result, error) {
	if _, err := m.efforts.Show(slug); err != nil {
		return Result{}, err
	}
	path, err := m.managed(slug)
	if err != nil {
		return Result{}, err
	}
	if _, err := managedLstat(path); err == nil {
		return Result{}, refusal("topology", "managed path already exists", false, "inspect the existing path and retry add only after safe manual cleanup")
	} else if !errors.Is(err, os.ErrNotExist) {
		return Result{}, err
	}
	regs, err := registrations(m.ctx, m.run, m.roots.InvokingRoot)
	if err != nil {
		return Result{}, err
	}
	wantBranch := "refs/heads/" + branch(slug)
	for _, registration := range regs {
		if filepath.Clean(registration.path) == filepath.Clean(path) || registration.branch == wantBranch {
			return Result{}, refusal("topology", "managed registration or branch is already present", false, "inspect `git worktree list --porcelain` and retry after safe cleanup")
		}
	}
	exists, err := branchExists(m.ctx, m.run, m.roots.InvokingRoot, branch(slug))
	if err != nil {
		return Result{}, err
	}
	if exists {
		return Result{}, refusal("topology", "managed branch already exists", false, "inspect the branch and retry after safe cleanup")
	}
	if err := m.operationFree(m.roots.InvokingRoot); err != nil {
		return Result{}, err
	}
	if base == "" {
		base = "HEAD"
	}
	full, err := resolve(m.ctx, m.run, m.roots.InvokingRoot, base)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil { // coverage-ignore: ResidentRoot proved the owned resident ancestry; failure requires a concurrent namespace or storage fault
		return Result{}, fmt.Errorf("create managed worktree root: %w", err)
	}
	if err := m.validateLiveInvokingCheckout(); err != nil {
		return Result{}, err
	}
	if _, err := m.run(m.ctx, m.roots.InvokingRoot, "worktree", "add", "-b", branch(slug), path, full); err != nil {
		changed := m.topologyPresent(slug, path)
		return Result{}, refusalCause("add", "git worktree add failed", changed, "inspect actual Git topology and retry add, or clean only the named path, registration, and branch with native Git", err)
	}
	if err := exactRegistration(m.ctx, m.run, m.roots.InvokingRoot, path, wantBranch); err != nil {
		return Result{}, refusalCause("repository-identity", "Git add returned without exact managed registration", true, "inspect actual Git topology and perform safe native-Git cleanup before retrying", err)
	}
	return Result{
		Condition: "managed worktree added for " + slug, ChangedTopology: true,
		NextAction: "continue the effort in " + path, Path: path, Branch: branch(slug),
	}, nil
}

// NewEffort publishes the effort residents, then creates its managed
// worktree via the same standalone Add machinery (ADR-0189). On Add
// failure it rolls back through restartable finish, removing the effort
// only when the finish flow proves managed topology absent.
func (m *Manager) NewEffort(title, base string) (effort.Record, Result, error) {
	record, err := m.efforts.New(title)
	if err != nil {
		return effort.Record{}, Result{}, err
	}
	result, addErr := m.Add(record.Slug, base)
	if addErr == nil {
		return record, result, nil
	}
	return effort.Record{}, Result{}, m.rollback(record.Slug, addErr)
}

// rollback composes the one creation failure from the structured finish
// outcome and the managed-topology classification, never from error prose.
func (m *Manager) rollback(slug string, addErr error) error {
	finishResult, finishErr := m.efforts.Finish(slug)
	switch {
	case finishErr == nil:
		return fmt.Errorf("worktree creation failed: %w; effort %s rolled back; next action: fix the reported cause and retry `awf effort new`", addErr, slug)
	case errors.Is(finishErr, effort.ErrManagedTopologyPresent):
		return fmt.Errorf("worktree creation failed: %w; effort %s retained: managed topology remains; next action: inspect `git worktree list --porcelain`, clean up with native Git or `awf effort worktree remove %s`, then retry `awf effort worktree add %s` or finish the effort", addErr, slug, slug, slug)
	case finishResult.Renamed:
		return fmt.Errorf("worktree creation failed: %w; effort %s rollback interrupted after rename: %w; next action: retry `awf effort finish %s`", addErr, slug, finishErr, slug)
	default:
		return fmt.Errorf("worktree creation failed: %w; effort %s retained: rollback failed: %w; next action: resolve the rollback failure, then retry `awf effort worktree add %s` or `awf effort finish %s`", addErr, slug, finishErr, slug, slug)
	}
}

func (m *Manager) topologyPresent(slug, path string) bool {
	if _, err := managedLstat(path); err == nil {
		return true
	}
	regs, err := registrations(m.ctx, m.run, m.roots.InvokingRoot)
	if err == nil {
		for _, registration := range regs {
			if filepath.Clean(registration.path) == filepath.Clean(path) || registration.branch == "refs/heads/"+branch(slug) {
				return true
			}
		}
	}
	exists, err := branchExists(m.ctx, m.run, m.roots.InvokingRoot, branch(slug))
	return err != nil || exists
}

func (m *Manager) Integrate(slug, gateCommand string) (Result, error) {
	if _, err := m.efforts.Show(slug); err != nil {
		return Result{}, err
	}
	path, err := m.managed(slug)
	if err != nil {
		return Result{}, err
	}
	if filepath.Clean(path) == filepath.Clean(m.roots.InvokingRoot) {
		return Result{}, refusal("repository-identity", "integration must run from the receiving checkout, not the managed worktree", false, "change to the intended clean target checkout and retry")
	}
	if err := m.validateManagedTarget(path); err != nil {
		return Result{}, err
	}
	if err := exactRegistration(m.ctx, m.run, m.roots.InvokingRoot, path, "refs/heads/"+branch(slug)); err != nil {
		return Result{}, err
	}
	if err := m.operationFree(m.roots.InvokingRoot); err != nil {
		return Result{}, err
	}
	if err := status(m.ctx, m.run, m.roots.InvokingRoot); err != nil {
		return Result{}, err
	}
	targetBranchRaw, err := m.run(m.ctx, m.roots.InvokingRoot, "symbolic-ref", "-q", "--short", "HEAD")
	if err != nil {
		return Result{}, refusal("topology", "receiving checkout is detached", false, "check out the intended target branch and retry")
	}
	if strings.TrimSpace(string(targetBranchRaw)) == branch(slug) {
		return Result{}, refusal("topology", "receiving checkout is the effort branch", false, "change to the intended target branch and retry")
	}
	tip, err := resolve(m.ctx, m.run, m.roots.InvokingRoot, branch(slug))
	if err != nil {
		return Result{}, err
	}
	target, err := resolve(m.ctx, m.run, m.roots.InvokingRoot, "HEAD")
	if err != nil {
		return Result{}, err
	}
	already, err := ancestor(m.ctx, m.run, m.roots.InvokingRoot, tip, target)
	if err != nil {
		return Result{}, err
	}
	if already {
		return Result{Condition: "effort tip is already integrated into the target", ChangedTopology: false, NextAction: "run `awf effort worktree remove " + slug + "` after terminal review is settled"}, nil
	}
	fastForward, err := ancestor(m.ctx, m.run, m.roots.InvokingRoot, target, tip)
	if err != nil {
		return Result{}, err
	}
	if err := m.validateIntegrationFacts(path, slug, target, tip); err != nil {
		return Result{}, err
	}
	if fastForward {
		if _, err := m.run(m.ctx, m.roots.InvokingRoot, "merge", "--ff-only", branch(slug)); err != nil {
			return Result{}, refusalCause("integration", "fast-forward failed", m.targetChanged(target), "inspect the receiving checkout and retry only from clean verified topology", err)
		}
		return Result{Condition: "target fast-forwarded to effort tip", ChangedTopology: true, NextAction: "settle terminal review, then remove the managed worktree"}, nil
	}
	base, err := m.run(m.ctx, m.roots.InvokingRoot, "merge-base", "HEAD", branch(slug))
	if err != nil || strings.TrimSpace(string(base)) == "" {
		return Result{}, refusalCause("ancestry", "target and effort have no proven common ancestor", false, "inspect repository and branch identity; do not use --allow-unrelated-histories", err)
	}
	gateStep := integrationGateStep(gateCommand)
	if _, err := m.run(m.ctx, m.roots.InvokingRoot, "merge", "--no-ff", "--no-commit", branch(slug)); err != nil {
		return Result{}, refusalCause("merge-conflict", "divergent integration stopped with visible conflict state", true, "resolve or abort the merge; after resolution run `./awf check --staged`, "+gateStep+", commit, and renew terminal review", err)
	}
	return Result{Condition: "divergent integration is staged without a commit", ChangedTopology: true, NextAction: "run `./awf check --staged`, " + gateStep + ", commit the merge, and renew terminal implementation review"}, nil
}

func integrationGateStep(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return "the project gate"
	}
	return "`" + command + "`"
}

func (m *Manager) validateIntegrationFacts(path, slug, target, tip string) error {
	if err := m.validateLiveInvokingCheckout(); err != nil {
		return err
	}
	if err := m.validateManagedTarget(path); err != nil {
		return err
	}
	if err := exactRegistration(m.ctx, m.run, m.roots.InvokingRoot, path, "refs/heads/"+branch(slug)); err != nil {
		return err
	}
	currentTarget, err := resolve(m.ctx, m.run, m.roots.InvokingRoot, "HEAD")
	if err != nil || currentTarget != target {
		return refusalCause("topology", "target HEAD changed during integration preflight", false, "restart integration from the clean intended target checkout", err)
	}
	currentTip, err := resolve(m.ctx, m.run, m.roots.InvokingRoot, branch(slug))
	if err != nil || currentTip != tip {
		return refusalCause("topology", "effort branch changed during integration preflight", false, "restart integration after the effort writer settles the branch", err)
	}
	return nil
}

func (m *Manager) targetChanged(before string) bool {
	after, err := resolve(m.ctx, m.run, m.roots.InvokingRoot, "HEAD")
	return err != nil || after != before
}

func (m *Manager) Remove(slug string) (Result, error) {
	if _, err := m.efforts.Show(slug); err != nil {
		return Result{}, err
	}
	path, err := m.managed(slug)
	if err != nil {
		return Result{}, err
	}
	if filepath.Clean(path) == filepath.Clean(m.roots.InvokingRoot) {
		return Result{}, refusal("repository-identity", "removal must run from the intended target checkout", false, "change to the target checkout and retry")
	}
	if err := m.operationFree(m.roots.InvokingRoot); err != nil {
		return Result{}, err
	}
	if err := status(m.ctx, m.run, m.roots.InvokingRoot); err != nil {
		return Result{}, err
	}
	changed := false
	for {
		pathPresent := false
		if _, statErr := managedLstat(path); statErr == nil {
			pathPresent = true
		} else if !errors.Is(statErr, os.ErrNotExist) { // coverage-ignore: local lstat reports an inode or os.ErrNotExist absent a kernel fault
			return Result{}, statErr
		}
		regs, regsErr := registrations(m.ctx, m.run, m.roots.InvokingRoot)
		if regsErr != nil {
			return Result{}, regsErr
		}
		var exact *registration
		for index := range regs {
			registration := &regs[index]
			if filepath.Clean(registration.path) == filepath.Clean(path) {
				if exact != nil || registration.branch != "refs/heads/"+branch(slug) || registration.detached || registration.bare {
					return Result{}, refusal("repository-identity", "managed path registration is not exact", changed, "inspect native Git topology and clean it manually without discarding work")
				}
				exact = registration
			}
			if registration.branch == "refs/heads/"+branch(slug) && filepath.Clean(registration.path) != filepath.Clean(path) {
				return Result{}, refusal("repository-identity", "managed branch is registered at a foreign path", changed, "inspect native Git topology and clean it manually without discarding work")
			}
		}
		branchPresent, branchErr := branchExists(m.ctx, m.run, m.roots.InvokingRoot, branch(slug))
		if branchErr != nil {
			return Result{}, branchErr
		}
		if !pathPresent && exact == nil && !branchPresent {
			return Result{Condition: "managed worktree topology is absent", ChangedTopology: changed, NextAction: "continue to retrospective, then finish the effort"}, nil
		}
		if branchPresent {
			merged, ancestryErr := ancestor(m.ctx, m.run, m.roots.InvokingRoot, branch(slug), "HEAD")
			if ancestryErr != nil {
				return Result{}, ancestryErr
			}
			if !merged {
				return Result{}, refusal("ancestry", "managed branch is not merged into the target", changed, "integrate and settle terminal review, or inspect and discard explicitly with native Git")
			}
		}
		if pathPresent {
			if err := m.validateManagedTarget(path); err != nil {
				return Result{}, err
			}
			if err := m.operationFree(path); err != nil {
				return Result{}, err
			}
			if err := status(m.ctx, m.run, path); err != nil {
				return Result{}, refusalCause("cleanliness", "managed worktree is dirty", changed, "commit or explicitly inspect and discard changes with native Git, then retry ordinary removal", err)
			}
			if exact != nil {
				if _, err := m.run(m.ctx, m.roots.InvokingRoot, "worktree", "remove", path); err != nil {
					return Result{}, refusalCause("removal", "native Git worktree removal failed", changed, "inspect actual topology and retry ordinary removal", err)
				}
			} else {
				if err := os.RemoveAll(path); err != nil { // coverage-ignore: path identity and cleanliness were just proven; recursive removal failure requires a concurrent namespace or storage fault
					return Result{}, refusalCause("removal", "proven unregistered managed path cleanup failed", changed, "inspect the path and retry ordinary removal", err)
				}
			}
			changed = true
			continue
		}
		if exact != nil {
			if _, err := m.run(m.ctx, m.roots.InvokingRoot, "worktree", "prune", "--expire", "now"); err != nil {
				return Result{}, refusalCause("removal", "prunable registration cleanup failed", changed, "inspect `git worktree list --porcelain` and retry", err)
			}
			changed = true
			continue
		}
		if branchPresent {
			if _, err := m.run(m.ctx, m.roots.InvokingRoot, "branch", "-d", branch(slug)); err != nil {
				return Result{}, refusalCause("removal", "safe managed branch deletion failed", changed, "inspect branch ancestry and retry without force", err)
			}
			changed = true
		}
	}
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
