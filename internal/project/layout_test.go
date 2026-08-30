package project

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
)

func layout(p renderInputs) Layout {
	d := config.DocsDir
	dec := d + "/decisions"
	docs := map[string]string{}
	singletons := map[string]string{}
	if projectCatalog(p) != nil {
		for name, entry := range projectCatalog(p).Docs {
			docs[name] = docOutPath(p, name)
			if !entry.AgentsDoc && entry.TemplateKey != "" {
				singletons[entry.TemplateKey] = docOutPath(p, name)
			}
		}
	}
	return Layout{DocsDir: d, ADRDir: dec, IndexMd: dec + "/INDEX.md", Docs: docs, Singletons: singletons, DomainsDir: d + "/domains"}
}

// invariant: rendering/doc-outputs:layout-derivation (TestLayoutUsesFixedDocsRootAndFullCatalog)
// invariant: rendering/doc-outputs:docs-root-fixed (TestLayoutUsesFixedDocsRootAndFullCatalog)
func TestLayoutUsesFixedDocsRootAndFullCatalog(t *testing.T) {
	cfg := &config.Config{}
	p := testState(cfg)
	l := layout(newRenderInputs(p, cfg, nil))
	if l.DocsDir != config.DocsDir || l.ADRDir != "docs/decisions" ||
		l.IndexMd != "docs/decisions/INDEX.md" {
		t.Errorf("layout = %+v", l)
	}
	// invariant: rendering/doc-outputs:domains-dir-given (TestLayoutUsesFixedDocsRootAndFullCatalog)
	if l.DomainsDir != "docs/domains" {
		t.Errorf("domainsDir = %q", l.DomainsDir)
	}
	// invariant: rendering/doc-outputs:layout-docs-profile-projection (TestLayoutUsesFixedDocsRootAndFullCatalog)
	if len(l.Docs) != len(catalog.Standard.Docs) {
		t.Errorf("Docs has %d entries, want full catalog of %d: %v", len(l.Docs), len(catalog.Standard.Docs), l.Docs)
	}
	for name, entry := range catalog.Standard.Docs {
		want := "docs/" + name + ".md"
		if entry.Path != "" {
			want = "docs/" + entry.Path
		}
		if entry.AgentsDoc {
			want = "AGENTS.md"
		}
		if got, ok := l.Docs[name]; !ok || got != want {
			t.Errorf("Docs[%q] = %q (present %t), want catalog-derived %q", name, got, ok, want)
		}
	}
	// templateMap reproduces the historical .layout map by literal value (ConfigHash
	// stability). The fixed directory keys are hand-built; the mandatory-singleton
	// keys derive from the catalog (ADR-0061) - assert each one's exact value so a
	// wrong derivation is caught, not just a present key.
	tm := l.templateMap()
	wantTM := map[string]string{
		"docsDir":                "docs",
		"adrDir":                 "docs/decisions",
		"indexMd":                "docs/decisions/INDEX.md",
		"domainsDir":             "docs/domains",
		"adrReadme":              "docs/decisions/README.md",
		"adrTemplate":            "docs/decisions/template.md",
		"workflowRef":            "docs/workflow.md",
		"docStandard":            "docs/doc-standard.md",
		"agentsMdStandard":       "docs/agents-md-standard.md",
		"workingWithAwf":         "docs/working-with-awf.md",
		"maintainableCodeDesign": "docs/maintainable-code-design.md",
	}
	for k, want := range wantTM {
		if tm[k] != want {
			t.Errorf("templateMap[%q] = %v, want %q", k, tm[k], want)
		}
	}
	if got, ok := tm["docs"].(map[string]any); !ok || got["architecture"] != "docs/architecture.md" || got["debugging"] != "docs/debugging.md" {
		t.Errorf("templateMap[docs] = %v", tm["docs"])
	}
	// 5 fixed dir keys + docs + 8 mandatory-singleton keys = 14 (agents-doc has
	// no TemplateKey and is excluded; removed plan singletons no longer contribute
	// layout keys, while the generated config reference remains layout-exposed).
	if len(tm) != 14 {
		t.Errorf("templateMap has %d keys, want 14", len(tm))
	}
	if got := docOutPath(renderInputsForTest(p), "architecture"); got != "docs/architecture.md" {
		t.Errorf("docOutPath = %q", got)
	}
}
