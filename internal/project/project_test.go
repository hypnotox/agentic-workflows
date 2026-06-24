// internal/project/project_test.go
package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func scaffold(t *testing.T, awfYAML string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude", "awf.yaml"), []byte(awfYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

const sampleYAML = `prefix: example
vars:
  testCmd: go test ./...
  gateCmd: make gate
skills:
  tdd:
    data:
      testSurfaces:
        - {name: Logic, location: internal, kind: Go unit}
agents: [code-reviewer]
hooks: [pre-commit, pre-push]
`

func TestSyncWritesFilesAndLock(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(root, ".claude/skills/example-tdd/SKILL.md")
	b, err := os.ReadFile(skill)
	if err != nil {
		t.Fatalf("skill not written: %v", err)
	}
	if !strings.Contains(string(b), "# example-tdd") || strings.Contains(string(b), "awf:section") {
		t.Errorf("rendered skill wrong:\n%s", b)
	}
	for _, rel := range []string{".claude/agents/code-reviewer.md", ".githooks/pre-commit", ".githooks/pre-push", ".claude/awf.lock"} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("missing %s: %v", rel, err)
		}
	}
}

func TestCheckCleanAfterSync(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(root)
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check()
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) != 0 {
		t.Errorf("expected clean, got drift: %#v", drift)
	}
}

func TestCheckDetectsHandEdit(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(root)
	_ = p.Sync()
	skill := filepath.Join(root, ".claude/skills/example-tdd/SKILL.md")
	_ = os.WriteFile(skill, []byte("hand edited\n"), 0o644)
	drift, _ := p.Check()
	if len(drift) == 0 || drift[0].Kind != "hand-edited" {
		t.Errorf("expected hand-edited drift, got %#v", drift)
	}
}

func TestSyncPrunesRemovedSkill(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(root)
	_ = p.Sync()
	// Rewrite config without the tdd skill, re-open, re-sync.
	noTDD := strings.Replace(sampleYAML, `skills:
  tdd:
    data:
      testSurfaces:
        - {name: Logic, location: internal, kind: Go unit}`, "skills: {}", 1)
	_ = os.WriteFile(filepath.Join(root, ".claude", "awf.yaml"), []byte(noTDD), 0o644)
	p2, _ := Open(root)
	if err := p2.Sync(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/skills/example-tdd/SKILL.md")); !os.IsNotExist(err) {
		t.Errorf("removed skill should be pruned")
	}
}
