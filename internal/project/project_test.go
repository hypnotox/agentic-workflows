package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentic-workflows/internal/config"
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

func TestOpenRejectsUnknownDoc(t *testing.T) {
	yaml := `prefix: example
skills: {}
agents: {}
hooks: []
docs:
  nonexistent: {}
`
	root := scaffold(t, yaml)
	_, err := Open(root)
	if err == nil {
		t.Fatal("expected error for unknown doc")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention the offending doc name, got: %v", err)
	}
}

func TestSyncRendersDeclaredDoc(t *testing.T) {
	yaml := `prefix: example
skills: {}
agents: {}
hooks: []
docs:
  architecture: {}
`
	root := scaffold(t, yaml)
	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, "docs", "architecture.md"))
	if err != nil {
		t.Fatalf("docs/architecture.md not written: %v", err)
	}
	if !strings.Contains(string(b), "# Architecture") {
		t.Errorf("docs/architecture.md missing heading:\n%s", b)
	}
}

// TestSyncAutoLinksDocsInAgentsDoc covers the project-level wiring that the
// template golden cannot: RenderAll injects resolvedDocs() into the agents-doc
// data map so the Document map auto-links every declared (non-local) doc with
// its catalog title/desc. A local doc must not appear.
func TestSyncAutoLinksDocsInAgentsDoc(t *testing.T) {
	yaml := `prefix: example
vars:
  gateCmd: ""
skills: {}
agents: {}
hooks: []
docs:
  architecture: {}
  glossary: {local: true}
agentsDoc: {}
`
	root := scaffold(t, yaml)
	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("AGENTS.md not written: %v", err)
	}
	got := string(b)
	if !strings.Contains(got, "[docs/architecture.md](docs/architecture.md)") {
		t.Errorf("Document map should auto-link the declared architecture doc:\n%s", got)
	}
	if !strings.Contains(got, "system shape, packages, key components, dependencies") {
		t.Errorf("Document map should carry the catalog desc for architecture:\n%s", got)
	}
	if strings.Contains(got, "docs/glossary.md") {
		t.Errorf("local doc must not appear in the Document map:\n%s", got)
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

// TestSyncErrorsOnMissingVar verifies that Sync returns an error when a
// rendered template would contain the literal string "<no value>" due to a
// missing var.  The tdd skill references {{ .vars.testCmd }} and
// {{ .vars.gateCmd }} without guards, so omitting them from vars triggers the
// check (Change R3).
func TestSyncErrorsOnMissingVar(t *testing.T) {
	missingVarsYAML := `prefix: example
vars: {}
skills:
  tdd:
    data:
      testSurfaces:
        - {name: Logic, location: internal, kind: Go unit}
agents: {}
hooks: []
`
	root := scaffold(t, missingVarsYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	err = p.Sync()
	if err == nil {
		t.Fatal("expected Sync to return an error when vars are missing, got nil")
	}
	if !strings.Contains(err.Error(), "<no value>") {
		t.Errorf("error should mention \"<no value>\", got: %v", err)
	}
}

func TestSyncRendersAgentsDoc(t *testing.T) {
	t.Run("with agentsDoc config", func(t *testing.T) {
		yaml := `prefix: example
vars:
  testCmd: go test ./...
  gateCmd: make gate
skills: {}
agents: {}
hooks: []
agentsDoc: {}
`
		root := scaffold(t, yaml)
		p, err := Open(root)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if err := p.Sync(); err != nil {
			t.Fatalf("Sync: %v", err)
		}
		agentsPath := filepath.Join(root, "AGENTS.md")
		b, err := os.ReadFile(agentsPath)
		if err != nil {
			t.Fatalf("AGENTS.md not written: %v", err)
		}
		if !strings.Contains(string(b), "example") {
			t.Errorf("AGENTS.md should contain prefix 'example', got:\n%s", b)
		}
	})

	t.Run("without agentsDoc config", func(t *testing.T) {
		yaml := `prefix: example
vars:
  testCmd: go test ./...
  gateCmd: make gate
skills: {}
agents: {}
hooks: []
`
		root := scaffold(t, yaml)
		p, err := Open(root)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if err := p.Sync(); err != nil {
			t.Fatalf("Sync: %v", err)
		}
		agentsPath := filepath.Join(root, "AGENTS.md")
		if _, err := os.Stat(agentsPath); !os.IsNotExist(err) {
			t.Errorf("AGENTS.md should not exist when agentsDoc is not configured")
		}
	})
}

// TestSyncPrunesEmptySkillDir verifies that after a skill is removed from the
// config and Sync is run again, not only is the SKILL.md file removed, but the
// now-empty parent directory .claude/skills/<prefix>-<skill>/ is also removed
// (Change R5).
func TestSyncPrunesEmptySkillDir(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("first Sync: %v", err)
	}
	skillDir := filepath.Join(root, ".claude/skills/example-tdd")
	if _, err := os.Stat(skillDir); err != nil {
		t.Fatalf("skill dir should exist after first sync: %v", err)
	}

	// Rewrite config without the tdd skill, re-open, re-sync.
	noTDD := strings.Replace(sampleYAML, `skills:
  tdd:
    data:
      testSurfaces:
        - {name: Logic, location: internal, kind: Go unit}`, "skills: {}", 1)
	if err := os.WriteFile(filepath.Join(root, ".claude", "awf.yaml"), []byte(noTDD), 0o644); err != nil {
		t.Fatalf("rewrite yaml: %v", err)
	}
	p2, err := Open(root)
	if err != nil {
		t.Fatalf("Open after removing tdd skill: %v", err)
	}
	if err := p2.Sync(); err != nil {
		t.Fatalf("second Sync: %v", err)
	}

	// The SKILL.md file should be gone.
	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); !os.IsNotExist(err) {
		t.Errorf("SKILL.md should have been pruned")
	}
	// The parent directory should also be gone (R5: prune empty skill dirs).
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Errorf("skill directory %s should have been pruned when empty", skillDir)
	}
}

func TestLayoutDerivesFromDocsDir(t *testing.T) {
	p := &Project{Cfg: &config.Config{DocsDir: "documentation"}}
	l := p.layout()
	want := map[string]string{
		"docsDir":     "documentation",
		"adrDir":      "documentation/decisions",
		"activeMd":    "documentation/decisions/ACTIVE.md",
		"adrReadme":   "documentation/decisions/README.md",
		"adrTemplate": "documentation/decisions/template.md",
		"plansDir":    "documentation/plans",
	}
	for k, v := range want {
		if got := l[k]; got != v {
			t.Errorf("layout[%q] = %v, want %q", k, got, v)
		}
	}
	if got := p.docOutPath("architecture"); got != "documentation/architecture.md" {
		t.Errorf("docOutPath = %q, want documentation/architecture.md", got)
	}
}

func TestSyncGeneratesActiveMDAndCheckDetectsStaleness(t *testing.T) {
	yaml := `prefix: example
skills: {}
agents: {}
hooks: []
`
	root := scaffold(t, yaml)
	adrDir := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	adr := "---\nstatus: Accepted\ndate: 2026-06-25\ntags: [x]\n---\n# ADR-0001: First\n## Context\nx\n"
	if err := os.WriteFile(filepath.Join(adrDir, "0001-first.md"), []byte(adr), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	active, err := os.ReadFile(filepath.Join(adrDir, "ACTIVE.md"))
	if err != nil {
		t.Fatalf("ACTIVE.md not generated: %v", err)
	}
	if !strings.Contains(string(active), "ADR-0001: First") {
		t.Errorf("ACTIVE.md missing the ADR entry:\n%s", active)
	}
	if drift, err := p.Check(); err != nil || len(drift) != 0 {
		t.Fatalf("expected clean check after sync, got drift=%#v err=%v", drift, err)
	}

	// Change frontmatter status; the on-disk ACTIVE.md is now stale.
	adr2 := strings.Replace(adr, "status: Accepted", "status: Implemented", 1)
	if err := os.WriteFile(filepath.Join(adrDir, "0001-first.md"), []byte(adr2), 0o644); err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	found := false
	for _, d := range drift {
		if strings.HasSuffix(d.Path, "decisions/ACTIVE.md") && d.Kind == "stale" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected stale drift for ACTIVE.md, got %#v", drift)
	}
}

func TestSyncProducesNoActiveMDWithoutADRs(t *testing.T) {
	yaml := `prefix: example
skills: {}
agents: {}
hooks: []
`
	root := scaffold(t, yaml)
	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "decisions", "ACTIVE.md")); !os.IsNotExist(err) {
		t.Errorf("expected no ACTIVE.md when no ADRs exist, stat err=%v", err)
	}
	if drift, err := p.Check(); err != nil || len(drift) != 0 {
		t.Errorf("expected clean check with no ADRs, got drift=%#v err=%v", drift, err)
	}
}
