package upgrade

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// invariant: config/migrations-and-locks:lock-atomic-save (TestRunCommitsPlannedMigrationBeforeOrdinarySyncWithLockLast)
func TestRunCommitsPlannedMigrationBeforeOrdinarySyncWithLockLast(t *testing.T) {
	root := t.TempDir()
	operationLock(t, root)
	const path = ".awf/future.yaml"
	const removedPath = ".awf/retired.yaml"
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(removedPath)), []byte("retired\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	var sawCurrentLock bool
	_, err := Run(context.Background(), root,
		func(context.Context, string) (presentation.Mutation, error) {
			lock, loadErr := manifest.Load(config.LockPath(root))
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			sawCurrentLock = lock.SchemaVersion == 47
			return presentation.Mutation{Status: "synced"}, nil
		},
		func(context.Context, string) error { return nil },
		func(string) bool { return true },
		func() (int, int) { return 46, 47 },
		func(string) (string, int, error) { return "gate", 46, nil },
		func(context.Context, string) (MigrationResult, error) {
			return MigrationResult{Applied: []string{"future"}, Mutations: []FileMutation{
				{Path: path, Content: []byte("future\n"), Mode: 0o600},
				{Path: removedPath, Remove: true},
			}}, nil
		},
		func() string { return "current schema" },
	)
	if err != nil {
		t.Fatal(err)
	}
	if !sawCurrentLock {
		t.Fatal("ordinary sync ran before the migration journal committed its current lock")
	}
	contents, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if readErr != nil || string(contents) != "future\n" {
		t.Fatalf("planned mutation = %q, err=%v", contents, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(removedPath))); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("planned removal remains: %v", statErr)
	}
	if found, journalErr := JournalPresent(root); journalErr != nil || found {
		t.Fatalf("journal after committed migration: found=%t err=%v", found, journalErr)
	}
}

func TestCommitMigrationRejectsInvalidPlansBeforeMutation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		mutations []FileMutation
		want      string
	}{
		{"lock path", []FileMutation{{Path: LockRel(), Mode: 0o600}}, "invalid planned migration path"},
		{"unsafe path", []FileMutation{{Path: "../outside", Mode: 0o600}}, "invalid planned migration path"},
		{"duplicate path", []FileMutation{{Path: "a", Mode: 0o600}, {Path: "a", Mode: 0o600}}, "invalid planned migration path"},
		{"missing mode", []FileMutation{{Path: "a"}}, "has no mode"},
		{"removal data", []FileMutation{{Path: "a", Remove: true, Content: []byte("x")}}, "carries replacement data"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			operationLock(t, root)
			lock, err := manifest.Load(config.LockPath(root))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := commitMigration(root, lock, 47, tc.mutations); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("commitMigration error = %v, want %q", err, tc.want)
			}
			got, err := manifest.Load(config.LockPath(root))
			if err != nil || got.SchemaVersion != 46 {
				t.Fatalf("lock changed after rejected plan: schema=%v err=%v", got.SchemaVersion, err)
			}
		})
	}
}

func TestCommitMigrationPreservesPlanningReadFailures(t *testing.T) {
	t.Run("mutation", func(t *testing.T) {
		root := t.TempDir()
		operationLock(t, root)
		lock, err := manifest.Load(config.LockPath(root))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, ".awf", "loop")
		if err := os.Symlink("loop", path); err != nil {
			t.Fatal(err)
		}
		if _, err := commitMigration(root, lock, 47, []FileMutation{{Path: ".awf/loop", Mode: 0o600}}); !errors.Is(err, syscall.ELOOP) {
			t.Fatalf("mutation read error = %v", err)
		}
	})

	t.Run("lock", func(t *testing.T) {
		root := t.TempDir()
		lockPath := operationLock(t, root)
		lock, err := manifest.Load(lockPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(lockPath); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("awf.lock", lockPath); err != nil {
			t.Fatal(err)
		}
		if _, err := commitMigration(root, lock, 47, nil); !errors.Is(err, syscall.ELOOP) {
			t.Fatalf("lock read error = %v", err)
		}
	})
}

func TestCommitMigrationRollsBackAppliedFilesBeforeLock(t *testing.T) {
	root := t.TempDir()
	operationLock(t, root)
	lock, err := manifest.Load(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	mutations := []FileMutation{
		{Path: ".awf/first", Content: []byte("first\n"), Mode: 0o600},
		{Path: ".awf/second", Content: []byte("second\n"), Mode: 0o600},
	}
	operation := productionJournalOperation()
	operation.applyImage = func(root, path string, image Image) error {
		if path == ".awf/second" && image.Present && string(image.Content) == "second\n" {
			return errors.New("injected second apply failure")
		}
		return applyImage(root, path, image)
	}
	if _, err := commitMigrationWith(root, lock, 47, mutations, operation); err == nil || !strings.Contains(err.Error(), "apply .awf/second") {
		t.Fatalf("commitMigration error = %v, want apply failure", err)
	}
	for _, name := range []string{"first", "second"} {
		if _, err := os.Stat(filepath.Join(root, ".awf", name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s survived rollback: %v", name, err)
		}
	}
	if found, err := JournalPresent(root); err != nil || found {
		t.Fatalf("journal after complete rollback: found=%t err=%v", found, err)
	}
	got, err := manifest.Load(config.LockPath(root))
	if err != nil || got.SchemaVersion != 46 {
		t.Fatalf("lock after rollback: schema=%v err=%v", got.SchemaVersion, err)
	}
}

func TestCommitMigrationRejectsInvalidFinalAuthority(t *testing.T) {
	root := t.TempDir()
	operationLock(t, root)
	lock := &manifest.Lock{AWFVersion: "0.39.2", SchemaVersion: 46, Files: map[string]manifest.Entry{}, InitializedWithVersion: "bad"}
	if _, err := commitMigration(root, lock, 47, nil); err == nil || !strings.Contains(err.Error(), "initializedWithVersion") {
		t.Fatalf("invalid final authority error = %v", err)
	}
}

func TestRunWrapsRejectedMigrationPlanAsJournalFailure(t *testing.T) {
	root := t.TempDir()
	operationLock(t, root)
	_, err := Run(context.Background(), root, nil, nil, func(string) bool { return true },
		func() (int, int) { return 46, 47 },
		func(string) (string, int, error) { return "gate", 46, nil },
		func(context.Context, string) (MigrationResult, error) {
			return MigrationResult{Applied: []string{"future"}, Mutations: []FileMutation{{Path: LockRel(), Mode: 0o600}}}, nil
		},
		func() string { return "current schema" },
	)
	var failure journalFailure
	if !errors.As(err, &failure) || !strings.Contains(err.Error(), "invalid planned migration path") {
		t.Fatalf("Run error = %T %v, want journal failure", err, err)
	}
}
