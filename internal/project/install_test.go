package project

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func uninstallProject(t *testing.T, root string) (resident.UninstallReport, error) {
	t.Helper()
	lease, err := filesystem.AcquireProjectLease(testContext(t), root, awfgit.ProjectResidentRoot(testContext(t), root))
	if err != nil {
		t.Fatal(err)
	}
	report, uninstallErr := resident.UninstallLeased(testContext(t), root, nil, lease)
	if err := lease.Release(); err != nil {
		t.Fatalf("release uninstall lease: %v", err)
	}
	return report, uninstallErr
}

func TestSyncPrunesResidentLockEntryFromResidentRoot(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
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
	if _, _, err := syncReportProject(p); err != nil {
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
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
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
	report, err := uninstallProject(t, root)
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
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	report, err := uninstallProject(t, root)
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
			p, err := loadTestSession(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			if err := syncProject(p); err != nil {
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
			if _, err := uninstallProject(t, root); err == nil {
				t.Fatalf("unsafe %s efforts root accepted", kind)
			}
			if _, err := os.Stat(lockFile(root)); err != nil {
				t.Fatalf("lock mutated after refusal: %v", err)
			}
		})
	}
}

func TestSyncReportRefusesForeignDirectoryWithoutMutation(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := initializeReportProject(p, publisher.InitAuthority{InitializedWithVersion: Version}); err != nil {
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
	_, _, err = syncReportProject(p)
	if err == nil || !strings.Contains(err.Error(), "AGENTS.md") {
		t.Fatalf("SyncReport foreign collision error = %v", err)
	}
	if info, statErr := os.Stat(foreign); statErr != nil || !info.IsDir() {
		t.Fatalf("foreign source changed after collision refusal: info=%v error=%v", info, statErr)
	}
	if _, statErr := os.Stat(foreign + ".awf-bak"); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("foreign collision backup = %v", statErr)
	}
}
