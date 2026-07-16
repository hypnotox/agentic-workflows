package project

import (
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
)

// invariant: kind-dispatch-single-table
func TestKindDescriptorsCoverAllKinds(t *testing.T) {
	got := make([]string, len(kindDescriptors))
	for i, d := range kindDescriptors {
		got[i] = d.Plural
	}
	want := []string{"skills", "agents", "docs", "domains"}
	if !slices.Equal(got, want) {
		t.Fatalf("kind set drift: got %v want %v", got, want)
	}
	// Every catalog-backed kind resolves a pool; domains (freeform) must not.
	for _, d := range kindDescriptors {
		hasPool := d.poolNames != nil
		if (d.Plural == "domains") == hasPool {
			t.Errorf("%s poolNames presence wrong (hasPool=%v)", d.Plural, hasPool)
		}
	}
}

func TestKindLookups(t *testing.T) {
	// invariant: cli-config-kinds
	if got := Kinds(); !slices.Equal(got, []string{"skill", "agent", "doc", "domain"}) {
		t.Fatalf("Kinds() = %v", got)
	}
	if pl, ok := PluralKind("skill"); !ok || pl != "skills" {
		t.Errorf("PluralKind(skill) = %q,%v", pl, ok)
	}
	if _, ok := PluralKind("bogus"); ok {
		t.Error("PluralKind(bogus) should be false")
	}
	if _, ok := descriptorByPlural("bogus"); ok {
		t.Error("descriptorByPlural(bogus) should be false")
	}
}

func descriptorMust(t *testing.T, plural string) kindDescriptor {
	t.Helper()
	d, ok := descriptorByPlural(plural)
	if !ok {
		t.Fatalf("descriptor %q missing", plural)
	}
	return d
}

func TestKindAccessors(t *testing.T) {
	cfg := &config.Config{
		Prefix: "awf",
		Skills: []string{"tdd"}, Agents: []string{"rev"}, Docs: []string{"arch"},
		Domains: []string{"tooling"},
	}
	cat := &catalog.Catalog{
		Skills:    map[string]catalog.SkillSpec{"tdd": {Sections: []string{"a"}}},
		Agents:    map[string]catalog.AgentSpec{"rev": {Name: "rev", Description: "reviewer", Sections: []string{"b"}}},
		Docs:      map[string]catalog.DocEntry{"arch": {Sections: []string{"c"}, TID: "docs/arch.md.tmpl"}},
		DomainDoc: catalog.TargetSpec{Sections: []string{"d"}},
	}

	// enable facet (via EnabledNames) for every kind, plus the unknown branch.
	for _, c := range []struct{ kind, want string }{
		{"skill", "tdd"}, {"agent", "rev"}, {"doc", "arch"}, {"domain", "tooling"},
	} {
		names, ok := EnabledNames(cfg, c.kind)
		if !ok || !slices.Equal(names, []string{c.want}) {
			t.Errorf("EnabledNames(%s) = %v,%v", c.kind, names, ok)
		}
	}
	if _, ok := EnabledNames(cfg, "bogus"); ok {
		t.Error("EnabledNames(bogus) should be false")
	}

	// poolNames facet (via CatalogNames) for every catalog-backed kind, plus the
	// no-pool (domain) and unknown branches.
	for _, c := range []struct{ kind, want string }{
		{"skill", "tdd"}, {"agent", "rev"}, {"doc", "arch"},
	} {
		pool, ok := CatalogNames(cat, c.kind)
		if !ok || !slices.Contains(pool, c.want) {
			t.Errorf("CatalogNames(%s) = %v,%v", c.kind, pool, ok)
		}
	}
	if _, ok := CatalogNames(cat, "domain"); ok {
		t.Error("CatalogNames(domain) should be false (no pool)")
	}
	if _, ok := CatalogNames(cat, "bogus"); ok {
		t.Error("CatalogNames(bogus) should be false")
	}

	// sections facet: catalog-backed kinds report presence; domains keep the
	// singleton's sections but report no per-name presence.
	if s, ok := descriptorMust(t, "skills").sections(cat, "tdd"); !ok || !slices.Equal(s, []string{"a"}) {
		t.Errorf("skills sections = %v,%v", s, ok)
	}
	if s, ok := descriptorMust(t, "agents").sections(cat, "rev"); !ok || !slices.Equal(s, []string{"b"}) {
		t.Errorf("agents sections = %v,%v", s, ok)
	}
	if s, ok := descriptorMust(t, "docs").sections(cat, "arch"); !ok || !slices.Equal(s, []string{"c"}) {
		t.Errorf("docs sections = %v,%v", s, ok)
	}
	if s, ok := descriptorMust(t, "domains").sections(cat, "tooling"); ok || !slices.Equal(s, []string{"d"}) {
		t.Errorf("domains sections = %v,%v", s, ok)
	}

	// outPath facet: skills/agents place adapter artifacts; docs/domains are neutral (nil).
	tgt := Target{SkillDir: ".claude/skills", AgentDir: ".claude/agents"}
	if got := descriptorMust(t, "skills").outPath(tgt, "awf", "tdd"); got != ".claude/skills/awf-tdd/SKILL.md" {
		t.Errorf("skills outPath = %q", got)
	}
	if got := descriptorMust(t, "agents").outPath(tgt, "awf", "rev"); got != ".claude/agents/rev.md" {
		t.Errorf("agents outPath = %q", got)
	}
	for _, pl := range []string{"docs", "domains"} {
		if descriptorMust(t, pl).outPath != nil {
			t.Errorf("%s outPath should be nil", pl)
		}
	}
}
