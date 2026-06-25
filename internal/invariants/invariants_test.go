package invariants_test

import (
	"os"
	"path/filepath"
	"testing"

	"agentic-workflows/internal/invariants"
)

func writeADR(t *testing.T, dir, name, status, invBody string) {
	t.Helper()
	content := "---\nstatus: " + status + "\ndate: 2026-06-25\ntags: [x]\n---\n# ADR-X: T\n## Invariants\n" + invBody + "\n## Consequences\nc\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// invariant: invariants-implemented-only
func TestCheckImplementedOnly(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	// Implemented ADR with an unbacked slug -> required.
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-impl` — x.")
	// Proposed/Accepted/Superseded ADRs -> not required, even though unbacked.
	writeADR(t, dir, "0002-b.md", "Proposed", "- `inv: fixture-prop` — x.")
	writeADR(t, dir, "0003-c.md", "Accepted", "- `inv: fixture-acc` — x.")
	writeADR(t, dir, "0004-d.md", "Superseded by ADR-0001", "- `inv: fixture-sup` — x.")
	findings, err := invariants.Check(dir, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Slug != "fixture-impl" {
		t.Errorf("expected only fixture-impl required, got %#v", findings)
	}
}

// invariant: invariants-unbacked-detected
func TestCheckUnbackedAndBacked(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-backed` — x.\n- `inv: fixture-missing` — y.")
	// Back only one slug, in a .go file under root.
	src := "package x\n// invariant: fixture-backed\nfunc T() {}\n"
	if err := os.WriteFile(filepath.Join(root, "x.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err := invariants.Check(dir, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Slug != "fixture-missing" {
		t.Errorf("expected only fixture-missing unbacked, got %#v", findings)
	}
}

// invariant: invariants-duplicate-slug
func TestCheckDuplicateSlug(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-dup` — x.")
	writeADR(t, dir, "0002-b.md", "Implemented", "- `inv: fixture-dup` — y.")
	if _, err := invariants.Check(dir, root); err == nil {
		t.Error("expected error for duplicate slug")
	}
}
