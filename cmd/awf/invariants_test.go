package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// invariantsNoteYAML configures a top-level `*.go` source scan with the `//` marker.
const invariantsNoteYAML = "prefix: example\ninvariants:\n  sources:\n    - globs: [\"*.go\"]\n      marker: \"//\"\nskills: []\nagents: []\n"

// backedPlusDangling scaffolds a synced project whose sole Implemented ADR
// declares a backed slug (backed by a root .go marker) plus a dangling proof
// marker naming an undeclared slug - the advisory-note fixture shared by the
// runInvariants and runCheck note-channel tests.
func backedPlusDangling(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, invariantsNoteYAML)
	if err := runSync(root, io.Discard); err != nil {
		t.Fatalf("runSync: %v", err)
	}
	adrDir := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	adr := testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
		testsupport.WithTitle("0001: X"), testsupport.WithBody("## Decision\n\n1. x.\n## Invariants\n- `invariant: real-one` - x.\n## Consequences\nc\n"))
	testsupport.WriteFile(t, filepath.Join(adrDir, "0001-x.md"), adr)
	// real-one backed; ghost is a dangling proof marker → advisory note only.
	src := "package x\n// invariant: real-one\n// invariant: ghost\n"
	if err := os.WriteFile(filepath.Join(root, "backing.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	// Re-sync so ACTIVE.md is current and the tree stays drift-clean.
	if err := runSync(root, io.Discard); err != nil {
		t.Fatalf("runSync: %v", err)
	}
	return root
}

// TestRunInvariantsPrintsAdvisoryNotes covers the note: channel in runInvariants:
// a dangling proof marker prints a note while findings stay clean.
func TestRunInvariantsPrintsAdvisoryNotes(t *testing.T) {
	var buf bytes.Buffer
	if err := runInvariants(backedPlusDangling(t), &buf); err != nil {
		t.Fatalf("runInvariants must stay clean (notes never fail): %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "note: ") || !strings.Contains(out, "ghost") {
		t.Errorf("expected a dangling-marker note for ghost, got: %q", out)
	}
	if !strings.Contains(out, "awf invariants: clean") {
		t.Errorf("expected clean status alongside the note, got: %q", out)
	}
}

// TestRunCheckPrintsInvariantNotes covers the invariant note: channel in runCheck:
// a dangling proof marker prints a note without failing the drift/finding count.
func TestRunCheckPrintsInvariantNotes(t *testing.T) {
	var buf bytes.Buffer
	if err := runCheck(backedPlusDangling(t), &buf); err != nil {
		t.Fatalf("runCheck must stay clean (invariant notes never fail): %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "note: ") || !strings.Contains(out, "ghost") {
		t.Errorf("expected a dangling-marker note for ghost, got: %q", out)
	}
	if !strings.Contains(out, "awf check: clean") {
		t.Errorf("expected clean status alongside the note, got: %q", out)
	}
}

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
	adr := testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
		testsupport.WithTitle("0001: X"), testsupport.WithBody("## Decision\n\n1. x.\n## Invariants\n- `invariant: cmd-needs-backing` - x.\n## Consequences\nc\n"))
	testsupport.WriteFile(t, filepath.Join(adrDir, "0001-x.md"), adr)
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
