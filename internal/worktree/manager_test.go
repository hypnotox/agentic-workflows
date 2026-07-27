package worktree

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/effort"
)

// invariant: tooling/effort-management:managed-worktree-lifecycle
func TestManagedWorktreeLifecycle(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo")
	git(t, "init", root)
	if err := os.WriteFile(filepath.Join(root, "base"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, "-C", root, "add", "base")
	git(t, "-C", root, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "base")
	m, err := Open(t.Context(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	// The fixed UUID is a real schema-v4 identifier and makes branch/path assertions exact.
	record, err := m.efforts.New("managed topology", false)
	if err != nil {
		t.Fatal(err)
	}
	added, err := m.Add(record.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".awf", "worktrees", record.ID)
	if added.Worktree == nil || added.Worktree.Branch != "awf/"+record.ID {
		t.Fatalf("add=%#v", added)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	// A clean caller fast-forwards the fixed branch and retains disposition after removal.
	if err := os.WriteFile(filepath.Join(path, "change"), []byte("change\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, "-C", path, "add", "change")
	git(t, "-C", path, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "change")
	integrated, err := m.Integrate(record.ID, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if integrated.Integration != "fast-forward" {
		t.Fatalf("integration=%s", integrated.Integration)
	}
	removed, err := m.Remove(record.ID, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if removed.Worktree != nil || removed.Integration != "fast-forward" {
		t.Fatalf("remove=%#v", removed)
	}
}
func TestManagerPartialEvidencePublicFaults(t *testing.T) {
	m, err := Open(t.Context(), newWorktreeRepo(t), Options{})
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := m.efforts.New("partial evidence faults", false)
	if err != nil {
		t.Fatal(err)
	}
	path, err := m.managed(fresh.ID)
	if err != nil {
		t.Fatal(err)
	}
	partialPath := filepath.Join(m.roots.PrimaryRoot, ".awf", "efforts", "."+fresh.ID+".worktree.partial")
	if err := os.WriteFile(partialPath, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Add(fresh.ID, "HEAD"); err == nil {
		t.Fatal("duplicate partial evidence accepted")
	}
	_ = os.Remove(partialPath)

	clearEffort, err := m.efforts.New("clear fault", false)
	if err != nil {
		t.Fatal(err)
	}
	original := m.run
	m.run = func(ctx context.Context, root string, args ...string) ([]byte, error) {
		out, err := original(ctx, root, args...)
		if err == nil && strings.HasPrefix(strings.Join(args, " "), "worktree add") {
			_ = os.Remove(filepath.Join(m.roots.PrimaryRoot, ".awf", "efforts", "."+clearEffort.ID+".worktree.partial"))
		}
		return out, err
	}
	if _, err := m.Add(clearEffort.ID, "HEAD"); err == nil || !strings.Contains(err.Error(), "settle worktree evidence") {
		t.Fatalf("cleanup fault = %v", err)
	}
	_ = path
}

func TestManagerPartialEvidenceMutationRefusals(t *testing.T) {
	t.Run("integration duplicate", func(t *testing.T) {
		m, id := attachedManager(t)
		path := mustManagedPath(t, m, string(id))
		tip, err := resolve(t.Context(), m.run, path, "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		target, err := m.run(m.ctx, m.roots.InvokingRoot, "symbolic-ref", "-q", "--short", "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		e := effort.PartialEvidence{SchemaVersion: 1, EffortID: string(id), Action: "integration", Branch: branch(string(id)), CommonDir: filepath.Clean(m.roots.CommonDir), Tip: tip, TargetPath: filepath.Clean(m.roots.InvokingRoot), TargetBranch: strings.TrimSpace(string(target)), Integration: effort.IntegrationFastForward}
		if err := m.efforts.RecordPartial(e); err != nil {
			t.Fatal(err)
		}
		if _, err := m.Integrate(string(id), false, ""); err == nil {
			t.Fatal("duplicate integration evidence accepted")
		}
	})
	t.Run("removal duplicate", func(t *testing.T) {
		m, id := attachedManager(t)
		tip, err := resolve(t.Context(), m.run, m.roots.InvokingRoot, branch(string(id)))
		if err != nil {
			t.Fatal(err)
		}
		e := effort.PartialEvidence{SchemaVersion: 1, EffortID: string(id), Action: "removal", Branch: branch(string(id)), CommonDir: filepath.Clean(m.roots.CommonDir), BranchTip: tip}
		if err := m.efforts.RecordPartial(e); err != nil {
			t.Fatal(err)
		}
		if _, err := m.Remove(string(id), true, "approved"); err == nil {
			t.Fatal("duplicate removal evidence accepted")
		}
	})
}

func TestManagerPartialSettlementFaults(t *testing.T) {
	t.Run("integration merge failure cleanup", func(t *testing.T) {
		m, id := attachedManager(t)
		original := m.run
		m.run = func(ctx context.Context, root string, args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == "merge" {
				_ = m.efforts.ClearPartial(string(id), "integration")
				return nil, errors.New("merge")
			}
			return original(ctx, root, args...)
		}
		if _, err := m.Integrate(string(id), false, ""); err == nil || !strings.Contains(err.Error(), "settle failed integration evidence") {
			t.Fatalf("merge cleanup fault = %v", err)
		}
	})
	t.Run("integration settlement", func(t *testing.T) {
		m, id := attachedManager(t)
		original := m.run
		m.run = func(ctx context.Context, root string, args ...string) ([]byte, error) {
			out, err := original(ctx, root, args...)
			if err == nil && len(args) > 0 && args[0] == "merge" {
				_ = m.efforts.ClearPartial(string(id), "integration")
			}
			return out, err
		}
		if _, err := m.Integrate(string(id), false, ""); err == nil || !strings.Contains(err.Error(), "settle integration evidence") {
			t.Fatalf("integration cleanup fault = %v", err)
		}
	})
	t.Run("removal resolver", func(t *testing.T) {
		m, id := attachedManager(t)
		original := m.run
		m.run = func(ctx context.Context, root string, args ...string) ([]byte, error) {
			if len(args) >= 3 && args[0] == "rev-parse" && strings.HasPrefix(args[2], "awf/") {
				return nil, errors.New("resolve")
			}
			return original(ctx, root, args...)
		}
		if _, err := m.Remove(string(id), true, "approved"); err == nil {
			t.Fatal("removal resolver fault hidden")
		}
	})
	t.Run("removal settlement", func(t *testing.T) {
		m, id := attachedManager(t)
		original := m.run
		m.run = func(ctx context.Context, root string, args ...string) ([]byte, error) {
			out, err := original(ctx, root, args...)
			if err == nil && len(args) >= 2 && args[0] == "branch" {
				_ = os.Remove(filepath.Join(m.roots.PrimaryRoot, ".awf", "efforts", "."+string(id)+".removal.partial"))
			}
			return out, err
		}
		if _, err := m.Remove(string(id), true, "approved"); err == nil || !strings.Contains(err.Error(), "settle removal evidence") {
			t.Fatalf("removal cleanup fault = %v", err)
		}
	})
}

func git(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
}
