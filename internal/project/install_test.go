package project

import (
	"os"
	"path/filepath"
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
	removed, err := Uninstall(root)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1 (the in-tree file only)", removed)
	}
	if _, err := os.Stat(victim); err != nil {
		t.Errorf("escaping lock entry deleted the out-of-tree file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, inTree)); !os.IsNotExist(err) {
		t.Errorf("in-tree lock entry not removed (err = %v)", err)
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
	// A malformed ADR makes generateActiveMD (inside PlannedOutputs) fail.
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
