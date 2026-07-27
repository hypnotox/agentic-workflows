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

func TestPhase2SafeManagedPathChecksComponents(t *testing.T) {
	root := t.TempDir()
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
