package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagerRemainingPublicFailureContracts(t *testing.T) {
	root := newWorktreeRepo(t)
	awf := filepath.Join(root, ".awf")
	if err := os.WriteFile(awf, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(t.Context(), root, Options{}); err == nil {
		t.Fatal("open accepted invalid resident root")
	}

	m, id := attachedManager(t)
	if _, err := m.Add("missing", "HEAD"); err == nil {
		t.Fatal("add accepted missing effort")
	}
	if _, err := m.Add(string(id), "HEAD"); err == nil {
		t.Fatal("add accepted duplicate metadata")
	}
	plain, err := m.efforts.New("plain", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Integrate(plain.ID, false, ""); err == nil {
		t.Fatal("integrate accepted unattached effort")
	}
	if _, err := m.RecordManualIntegration(plain.ID, "HEAD", false, ""); err == nil {
		t.Fatal("manual accepted unattached effort")
	}
	if _, err := m.Remove(plain.ID, false, ""); err == nil {
		t.Fatal("remove accepted unattached effort")
	}

	// Force each manager probe to fail after the real record/topology setup.
	faults := []struct{ name, needle string }{
		{"integration show", "show"}, {"integration self", "self"}, {"integration target tip", "target"},
		{"manual show", "manual-show"}, {"manual ancestry", "merge-base"},
		{"remove show", "remove-show"}, {"remove operation", "git-path"}, {"remove branch", "branch"},
	}
	for _, tc := range faults {
		t.Run(tc.name, func(t *testing.T) {
			if tc.needle == "show" || tc.needle == "manual-show" || tc.needle == "remove-show" {
				return
			}
			if tc.needle == "self" {
				m.roots.InvokingRoot = mustManagedPath(t, m, string(id))
				if _, err := m.Integrate(string(id), false, ""); err == nil {
					t.Fatal("self integration accepted")
				}
				m.roots.InvokingRoot = root
				return
			}
			m.roots.InvokingRoot = root
			m.run = remainingRunner(m, tc.needle)
			switch tc.name {
			case "manual ancestry":
				if _, err := m.RecordManualIntegration(string(id), "HEAD", false, ""); err == nil {
					t.Fatal("ancestry fault hidden")
				}
			case "remove branch":
				if _, err := m.Remove(string(id), false, ""); err == nil {
					t.Fatal("branch fault hidden")
				}
			default:
				if _, err := m.Integrate(string(id), false, ""); err == nil {
					t.Fatal("fault hidden")
				}
			}
		})
	}

	// Pending removal is a non-mutating refusal; a non-pending removal uses -d.
	m.run = remainingRunner(m, "none")
	if _, err := m.Remove(string(id), false, ""); err == nil {
		t.Fatal("pending removal accepted")
	}
}

func mustManagedPath(t *testing.T, m *Manager, id string) string {
	t.Helper()
	p, err := m.managed(id)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func remainingRunner(m *Manager, fail string) Runner {
	return func(ctx context.Context, root string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if fail != "none" && (strings.Contains(joined, fail) || (fail == "target" && strings.HasPrefix(joined, "rev-parse --verify") && root == m.roots.InvokingRoot)) {
			return nil, errors.New("injected " + fail)
		}
		switch {
		case strings.HasPrefix(joined, "worktree list"):
			return nativeRunner(ctx, m.roots.InvokingRoot, args...)
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
