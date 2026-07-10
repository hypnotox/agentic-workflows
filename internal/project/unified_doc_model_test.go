package project

import (
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
)

// The whole doc surface derives from the single catalog doc collection: every
// projection is a function of the Mandatory/AgentsDoc/TemplateKey/Path metadata,
// with no independent hand-maintained list (ADR-0061).
// invariant: unified-doc-model
func TestUnifiedDocModelProjections(t *testing.T) {
	// (a) SingletonKinds == exactly the Mandatory entries.
	var wantSK []string
	for k, e := range catalog.Standard.Docs {
		if e.Mandatory {
			wantSK = append(wantSK, k)
		}
	}
	slices.Sort(wantSK)
	if sk := catalog.SingletonKinds(); !slices.Equal(sk, wantSK) {
		t.Errorf("SingletonKinds()=%v, want Mandatory entries %v", sk, wantSK)
	}

	// (b) plainSingletons == exactly Mandatory && !AgentsDoc && !Generated, and
	// no other kind (the generated config reference renders outside RenderAll).
	var got []string
	for _, s := range plainSingletons {
		got = append(got, s.kind)
	}
	slices.Sort(got)
	var wantPS []string
	for k, e := range catalog.Standard.Docs {
		if e.Mandatory && !e.AgentsDoc && !e.Generated {
			wantPS = append(wantPS, k)
		}
	}
	slices.Sort(wantPS)
	if !slices.Equal(got, wantPS) {
		t.Errorf("plainSingletons kinds=%v, want %v", got, wantPS)
	}

	// (c) every mandatory non-agents-doc entry's TemplateKey/Path lands in
	// templateMap at the derived docsDir path.
	tm := (&Project{Cfg: &config.Config{DocsDir: "documentation"}}).layout().templateMap()
	for _, e := range catalog.Standard.Docs {
		if !e.Mandatory || e.AgentsDoc {
			continue
		}
		if v := tm[e.TemplateKey]; v != "documentation/"+e.Path {
			t.Errorf("templateMap[%q]=%v, want %q", e.TemplateKey, v, "documentation/"+e.Path)
		}
	}
}

// No Mandatory entry appears in the toggleable-doc pool, so a singleton is never
// addable/removable via the doc CLI or validated as a toggleable doc (ADR-0061).
// invariant: mandatory-doc-pool-exclusion
func TestMandatoryDocsExcludedFromPool(t *testing.T) {
	pool, ok := CatalogNames(catalog.Standard, "doc")
	if !ok {
		t.Fatal("doc pool absent")
	}
	for _, n := range pool {
		if catalog.Standard.Docs[n].Mandatory {
			t.Errorf("mandatory doc %q leaked into the toggleable pool", n)
		}
	}
	sk := catalog.SingletonKinds()
	for _, n := range catalog.NonMandatoryDocNames(catalog.Standard) {
		if slices.Contains(sk, n) {
			t.Errorf("%q is both non-mandatory and a singleton kind", n)
		}
	}
}
