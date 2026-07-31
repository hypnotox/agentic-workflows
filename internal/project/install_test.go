package project

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// A lock entry escaping the repo root (corrupted or malicious lock) must be
// skipped: the out-of-tree target survives and the empty-dir ancestor walk
// terminates instead of looping forever below the root.
func TestUninstallSkipsEscapingLockPaths(t *testing.T) {
	root := t.TempDir()
	victim := filepath.Join(root, "..", "victim.txt")
	testsupport.WriteFile(t, victim, "keep me\n")
	const inTree = ".claude/skills/x/SKILL.md"
	testsupport.WriteFile(t, filepath.Join(root, inTree), "x\n")
	lock := &manifest.Lock{Files: map[string]manifest.Entry{
		"../victim.txt": {},
		inTree:          {},
	}}
	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	report, err := Uninstall(testContext(t), root)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if report.Removed != 1 {
		t.Errorf("removed = %d, want 1 (the in-tree file only)", report.Removed)
	}
	if _, err := os.Stat(victim); err != nil {
		t.Errorf("escaping lock entry deleted the out-of-tree file: %v", err)
	}
	// invariant: rendering/sync-and-drift:uninstall-removes-lock-entries
	if _, err := os.Stat(filepath.Join(root, inTree)); !os.IsNotExist(err) {
		t.Errorf("in-tree lock entry not removed (err = %v)", err)
	}
}

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
	resident := filepath.Join(root, ".awf", "efforts", "efforts", "e", "sessions", "s.jsonl")
	testsupport.WriteFile(t, resident, "resident\n")
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	lock.Files[".awf/efforts/efforts/e/sessions/s.jsonl"] = manifest.Entry{}
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	report, err := Uninstall(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(report.PreservedRoots, "efforts") {
		t.Fatal("resident efforts were not reported preserved")
	}
	for _, path := range []string{resident, filepath.Join(root, ".awf", "efforts", ".gitignore")} {
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
	report, err := Uninstall(testContext(t), root)
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
			if _, err := Uninstall(testContext(t), root); err == nil {
				t.Fatalf("unsafe %s efforts root accepted", kind)
			}
			if _, err := os.Stat(lockFile(root)); err != nil {
				t.Fatalf("lock mutated after refusal: %v", err)
			}
		})
	}
}

func TestInspectResidentRootsTreatsAnyDirectChildAsData(t *testing.T) {
	root := t.TempDir()
	efforts := filepath.Join(root, ".awf", "efforts")
	if err := os.MkdirAll(efforts, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(efforts, "unreadable-entry")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	preserved, err := inspectResidentRoots(root)
	if err != nil || !slices.Contains(preserved, "efforts") {
		t.Fatalf("preserved=%v err=%v", preserved, err)
	}
}

func TestInitCollisionsSurfacesPlannedOutputsError(t *testing.T) {
	root := t.TempDir()
	awf := filepath.Join(root, ".awf")
	if err := os.MkdirAll(awf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(awf, "config.yaml"),
		[]byte("prefix: awf\nintegrationBranch: main\nskills: []\nagents: []\ndocs: []\n"), 0o644); err != nil {
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
