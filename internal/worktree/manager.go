package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/effort"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

type Options struct {
	Runner Runner
	Clock  func() time.Time
}

var makeManagedDir = os.MkdirAll

type Manager struct {
	ctx          context.Context
	roots        awfgit.ControlRoots
	efforts      *effort.Service
	run          Runner
	clock        func() time.Time
	clearPartial func(string, string) error
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
	service, err := effort.Open(ctx, invoking, effort.Options{Clock: options.Clock, Git: func(ctx context.Context, root string, args ...string) ([]byte, error) {
		return run(ctx, root, args...)
	}})
	if err != nil {
		return nil, err
	}
	clock := options.Clock
	if clock == nil {
		clock = time.Now
	}
	return &Manager{ctx: ctx, roots: roots, efforts: service, run: run, clock: clock, clearPartial: service.ClearPartial}, nil
}
func (m *Manager) managed(id string) (string, error) {
	root, err := m.roots.ResidentRoot(awfgit.ResidentWorktrees)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, id), nil
}
func branch(id string) string                 { return "awf/" + id }
func approved(force bool, reason string) bool { return force && strings.TrimSpace(reason) != "" }
func (m *Manager) validateManagedTarget(path string) error {
	if _, err := m.roots.ResidentRoot(awfgit.ResidentWorktrees); err != nil {
		return err
	}
	// coverage-ignore: platform path-swap fault injection.
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

// validateLiveInvokingCheckout is deliberately repeated at mutation boundaries.
// The checkout may have been replaced after Open or after an earlier probe.
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
			return &RefusalError{Category: "operation", Risk: "checkout has an in-progress Git operation", Forceable: false}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
func (m *Manager) settleAddFailure(id string, path string, cause error) error {
	regs, verifyErr := registrations(m.ctx, m.run, m.roots.InvokingRoot)
	mutated := false
	if verifyErr == nil {
		for _, r := range regs {
			if filepath.Clean(r.path) == filepath.Clean(path) || r.branch == "refs/heads/"+branch(id) {
				mutated = true
				break
			}
		}
		// Only a positive ENOENT proves that the checkout was not created.
		// Any other stat result leaves durable evidence for repair.
		if !mutated {
			if _, statErr := managedLstat(path); statErr == nil {
				mutated = true
			} else if !errors.Is(statErr, os.ErrNotExist) {
				return &PartialMutationError{EffortID: id, Repair: "record-worktree", Err: fmt.Errorf("worktree add failure is unverifiable: %w (cause: %w)", statErr, cause)}
			}
		}
	}
	// coverage-ignore: native Git topology race during failure verification.
	if verifyErr != nil || mutated {
		return &PartialMutationError{EffortID: id, Repair: "record-worktree", Err: fmt.Errorf("worktree add failed before registration could settle: %w", cause)}
	}
	// coverage-ignore: evidence-directory fsync fault injection.
	if clearErr := m.clearPartial(id, "worktree"); clearErr != nil {
		return &PartialMutationError{EffortID: id, Repair: "record-worktree", Err: fmt.Errorf("settle worktree evidence after failed add: %w", clearErr)}
	}
	return cause
}

func (m *Manager) Add(id, base string) (effort.Record, error) {
	record, err := m.efforts.Show(id)
	if err != nil {
		return effort.Record{}, err
	}
	if record.Worktree != nil {
		return effort.Record{}, &RefusalError{Category: "topology", Risk: "effort already records a managed worktree", Forceable: false}
	}
	path, err := m.managed(id)
	if err != nil {
		return effort.Record{}, err
	}
	if _, err := managedLstat(path); err == nil {
		// Do not classify a hostile resident path as an ordinary retryable
		// collision. The path is manager-owned and must never be a symlink or
		// another file type, even before Git is asked to create it.
		if err := m.validateManagedTarget(path); err != nil {
			return effort.Record{}, err
		}
		return effort.Record{}, &RefusalError{Category: "topology", Risk: "managed path already exists", Forceable: true}
	} else if !errors.Is(err, os.ErrNotExist) {
		return effort.Record{}, err
	}
	if base == "" {
		base = "HEAD"
	}
	full, err := resolve(m.ctx, m.run, m.roots.InvokingRoot, base)
	if err != nil {
		return effort.Record{}, err
	}
	evidence := effort.PartialEvidence{SchemaVersion: 1, EffortID: id, Action: "worktree", Base: full, Branch: branch(id), Path: path, CommonDir: filepath.Clean(m.roots.CommonDir)}
	if err := m.efforts.RecordPartial(evidence); err != nil {
		return effort.Record{}, fmt.Errorf("record worktree partial evidence: %w", err)
	}
	if err := makeManagedDir(filepath.Dir(path), 0o700); err != nil {
		return effort.Record{}, m.settleAddFailure(id, path, err)
	}
	if err := runWorktreeAdd(m.ctx, m.run, m.roots.InvokingRoot, branch(id), path, full); err != nil {
		return effort.Record{}, m.settleAddFailure(id, path, err)
	}
	result, err := m.efforts.AttachWorktree(id, full)
	if err != nil {
		return effort.Record{}, &PartialMutationError{EffortID: id, Repair: "record-worktree", Err: fmt.Errorf("attach worktree record: %w", err)}
	}
	if err := m.efforts.ClearPartial(id, "worktree"); err != nil {
		return effort.Record{}, &PartialMutationError{EffortID: id, Repair: "record-worktree", Err: fmt.Errorf("settle worktree evidence: %w", err)}
	}
	return result, nil
}
func runWorktreeAdd(ctx context.Context, run Runner, root, name, path, full string) error {
	_, err := run(ctx, root, "worktree", "add", "-b", name, path, full)
	return err
}

func (m *Manager) Integrate(id string, force bool, reason string) (effort.Record, error) {
	record, err := m.efforts.Show(id)
	if err != nil {
		return effort.Record{}, err
	}
	if record.Worktree == nil {
		return effort.Record{}, errors.New("effort has no managed worktree")
	}
	path, err := m.managed(id)
	if err != nil {
		return effort.Record{}, err
	}
	if err = m.validateManagedTarget(path); err != nil {
		return effort.Record{}, err
	}
	if filepath.Clean(m.roots.InvokingRoot) == filepath.Clean(path) {
		return effort.Record{}, &awfgit.HardSafetyError{Category: "repository-identity", Path: path, Err: errors.New("cannot integrate from managed worktree")}
	}
	if err = exactRegistration(m.ctx, m.run, m.roots.InvokingRoot, path, "refs/heads/"+branch(id)); err != nil {
		return effort.Record{}, err
	}
	if err = m.operationFree(m.roots.InvokingRoot); err != nil {
		return effort.Record{}, err
	}
	if err = status(m.ctx, m.run, m.roots.InvokingRoot); err != nil {
		if r, ok := errors.AsType[*RefusalError](err); ok && r.Forceable && approved(force, reason) {
		} else {
			return effort.Record{}, err
		}
	}
	targetRaw, err := m.run(m.ctx, m.roots.InvokingRoot, "symbolic-ref", "-q", "--short", "HEAD")
	if err != nil {
		return effort.Record{}, &RefusalError{Category: "topology", Risk: "invoking checkout is detached", Forceable: false}
	}
	target := strings.TrimSpace(string(targetRaw))
	if target == branch(id) {
		return effort.Record{}, &RefusalError{Category: "topology", Risk: "invoking checkout is the effort branch", Forceable: false}
	}
	tip, err := resolve(m.ctx, m.run, path, "HEAD")
	if err != nil {
		return effort.Record{}, err
	}
	targetTip, err := resolve(m.ctx, m.run, m.roots.InvokingRoot, "HEAD")
	if err != nil {
		return effort.Record{}, err
	}
	ff, err := ancestor(m.ctx, m.run, m.roots.InvokingRoot, targetTip, tip)
	if err != nil {
		return effort.Record{}, err
	}
	disposition := effort.IntegrationMerge
	var args []string
	if ff {
		args = []string{"merge", "--ff-only", branch(id)}
		disposition = effort.IntegrationFastForward
	} else {
		args = []string{"merge", "--no-ff", "-m", "Merge effort " + id, branch(id)}
	}
	evidence := effort.PartialEvidence{SchemaVersion: 1, EffortID: id, Action: "integration", Branch: branch(id), CommonDir: filepath.Clean(m.roots.CommonDir), Tip: tip, TargetPath: filepath.Clean(m.roots.InvokingRoot), TargetBranch: target, Integration: disposition}
	if err := m.efforts.RecordPartial(evidence); err != nil {
		return effort.Record{}, fmt.Errorf("record integration partial evidence: %w", err)
	}
	if err := m.validateLiveInvokingCheckout(); err != nil {
		_ = m.efforts.ClearPartial(id, "integration")
		return effort.Record{}, err
	}
	if _, err = m.run(m.ctx, m.roots.InvokingRoot, args...); err != nil {
		if settleErr := m.efforts.ClearPartial(id, "integration"); settleErr != nil {
			return effort.Record{}, &PartialMutationError{EffortID: id, Repair: "record-integration", Err: fmt.Errorf("settle failed integration evidence after merge failure: %w", settleErr)}
		}
		return effort.Record{}, &awfgit.HardSafetyError{Category: "merge-conflict", Path: m.roots.InvokingRoot, Err: err}
	}
	result, err := m.efforts.SetIntegration(id, disposition)
	if err != nil {
		return effort.Record{}, &PartialMutationError{EffortID: id, Repair: "record-integration", Err: fmt.Errorf("record integration disposition: %w", err)}
	}
	if err := m.efforts.ClearPartial(id, "integration"); err != nil {
		return effort.Record{}, &PartialMutationError{EffortID: id, Repair: "record-integration", Err: fmt.Errorf("settle integration evidence: %w", err)}
	}
	return result, nil
}
func (m *Manager) RecordManualIntegration(id, commit string, force bool, reason string) (effort.Record, error) {
	record, err := m.efforts.Show(id)
	if err != nil {
		return effort.Record{}, err
	}
	if record.Worktree == nil {
		return effort.Record{}, errors.New("effort has no managed worktree")
	}
	path, err := m.managed(id)
	if err != nil {
		return effort.Record{}, err
	}
	// Validate the confined target before any registration lookup or HEAD
	// resolution. A swapped checkout must remain pending and untouched.
	if err = m.validateManagedTarget(path); err != nil {
		return effort.Record{}, err
	}
	if filepath.Clean(m.roots.InvokingRoot) == filepath.Clean(path) {
		return effort.Record{}, &awfgit.HardSafetyError{Category: "repository-identity", Path: path, Err: errors.New("cannot record integration from managed worktree")}
	}
	if err = exactRegistration(m.ctx, m.run, m.roots.InvokingRoot, path, "refs/heads/"+branch(id)); err != nil {
		return effort.Record{}, err
	}
	tip, err := resolve(m.ctx, m.run, path, "HEAD")
	if err != nil {
		return effort.Record{}, err
	}
	target, err := resolve(m.ctx, m.run, m.roots.InvokingRoot, commit)
	if err != nil {
		return effort.Record{}, err
	}
	ok, err := ancestor(m.ctx, m.run, m.roots.InvokingRoot, tip, target)
	if err != nil {
		return effort.Record{}, err
	}
	if !ok && !approved(force, reason) {
		return effort.Record{}, &RefusalError{Category: "ancestry", Risk: "named commit does not contain the effort tip", Forceable: true}
	}
	return m.efforts.SetIntegration(id, effort.IntegrationManual)
}
func (m *Manager) Remove(id string, force bool, reason string) (effort.Record, error) {
	record, err := m.efforts.Show(id)
	if err != nil {
		return effort.Record{}, err
	}
	if record.Worktree == nil {
		return effort.Record{}, errors.New("effort has no managed worktree")
	}
	path, err := m.managed(id)
	if err != nil {
		return effort.Record{}, err
	}
	if err = m.validateManagedTarget(path); err != nil {
		return effort.Record{}, err
	}
	// Registration is the identity authority. Check it before touching the
	// filesystem so a missing or foreign path is a non-forceable identity
	// refusal rather than an incidental filesystem error.
	if err = exactRegistration(m.ctx, m.run, m.roots.InvokingRoot, path, "refs/heads/"+branch(id)); err != nil {
		return effort.Record{}, err
	}
	if err = m.operationFree(path); err != nil {
		return effort.Record{}, err
	}
	worktreeRemoveForce := false
	if err = status(m.ctx, m.run, path); err != nil {
		if r, ok := errors.AsType[*RefusalError](err); ok && r.Forceable && approved(force, reason) {
			worktreeRemoveForce = true
		} else {
			return effort.Record{}, err
		}
	}
	pending := record.Integration == effort.IntegrationPending
	if pending && !approved(force, reason) {
		return effort.Record{}, &RefusalError{Category: "integration", Risk: "pending worktree has no recorded integration", Forceable: true}
	}
	branchTip, err := resolve(m.ctx, m.run, m.roots.InvokingRoot, branch(id))
	if err != nil {
		return effort.Record{}, err
	}
	branchDeleteForce := pending && approved(force, reason)
	if record.Integration == effort.IntegrationManual {
		targetTip, targetErr := resolve(m.ctx, m.run, m.roots.InvokingRoot, "HEAD")
		if targetErr != nil {
			return effort.Record{}, targetErr
		}
		contained, ancestryErr := ancestor(m.ctx, m.run, m.roots.InvokingRoot, branchTip, targetTip)
		if ancestryErr != nil {
			return effort.Record{}, ancestryErr
		}
		if !contained && !approved(force, reason) {
			return effort.Record{}, &RefusalError{Category: "ancestry", Risk: "manual integration does not contain the effort tip", Forceable: true}
		}
		if !contained {
			branchDeleteForce = true
		}
	}
	if pending {
		worktreeRemoveForce = true
	}
	evidence := effort.PartialEvidence{SchemaVersion: 1, EffortID: id, Action: "removal", Branch: branch(id), CommonDir: filepath.Clean(m.roots.CommonDir), WorktreeRemoveForce: worktreeRemoveForce, BranchDeleteForce: branchDeleteForce, BranchTip: branchTip}
	if err := m.efforts.RecordPartial(evidence); err != nil {
		return effort.Record{}, fmt.Errorf("record removal partial evidence: %w", err)
	}
	if err := m.validateLiveInvokingCheckout(); err != nil {
		_ = m.efforts.ClearPartial(id, "removal")
		return effort.Record{}, err
	}
	removeArgs := []string{"worktree", "remove"}
	if worktreeRemoveForce {
		removeArgs = append(removeArgs, "--force")
	}
	removeArgs = append(removeArgs, path)
	if _, err = m.run(m.ctx, m.roots.InvokingRoot, removeArgs...); err != nil {
		return effort.Record{}, err
	}
	deleteFlag := "-d"
	if branchDeleteForce {
		deleteFlag = "-D"
	}
	if err := m.validateLiveInvokingCheckout(); err != nil {
		return effort.Record{}, err
	}
	if _, err = m.run(m.ctx, m.roots.InvokingRoot, "branch", deleteFlag, branch(id)); err != nil {
		return effort.Record{}, &PartialMutationError{EffortID: id, Repair: "delete-worktree-branch", Err: fmt.Errorf("delete managed branch: %w", err)}
	}
	result, err := m.efforts.RemoveWorktreeMetadata(id, pending)
	if err != nil {
		return effort.Record{}, &PartialMutationError{EffortID: id, Repair: "record-worktree-removal", Err: fmt.Errorf("clear worktree metadata: %w", err)}
	}
	if err := m.efforts.ClearPartial(id, "removal"); err != nil {
		return effort.Record{}, &PartialMutationError{EffortID: id, Repair: "record-worktree-removal", Err: fmt.Errorf("settle removal evidence: %w", err)}
	}
	return result, nil
}
