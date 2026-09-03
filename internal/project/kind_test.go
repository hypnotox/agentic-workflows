package project

import (
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/artifactregistry"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

func TestResidualKindDescriptorProjectsDocTemplateAndUnknownSections(t *testing.T) {
	descriptor, ok := descriptorByPlural("docs")
	if !ok || descriptor.templateID(catalog.Standard, "architecture") != catalog.Standard.Docs["architecture"].TID {
		t.Fatalf("docs descriptor = %#v, %t", descriptor, ok)
	}
	domain, ok := descriptorByPlural("domains")
	if !ok || domain.templateID(catalog.Standard, "example") != "domains/domain.md.tmpl" {
		t.Fatalf("domain descriptor = %#v, %t", domain, ok)
	}
}

func TestKindLookups(t *testing.T) {
	if got := Kinds(); !slices.Equal(got, []string{"skill", "doc", "domain"}) {
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
	cat := &catalog.Catalog{
		Skills:    map[string]catalog.SkillSpec{"awf-test": {Sections: []string{"a"}}},
		Docs:      map[string]catalog.DocEntry{"arch": {Sections: []string{"c"}, TID: "docs/arch.md.tmpl"}},
		DomainDoc: catalog.TargetSpec{Sections: []string{"d"}},
	}

	for _, c := range []struct{ kind, want string }{
		{"skill", "awf-test"}, {"doc", "arch"},
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

	if s, ok := descriptorMust(t, "skills").sections(cat, "awf-test"); !ok || !slices.Equal(s, []string{"a"}) {
		t.Errorf("skills sections = %v,%v", s, ok)
	}
	if s, ok := descriptorMust(t, "docs").sections(cat, "arch"); !ok || !slices.Equal(s, []string{"c"}) {
		t.Errorf("docs sections = %v,%v", s, ok)
	}
	if s, ok := descriptorMust(t, "domains").sections(cat, "tooling"); ok || !slices.Equal(s, []string{"d"}) {
		t.Errorf("domains sections = %v,%v", s, ok)
	}

	tgt := artifactregistry.Target{SkillDir: ".claude/skills"}
	if got := descriptorMust(t, "skills").outPath(tgt, "ignored", "awf-test"); got != ".claude/skills/awf-test/SKILL.md" {
		t.Errorf("skills outPath = %q", got)
	}
	for _, pl := range []string{"docs", "domains"} {
		if descriptorMust(t, pl).outPath != nil {
			t.Errorf("%s outPath should be nil", pl)
		}
	}
}
