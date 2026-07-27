package worktree

import (
	"context"
	"errors"
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
	ctx     context.Context
	roots   awfgit.ControlRoots
	efforts *effort.Service
	run     Runner
	clock   func() time.Time
}

func Open(ctx context.Context, invoking string, options Options) (*Manager, error) {
	roots, err := awfgit.ResolveControlRoots(ctx, invoking)
	if err != nil {
		return nil, err
	}
	service, err := effort.Open(ctx, invoking, effort.Options{Clock: options.Clock})
	if err != nil {
		return nil, err
	}
	run := options.Runner
	if run == nil {
		run = nativeRunner
	}
	clock := options.Clock
	if clock == nil {
		clock = time.Now
	}
	return &Manager{ctx: ctx, roots: roots, efforts: service, run: run, clock: clock}, nil
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
	if _, err := os.Lstat(path); err == nil {
		// Do not classify a hostile resident path as an ordinary retryable
		// collision. The path is manager-owned and must never be a symlink or
		// another file type, even before Git is asked to create it.
		if err := safeManagedPath(path); err != nil {
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
	if err := makeManagedDir(filepath.Dir(path), 0o700); err != nil {
		return effort.Record{}, err
	}
	if err := runWorktreeAdd(m.ctx, m.run, m.roots.InvokingRoot, branch(id), path, full); err != nil {
		return effort.Record{}, err
	}
	result, err := m.efforts.AttachWorktree(id, full)
	if err != nil {
		return effort.Record{}, &PartialMutationError{EffortID: id, Repair: "record-worktree"}
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
	if _, err = m.run(m.ctx, m.roots.InvokingRoot, args...); err != nil {
		return effort.Record{}, &awfgit.HardSafetyError{Category: "merge-conflict", Path: m.roots.InvokingRoot, Err: err}
	}
	result, err := m.efforts.SetIntegration(id, disposition)
	if err != nil {
		return effort.Record{}, &PartialMutationError{EffortID: id, Repair: "record-integration"}
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
	// Registration is the identity authority. Check it before touching the
	// filesystem so a missing or foreign path is a non-forceable identity
	// refusal rather than an incidental filesystem error.
	if err = exactRegistration(m.ctx, m.run, m.roots.InvokingRoot, path, "refs/heads/"+branch(id)); err != nil {
		return effort.Record{}, err
	}
	if err = safeManagedPath(path); err != nil {
		return effort.Record{}, err
	}
	if err = m.operationFree(path); err != nil {
		return effort.Record{}, err
	}
	forceRemove := false
	if err = status(m.ctx, m.run, path); err != nil {
		if r, ok := errors.AsType[*RefusalError](err); ok && r.Forceable && approved(force, reason) {
			forceRemove = true
		} else {
			return effort.Record{}, err
		}
	}
	pending := record.Integration == effort.IntegrationPending
	if pending && !approved(force, reason) {
		return effort.Record{}, &RefusalError{Category: "integration", Risk: "pending worktree has no recorded integration", Forceable: true}
	}
	if pending {
		forceRemove = true
	}
	removeArgs := []string{"worktree", "remove"}
	if forceRemove {
		removeArgs = append(removeArgs, "--force")
	}
	removeArgs = append(removeArgs, path)
	if _, err = m.run(m.ctx, m.roots.InvokingRoot, removeArgs...); err != nil {
		return effort.Record{}, err
	}
	deleteFlag := "-d"
	// -D is only the explicitly approved destructive pending-removal path.
	if pending && approved(force, reason) {
		deleteFlag = "-D"
	}
	if _, err = m.run(m.ctx, m.roots.InvokingRoot, "branch", deleteFlag, branch(id)); err != nil {
		return effort.Record{}, &PartialMutationError{EffortID: id, Repair: "delete-worktree-branch"}
	}
	result, err := m.efforts.RemoveWorktreeMetadata(id, pending)
	if err != nil {
		return effort.Record{}, &PartialMutationError{EffortID: id, Repair: "record-worktree-removal"}
	}
	return result, nil
}
