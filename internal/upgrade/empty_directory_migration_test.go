package upgrade

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

func emptyDirectoryFixture(t *testing.T) (string, *manifest.Lock, Image, []FileMutation) {
	t.Helper()
	root := t.TempDir()
	operationLock(t, root)
	oldDir := filepath.Join(root, ".awf", "old", "nested")
	if err := os.MkdirAll(oldDir, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(oldDir, 0o777); err != nil {
		t.Fatal(err)
	}
	oldFile := filepath.Join(oldDir, "part.md")
	if err := os.WriteFile(oldFile, []byte("custom\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(oldFile, 0o640); err != nil {
		t.Fatal(err)
	}
	lock, err := manifest.Load(filepath.Join(root, LockRel()))
	if err != nil {
		t.Fatal(err)
	}
	mutations := []FileMutation{
		{Path: ".awf/old/nested/part.md", Expected: Image{Present: true, Mode: 0o640, Content: []byte("custom\n")}, Remove: true},
		{Path: ".awf/old/nested", Expected: Image{Present: true, Mode: 0o777}, ExpectedEntries: []string{"part.md"}, Remove: true, EmptyDirectory: true},
	}
	return root, lock, currentLockImage(t, root), mutations
}

// invariant: config/migrations-and-locks:workflow-surface-source-migration (TestCommitMigrationPrunesPlannedEmptyDirectory)
func TestCommitMigrationPrunesPlannedEmptyDirectory(t *testing.T) {
	root, lock, lockImage, mutations := emptyDirectoryFixture(t)
	if _, err := commitMigration(root, lock, lockImage, 51, mutations); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(root, ".awf", "old", "nested")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("obsolete directory remains: %v", err)
	}
	got, err := manifest.Load(filepath.Join(root, LockRel()))
	if err != nil || got.SchemaVersion != 51 {
		t.Fatalf("final lock schema=%v err=%v", got.SchemaVersion, err)
	}
}

// invariant: config/migrations-and-locks:migration-preimage-safe (TestCommitMigrationRefusesChangedDirectoryInventoryBeforeJournal)
func TestCommitMigrationRefusesChangedDirectoryInventoryBeforeJournal(t *testing.T) {
	root, lock, lockImage, mutations := emptyDirectoryFixture(t)
	extra := filepath.Join(root, ".awf", "old", "nested", "external.txt")
	if err := os.WriteFile(extra, []byte("external\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := commitMigration(root, lock, lockImage, 51, mutations); err == nil || !strings.Contains(err.Error(), "changed after planning") {
		t.Fatalf("commit error = %v", err)
	}
	if contents, err := os.ReadFile(extra); err != nil || string(contents) != "external\n" {
		t.Fatalf("external child = %q err=%v", contents, err)
	}
	if found, err := JournalPresent(root); err != nil || found {
		t.Fatalf("journal after preimage refusal found=%t err=%v", found, err)
	}
}

// invariant: config/migrations-and-locks:migration-preimage-safe (TestCommitMigrationPreservesChildAddedDuringDirectoryPrune)
func TestCommitMigrationPreservesChildAddedDuringDirectoryPrune(t *testing.T) {
	root, lock, lockImage, mutations := emptyDirectoryFixture(t)
	operation := productionJournalOperation()
	applyExpected := operation.applyExpected
	operation.applyExpected = func(root, path string, prior, replacement Image) error {
		err := applyExpected(root, path, prior, replacement)
		if err == nil && path == ".awf/old/nested/part.md" {
			err = os.WriteFile(filepath.Join(root, ".awf", "old", "nested", "external.txt"), []byte("external\n"), 0o600)
		}
		return err
	}
	outcome, err := commitMigrationWith(root, lock, lockImage, 51, mutations, operation)
	if err == nil || !errors.Is(err, filesystem.ErrDirectoryNotEmpty) {
		t.Fatalf("commit error = %v", err)
	}
	if len(outcome.Changed) != 0 {
		t.Fatalf("rollback retained changed axes: %#v", outcome.Changed)
	}
	for path, want := range map[string]string{
		filepath.Join(root, ".awf", "old", "nested", "part.md"):      "custom\n",
		filepath.Join(root, ".awf", "old", "nested", "external.txt"): "external\n",
	} {
		if contents, readErr := os.ReadFile(path); readErr != nil || string(contents) != want {
			t.Errorf("preserved %s = %q err=%v", path, contents, readErr)
		}
	}
	if found, journalErr := JournalPresent(root); journalErr != nil || found {
		t.Fatalf("journal after rollback found=%t err=%v", found, journalErr)
	}
}

// invariant: config/migrations-and-locks:migration-preimage-safe (TestCommitMigrationRefusesDirectoryReplacementBeforePrune)
func TestCommitMigrationRefusesDirectoryReplacementBeforePrune(t *testing.T) {
	root, lock, lockImage, mutations := emptyDirectoryFixture(t)
	operation := productionJournalOperation()
	applyExpected := operation.applyExpected
	operation.applyExpected = func(root, path string, prior, replacement Image) error {
		err := applyExpected(root, path, prior, replacement)
		if err == nil && path == ".awf/old/nested/part.md" {
			current := filepath.Join(root, ".awf", "old", "nested")
			if err := os.Remove(current); err != nil {
				return err
			}
			if err := os.Mkdir(current, 0o777); err != nil {
				return err
			}
			if err := os.Chmod(current, 0o777); err != nil {
				return err
			}
		}
		return err
	}
	outcome, err := commitMigrationWith(root, lock, lockImage, 51, mutations, operation)
	if err == nil || !errors.Is(err, filesystem.ErrIdentityChanged) {
		t.Fatalf("commit error = %v", err)
	}
	replacement := filepath.Join(root, ".awf", "old", "nested")
	if info, statErr := os.Stat(replacement); statErr != nil || !info.IsDir() || info.Mode().Perm() != 0o777 {
		t.Fatalf("replacement directory was not preserved: info=%v err=%v", info, statErr)
	}
	if _, readErr := os.Lstat(filepath.Join(replacement, "part.md")); !errors.Is(readErr, os.ErrNotExist) {
		t.Fatalf("prior child was restored into replacement directory: %v", readErr)
	}
	if !journalPresence(t, root) || len(outcome.Changed) == 0 {
		t.Fatalf("unsafe rollback did not retain recovery evidence: %#v", outcome)
	}
}

// invariant: config/migrations-and-locks:migration-preimage-safe (TestCommitMigrationRefusesRecreatedDirectoryDuringRollbackAndRecovery)
func TestCommitMigrationRefusesRecreatedDirectoryDuringRollbackAndRecovery(t *testing.T) {
	root, lock, lockImage, mutations := emptyDirectoryFixture(t)
	operation := productionJournalOperation()
	applyExpected := operation.applyExpected
	operation.applyExpected = func(root, path string, prior, replacement Image) error {
		if path == LockRel() {
			recreated := filepath.Join(root, ".awf", "old", "nested")
			if err := os.Mkdir(recreated, 0o777); err != nil {
				return err
			}
			if err := os.Chmod(recreated, 0o777); err != nil {
				return err
			}
			return errors.New("injected lock failure")
		}
		return applyExpected(root, path, prior, replacement)
	}
	outcome, err := commitMigrationWith(root, lock, lockImage, 51, mutations, operation)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("commit error = %v", err)
	}
	part := filepath.Join(root, ".awf", "old", "nested", "part.md")
	if _, readErr := os.Lstat(part); !errors.Is(readErr, os.ErrNotExist) {
		t.Fatalf("prior child was restored into recreated directory: %v", readErr)
	}
	if !journalPresence(t, root) || len(outcome.Changed) == 0 {
		t.Fatalf("unsafe rollback did not retain recovery evidence: %#v", outcome)
	}
	if recovered, recoverErr := Recover(root); recoverErr == nil || !strings.Contains(recoverErr.Error(), "preflight directory identity is unavailable") {
		t.Fatalf("recovery = %#v, %v; want recreated-directory refusal", recovered, recoverErr)
	}
	if _, readErr := os.Lstat(part); !errors.Is(readErr, os.ErrNotExist) {
		t.Fatalf("recovery restored prior child into recreated directory: %v", readErr)
	}
	if !journalPresence(t, root) {
		t.Fatal("recovery removed journal despite unsafe recreated directory")
	}
}

// invariant: config/migrations-and-locks:migration-preimage-safe (TestCommitMigrationRestoresPrunedDirectoryOnLaterFailure)
func TestCommitMigrationRestoresPrunedDirectoryOnLaterFailure(t *testing.T) {
	root, lock, lockImage, mutations := emptyDirectoryFixture(t)
	operation := productionJournalOperation()
	applyExpected := operation.applyExpected
	operation.applyExpected = func(root, path string, prior, replacement Image) error {
		if path == LockRel() {
			return errors.New("injected lock failure")
		}
		return applyExpected(root, path, prior, replacement)
	}
	outcome, err := commitMigrationWith(root, lock, lockImage, 51, mutations, operation)
	if err == nil || !strings.Contains(err.Error(), "injected lock failure") {
		t.Fatalf("commit error = %v", err)
	}
	if len(outcome.Changed) != 0 {
		t.Fatalf("rollback retained changed axes: %#v", outcome.Changed)
	}
	part := filepath.Join(root, ".awf", "old", "nested", "part.md")
	contents, readErr := os.ReadFile(part)
	info, statErr := os.Stat(filepath.Dir(part))
	if readErr != nil || statErr != nil || string(contents) != "custom\n" || info.Mode().Perm() != 0o777 {
		t.Fatalf("restored tree content=%q mode=%v errors=%v", contents, info, errors.Join(readErr, statErr))
	}
}
