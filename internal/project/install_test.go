package project

import (
	"os"
	"path/filepath"
	"strings"
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
	report, err := Uninstall(root)
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

func TestUninstallPreservesResidentWorkflowMetrics(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	resident := filepath.Join(root, ".awf", "metrics", "efforts", "e", "sessions", "s.jsonl")
	testsupport.WriteFile(t, resident, "resident\n")
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	lock.Files[".awf/metrics/efforts/e/sessions/s.jsonl"] = manifest.Entry{}
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	report, err := Uninstall(root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.MetricsPreserved {
		t.Fatal("resident metrics were not reported preserved")
	}
	for _, path := range []string{resident, filepath.Join(root, ".awf", "metrics", ".gitignore")} {
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("preserved path %s: %v", path, err)
		}
	}
	if _, err := os.Stat(lockFile(root)); !os.IsNotExist(err) {
		t.Fatalf("lock survived uninstall: %v", err)
	}
}

func TestUninstallRemovesEmptyMetricsRoot(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	report, err := Uninstall(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.MetricsPreserved {
		t.Fatal("empty metrics root reported preserved")
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "metrics")); !os.IsNotExist(err) {
		t.Fatalf("empty metrics root survived: %v", err)
	}
}

func TestUninstallRejectsUnsafeMetricsRoot(t *testing.T) {
	for _, kind := range []string{"file", "symlink", "unreadable"} {
		t.Run(kind, func(t *testing.T) {
			root := scaffold(t, sampleYAML)
			p, err := Open(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := p.Sync(); err != nil {
				t.Fatal(err)
			}
			metrics := filepath.Join(root, ".awf", "metrics")
			if err := os.RemoveAll(metrics); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "file":
				testsupport.WriteFile(t, metrics, "unsafe\n")
			case "symlink":
				outside := t.TempDir()
				if err := os.Symlink(outside, metrics); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			case "unreadable":
				if err := os.Mkdir(metrics, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(metrics, 0); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(metrics, 0o700) })
			}
			if _, err := Uninstall(root); err == nil {
				t.Fatalf("unsafe %s metrics root accepted", kind)
			}
			if _, err := os.Stat(lockFile(root)); err != nil {
				t.Fatalf("lock mutated after refusal: %v", err)
			}
		})
	}
}

func TestInspectResidentMetricsReportsLstatFailure(t *testing.T) {
	original := lstatResidentMetrics
	t.Cleanup(func() { lstatResidentMetrics = original })
	lstatResidentMetrics = func(string) (os.FileInfo, error) { return nil, os.ErrPermission }
	if _, err := inspectResidentMetrics(t.TempDir()); err == nil || !strings.Contains(err.Error(), "inspect resident workflow metrics") {
		t.Fatalf("inspectResidentMetrics error = %v", err)
	}
}

func TestInspectResidentMetricsRejectsUnsafeGovernedIgnore(t *testing.T) {
	root := t.TempDir()
	metrics := filepath.Join(root, ".awf", "metrics")
	if err := os.MkdirAll(metrics, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "missing"), filepath.Join(metrics, ".gitignore")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := inspectResidentMetrics(root); err == nil {
		t.Fatal("unsafe governed metrics ignore accepted")
	}
}

func TestInspectResidentMetricsTreatsAnyDirectChildAsData(t *testing.T) {
	root := t.TempDir()
	metrics := filepath.Join(root, ".awf", "metrics")
	if err := os.MkdirAll(metrics, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(metrics, "unreadable-entry")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	resident, err := inspectResidentMetrics(root)
	if err != nil || !resident {
		t.Fatalf("resident=%v err=%v", resident, err)
	}
}

func TestInitCollisionsSurfacesPlannedOutputsError(t *testing.T) {
	root := t.TempDir()
	awf := filepath.Join(root, ".awf")
	if err := os.MkdirAll(awf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(awf, "config.yaml"),
		[]byte("prefix: awf\nskills: []\nagents: []\ndocs: []\n"), 0o644); err != nil {
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
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.InitCollisions(); err == nil {
		t.Fatal("expected InitCollisions to surface the PlannedOutputs error")
	}
}
