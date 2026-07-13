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

// sliceSet builds a membership set from a name slice (test helper; its
// production twin left with the ADR-0086 sweep rewrite).
func sliceSet(s []string) map[string]bool {
	m := make(map[string]bool, len(s))
	for _, v := range s {
		m[v] = true
	}
	return m
}

// TestScaffoldParsesCleanly verifies that ScaffoldConfig with no overrides produces YAML
// that parses cleanly under the strict config.Load decoder.
func TestScaffoldParsesCleanly(t *testing.T) {
	b, _, err := ScaffoldConfig("example", nil, nil, nil)
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
	b, _, err := ScaffoldConfig("myproj", nil, nil, nil)
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
	// invariant: scaffold-core-only
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

	// A selected chain skill pulls its closure (ADR-0081 Decision 9): the trim
	// is closure-completed, its agents derived from the selection, and every
	// addition beyond the selection returned kind-prefixed.
	pickSkills := []string{"tdd", "brainstorming"}
	b, added, err := ScaffoldConfig("myproj", nil, &config.CatalogTrim{Skills: &pickSkills}, nil)
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}
	cfg, err := config.Load(writeScaffold(t, b))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if len(cfg.Skills) != 12 { // the 11-skill chain closure + tdd
		t.Errorf("closure-completed trim = %d skills, want 12: %v", len(cfg.Skills), cfg.Skills)
	}
	if len(cfg.Agents) != 3 {
		t.Errorf("derived agents = %v, want the three reviewers", cfg.Agents)
	}
	if len(added) != 13 { // 10 closure skills + 3 agents beyond the 2-skill selection
		t.Errorf("added = %d entries, want 13: %v", len(added), added)
	}
	if !slices.Contains(added, "skill reviewing-plan-resync") || !slices.Contains(added, "agent plan-reviewer") {
		t.Errorf("added missing expected kind-prefixed entries: %v", added)
	}
	if len(cfg.Docs) != 0 {
		t.Errorf("nil docs trim should yield no docs (no core docs remain), got %v", cfg.Docs)
	}

	// A leaves-only trim scaffolds exactly the leaves and zero agents.
	leafSkills := []string{"tdd"}
	bl, addedLeaf, err := ScaffoldConfig("myproj", nil, &config.CatalogTrim{Skills: &leafSkills}, nil)
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}
	cfgLeaf, err := config.Load(writeScaffold(t, bl))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if len(cfgLeaf.Skills) != 1 || cfgLeaf.Skills[0] != "tdd" || len(cfgLeaf.Agents) != 0 || len(addedLeaf) != 0 {
		t.Errorf("leaves-only trim = skills %v agents %v added %v, want [tdd] [] []", cfgLeaf.Skills, cfgLeaf.Agents, addedLeaf)
	}

	// A doc-gated selection gains its doc.
	gatedSkills := []string{"roadmap-graduation"}
	bg, addedGated, err := ScaffoldConfig("myproj", nil, &config.CatalogTrim{Skills: &gatedSkills}, nil)
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}
	cfgGated, err := config.Load(writeScaffold(t, bg))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if !slices.Contains(cfgGated.Docs, "roadmap") || !slices.Contains(addedGated, "doc roadmap") {
		t.Errorf("doc-gated trim = docs %v added %v, want the roadmap doc pulled in", cfgGated.Docs, addedGated)
	}

	// Docs deselected to empty; Skills nil -> keep core skills.
	emptyDocs := []string{}
	coreSkills := map[string]bool{}
	for name, spec := range cat.Skills {
		if spec.Core {
			coreSkills[name] = true
		}
	}
	b2, _, err := ScaffoldConfig("myproj", nil, &config.CatalogTrim{Docs: &emptyDocs}, nil)
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
	b, _, err := ScaffoldConfig("myproj", nil, nil, nil)
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
	b, _, err := ScaffoldConfig("example", nil, nil, nil)
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
	for _, e := range cat.Docs {
		paths = append(paths, e.TID) // merged-in singletons render from non-docs/ templates
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
			// invariant: scaffold-seeds-all-vars
			if _, ok := cfg.Vars[v]; !ok {
				t.Errorf("scaffold vars missing %q (referenced in %s)", v, tmplPath)
			}
		}
	}
}

// TestInitProducesCleanSyncableProject verifies that writing the scaffold to a
// temp project tree and opening + syncing it produces zero drift.
func TestInitProducesCleanSyncableProject(t *testing.T) {
	b, _, err := ScaffoldConfig("testproject", nil, nil, nil)
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
	b, _, err := ScaffoldConfig("example", nil, nil, nil)
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
	b, _, err := ScaffoldConfig("example", nil, nil, []string{"adr", "awf"})
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}
	for _, want := range []string{"audit:", "allowedScopes:", "- adr", "- awf"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("scaffold missing %q:\n%s", want, b)
		}
	}
	b2, _, err := ScaffoldConfig("example", nil, nil, nil)
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}
	if strings.Contains(string(b2), "audit:") {
		t.Errorf("nil scopes must write no audit block:\n%s", b2)
	}
}

// The untrimmed curated default satisfies the closure invariant: every
// scaffolded skill's and agent's direct requirements are themselves in the
// scaffolded arrays (ADR-0081 Decision 9; backing marker in scaffold.go).
func TestScaffoldDefaultIsClosed(t *testing.T) {
	b, added, err := ScaffoldConfig("myproj", nil, nil, nil)
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}
	if len(added) != 0 {
		t.Errorf("untrimmed default must report no additions, got %v", added)
	}
	cfg, err := config.Load(writeScaffold(t, b))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	enabled := map[catalog.Node]bool{}
	for _, s := range cfg.Skills {
		enabled[catalog.Node{Kind: "skill", Name: s}] = true
	}
	for _, a := range cfg.Agents {
		enabled[catalog.Node{Kind: "agent", Name: a}] = true
	}
	for _, d := range cfg.Docs {
		enabled[catalog.Node{Kind: "doc", Name: d}] = true
	}
	// invariant: init-set-closed
	for n := range enabled {
		for _, r := range catalog.RequiresOf(catalog.Standard, n) {
			if !enabled[r] {
				t.Errorf("default set unclosed: %v requires %v", n, r)
			}
		}
	}
}

// NeededVars (ADR-0086 Decision 6): the untrimmed default needs the hook
// payloads' var; a trim to tdd-only drops invariantTestPath, which only the
// adr-reviewer agent and retrospective skill reference (both outside tdd's
// closure), while agents-doc/workflow keep gateCmd needed.
func TestNeededVars(t *testing.T) {
	full, err := NeededVars(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"commitGateCmd", "gateCmd", "invariantTestPath"} {
		if !full[v] {
			t.Errorf("untrimmed default must need %s", v)
		}
	}
	trim := &config.CatalogTrim{Skills: &[]string{"tdd"}, Docs: &[]string{}}
	trimmed, err := NeededVars(trim)
	if err != nil {
		t.Fatal(err)
	}
	if trimmed["invariantTestPath"] {
		t.Error("a tdd-only trim must not need invariantTestPath")
	}
	for _, v := range []string{"commitGateCmd", "gateCmd"} {
		if !trimmed[v] {
			t.Errorf("hook payloads and always-on singletons keep %s needed", v)
		}
	}
}
