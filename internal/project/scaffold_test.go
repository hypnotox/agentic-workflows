package project

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"

	"gopkg.in/yaml.v3"
)

// TestScaffoldParsesCleanly verifies that ScaffoldConfig("example") produces YAML
// that parses cleanly under the strict config.Load decoder.
func TestScaffoldParsesCleanly(t *testing.T) {
	b, err := ScaffoldConfig("example")
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}
	var c config.Config
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		t.Fatalf("scaffold YAML does not parse under strict decoder: %v\n--- YAML ---\n%s", err, b)
	}
	if c.Prefix != "example" {
		t.Errorf("expected prefix 'example', got %q", c.Prefix)
	}
}

// writeScaffold writes scaffold bytes to a fresh awf dir as config.yaml and
// returns the dir (the argument config.Load expects).
func writeScaffold(t *testing.T, b []byte) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestScaffoldEnablesAllCatalogSkills asserts that the scaffolded config enables
// exactly the set of skills declared in the catalog (no more, no less).
func TestScaffoldEnablesAllCatalogSkills(t *testing.T) {
	b, err := ScaffoldConfig("myproj")
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}
	cfg, err := config.Load(writeScaffold(t, b))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	cat, err := catalog.Load(templates.FS)
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}

	for name := range cat.Skills {
		if !slices.Contains(cfg.Skills, name) {
			t.Errorf("scaffold missing catalog skill %q", name)
		}
	}
	for _, name := range cfg.Skills {
		if _, ok := cat.Skills[name]; !ok {
			t.Errorf("scaffold contains unknown skill %q (not in catalog)", name)
		}
	}
}

// TestScaffoldEnablesAllCatalogAgents asserts that the scaffolded config enables
// exactly the set of agents declared in the catalog.
func TestScaffoldEnablesAllCatalogAgents(t *testing.T) {
	b, err := ScaffoldConfig("myproj")
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}
	cfg, err := config.Load(writeScaffold(t, b))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	cat, err := catalog.Load(templates.FS)
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}

	for name := range cat.Agents {
		if !slices.Contains(cfg.Agents, name) {
			t.Errorf("scaffold missing catalog agent %q", name)
		}
	}
	for _, name := range cfg.Agents {
		if _, ok := cat.Agents[name]; !ok {
			t.Errorf("scaffold contains unknown agent %q (not in catalog)", name)
		}
	}
}

// TestScaffoldEnablesAllCatalogHooks asserts that the scaffolded config enables
// every hook in the catalog.
func TestScaffoldEnablesAllCatalogHooks(t *testing.T) {
	b, err := ScaffoldConfig("myproj")
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}
	cfg, err := config.Load(writeScaffold(t, b))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	cat, err := catalog.Load(templates.FS)
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}

	for _, h := range cat.Hooks {
		if !slices.Contains(cfg.Hooks, h) {
			t.Errorf("scaffold missing catalog hook %q", h)
		}
	}
	catHookSet := map[string]bool{}
	for _, h := range cat.Hooks {
		catHookSet[h] = true
	}
	for _, h := range cfg.Hooks {
		if !catHookSet[h] {
			t.Errorf("scaffold contains unknown hook %q (not in catalog)", h)
		}
	}
}

// TestScaffoldVarsCoverReferenced asserts that the scaffolded vars block is a
// superset of the ReferencedVars for representative skill templates.
func TestScaffoldVarsCoverReferenced(t *testing.T) {
	b, err := ScaffoldConfig("example")
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}
	cfg, err := config.Load(writeScaffold(t, b))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	for _, tmplPath := range []string{
		"skills/tdd/SKILL.md.tmpl",
		"skills/executing-plans/SKILL.md.tmpl",
	} {
		src, err := templates.FS.ReadFile(tmplPath)
		if err != nil {
			t.Fatalf("read %s: %v", tmplPath, err)
		}
		for _, v := range render.ReferencedVars(string(src)) {
			if _, ok := cfg.Vars[v]; !ok {
				t.Errorf("scaffold vars missing %q (referenced in %s)", v, tmplPath)
			}
		}
	}
}

// TestInitProducesCleanSyncableProject verifies that writing the scaffold to a
// temp project tree and opening + syncing it produces zero drift.
func TestInitProducesCleanSyncableProject(t *testing.T) {
	b, err := ScaffoldConfig("testproject")
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}

	root := t.TempDir()
	awfDir := filepath.Join(root, ".claude", "awf")
	if err := os.MkdirAll(awfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(awfDir, "config.yaml"), b, 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	drift, err := p.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(drift) != 0 {
		t.Errorf("expected zero drift after init+sync, got: %#v", drift)
	}
}

// TestScaffoldYAMLContainsNoPlaceholders verifies that scaffold output contains
// no "<no value>" tokens or unrendered template actions.
func TestScaffoldYAMLContainsNoPlaceholders(t *testing.T) {
	b, err := ScaffoldConfig("example")
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}
	if strings.Contains(string(b), "<no value>") {
		t.Errorf("scaffold YAML contains '<no value>':\n%s", b)
	}
	if strings.Contains(string(b), "{{") {
		t.Errorf("scaffold YAML contains unrendered template action:\n%s", b)
	}
}
