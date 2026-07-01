package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// invariant: invariants-in-check
func TestRunCheckFailsOnUnbackedInvariant(t *testing.T) {
	root := t.TempDir()
	yaml := "prefix: example\ninvariants:\n  sources:\n    - globs: [\"*.go\"]\n      marker: \"//\"\nskills: []\nagents: []\n"
	testsupport.WriteAwfConfig(t, root, yaml)
	if err := runSync(root, io.Discard); err != nil {
		t.Fatalf("runSync: %v", err)
	}
	adrDir := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	adr := "---\nstatus: Implemented\ndate: 2026-06-25\ntags: [x]\n---\n# ADR-0001: X\n## Invariants\n- `inv: cmd-needs-backing` — x.\n## Consequences\nc\n"
	if err := os.WriteFile(filepath.Join(adrDir, "0001-x.md"), []byte(adr), 0o644); err != nil {
		t.Fatal(err)
	}
	// Re-sync so ACTIVE.md is generated and the tree stays drift-clean; the only
	// outstanding issue is the unbacked invariant.
	if err := runSync(root, io.Discard); err != nil {
		t.Fatalf("runSync: %v", err)
	}
	if err := runCheck(root, io.Discard); err == nil {
		t.Error("expected runCheck to fail on unbacked invariant")
	}
	// Back the slug with a .go file under root -> runCheck clean.
	src := "package x\n// invariant: cmd-needs-backing\nfunc T() {}\n"
	if err := os.WriteFile(filepath.Join(root, "backing.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runCheck(root, io.Discard); err != nil {
		t.Errorf("expected runCheck clean after backing, got: %v", err)
	}
}
