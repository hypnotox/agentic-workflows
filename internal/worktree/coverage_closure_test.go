package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoverageClosurePublicFaultContracts(t *testing.T) {
	// An existing non-directory parent is a filesystem refusal, not a Git collision.
	root := newWorktreeRepo(t)
	m, err := Open(t.Context(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := m.efforts.New("parent fault", false)
	if err != nil {
		t.Fatal(err)
	}
	resident := filepath.Join(root, ".awf", "worktrees")
	if err := os.RemoveAll(resident); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resident, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Add(r.ID, "HEAD"); err == nil {
		t.Fatal("add accepted invalid parent")
	}
	oldMkdir := makeManagedDir
	if err := os.Remove(resident); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(resident, 0o700); err != nil {
		t.Fatal(err)
	}
	makeManagedDir = func(string, os.FileMode) error { return errors.New("injected mkdir") }
	fresh, err := m.efforts.New("mkdir fault", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Add(fresh.ID, "HEAD"); err == nil {
		t.Fatal("mkdir fault hidden")
	}
	makeManagedDir = oldMkdir
	freshGit, err := m.efforts.New("git add fault", false)
	if err != nil {
		t.Fatal(err)
	}
	m.run = func(ctx context.Context, root string, args ...string) ([]byte, error) {
		if strings.HasPrefix(strings.Join(args, " "), "worktree add") {
			return nil, errors.New("injected git add")
		}
		return nativeRunner(ctx, root, args...)
	}
	if _, err := m.Add(freshGit.ID, "HEAD"); err == nil {
		t.Fatal("git add fault hidden")
	} else if !strings.Contains(err.Error(), "injected git add") {
		t.Fatalf("unexpected git add fault: %v", err)
	}

	m, id := attachedManager(t)
	if _, err := m.Remove(string(id), false, ""); err == nil {
		t.Fatal("pending removal accepted")
	}
	path := mustManagedPath(t, m, string(id))
	baseRunner := m.run
	// Resolve the invoking target tip must be distinguished from resolving the effort tip.
	m.run = func(ctx context.Context, root string, args ...string) ([]byte, error) {
		if strings.HasPrefix(strings.Join(args, " "), "rev-parse --verify") && root == m.roots.InvokingRoot {
			return nil, errors.New("target fault")
		}
		return baseRunner(ctx, root, args...)
	}
	if _, err := m.Integrate(string(id), false, ""); err == nil {
		t.Fatal("target resolution fault hidden")
	}

	// An ancestry probe error is not evidence of divergence.
	m.run = func(ctx context.Context, root string, args ...string) ([]byte, error) {
		if strings.HasPrefix(strings.Join(args, " "), "merge-base") {
			return nil, errors.New("ancestry fault")
		}
		return baseRunner(ctx, root, args...)
	}
	if _, err := m.Integrate(string(id), false, ""); err == nil {
		t.Fatal("ancestry fault hidden")
	}
	if _, err := m.RecordManualIntegration(string(id), "HEAD", false, ""); err == nil {
		t.Fatal("manual ancestry fault hidden")
	}

	// Metadata writes after native mutation are partial mutations, never success.
	m.run = func(ctx context.Context, root string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.HasPrefix(joined, "merge ") {
			if err := os.Remove(filepath.Join(m.roots.InvokingRoot, ".awf", "efforts", string(id)+".json")); err != nil {
				t.Fatal(err)
			}
		}
		return baseRunner(ctx, root, args...)
	}
	if _, err := m.Integrate(string(id), false, ""); err == nil {
		t.Fatal("integration partial mutation hidden")
	}

	// Registration parsing rejects an empty record and handles the metadata form.
	if _, err := registrations(t.Context(), func(context.Context, string, ...string) ([]byte, error) { return []byte("\x00"), nil }, root); err == nil {
		t.Fatal("empty registration accepted")
	}
	_ = path
}

func TestCoverageClosureRemovePartialMutations(t *testing.T) {
	for _, tc := range []string{"worktree remove", "branch", "metadata"} {
		t.Run(tc, func(t *testing.T) {
			m, id := attachedManager(t)
			original := m.run
			m.run = func(ctx context.Context, root string, args ...string) ([]byte, error) {
				joined := strings.Join(args, " ")
				if strings.HasPrefix(joined, tc) || (tc == "metadata" && strings.HasPrefix(joined, "branch")) {
					if tc == "metadata" {
						_ = os.Remove(filepath.Join(m.roots.InvokingRoot, ".awf", "efforts", string(id)+".json"))
					} else {
						return nil, errors.New("injected " + tc)
					}
				}
				return original(ctx, root, args...)
			}
			if _, err := m.Remove(string(id), true, "recover"); err == nil {
				t.Fatalf("%s fault hidden", tc)
			}
		})
	}
}
