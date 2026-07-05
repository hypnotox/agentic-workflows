package project

import (
	"bytes"
	"maps"
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

// TestScaffoldParsesCleanly verifies that ScaffoldConfig with no overrides produces YAML
// that parses cleanly under the strict config.Load decoder.
func TestScaffoldParsesCleanly(t *testing.T) {
	b, err := ScaffoldConfig("example", nil, nil, nil, nil)
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
	if !bytes.Contains(b, []byte("bootstrap:")) || !bytes.Contains(b, []byte("enabled: true")) {
		t.Errorf("scaffold should seed bootstrap enabled by default:\n%s", b)
	}
	if c.Bootstrap == nil || !c.Bootstrap.Enabled {
		t.Errorf("scaffold bootstrap = %+v, want enabled true", c.Bootstrap)
	}
	// invariant: init-hooks-default-on
	if c.Hooks == nil || !c.Hooks.Enabled {
		t.Errorf("scaffold hooks = %+v, want enabled true (ADR-0048)", c.Hooks)
	}
	// The hook payloads' vars are seeded like every other referenced var, so an
	// init prompt answer for commitGateCmd is not silently dropped.
	if !bytes.Contains(b, []byte("commitGateCmd:")) {
		t.Errorf("scaffold should seed commitGateCmd (referenced by the hook payloads):\n%s", b)
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

// TestScaffoldEnablesCoreTargets asserts that the scaffolded config enables
// exactly the catalog's core skills and core docs (ADR-0022), with a concrete
// negative check that a known opt-in skill is omitted.
func TestScaffoldEnablesCoreTargets(t *testing.T) {
	b, err := ScaffoldConfig("myproj", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}
	cfg, err := config.Load(writeScaffold(t, b))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	cat := catalog.Standard

	wantSkills := map[string]bool{}
	for name, spec := range cat.Skills {
		if spec.Core {
			wantSkills[name] = true
		}
	}
	if got := sliceSet(cfg.Skills); !maps.Equal(got, wantSkills) {
		t.Errorf("scaffold skills = %v, want core set %v",
			slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(wantSkills)))
	}

	// No doc remains core (ADR-0043 promoted the only three core docs — workflow,
	// doc-standard, agents-md-standard — to mandatory singletons outside cat.Docs).
	if len(cfg.Docs) != 0 {
		t.Errorf("scaffold docs = %v, want none (no core docs remain)", cfg.Docs)
	}

	// Concrete negative: a known opt-in skill must not be scaffolded.
	if slices.Contains(cfg.Skills, "tdd") {
		t.Errorf("scaffold should not enable the opt-in skill tdd")
	}
}

// TestScaffoldCatalogTrim asserts a non-nil trim dimension replaces the curated
// core verbatim while a nil dimension keeps the core (full-deselectable trim).
// invariant: catalog-trim-applied
func TestScaffoldCatalogTrim(t *testing.T) {
	cat := catalog.Standard

	// Skills selected verbatim (incl. deselecting core); Docs nil -> no core docs to keep.
	pickSkills := []string{"tdd", "brainstorming"}
	b, err := ScaffoldConfig("myproj", nil, nil, &config.CatalogTrim{Skills: &pickSkills}, nil)
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}
	cfg, err := config.Load(writeScaffold(t, b))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if got := sliceSet(cfg.Skills); !maps.Equal(got, map[string]bool{"tdd": true, "brainstorming": true}) {
		t.Errorf("trim skills = %v, want [brainstorming tdd]", slices.Sorted(maps.Keys(got)))
	}
	if len(cfg.Docs) != 0 {
		t.Errorf("nil docs trim should yield no docs (no core docs remain), got %v", cfg.Docs)
	}

	// Docs deselected to empty; Skills nil -> keep core skills.
	emptyDocs := []string{}
	coreSkills := map[string]bool{}
	for name, spec := range cat.Skills {
		if spec.Core {
			coreSkills[name] = true
		}
	}
	b2, err := ScaffoldConfig("myproj", nil, nil, &config.CatalogTrim{Docs: &emptyDocs}, nil)
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}
	cfg2, err := config.Load(writeScaffold(t, b2))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if len(cfg2.Docs) != 0 {
		t.Errorf("empty docs trim should enable no docs, got %v", cfg2.Docs)
	}
	if got := sliceSet(cfg2.Skills); !maps.Equal(got, coreSkills) {
		t.Errorf("nil skills trim should keep core skills, got %v", slices.Sorted(maps.Keys(got)))
	}
}

// TestScaffoldEnablesAllCatalogAgents asserts that the scaffolded config enables
// exactly the set of agents declared in the catalog.
func TestScaffoldEnablesAllCatalogAgents(t *testing.T) {
	b, err := ScaffoldConfig("myproj", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}
	cfg, err := config.Load(writeScaffold(t, b))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	cat := catalog.Standard

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

// TestScaffoldVarsCoverAllReferenced asserts the scaffolded vars block seeds every
// var referenced by any catalog template family — skills, agents, and docs —
// backing inv: scaffold-seeds-all-vars. The expected set is re-derived from the
// templates here, independently of ScaffoldConfig's own collection, so an unseeded
// future var (e.g. a new doc var) fails this test.
func TestScaffoldVarsCoverAllReferenced(t *testing.T) {
	b, err := ScaffoldConfig("example", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}
	cfg, err := config.Load(writeScaffold(t, b))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cat := catalog.Standard

	var paths []string
	for name := range cat.Skills {
		paths = append(paths, "skills/"+name+"/SKILL.md.tmpl")
	}
	for name := range cat.Agents {
		paths = append(paths, "agents/"+name+".md.tmpl")
	}
	for name := range cat.Docs {
		paths = append(paths, "docs/"+name+".md.tmpl")
	}
	for _, sg := range plainSingletons {
		paths = append(paths, sg.tid)
	}
	for _, tmplPath := range paths {
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
	b, err := ScaffoldConfig("testproject", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}

	root := t.TempDir()
	awfDir := filepath.Join(root, ".awf")
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
	b, err := ScaffoldConfig("example", nil, nil, nil, nil)
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

// A resolved scope list lands under audit.allowedScopes; an empty list writes
// no audit key at all (ADR-0051).
// invariant: audit-scopes-descriptor-routed
func TestScaffoldWritesAuditScopes(t *testing.T) {
	b, err := ScaffoldConfig("example", nil, nil, nil, []string{"adr", "awf"})
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}
	for _, want := range []string{"audit:", "allowedScopes:", "- adr", "- awf"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("scaffold missing %q:\n%s", want, b)
		}
	}
	b2, err := ScaffoldConfig("example", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}
	if strings.Contains(string(b2), "audit:") {
		t.Errorf("nil scopes must write no audit block:\n%s", b2)
	}
}
