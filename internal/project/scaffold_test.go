package project

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"

	"gopkg.in/yaml.v3"
)

// TestReferencedVarsInScaffold verifies that ScaffoldConfig("example") produces YAML
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

// TestScaffoldEnablesAllCatalogSkills asserts that the scaffolded config enables
// exactly the set of skills declared in the catalog (no more, no less).
func TestScaffoldEnablesAllCatalogSkills(t *testing.T) {
	b, err := ScaffoldConfig("myproj")
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}
	cfgPath := writeTemp(t, b)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	cat, err := catalog.Load(templates.FS)
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}

	// Every catalog skill must be present in the scaffolded config.
	for name := range cat.Skills {
		if _, ok := cfg.Skills[name]; !ok {
			t.Errorf("scaffold missing catalog skill %q", name)
		}
	}
	// No extra non-local skills should be present.
	for name, sc := range cfg.Skills {
		if sc.Local {
			continue
		}
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
	cfgPath := writeTemp(t, b)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	cat, err := catalog.Load(templates.FS)
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}

	for name := range cat.Agents {
		if _, ok := cfg.Agents[name]; !ok {
			t.Errorf("scaffold missing catalog agent %q", name)
		}
	}
	for name, ac := range cfg.Agents {
		if ac.Local {
			continue
		}
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
	cfgPath := writeTemp(t, b)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	cat, err := catalog.Load(templates.FS)
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}

	catHookSet := map[string]bool{}
	for _, h := range cat.Hooks {
		catHookSet[h] = true
	}
	cfgHookSet := map[string]bool{}
	for _, h := range cfg.Hooks {
		cfgHookSet[h] = true
	}
	for _, h := range cat.Hooks {
		if !cfgHookSet[h] {
			t.Errorf("scaffold missing catalog hook %q", h)
		}
	}
	for _, h := range cfg.Hooks {
		if !catHookSet[h] {
			t.Errorf("scaffold contains unknown hook %q (not in catalog)", h)
		}
	}
}

// TestScaffoldVarsCoverReferenced asserts that the scaffolded vars block is a
// superset of the ReferencedVars for representative skill templates (tdd,
// executing-plans).
func TestScaffoldVarsCoverReferenced(t *testing.T) {
	b, err := ScaffoldConfig("example")
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}
	cfgPath := writeTemp(t, b)
	cfg, err := config.Load(cfgPath)
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
// temp directory and opening + syncing it produces zero drift.
func TestInitProducesCleanSyncableProject(t *testing.T) {
	b, err := ScaffoldConfig("testproject")
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}

	root := t.TempDir()
	cfgDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(cfgDir, "awf.yaml")
	if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
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

// writeTemp writes bytes to a temp file and returns the path.
func writeTemp(t *testing.T, b []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "awf.yaml")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
