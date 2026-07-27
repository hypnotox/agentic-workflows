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

func TestRemovePendingDirtyRequiresApprovedForcedNativeRemoval(t *testing.T) {
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
	r, err := m.efforts.New("remove pending", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Add(r.ID, ""); err != nil {
		t.Fatal(err)
	}
	path, err := m.managed(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "untracked"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := m.efforts.Show(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Remove(r.ID, false, ""); err == nil {
		t.Fatal("dirty pending removal accepted without paired recovery")
	} else {
		var refusal *RefusalError
		if !errors.As(err, &refusal) || refusal.Category != "cleanliness" || !refusal.Forceable {
			t.Fatalf("refusal=%T %v", err, err)
		}
	}
	after, err := m.efforts.Show(r.ID)
	if err != nil || after.Worktree == nil || after.Integration != before.Integration {
		t.Fatalf("failed removal changed record: %#v %v", after, err)
	}
	removed, err := m.Remove(r.ID, true, "operator accepts discarded pending changes")
	if err != nil {
		t.Fatal(err)
	}
	if removed.Worktree != nil || removed.Integration != effort.IntegrationNone {
		t.Fatalf("removed=%#v", removed)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("worktree remains: %v", err)
	}
	out, err := nativeRunner(t.Context(), root, "branch", "--list", "awf/"+r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("forced branch remains: %q", out)
	}
}

func TestTopologyHelpersRejectMalformedAndPropagateFaults(t *testing.T) {
	bad := func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("injected") }
	if _, err := resolve(t.Context(), bad, ".", "HEAD"); err == nil {
		t.Fatal("resolve accepted injected failure")
	}
	if err := status(t.Context(), bad, "."); err == nil {
		t.Fatal("status accepted injected failure")
	}
	if _, err := ancestor(t.Context(), bad, ".", "a", "b"); err == nil {
		t.Fatal("ancestor hid injected failure")
	}
	if _, err := registrations(t.Context(), func(context.Context, string, ...string) ([]byte, error) {
		return []byte("worktree /x\x00HEAD abc\x00"), nil
	}, "."); err == nil {
		t.Fatal("unterminated porcelain accepted")
	}
	if _, err := registrations(t.Context(), func(context.Context, string, ...string) ([]byte, error) {
		return []byte("worktree /x\x00unknown x\x00\x00"), nil
	}, "."); err == nil {
		t.Fatal("unknown porcelain field accepted")
	}
}
