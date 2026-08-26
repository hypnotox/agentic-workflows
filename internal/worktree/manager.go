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
	ChangedTopology bool // legacy summary; Topology contains exact axes.
	Topology        TopologyEffects
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
			return refusal("operation", "checkout has an in-progress Git operation", "finish or abort the native Git operation", "retry")
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
	return refusal("operation", condition, "finish or abort this merge only if you started it", "otherwise wait until this checkout is clean", "retry")
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
		return refusal("cleanliness", "checkout has tracked, untracked, or staged changes", "confirm the changes are yours and not a concurrent effort's work", "commit the changes with native Git, or inspect them with native Git", "if inspection selected discarding, discard the changes explicitly with native Git", "retry")
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
		return Result{}, refusal("topology", "managed path already exists", "inspect the existing path", "perform safe manual cleanup", "retry add")
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
			return Result{}, refusal("topology", "managed registration or branch is already present", "inspect `git worktree list --porcelain`", "perform safe cleanup", "retry add")
		}
	}
	exists, err := m.git.BranchExists(ctx, branch(slug))
	if err != nil {
		return Result{}, err
	}
	if exists {
		return Result{}, refusal("topology", "managed branch already exists", "inspect the branch", "perform safe cleanup", "retry add")
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
		effects := m.topologyEffects(ctx, slug, path)
		if effects.Changed() {
			return Result{}, &RefusalError{Category: "operation", Condition: "git worktree add failed", ChangedTopology: true, Topology: effects, Err: err,
				NextAction: "inspect actual Git topology", NextActions: []string{"inspect actual Git topology", "clean only the named managed path, registration, and branch with native Git", "retry add"}}
		}
		return Result{}, refusalCause("operation", "git worktree add failed", false, err, "address or resolve the reported failed Git call", "retry add")
	}
	if err := exactRegistration(ctx, m.git, path, wantBranch); err != nil {
		return Result{}, &RefusalError{Category: "repository-identity", Condition: "Git add returned without exact managed registration", ChangedTopology: true, Topology: TopologyEffects{ManagedPath: true, GitRegistration: true, Branch: true}, Err: err, NextAction: "inspect actual Git topology", NextActions: []string{"inspect actual Git topology", "perform safe native-Git cleanup", "retry"}}
	}
	return Result{
		Condition: "managed worktree added for " + slug, ChangedTopology: true,
		Topology:   TopologyEffects{ManagedPath: true, GitRegistration: true, Branch: true},
		NextAction: "continue the effort in " + path, Path: path, Branch: branch(slug),
	}, nil
}

// NewEffort publishes the effort residents, then creates its managed
// worktree via the same standalone Add machinery (ADR-0189). On Add
// failure it invokes the narrow identity-bound creation rollback, removing the
// effort only when actual managed topology is proven absent.
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

// rollback composes the one creation failure from the structured rollback
// outcome and the managed-topology classification, never from error prose.
func (m *Manager) rollback(ctx context.Context, record effort.Record, addErr error) error {
	slug := record.Slug
	path, pathErr := m.managed(slug)
	effects := topologyFromError(addErr)
	if pathErr != nil {
		effects.Uncertain = true
	} else {
		effects = effects.merge(m.topologyEffects(ctx, slug, path))
	}
	rollbackResult, rollbackErr := m.efforts.RollbackCreation(ctx, record)
	if pathErr == nil {
		effects = effects.merge(m.topologyEffects(ctx, slug, path))
	}
	changedTopology := effects.Changed()
	switch {
	case rollbackErr == nil:
		return &CreationError{
			Message:   fmt.Sprintf("worktree creation failed: %v; effort %s rolled back; next action: retry `awf effort new --slug %q %q`", addErr, slug, record.Slug, record.Title),
			Condition: "managed worktree creation failed and the effort was rolled back", ChangedTopology: changedTopology, Topology: effects, Cause: addErr,
			Steps: []string{"fix the reported cause", fmt.Sprintf("retry `awf effort new --slug %q %q`", record.Slug, record.Title)},
		}
	case errors.Is(rollbackErr, effort.ErrManagedTopologyPresent):
		return &CreationError{
			Message:   fmt.Sprintf("worktree creation failed: %v; effort %s retained: managed topology remains; next action: inspect `git worktree list --porcelain`", addErr, slug),
			Condition: "managed worktree creation failed and topology remains", ChangedEffort: true, ChangedTopology: changedTopology, Topology: effects, Cause: addErr, RollbackCause: rollbackErr,
			Steps: []string{"inspect `git worktree list --porcelain`", fmt.Sprintf("clean up with native Git or `awf effort worktree remove %s`", slug), fmt.Sprintf("retry `awf effort worktree add %s` or finish the effort", slug)},
		}
	case rollbackResult.Removed:
		reservation := ".awf/efforts/.finishing-" + record.ID + "-" + slug
		return &CreationError{
			Message:   fmt.Sprintf("worktree creation failed: %v; effort %s deletion completed with parent durability uncertainty: %v; next action: verify the active resident and %s are absent", addErr, slug, rollbackErr, reservation),
			Condition: "managed worktree creation failed after effort deletion with durability uncertainty", ChangedTopology: changedTopology, Topology: effects, Cause: addErr, RollbackCause: rollbackErr,
			Steps: []string{fmt.Sprintf("verify `.awf/efforts/%s` is absent", slug), fmt.Sprintf("verify `%s` is absent", reservation), "retry effort creation only after both paths are absent"},
		}
	case rollbackResult.Reserved:
		return &CreationError{
			Message:   fmt.Sprintf("worktree creation failed: %v; effort %s deletion rollback was interrupted: %v; next action: inspect the finishing reservation", addErr, slug, rollbackErr),
			Condition: "managed worktree creation failed and effort deletion rollback was interrupted", ChangedEffort: true, ChangedTopology: changedTopology, Topology: effects, Cause: addErr, RollbackCause: rollbackErr,
			Steps: []string{"inspect the identity-bound finishing reservation", "complete safe manual cleanup only after verifying its immutable identity"},
		}
	default:
		return &CreationError{
			Message:   fmt.Sprintf("worktree creation failed: %v; effort %s retained: rollback failed: %v; next action: retry `awf effort worktree add %s` or inspect the resident", addErr, slug, rollbackErr, slug),
			Condition: "managed worktree creation failed and effort rollback failed", ChangedEffort: true, ChangedTopology: changedTopology, Topology: effects, Cause: addErr, RollbackCause: rollbackErr,
			Steps: []string{"resolve the rollback failure", fmt.Sprintf("retry `awf effort worktree add %s` or `awf effort finish %s`", slug, slug)},
		}
	}
}

func topologyFromError(err error) TopologyEffects {
	var refusal *RefusalError
	if errors.As(err, &refusal) {
		return refusal.Topology
	}
	return TopologyEffects{}
}

func (m *Manager) topologyEffects(ctx context.Context, slug, path string) TopologyEffects {
	effects := TopologyEffects{}
	if _, err := managedLstat(path); err == nil {
		effects.ManagedPath = true
	} else if !errors.Is(err, os.ErrNotExist) {
		effects.Uncertain = true
	}
	regs, err := m.git.WorktreeList(ctx)
	if err != nil {
		effects.Uncertain = true
	} else {
		for _, registration := range regs {
			if filepath.Clean(registration.Path) == filepath.Clean(path) {
				effects.GitRegistration = true
			}
			if registration.Branch == "refs/heads/"+branch(slug) {
				effects.Branch = true
			}
		}
	}
	exists, err := m.git.BranchExists(ctx, branch(slug))
	if err != nil {
		effects.Uncertain = true
	} else if exists {
		effects.Branch = true
	}
	return effects
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
		return Result{}, refusal("repository-identity", "integration must run from the receiving checkout, not the managed worktree", "change to the intended clean target checkout", "retry")
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
		return Result{}, refusal("topology", "receiving checkout is detached", "check out the intended target branch", "retry")
	}
	if targetBranch == branch(slug) {
		return Result{}, refusal("topology", "receiving checkout is the effort branch", "change to the intended target branch", "retry")
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
			changed := m.targetChanged(ctx, target)
			if changed {
				return Result{}, &RefusalError{Category: "operation", Condition: "fast-forward failed", ChangedTopology: true, Topology: TopologyEffects{ReceivingHEAD: true}, Err: err, NextAction: "inspect the receiving checkout", NextActions: []string{"inspect the receiving checkout", "retry only from clean verified topology"}}
			}
			return Result{}, refusalCause("operation", "fast-forward failed", false, err, "inspect the receiving checkout", "retry only from clean verified topology")
		}
		return Result{Condition: "target fast-forwarded to effort tip", ChangedTopology: true, Topology: TopologyEffects{ReceivingHEAD: true}, NextAction: "settle terminal review, then remove the managed worktree"}, nil
	}
	base, err := m.git.MergeBase(ctx, "HEAD", branch(slug))
	if err != nil || strings.TrimSpace(base) == "" {
		return Result{}, refusalCause("ancestry", "target and effort have no proven common ancestor", false, err, "inspect repository and branch identity", "do not use --allow-unrelated-histories")
	}
	gateStep := integrationGateStep(gateCommand)
	if err := m.git.MergeNoCommit(ctx, branch(slug)); err != nil {
		return Result{}, refusalCause("merge-conflict", "divergent integration stopped with visible conflict state", true, err, "resolve or abort the merge", "run `./awf check staged`", "run "+gateStep, "commit the merge", "renew terminal review")
	}
	return Result{Condition: "divergent integration is staged without a commit", ChangedTopology: true, Topology: TopologyEffects{ReceivingHEAD: true}, NextAction: "run `./awf check staged`, " + gateStep + ", commit the merge, and renew terminal implementation review"}, nil
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
		return refusalCause("topology", "target HEAD changed during integration preflight", false, err, "restart integration from the clean intended target checkout")
	}
	currentTip, err := m.git.ResolveCommit(ctx, branch(slug))
	if err != nil || currentTip != tip {
		return refusalCause("topology", "effort branch changed during integration preflight", false, err, "restart integration after the effort writer settles the branch")
	}
	return nil
}

func (m *Manager) targetChanged(ctx context.Context, before string) bool {
	after, err := m.git.ResolveCommit(ctx, "HEAD")
	return err != nil || after != before
}

const removalProbeRecovery = "run `git worktree list --porcelain`"

var removalProbeRecoveryActions = []string{
	removalProbeRecovery,
	"inspect the managed path and branch",
	"resolve the reported probe failure",
	"retry ordinary removal",
}

// removalProbeFailure preserves a mechanism error before any mutation, but
// after one converts it to the actionable topology outcome that a caller needs
// to recover safely from the residue before retrying.
func removalProbeFailure(changed bool, effects TopologyEffects, condition string, err error) error {
	if !changed {
		return err
	}
	effects.Uncertain = true
	category := "operation"
	var prior *RefusalError
	if errors.As(err, &prior) {
		category = prior.Category
	}
	var safety *awfgit.HardSafetyError
	if errors.As(err, &safety) {
		category = "repository-identity"
	}
	return &RefusalError{
		Category: category, Condition: condition, ChangedTopology: true, Topology: effects,
		NextAction: removalProbeRecovery, NextActions: removalProbeRecoveryActions, Err: err,
	}
}

func removalRefusal(effects TopologyEffects, category, condition string, err error, actions ...string) error {
	changed := effects.Changed()
	return &RefusalError{Category: category, Condition: condition, ChangedTopology: changed, Topology: effects, NextAction: strings.Join(actions, "; "), NextActions: actions, Err: err}
}

// removalMutationFailure re-observes a failed destructive command. It records
// only axes that differ from the successful pre-command snapshot, plus prior
// completed removal effects; an unavailable re-observation is uncertainty.
func (m *Manager) removalMutationFailure(ctx context.Context, slug, path string, effects, before TopologyEffects, condition string, err error, actions ...string) error {
	after := m.topologyEffects(ctx, slug, path)
	effects = effects.merge(after.changedSince(before))
	return removalRefusal(effects, "operation", condition, err, actions...)
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
		return Result{}, refusal("repository-identity", "removal must run from the intended target checkout", "change to the target checkout", "retry")
	}
	if err := operationFree(ctx, m.git); err != nil {
		return Result{}, err
	}
	if err := requireClean(ctx, m.git); err != nil {
		return Result{}, err
	}
	changed := false
	effects := TopologyEffects{}
	for {
		pathPresent := false
		if _, statErr := managedLstat(path); statErr == nil {
			pathPresent = true
		} else if !errors.Is(statErr, os.ErrNotExist) { // coverage-ignore: local lstat reports an inode or os.ErrNotExist absent a kernel fault
			return Result{}, removalProbeFailure(changed, effects, "managed path probe failed during removal", statErr)
		}
		regs, regsErr := m.git.WorktreeList(ctx)
		if regsErr != nil {
			return Result{}, removalProbeFailure(changed, effects, "managed topology probe failed during removal", regsErr)
		}
		var exact *awfgit.WorktreeRegistration
		for index := range regs {
			registration := &regs[index]
			if filepath.Clean(registration.Path) == filepath.Clean(path) {
				if exact != nil || registration.Branch != "refs/heads/"+branch(slug) || registration.Detached || registration.Bare {
					return Result{}, removalRefusal(effects, "repository-identity", "managed path registration is not exact", nil, "inspect native Git topology", "clean it manually without discarding work")
				}
				exact = registration
			}
			if registration.Branch == "refs/heads/"+branch(slug) && filepath.Clean(registration.Path) != filepath.Clean(path) {
				return Result{}, removalRefusal(effects, "repository-identity", "managed branch is registered at a foreign path", nil, "inspect native Git topology", "clean it manually without discarding work")
			}
		}
		branchPresent, branchErr := m.git.BranchExists(ctx, branch(slug))
		if branchErr != nil {
			return Result{}, removalProbeFailure(changed, effects, "managed branch probe failed during removal", branchErr)
		}
		observed := TopologyEffects{ManagedPath: pathPresent, GitRegistration: exact != nil, Branch: branchPresent}
		if !pathPresent && exact == nil && !branchPresent {
			return Result{Condition: "managed worktree topology is absent", ChangedTopology: changed, Topology: effects, NextAction: "continue to retrospective, then finish the effort"}, nil
		}
		if branchPresent {
			merged, ancestryErr := m.git.Ancestor(ctx, branch(slug), "HEAD")
			if ancestryErr != nil {
				return Result{}, removalProbeFailure(changed, effects, "managed branch ancestry probe failed during removal", ancestryErr)
			}
			if !merged {
				return Result{}, removalRefusal(effects, "ancestry", "managed branch is not merged into the target", nil, "integrate the managed branch, or inspect the branch with native Git", "if integration was selected, settle terminal review", "if inspection selected discarding, discard the branch explicitly with native Git")
			}
		}
		if pathPresent {
			if err := m.validateManagedTarget(ctx, path); err != nil {
				return Result{}, removalProbeFailure(changed, effects, "managed target validation failed during removal", err)
			}
			managedCheckout, openErr := m.open(path)
			if openErr != nil {
				return Result{}, removalProbeFailure(changed, effects, "managed checkout open failed during removal", openErr)
			}
			if err := operationFree(ctx, managedCheckout); err != nil {
				return Result{}, removalProbeFailure(changed, effects, "managed checkout operation probe failed during removal", err)
			}
			if err := requireClean(ctx, managedCheckout); err != nil {
				return Result{}, removalRefusal(effects, "cleanliness", "managed worktree is dirty", err, "commit the changes with native Git, or inspect them with native Git", "if inspection selected discarding, discard the changes explicitly with native Git", "retry ordinary removal")
			}
			if exact != nil {
				if err := m.git.WorktreeRemove(ctx, path); err != nil {
					return Result{}, m.removalMutationFailure(ctx, slug, path, effects, observed, "native Git worktree removal failed", err, "inspect actual topology", "retry ordinary removal")
				}
				effects.ManagedPath, effects.GitRegistration = true, true
			} else {
				if err := os.RemoveAll(path); err != nil { // coverage-ignore: path identity and cleanliness were just proven; recursive removal failure requires a concurrent namespace or storage fault
					return Result{}, m.removalMutationFailure(ctx, slug, path, effects, observed, "proven unregistered managed path cleanup failed", err, "inspect the path", "retry ordinary removal")
				}
				effects.ManagedPath = true
			}
			changed = true
			continue
		}
		if exact != nil {
			if err := m.git.WorktreePrune(ctx); err != nil {
				return Result{}, m.removalMutationFailure(ctx, slug, path, effects, observed, "prunable registration cleanup failed", err, "inspect `git worktree list --porcelain`", "retry ordinary removal")
			}
			effects.GitRegistration = true
			changed = true
			continue
		}
		if branchPresent {
			if err := m.git.BranchDelete(ctx, branch(slug)); err != nil {
				return Result{}, m.removalMutationFailure(ctx, slug, path, effects, observed, "safe managed branch deletion failed", err, "inspect branch ancestry", "retry without force")
			}
			effects.Branch = true
			changed = true
		}
	}
}
