package worktree

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestManagerFaultMatrix(t *testing.T) {
	cases := []struct{ name, op, needle string }{
		{"add resolve", "add", "rev-parse --verify"}, {"add git", "add", "worktree add"},
		{"integrate registration", "integrate", "worktree list"}, {"integrate operation", "integrate", "--git-path MERGE_HEAD"}, {"integrate status", "integrate", "status --porcelain"}, {"integrate ref", "integrate", "symbolic-ref"}, {"integrate tip", "integrate", "rev-parse --verify HEAD"}, {"integrate merge", "integrate", "merge --ff-only"},
		{"manual registration", "manual", "worktree list"}, {"manual tip", "manual", "rev-parse --verify HEAD"}, {"manual target", "manual", "rev-parse --verify named"},
		{"remove registration", "remove", "worktree list"}, {"remove operation", "remove", "--git-path MERGE_HEAD"}, {"remove status", "remove", "status --porcelain"}, {"remove git", "remove", "worktree remove"}, {"remove branch", "remove", "branch -d"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, r := attachedManager(t)
			m.run = stubWorktreeRunner(m, tc.needle)
			var err error
			switch tc.op {
			case "add":
				// Add's normal path is already occupied by the attached fixture; use a fresh record.
				fresh, _ := m.efforts.New("fresh", false)
				m.run = stubWorktreeRunner(m, tc.needle)
				_, err = m.Add(fresh.ID, "HEAD")
			case "integrate":
				_, err = m.Integrate(string(r), false, "")
			case "manual":
				_, err = m.RecordManualIntegration(string(r), "named", false, "")
			case "remove":
				_, err = m.Remove(string(r), false, "")
			}
			if err == nil {
				t.Fatalf("fault %q was not reached", tc.needle)
			}
		})
	}
}

// Keep the setup typed through a small local adapter to make each fault test use
// the real effort store and native registration before replacing only Git calls.
type recordID string

func attachedManager(t *testing.T) (*Manager, recordID) {
	root := newWorktreeRepo(t)
	m, err := Open(t.Context(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := m.efforts.New("fault matrix", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Add(r.ID, "HEAD"); err != nil {
		t.Fatal(err)
	}
	return m, recordID(r.ID)
}
func stubWorktreeRunner(m *Manager, fail string) Runner {
	return func(ctx context.Context, root string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, fail) {
			return nil, fmt.Errorf("injected %s", fail)
		}
		switch {
		case strings.HasPrefix(joined, "worktree list"):
			// Preserve the real registration bytes while fault-injecting other probes.
			out, _ := nativeRunner(ctx, m.roots.InvokingRoot, "worktree", "list", "--porcelain", "-z")
			return out, nil
		case strings.HasPrefix(joined, "status"):
			return nil, nil
		case strings.HasPrefix(joined, "rev-parse --git-path"):
			return []byte(".git/nonexistent\n"), nil
		case strings.HasPrefix(joined, "symbolic-ref"):
			return []byte("main\n"), nil
		case strings.HasPrefix(joined, "rev-parse --verify"):
			return []byte("0123456789012345678901234567890123456789\n"), nil
		case strings.HasPrefix(joined, "merge-base"):
			return nil, nil
		default:
			return nil, nil
		}
	}
}
