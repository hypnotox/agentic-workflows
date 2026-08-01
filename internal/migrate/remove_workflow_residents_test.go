package migrate

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// invariant: config/migrations-and-locks:workflow-telemetry-config-migration (TestRemoveWorkflowResidentsMigration)
// invariant: rendering/singletons-and-payloads:resident-output-preservation (TestRemoveWorkflowResidentsMigration)
func TestRemoveWorkflowResidentsMigration(t *testing.T) {
	newRoot := func(t *testing.T) string {
		t.Helper()
		return t.TempDir()
	}
	makeResidents := func(t *testing.T, root string) {
		t.Helper()
		for _, name := range []string{"metrics", "assignments"} {
			if err := os.MkdirAll(filepath.Join(root, ".awf", name, "nested"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".awf", name, "nested", "resident"), []byte(name), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		for _, name := range []string{"efforts", "memory", "worktrees"} {
			if err := os.MkdirAll(filepath.Join(root, ".awf", name, "retained"), 0o700); err != nil {
				t.Fatal(err)
			}
		}
	}
	assertRetained := func(t *testing.T, root string) {
		t.Helper()
		for _, name := range []string{"efforts", "memory", "worktrees"} {
			if _, err := os.Stat(filepath.Join(root, ".awf", name, "retained")); err != nil {
				t.Fatalf("retained %s was touched: %v", name, err)
			}
		}
	}

	t.Run("inspection-error", func(t *testing.T) {
		if err := removeWorkflowResidents(newRoot(t), &bytes.Buffer{}, func(string) (os.FileInfo, error) { return nil, os.ErrPermission }, os.RemoveAll); err == nil {
			t.Fatal("inspection error accepted")
		}
	})
	t.Run("non-Git fixture falls back to supplied root", func(t *testing.T) {
		root := newRoot(t)
		makeResidents(t, root)
		if err := applyRemoveWorkflowResidents(testContext(t), root, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"metrics", "assignments"} {
			if _, err := os.Lstat(filepath.Join(root, ".awf", name)); !os.IsNotExist(err) {
				t.Fatalf("fixture %s still exists: %v", name, err)
			}
		}
	})
	t.Run("broken Git checkout propagates without deletion", func(t *testing.T) {
		root := newRoot(t)
		makeResidents(t, root)
		if err := os.WriteFile(filepath.Join(root, ".git"), []byte("not a gitdir pointer\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := awfgit.Open(root); err == nil || errors.Is(err, awfgit.ErrNotARepository) {
			t.Fatalf("fixture did not produce a present broken Git error: %v", err)
		}
		if err := applyRemoveWorkflowResidents(testContext(t), root, &bytes.Buffer{}); err == nil {
			t.Fatal("broken Git checkout fell back to fixture root")
		}
		for _, name := range []string{"metrics", "assignments"} {
			if _, err := os.Stat(filepath.Join(root, ".awf", name, "nested", "resident")); err != nil {
				t.Fatalf("broken checkout changed %s: %v", name, err)
			}
		}
	})
	t.Run("Git safety error propagates without deletion", func(t *testing.T) {
		primary := filepath.Join(t.TempDir(), "primary")
		gitfixture.InitNativeAt(t, primary)
		makeResidents(t, primary)
		alias := filepath.Join(filepath.Dir(primary), "alias")
		if err := os.Symlink(primary, alias); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if err := applyRemoveWorkflowResidents(testContext(t), alias, &bytes.Buffer{}); err == nil {
			t.Fatal("unsafe Git checkout fell back to fixture root")
		}
		for _, name := range []string{"metrics", "assignments"} {
			if _, err := os.Stat(filepath.Join(primary, ".awf", name, "nested", "resident")); err != nil {
				t.Fatalf("safety error changed %s: %v", name, err)
			}
		}
	})
	t.Run("absent", func(t *testing.T) {
		root := newRoot(t)
		var out bytes.Buffer
		if err := applyRemoveWorkflowResidents(testContext(t), root, &out); err != nil {
			t.Fatal(err)
		}
		if got, want := out.String(), "remove-workflow-residents: metrics already absent\nremove-workflow-residents: assignments already absent\n"; got != want {
			t.Fatalf("output = %q, want %q", got, want)
		}
	})
	t.Run("nested-roots", func(t *testing.T) {
		root := newRoot(t)
		makeResidents(t, root)
		var out bytes.Buffer
		if err := applyRemoveWorkflowResidents(testContext(t), root, &out); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"metrics", "assignments"} {
			if _, err := os.Lstat(filepath.Join(root, ".awf", name)); !os.IsNotExist(err) {
				t.Fatalf("%s still exists: %v", name, err)
			}
		}
		assertRetained(t, root)
	})
	t.Run("ordered-output", func(t *testing.T) {
		root := newRoot(t)
		makeResidents(t, root)
		var out bytes.Buffer
		if err := applyRemoveWorkflowResidents(testContext(t), root, &out); err != nil {
			t.Fatal(err)
		}
		if got, want := out.String(), "remove-workflow-residents: metrics removed\nremove-workflow-residents: assignments removed\n"; got != want {
			t.Fatalf("output = %q, want %q", got, want)
		}
	})
	for _, name := range []string{"unsafe-symlink", "non-directory"} {
		t.Run(name, func(t *testing.T) {
			root := newRoot(t)
			path := filepath.Join(root, ".awf", "metrics")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if name == "unsafe-symlink" {
				if err := os.Symlink(t.TempDir(), path); err != nil {
					t.Skip(err)
				}
			} else if err := os.WriteFile(path, []byte("unsafe"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := applyRemoveWorkflowResidents(testContext(t), root, &bytes.Buffer{}); err == nil {
				t.Fatal("unsafe root accepted")
			}
			if _, err := os.Lstat(path); err != nil {
				t.Fatalf("unsafe root was removed: %v", err)
			}
		})
	}
	t.Run("partial-failure-then-retry", func(t *testing.T) {
		root := newRoot(t)
		makeResidents(t, root)
		var out bytes.Buffer
		calls := 0
		err := removeWorkflowResidents(root, &out, os.Lstat, func(path string) error {
			calls++
			if calls == 2 {
				return os.ErrPermission
			}
			return os.RemoveAll(path)
		})
		if err == nil || !strings.Contains(err.Error(), "assignments") {
			t.Fatalf("partial error = %v", err)
		}
		if _, err := os.Lstat(filepath.Join(root, ".awf", "metrics")); !os.IsNotExist(err) {
			t.Fatalf("metrics not removed before failure: %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, ".awf", "assignments", "nested", "resident")); err != nil {
			t.Fatalf("assignments changed before retry: %v", err)
		}
		if err := applyRemoveWorkflowResidents(testContext(t), root, &out); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "metrics already absent\nremove-workflow-residents: assignments removed\n") {
			t.Fatalf("retry output = %q", out.String())
		}
		assertRetained(t, root)
	})
	t.Run("linked-worktree-primary-root", func(t *testing.T) {
		primary := filepath.Join(t.TempDir(), "primary")
		repo := gitfixture.InitNativeAt(t, primary)
		if err := os.WriteFile(filepath.Join(primary, "tracked"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		gitfixture.NativeAdd(t, repo, "tracked")
		gitfixture.NativeCommit(t, repo, "base")
		linked := filepath.Join(filepath.Dir(primary), "linked")
		gitfixture.NativeWorktreeAddDetached(t, repo, linked, "HEAD")
		makeResidents(t, primary)
		var out bytes.Buffer
		if err := applyRemoveWorkflowResidents(testContext(t), linked, &out); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"metrics", "assignments"} {
			if _, err := os.Lstat(filepath.Join(primary, ".awf", name)); !os.IsNotExist(err) {
				t.Fatalf("primary %s still exists: %v", name, err)
			}
			if _, err := os.Lstat(filepath.Join(linked, ".awf", name)); !os.IsNotExist(err) {
				t.Fatalf("linked %s unexpectedly changed: %v", name, err)
			}
		}
		assertRetained(t, primary)
	})
}
