package project

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

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
	obsolete := []string{
		".awf/efforts/obsolete",
		".awf/worktrees/obsolete",
		".awf/effort-archive/id-slug/nested/obsolete",
	}
	for _, path := range obsolete {
		lock.Files[path] = manifest.Entry{}
	}
	archivePath := filepath.Join(root, filepath.FromSlash(obsolete[2]))
	const archiveBytes = "archive sentinel\n"
	testsupport.WriteFile(t, archivePath, archiveBytes)
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.SyncReport(testContext(t)); err != nil {
		t.Fatalf("resident-root prune path failed: %v", err)
	}
	lock, err = manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range obsolete {
		if _, ok := lock.Files[path]; ok {
			t.Errorf("resident descendant remained in lock: %s", path)
		}
	}
	if got, err := os.ReadFile(archivePath); err != nil || string(got) != archiveBytes {
		t.Fatalf("archive descendant after lock pruning = %q, %v; want byte-identical", got, err)
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
	dynamic := map[string]string{
		"efforts":        filepath.Join(root, ".awf", "efforts", "e", "sessions", "s.jsonl"),
		"worktrees":      filepath.Join(root, ".awf", "worktrees", "w", "nested", "state"),
		"effort-archive": filepath.Join(root, ".awf", "effort-archive", "id-e", "nested", "adversarial.go"),
	}
	for name, path := range dynamic {
		testsupport.WriteFile(t, path, name+"\n")
	}
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		".awf/efforts/e/sessions/s.jsonl",
		".awf/worktrees/w/nested/state",
		".awf/effort-archive/id-e/nested/adversarial.go",
	} {
		lock.Files[path] = manifest.Entry{}
	}
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	report, err := resident.Uninstall(testContext(t), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, path := range dynamic {
		if !slices.Contains(report.PreservedRoots, name) {
			t.Errorf("resident %s was not reported preserved", name)
		}
		if got, err := os.ReadFile(path); err != nil || string(got) != name+"\n" {
			t.Errorf("preserved path %s = %q, %v", path, got, err)
		}
		if _, err := os.Lstat(filepath.Join(root, ".awf", name, ".gitignore")); err != nil {
			t.Errorf("preserved marker for %s: %v", name, err)
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
	report, err := resident.Uninstall(testContext(t), root, nil)
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
			if _, err := resident.Uninstall(testContext(t), root, nil); err == nil {
				t.Fatalf("unsafe %s efforts root accepted", kind)
			}
			if _, err := os.Stat(lockFile(root)); err != nil {
				t.Fatalf("lock mutated after refusal: %v", err)
			}
		})
	}
}

func TestBackupFileConfinedReturnsSourceInspectionError(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	filesystems, closeAll, err := p.openSyncFilesystems()
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	if _, err := p.backupFileConfined("missing", filesystems.tracked); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("backup missing source error = %v, want not exist", err)
	}
	if err := os.Mkdir(filepath.Join(root, "directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := p.backupFileConfined("directory", filesystems.tracked); err == nil {
		t.Fatal("backup accepted directory source")
	}
}

func TestSyncReportPropagatesForeignBackupFailure(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.InitializeReport(testContext(t), InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatal(err)
	}
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	delete(lock.Files, "AGENTS.md")
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(root, "AGENTS.md")
	if err := os.Remove(foreign); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(foreign, 0o755); err != nil {
		t.Fatal(err)
	}
	_, _, _, err = p.SyncReport(testContext(t))
	if err == nil || !strings.Contains(err.Error(), "back up AGENTS.md") || !strings.Contains(err.Error(), "read backup source") {
		t.Fatalf("SyncReport foreign backup error = %v", err)
	}
	var pathError *os.PathError
	if !errors.As(err, &pathError) {
		t.Fatalf("SyncReport foreign backup error identity = %T, want *os.PathError", err)
	}
	if info, statErr := os.Stat(foreign); statErr != nil || !info.IsDir() {
		t.Fatalf("foreign source changed after backup refusal: info=%v error=%v", info, statErr)
	}
}

type collisionFilesystem struct {
	syncFilesystem
	root     string
	competed bool
}

func (f *collisionFilesystem) Publish(path string, contents []byte, mode os.FileMode) error {
	if !f.competed {
		f.competed = true
		if err := os.WriteFile(filepath.Join(f.root, path), []byte("concurrent winner"), 0o600); err != nil {
			return err
		}
	}
	return f.syncFilesystem.Publish(path, contents, mode)
}

// invariant: rendering/sync-and-drift:sync-backs-up-foreign (TestBackupFileRetriesOnlyPublicationCollision)
func TestBackupFileRetriesOnlyPublicationCollision(t *testing.T) {
	root := scaffold(t, sampleYAML)
	const source = "rescue source"
	sourcePath := filepath.Join(root, "artifact")
	if err := os.WriteFile(sourcePath, []byte(source), 0o640); err != nil {
		t.Fatal(err)
	}
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	filesystems, closeAll, err := p.openSyncFilesystems()
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	if _, err := p.backupFileConfined("artifact", &collisionFilesystem{syncFilesystem: filesystems.tracked, root: root}); err != nil {
		t.Fatalf("backup collision retry: %v", err)
	}
	winner, err := os.ReadFile(filepath.Join(root, "artifact.awf-bak"))
	if err != nil || string(winner) != "concurrent winner" {
		t.Fatalf("winner backup = %q, error = %v", winner, err)
	}
	retriedPath := filepath.Join(root, "artifact.awf-bak.1")
	backup, err := os.ReadFile(retriedPath)
	if err != nil || string(backup) != source {
		t.Fatalf("retried backup = %q, error = %v", backup, err)
	}
	backupInfo, err := os.Stat(retriedPath)
	if err != nil {
		t.Fatal(err)
	}
	if backupInfo.Mode().Perm() != sourceInfo.Mode().Perm() {
		t.Fatalf("retried backup mode = %v, want source mode %v", backupInfo.Mode().Perm(), sourceInfo.Mode().Perm())
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
