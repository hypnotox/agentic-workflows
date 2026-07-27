package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagerMutationAndProbeFailures(t *testing.T) {
	m, _ := attachedManager(t)
	originalPrimary := m.roots.PrimaryRoot
	fresh, err := m.efforts.New("mutation failures", false)
	if err != nil {
		t.Fatal(err)
	}
	fileRoot := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(fileRoot, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	m.roots.PrimaryRoot = fileRoot
	if _, err := m.Add(fresh.ID, "HEAD"); err == nil {
		t.Fatal("mkdir failure hidden")
	}
	m.roots.PrimaryRoot = originalPrimary

	partial, err := m.efforts.New("partial add", false)
	if err != nil {
		t.Fatal(err)
	}
	m.run = func(ctx context.Context, root string, args ...string) ([]byte, error) {
		if strings.HasPrefix(strings.Join(args, " "), "rev-parse --verify") {
			return []byte(strings.Repeat("a", 40) + "\n"), nil
		}
		if strings.HasPrefix(strings.Join(args, " "), "worktree add") {
			_ = os.Remove(filepath.Join(originalPrimary, ".awf", "efforts", partial.ID+".json"))
			return nil, nil
		}
		return nil, nil
	}
	if _, err := m.Add(partial.ID, "HEAD"); err == nil {
		t.Fatal("partial add hidden")
	}

	integration, err := m.efforts.New("probe failures", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Add(integration.ID, "HEAD"); err != nil {
		t.Fatal(err)
	}
	m.run = probeRunner(m, "target")
	if _, err := m.Integrate(integration.ID, false, ""); err == nil {
		t.Fatal("target probe fault hidden")
	}
	m.run = probeRunner(m, "ancestor")
	if _, err := m.Integrate(integration.ID, false, ""); err == nil {
		t.Fatal("ancestor probe fault hidden")
	}
}

func probeRunner(m *Manager, fault string) Runner {
	return func(ctx context.Context, root string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.HasPrefix(joined, "worktree list") {
			return nativeRunner(ctx, m.roots.InvokingRoot, args...)
		}
		if fault == "target" && strings.HasPrefix(joined, "rev-parse --verify") && root == m.roots.InvokingRoot {
			return nil, errors.New("target fault")
		}
		if fault == "ancestor" && strings.HasPrefix(joined, "merge-base") {
			return nil, errors.New("ancestor fault")
		}
		switch {
		case strings.HasPrefix(joined, "status"):
			return nil, nil
		case strings.HasPrefix(joined, "rev-parse --git-path"):
			return []byte(filepath.Join(root, "absent") + "\n"), nil
		case strings.HasPrefix(joined, "symbolic-ref"):
			return []byte("main\n"), nil
		case strings.HasPrefix(joined, "rev-parse --verify"):
			return []byte(strings.Repeat("a", 40) + "\n"), nil
		case strings.HasPrefix(joined, "merge-base"):
			return nil, &fakeExit{code: 1}
		default:
			return nil, nil
		}
	}
}
