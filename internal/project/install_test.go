package project

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/filepublication"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestSyncPrunesResidentLockEntryFromResidentRoot(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	const obsolete = ".awf/efforts/obsolete"
	lock.Files[obsolete] = manifest.Entry{}
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.SyncReport(testContext(t)); err != nil {
		t.Fatalf("resident-root prune path failed: %v", err)
	}
}

func TestUninstallPreservesResidentState(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	dynamic := filepath.Join(root, ".awf", "efforts", "efforts", "e", "sessions", "s.jsonl")
	testsupport.WriteFile(t, dynamic, "resident\n")
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	lock.Files[".awf/efforts/efforts/e/sessions/s.jsonl"] = manifest.Entry{}
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	report, err := resident.Uninstall(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(report.PreservedRoots, "efforts") {
		t.Fatal("resident efforts were not reported preserved")
	}
	for _, path := range []string{dynamic, filepath.Join(root, ".awf", "efforts", ".gitignore")} {
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("preserved path %s: %v", path, err)
		}
	}
	if _, err := os.Stat(lockFile(root)); !os.IsNotExist(err) {
		t.Fatalf("lock survived uninstall: %v", err)
	}
}

func TestUninstallRemovesEmptyResidentRoot(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	report, err := resident.Uninstall(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(report.PreservedRoots, "efforts") {
		t.Fatal("empty efforts root reported preserved")
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "efforts")); !os.IsNotExist(err) {
		t.Fatalf("empty efforts root survived: %v", err)
	}
}

func TestUninstallRejectsUnsafeResidentRoot(t *testing.T) {
	for _, kind := range []string{"file", "symlink", "unreadable"} {
		t.Run(kind, func(t *testing.T) {
			root := scaffold(t, sampleYAML)
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			if err := p.Sync(); err != nil {
				t.Fatal(err)
			}
			efforts := filepath.Join(root, ".awf", "efforts")
			if err := os.RemoveAll(efforts); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "file":
				testsupport.WriteFile(t, efforts, "unsafe\n")
			case "symlink":
				outside := t.TempDir()
				if err := os.Symlink(outside, efforts); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			case "unreadable":
				if err := os.Mkdir(efforts, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(efforts, 0); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(efforts, 0o700) })
			}
			if _, err := resident.Uninstall(testContext(t), root); err == nil {
				t.Fatalf("unsafe %s efforts root accepted", kind)
			}
			if _, err := os.Stat(lockFile(root)); err != nil {
				t.Fatalf("lock mutated after refusal: %v", err)
			}
		})
	}
}

func TestBackupFileReturnsSourceInspectionError(t *testing.T) {
	p := &Project{Root: t.TempDir()}
	if _, err := p.BackupFile("missing"); !os.IsNotExist(err) {
		t.Fatalf("BackupFile missing source error = %v, want not exist", err)
	}
}

// invariant: rendering/sync-and-drift:sync-backs-up-foreign (TestBackupFilePropagatesNonCollisionPublicationError)
func TestBackupFilePropagatesNonCollisionPublicationError(t *testing.T) {
	root := t.TempDir()
	const source = "rescue source"
	if err := os.WriteFile(filepath.Join(root, "artifact"), []byte(source), 0o640); err != nil {
		t.Fatal(err)
	}
	publicationFailure := errors.New("publication storage failure")
	calls := 0
	p := &Project{Root: root}
	_, err := p.backupFile("artifact", func(path string, contents []byte, mode os.FileMode) error {
		calls++
		return publicationFailure
	})
	if !errors.Is(err, publicationFailure) {
		t.Fatalf("BackupFile error = %v, want non-collision publication failure", err)
	}
	if calls != 1 {
		t.Fatalf("publication calls = %d, want one without suffix retry", calls)
	}
	if _, err := os.Stat(filepath.Join(root, "artifact.awf-bak")); !os.IsNotExist(err) {
		t.Fatalf("backup created after publication failure: %v", err)
	}
}

func TestBackupFileRetriesOnlyPublicationCollision(t *testing.T) {
	root := t.TempDir()
	const source = "rescue source"
	if err := os.WriteFile(filepath.Join(root, "artifact"), []byte(source), 0o640); err != nil {
		t.Fatal(err)
	}
	p := &Project{Root: root}
	competed := false
	_, err := p.backupFile("artifact", func(path string, contents []byte, mode os.FileMode) error {
		if !competed {
			competed = true
			if err := os.WriteFile(path, []byte("concurrent winner"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		return filepublication.Publish(path, contents, mode)
	})
	if err != nil {
		t.Fatalf("BackupFile collision retry: %v", err)
	}
	winner, err := os.ReadFile(filepath.Join(root, "artifact.awf-bak"))
	if err != nil || string(winner) != "concurrent winner" {
		t.Fatalf("winner backup = %q, error = %v", winner, err)
	}
	backup, err := os.ReadFile(filepath.Join(root, "artifact.awf-bak.1"))
	if err != nil || string(backup) != source {
		t.Fatalf("retried backup = %q, error = %v", backup, err)
	}
}

func TestInitCollisionsSurfacesPlannedOutputsError(t *testing.T) {
	root := t.TempDir()
	awf := filepath.Join(root, ".awf")
	if err := os.MkdirAll(awf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(awf, "config.yaml"),
		[]byte("prefix: awf\nintegrationBranch: main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A malformed ADR makes generateIndexMD (inside PlannedOutputs) fail.
	dd := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(dd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dd, "0099-bad.md"), []byte("---\nstatus: [unclosed\n---\n# Bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.InitCollisions(testContext(t)); err == nil {
		t.Fatal("expected InitCollisions to surface the PlannedOutputs error")
	}
}
