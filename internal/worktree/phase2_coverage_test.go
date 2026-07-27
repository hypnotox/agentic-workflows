package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/effort"
)

// invariant: tooling/effort-management:managed-worktree-lifecycle
func TestPhase2FailureSettlementAndPreconditions(t *testing.T) {
	m := newManagerForPhase2(t)
	originalPrimary := m.roots.PrimaryRoot
	m.roots.PrimaryRoot = "relative"
	if err := m.validateManagedTarget(m.roots.InvokingRoot); err == nil {
		t.Fatal("invalid resident root accepted")
	}
	m.roots.PrimaryRoot = originalPrimary
	if err := m.validateManagedTarget(filepath.Join(m.roots.PrimaryRoot, ".awf", "worktrees", "missing")); err == nil {
		t.Fatal("missing managed target accepted")
	}
	foreign := newWorktreeRepo(t)
	if err := m.validateManagedTarget(foreign); err == nil {
		t.Fatal("foreign checkout accepted as managed target")
	}
	fresh, err := m.efforts.New("settle", false)
	if err != nil {
		t.Fatal(err)
	}
	m.run = func(ctx context.Context, root string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.HasPrefix(joined, "worktree list") || strings.HasPrefix(joined, "worktree add") {
			return nil, errors.New("topology unavailable")
		}
		return nativeRunner(ctx, root, args...)
	}
	if _, err := m.Add(fresh.ID, "HEAD"); err == nil || !strings.Contains(err.Error(), "partial Git mutation") {
		t.Fatalf("unverifiable add failure = %v", err)
	}
	duplicate, err := m.efforts.New("duplicate evidence", false)
	if err != nil {
		t.Fatal(err)
	}
	base, err := resolve(t.Context(), nativeRunner, m.roots.InvokingRoot, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.efforts.RecordPartial(effort.PartialEvidence{SchemaVersion: 1, EffortID: duplicate.ID, Action: "worktree", Base: base, Branch: branch(duplicate.ID), Path: filepath.Join(m.roots.PrimaryRoot, ".awf", "worktrees", duplicate.ID), CommonDir: filepath.Clean(m.roots.CommonDir)}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Add(duplicate.ID, "HEAD"); err == nil {
		t.Fatal("duplicate evidence accepted")
	}
}

// invariant: tooling/effort-management:managed-worktree-lifecycle
// invariant: tooling/effort-management:managed-worktree-lifecycle
func TestPhase2ManualDivergenceForceRepairSurvivesRestart(t *testing.T) {
	m := newManagerForPhase2(t)
	r, err := m.efforts.New("manual restart", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Add(r.ID, "HEAD"); err != nil {
		t.Fatal(err)
	}
	path, _ := m.managed(r.ID)
	commitFile(t, path, "restart", "restart")
	if _, err = m.RecordManualIntegration(r.ID, "HEAD", true, "verified external integration"); err != nil {
		t.Fatal(err)
	}
	base := m.run
	failed := false
	m.run = func(ctx context.Context, root string, args ...string) ([]byte, error) {
		if strings.Join(args, " ") == "branch -D awf/"+r.ID && !failed {
			failed = true
			return nil, errors.New("injected post-removal branch failure")
		}
		return base(ctx, root, args...)
	}
	if _, err = m.Remove(r.ID, true, "verified divergent removal"); err == nil {
		t.Fatal("post-boundary failure was hidden")
	}
	fresh, err := Open(t.Context(), m.roots.InvokingRoot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fresh.efforts.Repair(r.ID); err != nil {
		t.Fatalf("restart repair: %v", err)
	}
	repaired, err := fresh.efforts.Show(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if repaired.Worktree != nil || repaired.Integration != effort.IntegrationManual {
		t.Fatalf("repair lost truthful manual disposition: %+v", repaired)
	}
	if _, err = nativeRunner(t.Context(), m.roots.InvokingRoot, "show-ref", "--verify", "refs/heads/awf/"+r.ID); err == nil {
		t.Fatal("forced branch survived repair")
	}
}

// invariant: tooling/effort-management:managed-worktree-lifecycle
func TestPhase2ManualDivergenceUsesPairedForce(t *testing.T) {
	m := newManagerForPhase2(t)
	r, err := m.efforts.New("manual divergence", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Add(r.ID, "HEAD"); err != nil {
		t.Fatal(err)
	}
	path, _ := m.managed(r.ID)
	commitFile(t, path, "divergent", "divergent")
	if _, err = m.RecordManualIntegration(r.ID, "HEAD", true, "verified external integration"); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Remove(r.ID, false, ""); err == nil {
		t.Fatal("divergent manual removal accepted without force")
	}
	if _, err = m.Remove(r.ID, true, "verified removal"); err != nil {
		t.Fatal(err)
	}
}

func newManagerForPhase2(t *testing.T) *Manager {
	t.Helper()
	root := newWorktreeRepo(t)
	m, err := Open(t.Context(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// invariant: tooling/effort-management:managed-worktree-lifecycle
func TestPhase2AddFailureStatErrorRetainsEvidence(t *testing.T) {
	m := newManagerForPhase2(t)
	r, err := m.efforts.New("stat error", false)
	if err != nil {
		t.Fatal(err)
	}
	path, _ := m.managed(r.ID)
	oldLstat := managedLstat
	managedLstat = func(string) (os.FileInfo, error) { return nil, errors.New("injected inaccessible checkout") }
	defer func() { managedLstat = oldLstat }()
	if err := m.settleAddFailure(r.ID, path, errors.New("injected add failure")); err == nil || !strings.Contains(err.Error(), "unverifiable") {
		t.Fatalf("stat error settlement = %v", err)
	}
	managedLstat = oldLstat
	m.clearPartial = func(string, string) error { return errors.New("injected evidence clear") }
	if err := m.settleAddFailure(r.ID, path, errors.New("injected add failure")); err == nil || !strings.Contains(err.Error(), "evidence clear") {
		t.Fatalf("clear error settlement = %v", err)
	}
}

func TestPhase2SafeManagedPathChecksComponents(t *testing.T) {
	root := t.TempDir()
	oldOwner := managedOwner
	managedOwner = func(string, os.FileInfo) error { return errors.New("injected foreign owner") }
	if err := safeManagedPath(root); err == nil || !strings.Contains(err.Error(), "foreign owner") {
		t.Fatal("foreign owner accepted")
	}
	managedOwner = oldOwner
	link := filepath.Join(root, "link")
	if err := os.Symlink(t.TempDir(), link); err != nil {
		t.Fatal(err)
	}
	if err := safeManagedPath(filepath.Join(link, "missing")); err == nil {
		t.Fatal("symlink component accepted")
	}
	if err := safeManagedPath(filepath.Join(root, "missing", "leaf")); err == nil {
		t.Fatal("missing component accepted")
	}
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := safeManagedPath(file); err == nil {
		t.Fatal("file accepted")
	}
}
