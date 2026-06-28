package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitCollisionsSurfacesPlannedOutputsError(t *testing.T) {
	root := t.TempDir()
	awf := filepath.Join(root, ".awf")
	if err := os.MkdirAll(awf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(awf, "config.yaml"),
		[]byte("prefix: awf\nskills: []\nagents: []\nhooks: []\ndocs: []\n"), 0o644); err != nil {
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
