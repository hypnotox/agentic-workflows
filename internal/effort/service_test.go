package effort

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

func TestFinishRenamesCleansAndRetries(t *testing.T) {
	// invariant: tooling/effort-management:effort-record-authority
	root := initEffortRepo(t)
	failDelete := true
	service, err := Open(context.Background(), root, Options{
		Clock: func() time.Time { return time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC) },
		UUID:  func() (string, error) { return testIDA, nil },
		Fault: func(stage string) error {
			if stage == "finish.delete" && failDelete {
				failDelete = false
				return errors.New("interrupted")
			}
			return nil
		},
		Worktrees:    func(context.Context, string) ([]awfgit.WorktreeRegistration, error) { return nil, nil },
		BranchExists: func(context.Context, string, string) (bool, error) { return false, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.New("Restartable finish"); err != nil {
		t.Fatal(err)
	}
	result, err := service.Finish("restartable-finish")
	if err == nil || !result.Renamed || result.Cleaned || !strings.Contains(err.Error(), "changed bytes: yes") {
		t.Fatalf("first finish result=%#v err=%v", result, err)
	}
	if _, err := os.Lstat(filepath.Join(root, ".awf", "efforts", "restartable-finish")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("active directory remains: %v", err)
	}
	result, err = service.Finish("restartable-finish")
	if err != nil {
		t.Fatal(err)
	}
	if result.Renamed || !result.Cleaned {
		t.Fatalf("retry result = %#v", result)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".awf", "efforts"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("finish residue: %v", entries)
	}
}

func TestFinishRefusesEveryManagedTopologyFact(t *testing.T) {
	tests := map[string]struct {
		setup         func(*testing.T, string)
		registrations []awfgit.WorktreeRegistration
		branch        bool
	}{
		"path": {
			setup: func(t *testing.T, root string) {
				if err := os.MkdirAll(filepath.Join(root, ".awf", "worktrees", "guarded-finish"), 0o700); err != nil {
					t.Fatal(err)
				}
			},
		},
		"registration": {
			registrations: []awfgit.WorktreeRegistration{{Path: "/foreign", Branch: "refs/heads/awf/guarded-finish"}},
		},
		"branch": {branch: true},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			root := initEffortRepo(t)
			service, err := Open(context.Background(), root, Options{
				UUID: func() (string, error) { return testIDA, nil },
				Worktrees: func(context.Context, string) ([]awfgit.WorktreeRegistration, error) {
					return test.registrations, nil
				},
				BranchExists: func(context.Context, string, string) (bool, error) { return test.branch, nil },
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.New("Guarded finish"); err != nil {
				t.Fatal(err)
			}
			if test.setup != nil {
				test.setup(t, root)
			}
			result, err := service.Finish("guarded-finish")
			if err == nil || result != (FinishResult{}) || !strings.Contains(err.Error(), "changed bytes: no") || !strings.Contains(err.Error(), "worktree remove") {
				t.Fatalf("result=%#v err=%v", result, err)
			}
		})
	}
}

func TestFinishPreservesMismatchedAndMultipleTombstones(t *testing.T) {
	root := initEffortRepo(t)
	service, err := Open(context.Background(), root, Options{
		UUID:         func() (string, error) { return testIDA, nil },
		Worktrees:    func(context.Context, string) ([]awfgit.WorktreeRegistration, error) { return nil, nil },
		BranchExists: func(context.Context, string, string) (bool, error) { return false, nil },
		Fault: func(stage string) error {
			if stage == "finish.delete" {
				return errors.New("stop")
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.New("Foreign tombstone"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Finish("foreign-tombstone"); err == nil {
		t.Fatal("finish unexpectedly cleaned tombstone")
	}
	entries, err := os.ReadDir(filepath.Join(root, ".awf", "efforts"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("tombstone entries=%v err=%v", entries, err)
	}
	original := filepath.Join(root, ".awf", "efforts", entries[0].Name())
	duplicate := filepath.Join(root, ".awf", "efforts", finishingPrefix+testIDB+"-foreign-tombstone")
	if err := copyResidentDirectory(original, duplicate); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(duplicate, "state.json")
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.Replace(string(raw), testIDA, testIDB, 1))
	if err := os.WriteFile(statePath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = service.Finish("foreign-tombstone")
	if err == nil || !strings.Contains(err.Error(), "multiple finishing reservations") {
		t.Fatalf("multiple tombstone error = %v", err)
	}
	if _, statErr := os.Stat(original); statErr != nil {
		t.Fatalf("original tombstone changed: %v", statErr)
	}
	if _, statErr := os.Stat(duplicate); statErr != nil {
		t.Fatalf("duplicate tombstone changed: %v", statErr)
	}
}

func TestFinishFaultAndTopologyErrorBranches(t *testing.T) {
	for _, stage := range []string{"finish.rename", "finish.root-fsync", "finish.delete"} {
		t.Run(stage, func(t *testing.T) {
			root := initEffortRepo(t)
			service, err := Open(context.Background(), root, Options{
				UUID:         func() (string, error) { return testIDA, nil },
				Worktrees:    func(context.Context, string) ([]awfgit.WorktreeRegistration, error) { return nil, nil },
				BranchExists: func(context.Context, string, string) (bool, error) { return false, nil },
				Fault: func(got string) error {
					if got == stage {
						return errors.New("injected")
					}
					return nil
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.New("Finish faults"); err != nil {
				t.Fatal(err)
			}
			if _, err := service.Finish("finish-faults"); err == nil {
				t.Fatal("fault was ignored")
			}
		})
	}

	root := initEffortRepo(t)
	service, err := Open(context.Background(), root, Options{
		UUID: func() (string, error) { return testIDA, nil },
		Worktrees: func(context.Context, string) ([]awfgit.WorktreeRegistration, error) {
			return nil, errors.New("registration probe")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.New("Probe errors"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Finish("probe-errors"); err == nil || !strings.Contains(err.Error(), "registration") {
		t.Fatalf("registration error = %v", err)
	}
	service.worktrees = func(context.Context, string) ([]awfgit.WorktreeRegistration, error) { return nil, nil }
	service.branchExists = func(context.Context, string, string) (bool, error) { return false, errors.New("branch probe") }
	if _, err := service.Finish("probe-errors"); err == nil || !strings.Contains(err.Error(), "branch") {
		t.Fatalf("branch error = %v", err)
	}
	if _, err := service.Finish("bad_slug"); err == nil {
		t.Fatal("invalid finish slug accepted")
	}
	if _, err := service.Finish("missing-effort"); err == nil || !strings.Contains(err.Error(), "no active resident") {
		t.Fatalf("missing finish error = %v", err)
	}
	if yesNo(false) != "no" || yesNo(true) != "yes" {
		t.Fatal("yes/no rendering mismatch")
	}
}

func TestServiceResidentAndCorruptFinishBranches(t *testing.T) {
	root := initEffortRepo(t)
	if err := os.Symlink(t.TempDir(), filepath.Join(root, ".awf", "worktrees")); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(context.Background(), root, Options{}); err == nil {
		t.Fatal("symlinked worktrees root accepted")
	}

	root = initEffortRepo(t)
	service := openEffortService(t, root, time.Now().UTC())
	if err := os.Chmod(filepath.Join(root, ".awf", "efforts"), 0o111); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Finish("unreadable-finish"); err == nil {
		t.Fatal("unreadable finishing root accepted")
	}
	if err := os.Chmod(filepath.Join(root, ".awf", "efforts"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := service.New("Corrupt finish"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, ".awf", "efforts", "corrupt-finish", "memory.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Finish("corrupt-finish"); err == nil {
		t.Fatal("corrupt active effort finished")
	}
}

func TestFinishCleanupAndReservationBranches(t *testing.T) {
	root := initEffortRepo(t)
	service, err := Open(context.Background(), root, Options{
		UUID:         func() (string, error) { return testIDA, nil },
		Worktrees:    func(context.Context, string) ([]awfgit.WorktreeRegistration, error) { return nil, nil },
		BranchExists: func(context.Context, string, string) (bool, error) { return false, nil },
		Fault: func(stage string) error {
			if stage == "finish.delete" {
				return errors.New("retain tombstone")
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.New("Reserved finish"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Finish("reserved-finish"); err == nil {
		t.Fatal("finish fault ignored")
	}
	if _, err := service.New("Reserved finish"); err == nil || !strings.Contains(err.Error(), "reserved by finishing") {
		t.Fatalf("reservation error = %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".awf", "efforts"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries=%v err=%v", entries, err)
	}
	tombstone := filepath.Join(root, ".awf", "efforts", entries[0].Name())
	service.store.fault = nil
	service.removeTree = func(string) error { return errors.New("remove fault") }
	if _, err := service.Finish("reserved-finish"); err == nil || !strings.Contains(err.Error(), "remove fault") {
		t.Fatalf("remove error = %v", err)
	}
	if result, err := service.cleanTombstone("reserved-finish", filepath.Join(root, ".awf", "efforts", "wrong"), false); err == nil || result != (FinishResult{}) {
		t.Fatalf("wrong tombstone result=%#v err=%v", result, err)
	}
	mismatched := filepath.Join(root, ".awf", "efforts", finishingPrefix+testIDB+"-reserved-finish")
	if err := os.Rename(tombstone, mismatched); err != nil {
		t.Fatal(err)
	}
	if result, err := service.cleanTombstone("reserved-finish", mismatched, false); err == nil || result != (FinishResult{}) || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatched tombstone result=%#v err=%v", result, err)
	}
	tombstone = mismatched
	if _, err := os.Stat(tombstone); err != nil {
		t.Fatalf("tombstone changed: %v", err)
	}
}

func TestNativeEffortGitHelpers(t *testing.T) {
	root := initEffortRepo(t)
	if out, err := nativeGit(context.Background(), root, "rev-parse", "HEAD"); err != nil || strings.TrimSpace(string(out)) == "" {
		t.Fatalf("native git output=%q err=%v", out, err)
	}
	if _, err := nativeGit(context.Background(), root, "not-a-command"); err == nil {
		t.Fatal("native Git error hidden")
	}
	if exists, err := nativeBranchExists(context.Background(), root, "missing"); err != nil || exists {
		t.Fatalf("missing branch exists=%v err=%v", exists, err)
	}
	branch := strings.TrimSpace(runEffortGitOutput(t, "-C", root, "branch", "--show-current"))
	if exists, err := nativeBranchExists(context.Background(), root, branch); err != nil || !exists {
		t.Fatalf("current branch exists=%v err=%v", exists, err)
	}
	if _, err := nativeBranchExists(context.Background(), filepath.Join(root, "missing"), "master"); err == nil {
		t.Fatal("branch probe error hidden")
	}
}

func copyResidentDirectory(source, destination string) error {
	if err := os.Mkdir(destination, 0o700); err != nil {
		return err
	}
	for _, name := range []string{"state.json", "memory.md"} {
		raw, err := os.ReadFile(filepath.Join(source, name))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(destination, name), raw, 0o600); err != nil {
			return err
		}
	}
	return nil
}
