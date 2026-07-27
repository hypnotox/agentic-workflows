package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
func git(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
}
