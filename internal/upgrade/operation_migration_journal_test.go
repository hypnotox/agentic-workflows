package upgrade

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// invariant: config/migrations-and-locks:lock-atomic-save (TestRunCommitsPlannedMigrationBeforeOrdinarySyncWithLockLast)
func currentLockImage(t *testing.T, root string) Image {
	t.Helper()
	content, err := os.ReadFile(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	return Image{Present: true, Mode: uint32(info.Mode().Perm()), Content: content}
}

func TestRunCommitsPlannedMigrationBeforeOrdinarySyncWithLockLast(t *testing.T) {
	root := t.TempDir()
	operationLock(t, root)
	setOperationLockSchema(t, root, 49)
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
			sawCurrentLock = lock.SchemaVersion == 50
			return presentation.Mutation{Status: "synced"}, nil
		},
		func(context.Context, string) error { return nil },
		func(string) (bool, error) { return true, nil },
		func() (int, int) { return 49, 50 },
		func(string) (string, int, error) { return "gate", 49, nil },
		func(context.Context, string) (MigrationResult, error) {
			return MigrationResult{Planned: []string{"future"}, Mutations: []FileMutation{
				{Path: path, Content: []byte("future\n"), Mode: 0o600},
				{Path: removedPath, Expected: Image{Present: true, Content: []byte("retired\n"), Mode: 0o640}, Remove: true},
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

// invariant: config/migrations-and-locks:lock-atomic-save (TestCommitMigrationPhysicallyAppliesLockLast)
func TestCommitMigrationPhysicallyAppliesLockLast(t *testing.T) {
	root := t.TempDir()
	operationLock(t, root)
	lock, err := manifest.Load(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	operation := productionJournalOperation()
	applyExpected := operation.applyExpected
	var paths []string
	operation.applyExpected = func(root, path string, prior, replacement Image) error {
		paths = append(paths, path)
		return applyExpected(root, path, prior, replacement)
	}
	_, err = commitMigrationWith(root, lock, currentLockImage(t, root), 50, []FileMutation{
		{Path: ".awf/z-last.yaml", Content: []byte("z\n"), Mode: 0o600},
		{Path: ".awf/a-first.yaml", Content: []byte("a\n"), Mode: 0o600},
	}, operation)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".awf/a-first.yaml", ".awf/z-last.yaml", LockRel()}
	if !slices.Equal(paths, want) {
		t.Fatalf("physical apply order = %v, want %v", paths, want)
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
			if _, err := commitMigration(root, lock, currentLockImage(t, root), 50, tc.mutations); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("commitMigration error = %v, want %q", err, tc.want)
			}
			got, err := manifest.Load(config.LockPath(root))
			if err != nil || got.SchemaVersion != 50 {
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
		if _, err := commitMigration(root, lock, currentLockImage(t, root), 50, []FileMutation{{Path: ".awf/loop", Mode: 0o600}}); err == nil || !strings.Contains(err.Error(), "final symlinks are unsupported") {
			t.Fatalf("mutation symlink error = %v", err)
		}
	})

	t.Run("lock", func(t *testing.T) {
		root := t.TempDir()
		lockPath := operationLock(t, root)
		lock, err := manifest.Load(lockPath)
		if err != nil {
			t.Fatal(err)
		}
		lockExpected := currentLockImage(t, root)
		if err := os.Remove(lockPath); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("awf.lock", lockPath); err != nil {
			t.Fatal(err)
		}
		if _, err := commitMigration(root, lock, lockExpected, 50, nil); err == nil || !strings.Contains(err.Error(), "expected a regular file") {
			t.Fatalf("lock image error = %v", err)
		}
	})
}

func TestCommitMigrationRefusesSymlinkAncestorEscape(t *testing.T) {
	root := t.TempDir()
	operationLock(t, root)
	lock, err := manifest.Load(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	victim := filepath.Join(outside, "victim")
	if err := os.WriteFile(victim, []byte("outside\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ".awf", "escape")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err := commitMigration(root, lock, currentLockImage(t, root), 50, []FileMutation{{Path: ".awf/escape/victim", Content: []byte("changed\n"), Mode: 0o600}}); err == nil {
		t.Fatal("journal planning accepted a symlink-ancestor escape")
	}
	contents, readErr := os.ReadFile(victim)
	info, statErr := os.Stat(victim)
	if readErr != nil || statErr != nil || string(contents) != "outside\n" || info.Mode().Perm() != 0o640 {
		t.Fatalf("outside victim changed = %q mode=%v errors=%v", contents, info, errors.Join(readErr, statErr))
	}
	if found, journalErr := JournalPresent(root); journalErr != nil || found {
		t.Fatalf("journal after confined refusal: found=%t err=%v", found, journalErr)
	}
}

// invariant: config/migrations-and-locks:migration-preimage-safe (TestCommitMigrationRefusesChangedPreimagesBeforeJournal)
func TestCommitMigrationRefusesChangedPreimagesBeforeJournal(t *testing.T) {
	for _, tc := range []struct {
		name       string
		path       string
		expected   Image
		plannedNow string
	}{
		{name: "source edited", path: ".awf/source", expected: Image{Present: true, Content: []byte("planned\n"), Mode: 0o640}, plannedNow: "edited\n"},
		{name: "destination created", path: ".awf/destination", plannedNow: "occupied\n"},
		{name: "backup occupied", path: ".awf/source.awf-bak", plannedNow: "occupied backup\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			operationLock(t, root)
			lock, err := manifest.Load(config.LockPath(root))
			if err != nil {
				t.Fatal(err)
			}
			if tc.expected.Present {
				if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(tc.path)), tc.expected.Content, os.FileMode(tc.expected.Mode)); err != nil {
					t.Fatal(err)
				}
			}
			mutations := []FileMutation{
				{Path: ".awf/a-first", Content: []byte("must not apply\n"), Mode: 0o600},
				{Path: tc.path, Expected: tc.expected, Content: []byte("replacement\n"), Mode: 0o600},
			}
			if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(tc.path)), []byte(tc.plannedNow), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := commitMigration(root, lock, currentLockImage(t, root), 51, mutations); err == nil || !strings.Contains(err.Error(), "changed after planning") {
				t.Fatalf("commitMigration error = %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, ".awf", "a-first")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("earlier mutation applied before preimage refusal: %v", err)
			}
			got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(tc.path)))
			if err != nil || string(got) != tc.plannedNow {
				t.Fatalf("externally changed path = %q, %v", got, err)
			}
			if found, err := JournalPresent(root); err != nil || found {
				t.Fatalf("journal after preimage refusal: found=%t err=%v", found, err)
			}
		})
	}
}

func TestCommitMigrationRefusesNonRegularPlannedSourceBeforeJournal(t *testing.T) {
	root := t.TempDir()
	operationLock(t, root)
	lock, err := manifest.Load(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	const rel = ".awf/source"
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	mutation := FileMutation{Path: rel, Expected: Image{Present: true, Mode: 0o644, Content: []byte("planned")}, Remove: true}
	if _, err := commitMigration(root, lock, currentLockImage(t, root), 51, []FileMutation{mutation}); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("commitMigration error = %v, want non-regular refusal", err)
	}
	if info, statErr := os.Stat(path); statErr != nil || !info.IsDir() {
		t.Fatalf("non-regular source changed: info=%v err=%v", info, statErr)
	}
	if found, presentErr := JournalPresent(root); presentErr != nil || found {
		t.Fatalf("journal after non-regular refusal: found=%t err=%v", found, presentErr)
	}
}

func TestRunRefusesStaleAuthorityLockBeforeJournalCreation(t *testing.T) {
	root := t.TempDir()
	lockPath := operationLock(t, root)
	called := false
	_, err := runOperation(t, root,
		func(context.Context, string) (presentation.Mutation, error) {
			called = true
			return presentation.Mutation{}, nil
		},
		func(context.Context, string) error { called = true; return nil },
		func(context.Context, string) (MigrationResult, error) {
			lock, loadErr := manifest.Load(lockPath)
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			lock.AWFVersion = "0.20.0"
			if saveErr := lock.Save(lockPath); saveErr != nil {
				t.Fatal(saveErr)
			}
			return MigrationResult{Planned: []string{"future"}, Mutations: []FileMutation{{Path: ".awf/a-first", Content: []byte("new\n"), Mode: 0o600}}}, nil
		},
		func(string) (string, int, error) { return "behind", 50, nil },
	)
	if err == nil || !strings.Contains(err.Error(), "authority lock changed after planning") {
		t.Fatalf("Run error = %v, want stale authority refusal", err)
	}
	if called {
		t.Fatal("gate or sync ran after authority lock changed")
	}
	if _, statErr := os.Stat(filepath.Join(root, ".awf", "a-first")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("mutation applied before stale-lock refusal: %v", statErr)
	}
	if found, presentErr := JournalPresent(root); presentErr != nil || found {
		t.Fatalf("journal after stale-lock refusal: found=%t err=%v", found, presentErr)
	}
}

func TestCommitMigrationRefusesStaleAuthorityModeBeforeJournalCreation(t *testing.T) {
	root := t.TempDir()
	operationLock(t, root)
	lock, err := manifest.Load(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	lockExpected := currentLockImage(t, root)
	if err := os.Chmod(config.LockPath(root), 0o600); err != nil {
		t.Fatal(err)
	}
	mutation := FileMutation{Path: ".awf/a-first", Content: []byte("new\n"), Mode: 0o600}
	if _, err := commitMigration(root, lock, lockExpected, 51, []FileMutation{mutation}); err == nil || !strings.Contains(err.Error(), "authority lock changed after planning") {
		t.Fatalf("commitMigration error = %v, want stale lock-mode refusal", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".awf", "a-first")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("mutation applied before stale-lock refusal: %v", statErr)
	}
	if found, presentErr := JournalPresent(root); presentErr != nil || found {
		t.Fatalf("journal after stale-lock refusal: found=%t err=%v", found, presentErr)
	}
}

func TestCommitMigrationRefusesStaleAuthorityBeforeFinalLockAndRollsBackFiles(t *testing.T) {
	root := t.TempDir()
	operationLock(t, root)
	lock, err := manifest.Load(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	lockExpected := currentLockImage(t, root)
	operation := productionJournalOperation()
	write := operation.write
	mutated := false
	operation.write = func(rootArg string, journal Journal) error {
		if err := write(rootArg, journal); err != nil {
			return err
		}
		if journal.Phase == phaseApplying && !mutated {
			if err := os.WriteFile(config.LockPath(root), []byte("external lock\n"), 0o600); err != nil {
				return err
			}
			mutated = true
		}
		return nil
	}
	mutation := FileMutation{Path: ".awf/a-first", Content: []byte("new\n"), Mode: 0o600}
	outcome, err := commitMigrationWith(root, lock, lockExpected, 51, []FileMutation{mutation}, operation)
	if !errors.Is(err, filesystem.ErrIdentityChanged) {
		t.Fatalf("commitMigration error = %v, want stale lock identity refusal", err)
	}
	if len(outcome.Changed) != 0 {
		t.Fatalf("changed = %#v, want fully rolled back", outcome.Changed)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".awf", "a-first")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("earlier mutation after rollback: %v", statErr)
	}
	if got, readErr := os.ReadFile(config.LockPath(root)); readErr != nil || string(got) != "external lock\n" {
		t.Fatalf("external lock = %q, %v", got, readErr)
	}
	if found, presentErr := JournalPresent(root); presentErr != nil || found {
		t.Fatalf("journal after completed rollback: found=%t err=%v", found, presentErr)
	}
}

// invariant: config/migrations-and-locks:migration-preimage-safe (TestCommitMigrationRefusesEntryReplacementAfterJournalPreparation)
func TestCommitMigrationRefusesEntryReplacementAfterJournalPreparation(t *testing.T) {
	root := t.TempDir()
	operationLock(t, root)
	lock, err := manifest.Load(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	const rel = ".awf/source"
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.WriteFile(path, []byte("planned\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	operation := productionJournalOperation()
	write := operation.write
	replaced := false
	operation.write = func(root string, journal Journal) error {
		if err := write(root, journal); err != nil {
			return err
		}
		if journal.Phase == phaseApplying && !replaced {
			replacement := filepath.Join(root, ".awf", "external")
			if err := os.WriteFile(replacement, []byte("external\n"), 0o600); err != nil {
				return err
			}
			if err := os.Rename(replacement, path); err != nil {
				return err
			}
			replaced = true
		}
		return nil
	}
	mutation := FileMutation{Path: rel, Expected: Image{Present: true, Content: []byte("planned\n"), Mode: 0o640}, Remove: true}
	if _, err := commitMigrationWith(root, lock, currentLockImage(t, root), 51, []FileMutation{mutation}, operation); !errors.Is(err, filesystem.ErrIdentityChanged) {
		t.Fatalf("commitMigration error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "external\n" {
		t.Fatalf("external replacement = %q, %v", got, err)
	}
	if found, err := JournalPresent(root); err != nil || found {
		t.Fatalf("journal after identity refusal: found=%t err=%v", found, err)
	}
	gotLock, err := manifest.Load(config.LockPath(root))
	if err != nil || gotLock.SchemaVersion != 50 {
		t.Fatalf("lock after identity refusal: schema=%v err=%v", gotLock.SchemaVersion, err)
	}
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
	apply := operation.applyExpected
	operation.applyExpected = func(root, path string, prior, replacement Image) error {
		if path == ".awf/second" && replacement.Present && string(replacement.Content) == "second\n" {
			return errors.New("injected second apply failure")
		}
		return apply(root, path, prior, replacement)
	}
	if _, err := commitMigrationWith(root, lock, currentLockImage(t, root), 50, mutations, operation); err == nil || !strings.Contains(err.Error(), "apply .awf/second") {
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
	if err != nil || got.SchemaVersion != 50 {
		t.Fatalf("lock after rollback: schema=%v err=%v", got.SchemaVersion, err)
	}
}

func TestCommitMigrationRejectsInvalidFinalAuthority(t *testing.T) {
	root := t.TempDir()
	operationLock(t, root)
	lock := &manifest.Lock{AWFVersion: "0.39.2", SchemaVersion: 50, Files: map[string]manifest.Entry{}, InitializedWithVersion: "bad"}
	if _, err := commitMigration(root, lock, currentLockImage(t, root), 50, nil); err == nil || !strings.Contains(err.Error(), "initializedWithVersion") {
		t.Fatalf("invalid final authority error = %v", err)
	}
}

func TestRunWrapsRejectedMigrationPlanAsJournalFailure(t *testing.T) {
	root := t.TempDir()
	operationLock(t, root)
	_, err := Run(context.Background(), root, nil, nil, func(string) (bool, error) { return true, nil },
		func() (int, int) { return 50, 50 },
		func(string) (string, int, error) { return "gate", 50, nil },
		func(context.Context, string) (MigrationResult, error) {
			return MigrationResult{Planned: []string{"future"}, Mutations: []FileMutation{{Path: LockRel(), Mode: 0o600}}}, nil
		},
		func() string { return "current schema" },
	)
	var failure journalFailure
	if !errors.As(err, &failure) || !strings.Contains(err.Error(), "invalid planned migration path") {
		t.Fatalf("Run error = %T %v, want journal failure", err, err)
	}
}

func TestCommitMigrationPreservesInjectedPreimageFailures(t *testing.T) {
	for _, failedPath := range []string{".awf/future", LockRel()} {
		t.Run(failedPath, func(t *testing.T) {
			root := t.TempDir()
			operationLock(t, root)
			lock, err := manifest.Load(config.LockPath(root))
			if err != nil {
				t.Fatal(err)
			}
			failure := errors.New("migration preimage")
			operation := productionJournalOperation()
			image := operation.imageOf
			operation.imageOf = func(root, path string) (Image, error) {
				if path == failedPath {
					return Image{}, failure
				}
				return image(root, path)
			}
			if _, err := commitMigrationWith(root, lock, currentLockImage(t, root), 50, []FileMutation{{Path: ".awf/future", Content: []byte("future"), Mode: 0o600}}, operation); !errors.Is(err, failure) {
				t.Fatalf("error = %v, want %v", err, failure)
			}
			if found, err := JournalPresent(root); err != nil || found {
				t.Fatalf("journal = %t, %v", found, err)
			}
		})
	}
}
