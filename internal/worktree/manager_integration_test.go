package worktree

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/effort"
)

// invariant: tooling/effort-management:managed-worktree-lifecycle
func TestManualIntegrationAndNonFastForwardMerge(t *testing.T) {
	root := newWorktreeRepo(t)
	m, err := Open(t.Context(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}

	manual, err := m.efforts.New("manual", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Add(manual.ID, "HEAD"); err != nil {
		t.Fatal(err)
	}
	manualPath, _ := m.managed(manual.ID)
	commitFile(t, manualPath, "manual", "manual")
	if _, err = m.RecordManualIntegration(manual.ID, "HEAD", false, ""); err == nil {
		t.Fatal("manual integration accepted a commit without the effort tip")
	} else {
		var refusal *RefusalError
		if !errors.As(err, &refusal) || refusal.Category != "ancestry" || !refusal.Forceable {
			t.Fatalf("manual refusal=%T %v", err, err)
		}
	}
	r, err := m.RecordManualIntegration(manual.ID, "HEAD", true, "external integration was verified")
	if err != nil || r.Integration != effort.IntegrationManual {
		t.Fatalf("manual result=%#v err=%v", r, err)
	}

	merged, err := m.efforts.New("merge", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Add(merged.ID, "HEAD"); err != nil {
		t.Fatal(err)
	}
	mergePath, _ := m.managed(merged.ID)
	commitFile(t, root, "target", "target")
	commitFile(t, mergePath, "effort", "effort")
	r, err = m.Integrate(merged.ID, false, "")
	if err != nil || r.Integration != effort.IntegrationMerge {
		t.Fatalf("merge result=%#v err=%v", r, err)
	}
}

func newWorktreeRepo(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "repo")
	git(t, "init", root)
	git(t, "-C", root, "config", "user.name", "test")
	git(t, "-C", root, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(root, "base"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, "-C", root, "add", "base")
	git(t, "-C", root, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", "base")
	return root
}
func commitFile(t *testing.T, root, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(body+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, "-C", root, "add", name)
	git(t, "-C", root, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-m", name)
}
