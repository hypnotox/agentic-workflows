package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentic-workflows/internal/manifest"
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
agents:
  code-reviewer: {}
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

func TestCheckStaleTakesPrecedence(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(root)
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	skillPath := ".claude/skills/example-tdd/SKILL.md"
	// Make the lock entry stale by corrupting its TemplateHash.
	lock, err := manifest.Load(filepath.Join(root, ".claude", "awf.lock"))
	if err != nil {
		t.Fatal(err)
	}
	e := lock.Files[skillPath]
	e.TemplateHash = "sha256:bogus"
	lock.Files[skillPath] = e
	if err := lock.Save(filepath.Join(root, ".claude", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	// Also hand-edit the rendered file so its on-disk content differs too.
	if err := os.WriteFile(filepath.Join(root, skillPath), []byte("hand edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check()
	if err != nil {
		t.Fatal(err)
	}
	var forPath []manifest.Drift
	for _, d := range drift {
		if d.Path == skillPath {
			forPath = append(forPath, d)
		}
	}
	if len(forPath) != 1 {
		t.Fatalf("expected exactly one drift entry for %s, got %#v", skillPath, forPath)
	}
	if forPath[0].Kind != "stale" {
		t.Errorf("expected stale, got %q", forPath[0].Kind)
	}
}

func TestSyncSkipsLocalSkill(t *testing.T) {
	localYAML := strings.Replace(sampleYAML, `skills:
  tdd:`, `skills:
  adding-thing: {local: true}
  tdd:`, 1)
	root := scaffold(t, localYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	localPath := ".claude/skills/example-adding-thing/SKILL.md"
	if _, err := os.Stat(filepath.Join(root, localPath)); !os.IsNotExist(err) {
		t.Errorf("local skill should not be written to disk")
	}
	lock, err := manifest.Load(filepath.Join(root, ".claude", "awf.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := lock.Files[localPath]; ok {
		t.Errorf("local skill should be absent from lock")
	}
	if _, ok := lock.Files[".claude/skills/example-tdd/SKILL.md"]; !ok {
		t.Errorf("tdd skill should still be present in lock")
	}
}

func TestOpenRejectsUnknownAgent(t *testing.T) {
	yaml := `prefix: example
skills: {}
agents:
  does-not-exist: {}
hooks: []
`
	root := scaffold(t, yaml)
	_, err := Open(root)
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("error should mention the offending agent name, got: %v", err)
	}
}

func TestOpenRejectsUnknownHook(t *testing.T) {
	yaml := `prefix: example
skills: {}
agents: {}
hooks: [typo-hook]
`
	root := scaffold(t, yaml)
	_, err := Open(root)
	if err == nil {
		t.Fatal("expected error for unknown hook")
	}
	if !strings.Contains(err.Error(), "typo-hook") {
		t.Errorf("error should mention the offending hook name, got: %v", err)
	}
}

func TestOpenRejectsUnknownSkill(t *testing.T) {
	yaml := `prefix: example
skills:
  no-such-skill: {}
agents: {}
hooks: []
`
	root := scaffold(t, yaml)
	_, err := Open(root)
	if err == nil {
		t.Fatal("expected error for unknown skill")
	}
	if !strings.Contains(err.Error(), "no-such-skill") {
		t.Errorf("error should mention the offending skill name, got: %v", err)
	}
}

func TestOpenValidConfigSucceeds(t *testing.T) {
	root := scaffold(t, sampleYAML)
	_, err := Open(root)
	if err != nil {
		t.Fatalf("expected valid config to open cleanly, got: %v", err)
	}
}

func TestOpenAllowsLocalSkillNotInCatalog(t *testing.T) {
	localYAML := strings.Replace(sampleYAML, `skills:
  tdd:`, `skills:
  totally-unknown-local: {local: true}
  tdd:`, 1)
	root := scaffold(t, localYAML)
	_, err := Open(root)
	if err != nil {
		t.Fatalf("local skill not in catalog should be allowed, got: %v", err)
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

func TestOpenRejectsUnknownSectionOverride(t *testing.T) {
	// tdd in the catalog has sections [surfaces, notes]; "bogus" is not declared.
	yaml := `prefix: example
skills:
  tdd:
    sections:
      bogus: {drop: true}
agents:
  code-reviewer: {}
hooks: [pre-commit, pre-push]
`
	root := scaffold(t, yaml)
	_, err := Open(root)
	if err == nil {
		t.Fatal("expected error for unknown section override 'bogus'")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error should mention 'bogus', got: %v", err)
	}
}

func TestOpenAllowsValidSectionOverride(t *testing.T) {
	// "notes" is a declared section for tdd.
	yaml := `prefix: example
skills:
  tdd:
    sections:
      notes: {drop: true}
agents:
  code-reviewer: {}
hooks: [pre-commit, pre-push]
`
	root := scaffold(t, yaml)
	_, err := Open(root)
	if err != nil {
		t.Fatalf("valid section override 'notes' should succeed, got: %v", err)
	}
}

func TestOpenAllowsLocalAgentNotInCatalog(t *testing.T) {
	yaml := `prefix: example
skills: {}
agents:
  my-custom-agent: {local: true}
hooks: []
`
	root := scaffold(t, yaml)
	_, err := Open(root)
	if err != nil {
		t.Fatalf("local agent not in catalog should be allowed, got: %v", err)
	}
}

func TestOpenRejectsUnknownAgentSectionOverride(t *testing.T) {
	// code-reviewer in the catalog has sections universal-lenses/project-focus/doc-currency.
	yaml := `prefix: example
skills: {}
agents:
  code-reviewer:
    sections:
      bogus: {drop: true}
hooks: []
`
	root := scaffold(t, yaml)
	_, err := Open(root)
	if err == nil {
		t.Fatal("expected error for unknown agent section override 'bogus'")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error should mention 'bogus', got: %v", err)
	}
}

func TestSyncRendersAgentFromMap(t *testing.T) {
	agentYAML := `prefix: myproject
agents:
  code-reviewer: {}
hooks: []
skills: {}
`
	root := scaffold(t, agentYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	agentPath := filepath.Join(root, ".claude/agents/code-reviewer.md")
	b, err := os.ReadFile(agentPath)
	if err != nil {
		t.Fatalf("agent file not written: %v", err)
	}
	if !strings.Contains(string(b), "myproject") {
		t.Errorf("agent file should contain prefix 'myproject', got:\n%s", b)
	}
}
