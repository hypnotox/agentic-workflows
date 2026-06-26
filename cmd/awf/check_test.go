package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const checkYAML = `prefix: example
vars: {testCmd: go test ./..., gateCmd: make gate}
skills: [tdd]
agents: []
hooks: []
`

func TestRunCheckCleanThenDirty(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte(checkYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runSync(root, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := runCheck(root, io.Discard); err != nil {
		t.Errorf("expected clean check, got %v", err)
	}
	// Hand-edit the rendered skill.
	skill := filepath.Join(root, ".claude/skills/example-tdd/SKILL.md")
	if err := os.WriteFile(skill, []byte("tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runCheck(root, io.Discard); err == nil {
		t.Errorf("expected drift error after hand-edit")
	}
}

// TestRunCheckSurfacesInvariantError covers the CheckInvariants error path in
// runCheck: a clean project (p.Check passes) whose decisions hold two Implemented
// ADRs declaring the same `inv:` slug. p.Check does not detect the collision —
// only invariants.Check does — so its error must propagate out of runCheck.
func TestRunCheckSurfacesInvariantError(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte(checkYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runSync(root, io.Discard); err != nil {
		t.Fatal(err)
	}
	dec := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(dec, 0o755); err != nil {
		t.Fatal(err)
	}
	adr := func(n, title string) string {
		return "---\nstatus: Implemented\ndate: 2026-06-25\ntags: [x]\n---\n# ADR-" + n + ": " + title +
			"\n## Invariants\n- `inv: dup-slug`\n## Context\nx\n"
	}
	if err := os.WriteFile(filepath.Join(dec, "0001-a.md"), []byte(adr("0001", "A")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dec, "0002-b.md"), []byte(adr("0002", "B")), 0o644); err != nil {
		t.Fatal(err)
	}
	// Re-sync so the generated ACTIVE.md is current and p.Check stays clean.
	if err := runSync(root, io.Discard); err != nil {
		t.Fatal(err)
	}
	err := runCheck(root, io.Discard)
	if err == nil {
		t.Fatal("expected runCheck to surface the duplicate-slug invariant error")
	}
	if !strings.Contains(err.Error(), "duplicate inv slug") {
		t.Errorf("expected a duplicate-slug error, got: %v", err)
	}
}
