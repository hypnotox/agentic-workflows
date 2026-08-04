// Package worktree manages native-Git effort worktrees.
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

// Runner is this package's own contract on one checkout's Git surface. It
// names exactly the operations the manager performs and nothing more, so the
// manager depends on what it uses rather than on a repository object; the
// composition root satisfies it with the Git seam's handle.
type Runner interface {
	WorktreeList(ctx context.Context) ([]awfgit.WorktreeRegistration, error)
	WorktreeAdd(ctx context.Context, path, branch, base string) error
	WorktreeRemove(ctx context.Context, path string) error
	WorktreePrune(ctx context.Context) error
	BranchExists(ctx context.Context, name string) (bool, error)
	BranchDelete(ctx context.Context, name string) error
	Ancestor(ctx context.Context, older, newer string) (bool, error)
	MergeBase(ctx context.Context, a, b string) (string, error)
	MergeFastForward(ctx context.Context, rev string) error
	MergeNoCommit(ctx context.Context, rev string) error
	ResolveCommit(ctx context.Context, revision string) (string, error)
	CurrentBranch(ctx context.Context) (string, error)
	ChangeCounts(ctx context.Context) (tracked, untracked int, err error)
	GitPath(ctx context.Context, name string) (string, error)
}

// OpenCheckout opens the Git surface of the checkout rooted at root. The
// manager reasons about two checkouts at once - the one it was invoked from and
// the managed one it is creating or retiring - so it opens each by root instead
// of holding a single handle.
type OpenCheckout func(root string) (Runner, error)

// Result carries structured managed-topology facts for orchestration and presentation mapping.
type Result struct {
	Condition       string
	ChangedTopology bool
	NextAction      string
	Path            string
	Branch          string
}

type Manager struct {
	roots   awfgit.ControlRoots
	efforts *effort.Service
	// git is the invoking checkout's surface, opened once with the roots it was
	// proven against; open serves the managed checkout, which exists only part
	// of the time.
	git  Runner
	open OpenCheckout
}

// Open composes a manager over the control roots and dependencies it is given.
// The roots arrive already resolved so one command resolves them once and the
// manager and its effort service provably reason about the same repository
// identity. Both dependencies are required: a manager silently defaulting its
// Git access or its effort authority would make the composition root's choices
// unverifiable.
func Open(roots awfgit.ControlRoots, open OpenCheckout, efforts *effort.Service) (*Manager, error) {
	if open == nil {
		panic("worktree Manager: missing checkout opener dependency")
	}
	if efforts == nil {
		panic("worktree Manager: missing effort service dependency")
	}
	checkout, err := open(roots.InvokingRoot)
	if err != nil {
		return nil, err
	}
	return &Manager{roots: roots, efforts: efforts, git: checkout, open: open}, nil
}

func (m *Manager) managed(slug string) (string, error) {
	root, err := m.roots.ResidentRoot(awfgit.ResidentWorktrees)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, slug), nil
}

func branch(slug string) string { return "awf/" + slug }

func (m *Manager) validateManagedTarget(ctx context.Context, path string) error {
	if _, err := m.roots.ResidentRoot(awfgit.ResidentWorktrees); err != nil {
		return err
	}
	if err := safeManagedPath(path); err != nil {
		return err
	}
	roots, err := awfgit.ResolveControlRoots(ctx, path)
	if err != nil {
		return &awfgit.HardSafetyError{Category: "repository-identity", Path: path, Err: err}
	}
	if filepath.Clean(roots.CommonDir) != filepath.Clean(m.roots.CommonDir) || filepath.Clean(roots.InvokingRoot) != filepath.Clean(path) {
		return &awfgit.HardSafetyError{Category: "repository-identity", Path: path, Err: errors.New("managed checkout belongs to a different repository identity")}
	}
	return nil
}

func (m *Manager) validateLiveInvokingCheckout(ctx context.Context) error {
	if err := safeManagedPath(m.roots.InvokingRoot); err != nil {
		return &awfgit.HardSafetyError{Category: "repository-identity", Path: m.roots.InvokingRoot, Err: err}
	}
	live, err := awfgit.ResolveControlRoots(ctx, m.roots.InvokingRoot)
	if err != nil {
		return &awfgit.HardSafetyError{Category: "repository-identity", Path: m.roots.InvokingRoot, Err: err}
	}
	if filepath.Clean(live.InvokingRoot) != filepath.Clean(m.roots.InvokingRoot) || filepath.Clean(live.CommonDir) != filepath.Clean(m.roots.CommonDir) || filepath.Clean(live.PrimaryRoot) != filepath.Clean(m.roots.PrimaryRoot) {
		return &awfgit.HardSafetyError{Category: "repository-identity", Path: m.roots.InvokingRoot, Err: errors.New("invoking checkout identity changed")}
	}
	return nil
}

func operationFree(ctx context.Context, checkout Runner) error {
	for _, name := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "rebase-merge", "rebase-apply"} {
		candidate, err := checkout.GitPath(ctx, name)
		if err != nil {
			return err
		}
		if _, err = os.Lstat(candidate); err == nil {
			if name == "MERGE_HEAD" {
				return mergeRefusal(ctx, checkout)
			}
			return refusal("operation", "checkout has an in-progress Git operation", false, "finish or abort the native Git operation, then retry")
		} else if !errors.Is(err, os.ErrNotExist) { // coverage-ignore: local lstat reports an inode or os.ErrNotExist absent a kernel fault
			return err
		}
	}
	return nil
}

// mergeRefusal is the one refusal that conditions resolution on ownership.
// Finishing or aborting a merge destroys work when the caller did not start it,
// and an effort integration sits staged and uncommitted in the receiving
// checkout for the whole gate and renewed review. The caller's own stuck merge
// still needs an exit, though, and this same probe runs against a caller's
// managed checkout during removal, so the instruction guards resolution rather
// than forbidding it. Attribution decorates the condition; it never decides the
// instruction, because a merge nothing could name is no safer to abort.
func mergeRefusal(ctx context.Context, checkout Runner) error {
	condition := "a merge is in progress in this checkout"
	if slug := integrationHolder(ctx, checkout); slug != "" {
		condition = "a merge of effort " + slug + " is in progress in this checkout"
	}
	return refusal("operation", condition, false, "finish or abort this merge only if you started it; otherwise wait until this checkout is clean, then retry")
}

// integrationHolder names the effort whose branch is being merged here, or the
// empty string when none can be proven. It derives the answer from repository
// truth alone: MERGE_HEAD names the merged tip, and an effort branch is checked
// out at its own managed worktree, so a registration on an effort branch whose
// own HEAD is that tip identifies the holder. The registration carries the
// commit already, so the whole topology is read from one snapshot rather than
// re-resolved per branch. A probe that cannot answer leaves the merge
// unattributed rather than propagating, because the refusal it decorates is
// already correct without a name, and a wrong name is worse than none.
func integrationHolder(ctx context.Context, checkout Runner) string {
	tip, err := checkout.ResolveCommit(ctx, "MERGE_HEAD")
	if err != nil {
		return ""
	}
	registrations, err := checkout.WorktreeList(ctx)
	if err != nil {
		return ""
	}
	for _, registration := range registrations {
		slug, ok := strings.CutPrefix(registration.Branch, "refs/heads/"+branch(""))
		if ok && slug != "" && registration.HEAD == tip {
			return slug
		}
	}
	return ""
}

// requireClean refuses on any tracked, staged, or nonignored untracked change.
// The counts come from the seam's one cleanliness oracle, so this refusal and
// the audit rule that reports uncommitted work read the same repository facts.
// Owned resident state under .awf stays invisible here because awf renders the
// .gitignore that covers it, which is also what keeps a developer's own `git
// status` quiet about it.
func requireClean(ctx context.Context, checkout Runner) error {
	tracked, untracked, err := checkout.ChangeCounts(ctx)
	if err != nil {
		return err
	}
	if tracked > 0 || untracked > 0 {
		return refusal("cleanliness", "checkout has tracked, untracked, or staged changes", false, "confirm the changes are yours and not a concurrent effort's work, then commit them or inspect and discard them explicitly with native Git, and retry")
	}
	return nil
}

func (m *Manager) Add(ctx context.Context, slug, base string) (Result, error) {
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
	regs, err := m.git.WorktreeList(ctx)
	if err != nil {
		return Result{}, err
	}
	wantBranch := "refs/heads/" + branch(slug)
	for _, registration := range regs {
		if filepath.Clean(registration.Path) == filepath.Clean(path) || registration.Branch == wantBranch {
			return Result{}, refusal("topology", "managed registration or branch is already present", false, "inspect `git worktree list --porcelain` and retry after safe cleanup")
		}
	}
	exists, err := m.git.BranchExists(ctx, branch(slug))
	if err != nil {
		return Result{}, err
	}
	if exists {
		return Result{}, refusal("topology", "managed branch already exists", false, "inspect the branch and retry after safe cleanup")
	}
	if err := operationFree(ctx, m.git); err != nil {
		return Result{}, err
	}
	if base == "" {
		base = "HEAD"
	}
	full, err := m.git.ResolveCommit(ctx, base)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil { // coverage-ignore: ResidentRoot proved the owned resident ancestry; failure requires a concurrent namespace or storage fault
		return Result{}, fmt.Errorf("create managed worktree root: %w", err)
	}
	if err := m.validateLiveInvokingCheckout(ctx); err != nil {
		return Result{}, err
	}
	if err := m.git.WorktreeAdd(ctx, path, branch(slug), full); err != nil {
		changed := m.topologyPresent(ctx, slug, path)
		return Result{}, refusalCause("add", "git worktree add failed", changed, "inspect actual Git topology and retry add, or clean only the named path, registration, and branch with native Git", err)
	}
	if err := exactRegistration(ctx, m.git, path, wantBranch); err != nil {
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
func (m *Manager) NewEffort(ctx context.Context, input effort.NewInput, base string) (effort.Record, Result, error) {
	record, err := m.efforts.New(ctx, input)
	if err != nil {
		return effort.Record{}, Result{}, err
	}
	result, addErr := m.Add(ctx, record.Slug, base)
	if addErr == nil {
		return record, result, nil
	}
	return effort.Record{}, Result{}, m.rollback(ctx, record, addErr)
}

// rollback composes the one creation failure from the structured finish
// outcome and the managed-topology classification, never from error prose.
func (m *Manager) rollback(ctx context.Context, record effort.Record, addErr error) error {
	slug := record.Slug
	finishResult, finishErr := m.efforts.Finish(ctx, slug)
	switch {
	case finishErr == nil:
		return fmt.Errorf("worktree creation failed: %w; effort %s rolled back; next action: fix the reported cause and retry `awf effort new --slug %q %q`", addErr, slug, record.Slug, record.Title)
	case errors.Is(finishErr, effort.ErrManagedTopologyPresent):
		return fmt.Errorf("worktree creation failed: %w; effort %s retained: managed topology remains; next action: inspect `git worktree list --porcelain`, clean up with native Git or `awf effort worktree remove %s`, then retry `awf effort worktree add %s` or finish the effort", addErr, slug, slug, slug)
	case finishResult.Renamed:
		return fmt.Errorf("worktree creation failed: %w; effort %s rollback interrupted after rename: %w; next action: retry `awf effort finish %s`", addErr, slug, finishErr, slug)
	default:
		return fmt.Errorf("worktree creation failed: %w; effort %s retained: rollback failed: %w; next action: resolve the rollback failure, then retry `awf effort worktree add %s` or `awf effort finish %s`", addErr, slug, finishErr, slug, slug)
	}
}

func (m *Manager) topologyPresent(ctx context.Context, slug, path string) bool {
	if _, err := managedLstat(path); err == nil {
		return true
	}
	regs, err := m.git.WorktreeList(ctx)
	if err == nil {
		for _, registration := range regs {
			if filepath.Clean(registration.Path) == filepath.Clean(path) || registration.Branch == "refs/heads/"+branch(slug) {
				return true
			}
		}
	}
	exists, err := m.git.BranchExists(ctx, branch(slug))
	return err != nil || exists
}

func (m *Manager) Integrate(ctx context.Context, slug, gateCommand string) (Result, error) {
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
	if err := m.validateManagedTarget(ctx, path); err != nil {
		return Result{}, err
	}
	if err := exactRegistration(ctx, m.git, path, "refs/heads/"+branch(slug)); err != nil {
		return Result{}, err
	}
	if err := operationFree(ctx, m.git); err != nil {
		return Result{}, err
	}
	if err := requireClean(ctx, m.git); err != nil {
		return Result{}, err
	}
	targetBranch, err := m.git.CurrentBranch(ctx)
	if err != nil {
		return Result{}, err
	}
	if targetBranch == "" {
		return Result{}, refusal("topology", "receiving checkout is detached", false, "check out the intended target branch and retry")
	}
	if targetBranch == branch(slug) {
		return Result{}, refusal("topology", "receiving checkout is the effort branch", false, "change to the intended target branch and retry")
	}
	tip, err := m.git.ResolveCommit(ctx, branch(slug))
	if err != nil {
		return Result{}, err
	}
	target, err := m.git.ResolveCommit(ctx, "HEAD")
	if err != nil {
		return Result{}, err
	}
	already, err := m.git.Ancestor(ctx, tip, target)
	if err != nil {
		return Result{}, err
	}
	if already {
		return Result{Condition: "effort tip is already integrated into the target", ChangedTopology: false, NextAction: "run `awf effort worktree remove " + slug + "` after terminal review is settled"}, nil
	}
	fastForward, err := m.git.Ancestor(ctx, target, tip)
	if err != nil {
		return Result{}, err
	}
	if err := m.validateIntegrationFacts(ctx, path, slug, target, tip); err != nil {
		return Result{}, err
	}
	if fastForward {
		if err := m.git.MergeFastForward(ctx, branch(slug)); err != nil {
			return Result{}, refusalCause("integration", "fast-forward failed", m.targetChanged(ctx, target), "inspect the receiving checkout and retry only from clean verified topology", err)
		}
		return Result{Condition: "target fast-forwarded to effort tip", ChangedTopology: true, NextAction: "settle terminal review, then remove the managed worktree"}, nil
	}
	base, err := m.git.MergeBase(ctx, "HEAD", branch(slug))
	if err != nil || strings.TrimSpace(base) == "" {
		return Result{}, refusalCause("ancestry", "target and effort have no proven common ancestor", false, "inspect repository and branch identity; do not use --allow-unrelated-histories", err)
	}
	gateStep := integrationGateStep(gateCommand)
	if err := m.git.MergeNoCommit(ctx, branch(slug)); err != nil {
		return Result{}, refusalCause("merge-conflict", "divergent integration stopped with visible conflict state", true, "resolve or abort the merge; after resolution run `./awf check staged`, "+gateStep+", commit, and renew terminal review", err)
	}
	return Result{Condition: "divergent integration is staged without a commit", ChangedTopology: true, NextAction: "run `./awf check staged`, " + gateStep + ", commit the merge, and renew terminal implementation review"}, nil
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func integrationGateStep(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return "the project gate"
	}
	return "`" + command + "`"
}

func (m *Manager) validateIntegrationFacts(ctx context.Context, path, slug, target, tip string) error {
	if err := m.validateLiveInvokingCheckout(ctx); err != nil {
		return err
	}
	if err := m.validateManagedTarget(ctx, path); err != nil {
		return err
	}
	if err := exactRegistration(ctx, m.git, path, "refs/heads/"+branch(slug)); err != nil {
		return err
	}
	currentTarget, err := m.git.ResolveCommit(ctx, "HEAD")
	if err != nil || currentTarget != target {
		return refusalCause("topology", "target HEAD changed during integration preflight", false, "restart integration from the clean intended target checkout", err)
	}
	currentTip, err := m.git.ResolveCommit(ctx, branch(slug))
	if err != nil || currentTip != tip {
		return refusalCause("topology", "effort branch changed during integration preflight", false, "restart integration after the effort writer settles the branch", err)
	}
	return nil
}

func (m *Manager) targetChanged(ctx context.Context, before string) bool {
	after, err := m.git.ResolveCommit(ctx, "HEAD")
	return err != nil || after != before
}

func (m *Manager) Remove(ctx context.Context, slug string) (Result, error) {
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
	if err := operationFree(ctx, m.git); err != nil {
		return Result{}, err
	}
	if err := requireClean(ctx, m.git); err != nil {
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
		regs, regsErr := m.git.WorktreeList(ctx)
		if regsErr != nil {
			return Result{}, regsErr
		}
		var exact *awfgit.WorktreeRegistration
		for index := range regs {
			registration := &regs[index]
			if filepath.Clean(registration.Path) == filepath.Clean(path) {
				if exact != nil || registration.Branch != "refs/heads/"+branch(slug) || registration.Detached || registration.Bare {
					return Result{}, refusal("repository-identity", "managed path registration is not exact", changed, "inspect native Git topology and clean it manually without discarding work")
				}
				exact = registration
			}
			if registration.Branch == "refs/heads/"+branch(slug) && filepath.Clean(registration.Path) != filepath.Clean(path) {
				return Result{}, refusal("repository-identity", "managed branch is registered at a foreign path", changed, "inspect native Git topology and clean it manually without discarding work")
			}
		}
		branchPresent, branchErr := m.git.BranchExists(ctx, branch(slug))
		if branchErr != nil {
			return Result{}, branchErr
		}
		if !pathPresent && exact == nil && !branchPresent {
			return Result{Condition: "managed worktree topology is absent", ChangedTopology: changed, NextAction: "continue to retrospective, then finish the effort"}, nil
		}
		if branchPresent {
			merged, ancestryErr := m.git.Ancestor(ctx, branch(slug), "HEAD")
			if ancestryErr != nil {
				return Result{}, ancestryErr
			}
			if !merged {
				return Result{}, refusal("ancestry", "managed branch is not merged into the target", changed, "integrate and settle terminal review, or inspect and discard explicitly with native Git")
			}
		}
		if pathPresent {
			if err := m.validateManagedTarget(ctx, path); err != nil {
				return Result{}, err
			}
			managedCheckout, openErr := m.open(path)
			if openErr != nil {
				return Result{}, openErr
			}
			if err := operationFree(ctx, managedCheckout); err != nil {
				return Result{}, err
			}
			if err := requireClean(ctx, managedCheckout); err != nil {
				return Result{}, refusalCause("cleanliness", "managed worktree is dirty", changed, "commit or explicitly inspect and discard changes with native Git, then retry ordinary removal", err)
			}
			if exact != nil {
				if err := m.git.WorktreeRemove(ctx, path); err != nil {
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
			if err := m.git.WorktreePrune(ctx); err != nil {
				return Result{}, refusalCause("removal", "prunable registration cleanup failed", changed, "inspect `git worktree list --porcelain` and retry", err)
			}
			changed = true
			continue
		}
		if branchPresent {
			if err := m.git.BranchDelete(ctx, branch(slug)); err != nil {
				return Result{}, refusalCause("removal", "safe managed branch deletion failed", changed, "inspect branch ancestry and retry without force", err)
			}
			changed = true
		}
	}
}
