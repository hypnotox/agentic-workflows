package project

import (
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
// invariant: tooling/init-and-enablement:init-hooks-default-on
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
	// invariant: rendering/project-output-plan:scaffold-core-only
	if got := sliceSet(cfg.Skills); !maps.Equal(got, wantSkills) {
		t.Errorf("scaffold skills = %v, want core set %v",
			slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(wantSkills)))
	}
	// No doc remains core (ADR-0043 promoted the only three core docs - workflow,
	// doc-standard, agents-md-standard - to mandatory singletons outside cat.Docs).
	if len(cfg.Docs) != 0 {
		t.Errorf("scaffold docs = %v, want none (no core docs remain)", cfg.Docs)
	}

	// Concrete negative: a known opt-in skill must not be scaffolded.
	if slices.Contains(cfg.Skills, "tdd") {
		t.Errorf("scaffold should not enable the opt-in skill tdd")
	}
}

// A freshly scaffolded config carries the required integrationBranch key and
// therefore passes its own validation. Without the key the scaffold would emit
// a config that fails on the very next open (ADR-0194 Decision 6).
// invariant: config/configuration:integration-branch-explicit
func TestScaffoldWritesValidIntegrationBranch(t *testing.T) {
	b, _, err := ScaffoldConfig("myproj", nil, nil, nil)
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}
	if !strings.Contains(string(b), "integrationBranch: main\n") {
		t.Fatalf("scaffolded config does not write the key visibly:\n%s", b)
	}
	cfg, err := config.Load(writeScaffold(t, b))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("a freshly scaffolded config must validate: %v", err)
	}
}

// TestScaffoldCatalogTrim asserts a non-nil trim dimension replaces the curated
// core verbatim while a nil dimension keeps the core (full-deselectable trim).
// invariant: rendering/project-output-plan:catalog-trim-applied
func TestScaffoldCatalogTrim(t *testing.T) {
	cat := catalog.Standard

	// Advisory profile neighbors do not expand a trim.
	pickSkills := []string{"tdd", "brainstorming"}
	b, added, err := ScaffoldConfig("myproj", nil, &config.CatalogTrim{Skills: &pickSkills}, nil)
	if err != nil {
		t.Fatalf("ScaffoldConfig: %v", err)
	}
	cfg, err := config.Load(writeScaffold(t, b))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	// brainstorming's paired agent is a structural edge and must be scaffolded, or
	// the trimmed config would fail project open. Its advisory RequiresSkills are
	// what must stay out, and the wantSkills comparison below is what proves that.
	wantNodes := []catalog.Node{
		{Kind: "skill", Name: "tdd"}, {Kind: "skill", Name: "brainstorming"},
		{Kind: "agent", Name: "grounding-checker"},
	}
	wantSkills, wantAdded := map[string]bool{}, map[string]bool{}
	selected := map[string]bool{"tdd": true, "brainstorming": true}
	for _, node := range wantNodes {
		switch node.Kind {
		case "skill":
			wantSkills[node.Name] = true
			if !selected[node.Name] {
				wantAdded["skill "+node.Name] = true
			}
		case "agent":
			wantAdded["agent "+node.Name] = true
		}
	}
	if got := sliceSet(cfg.Skills); !maps.Equal(got, wantSkills) {
		t.Errorf("closure-completed trim skills = %v, want %v", slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(wantSkills)))
	}
	if got := sliceSet(added); !maps.Equal(got, wantAdded) {
		t.Errorf("closure additions = %v, want %v", slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(wantAdded)))
	}
	if got := sliceSet(cfg.Agents); !maps.Equal(got, map[string]bool{"grounding-checker": true}) {
		t.Errorf("advisory trim scaffolded agents=%v, want exactly the structurally required grounding-checker", cfg.Agents)
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

	// A selected reviewing skill pulls its required reviewing agent into the
	// closure-derived agent selection.
	reviewing := []string{"reviewing-plan"}
	_, agentNames, _, added := scaffoldSelection(cat, &config.CatalogTrim{Skills: &reviewing})
	if !slices.Contains(agentNames, "plan-reviewer") || !slices.Contains(added, "agent plan-reviewer") {
		t.Errorf("reviewing skill closure = agents %v, added %v", agentNames, added)
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
// var referenced by any catalog template family - skills, agents, and docs -
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
			// invariant: rendering/project-output-plan:scaffold-seeds-all-vars
			if _, ok := cfg.Vars[v]; !ok {
				t.Errorf("scaffold vars missing %q (referenced in %s)", v, tmplPath)
			}
		}
	}
}

// TestInitProducesCleanSyncableProject verifies that writing the scaffold to a
// temp project tree and opening + syncing it produces zero drift.
func TestInitProducesCleanSyncableProject(t *testing.T) {
	// A gateCmd answer keeps the scaffold's enabled hooks singleton valid under
	// the command-wiring rule (ADR-0156 Decision 5); an unanswered init still
	// scaffolds and initializes, but its first ordinary sync/check demands the
	// gate command (covered by TestValidateCommandWiring).
	b, _, err := ScaffoldConfig("testproject", map[string]string{"gateCmd": "make gate"}, nil, nil)
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

	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	drift, err := p.Check(testContext(t))
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
// invariant: tooling/audit-commands:audit-scopes-descriptor-routed
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
	// invariant: tooling/init-and-enablement:init-set-closed
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
