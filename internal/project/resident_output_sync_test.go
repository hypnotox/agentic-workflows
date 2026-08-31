package project

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestSyncNeverPrunesResidentEffortsDescendants(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	const rel = ".awf/efforts/efforts/e/sessions/s.jsonl"
	path := filepath.Join(root, filepath.FromSlash(rel))
	testsupport.WriteFile(t, path, "resident\n")
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	lock.Files[rel] = manifest.Entry{}
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	p, err = loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	_, _, pruned, err := syncReportProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(pruned, rel) {
		t.Fatalf("resident path reported pruned: %v", pruned)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("resident path removed: %v", err)
	}
}

func TestSyncRejectsUnsafeResidentEffortsRoot(t *testing.T) {
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
	if err := os.Symlink(t.TempDir(), efforts); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, _, _, err := syncReportProject(p); err == nil {
		t.Fatal("sync accepted an unsafe resident efforts root")
	}
}
