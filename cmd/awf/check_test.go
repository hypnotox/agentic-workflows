package main

import (
	"os"
	"path/filepath"
	"testing"
)

const checkYAML = `prefix: example
vars: {testCmd: go test ./..., gateCmd: make gate}
skills:
  tdd:
    data:
      testSurfaces:
        - {name: Logic, location: internal, kind: Go unit}
agents: {}
hooks: []
`

func TestRunCheckCleanThenDirty(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude", "awf.yaml"), []byte(checkYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runSync(root); err != nil {
		t.Fatal(err)
	}
	if err := runCheck(root); err != nil {
		t.Errorf("expected clean check, got %v", err)
	}
	// Hand-edit the rendered skill.
	skill := filepath.Join(root, ".claude/skills/example-tdd/SKILL.md")
	if err := os.WriteFile(skill, []byte("tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runCheck(root); err == nil {
		t.Errorf("expected drift error after hand-edit")
	}
}
