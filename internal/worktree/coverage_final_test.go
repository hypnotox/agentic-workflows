package worktree

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestManagerCoverageFinalBranches(t *testing.T) {
	m, id := attachedManager(t)
	originalPrimary := m.roots.PrimaryRoot
	if _, err := m.Integrate("missing", false, ""); err == nil {
		t.Fatal("missing integrate accepted")
	}
	if _, err := m.RecordManualIntegration("missing", "HEAD", false, ""); err == nil {
		t.Fatal("missing manual accepted")
	}
	if _, err := m.Remove("missing", false, ""); err == nil {
		t.Fatal("missing remove accepted")
	}
	m.roots.PrimaryRoot = "relative"
	if _, err := m.Add(string(id), "HEAD"); err == nil {
		t.Fatal("add escaped root")
	}
	if _, err := m.Integrate(string(id), false, ""); err == nil {
		t.Fatal("integrate escaped root")
	}
	if _, err := m.RecordManualIntegration(string(id), "HEAD", false, ""); err == nil {
		t.Fatal("manual escaped root")
	}
	if _, err := m.Remove(string(id), false, ""); err == nil {
		t.Fatal("remove escaped root")
	}
	m.roots.PrimaryRoot = originalPrimary

	pending, err := m.efforts.New("pending coverage", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Add(pending.ID, "HEAD"); err != nil {
		t.Fatal(err)
	}
	pendingPath, _ := m.managed(pending.ID)
	if err := os.RemoveAll(pendingPath); err != nil {
		t.Fatal(err)
	}
	m.run = func(context.Context, string, ...string) ([]byte, error) {
		return []byte("worktree " + pendingPath + "\x00HEAD x\x00branch refs/heads/awf/" + pending.ID + "\x00\x00"), nil
	}
	if _, err := m.Remove(pending.ID, false, ""); err == nil {
		t.Fatal("missing managed path accepted")
	}

	// Exercise the native non-exit-error branch with a cancelled command.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := nativeRunner(ctx, ".", "status"); err == nil {
		t.Fatal("cancelled native command accepted")
	}
	_ = strings.Builder{}
	_ = errors.New
}
