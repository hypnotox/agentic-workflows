package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// invariant: tooling/effort-management:managed-worktree-lifecycle
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

// invariant: tooling/effort-management:managed-worktree-lifecycle
func TestCoverageClosureSafetyAndRestartBranches(t *testing.T) {
	root := newWorktreeRepo(t)
	m, err := Open(t.Context(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}

	// A live directory which is not a repository is an identity fault, not a
	// permission or collision result.
	plain := filepath.Join(t.TempDir(), "plain")
	if err := os.MkdirAll(plain, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := m.validateManagedTarget(plain); err == nil {
		t.Fatal("non-repository target accepted")
	}

	// A registered worktree in this repository is an ordinary collision.
	collision, err := m.efforts.New("collision", false)
	if err != nil {
		t.Fatal(err)
	}
	collisionPath, err := m.managed(collision.ID)
	if err != nil {
		t.Fatal(err)
	}
	git(t, "-C", root, "worktree", "add", "-b", "collision-branch", collisionPath, "HEAD")
	if _, err := m.Add(collision.ID, "HEAD"); err == nil || !strings.Contains(err.Error(), "managed path already exists") {
		t.Fatalf("collision result = %v", err)
	}
	git(t, "-C", root, "worktree", "remove", collisionPath)

	// Each pre-registration failure has independently discriminating evidence.
	for _, mode := range []string{"registration", "resident"} {
		t.Run("add failure "+mode, func(t *testing.T) {
			r, err := m.efforts.New(mode, false)
			if err != nil {
				t.Fatal(err)
			}
			path, err := m.managed(r.ID)
			if err != nil {
				t.Fatal(err)
			}
			original := m.run
			m.run = func(ctx context.Context, commandRoot string, args ...string) ([]byte, error) {
				joined := strings.Join(args, " ")
				if strings.HasPrefix(joined, "worktree list") && mode == "registration" {
					return []byte("worktree " + path + "\x00branch refs/heads/awf/" + r.ID + "\x00\x00"), nil
				}
				if strings.HasPrefix(joined, "worktree add") {
					if mode == "resident" {
						if err := os.Mkdir(path, 0o700); err != nil {
							t.Fatal(err)
						}
					}
					return nil, errors.New("injected pre-registration add failure")
				}
				return original(ctx, commandRoot, args...)
			}
			_, err = m.Add(r.ID, "HEAD")
			m.run = original
			if err == nil || !strings.Contains(err.Error(), "partial Git mutation") {
				t.Fatalf("settlement result = %v", err)
			}
		})
	}

	// Removing a manually recorded integration must distinguish target and
	// ancestry probe faults, including after reopening the service.
	manual, err := m.efforts.New("manual probes", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Add(manual.ID, "HEAD"); err != nil {
		t.Fatal(err)
	}
	manualPath, _ := m.managed(manual.ID)
	commitFile(t, manualPath, "manual-probe", "manual-probe")
	if _, err = m.RecordManualIntegration(manual.ID, "HEAD", true, "external integration"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(t.Context(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	baseRunner := reopened.run
	reopened.run = func(ctx context.Context, commandRoot string, args ...string) ([]byte, error) {
		if strings.HasPrefix(strings.Join(args, " "), "rev-parse --verify HEAD^{commit}") && commandRoot == reopened.roots.InvokingRoot {
			return nil, errors.New("injected target probe failure")
		}
		return baseRunner(ctx, commandRoot, args...)
	}
	if _, err = reopened.Remove(manual.ID, true, "approved repair"); err == nil || !strings.Contains(err.Error(), "target probe failure") {
		t.Fatalf("target probe result = %v", err)
	}
	reopened.run = func(ctx context.Context, commandRoot string, args ...string) ([]byte, error) {
		if strings.HasPrefix(strings.Join(args, " "), "merge-base") {
			return nil, errors.New("injected ancestry probe failure")
		}
		return baseRunner(ctx, commandRoot, args...)
	}
	if _, err = reopened.Remove(manual.ID, true, "approved repair"); err == nil || !strings.Contains(err.Error(), "ancestry probe failure") {
		t.Fatalf("ancestry probe result = %v", err)
	}
}

func TestCoverageClosureSafeManagedPathRoot(t *testing.T) {
	if err := safeManagedPath(string(filepath.Separator)); err == nil {
		t.Fatal("root accepted as managed path")
	}
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
