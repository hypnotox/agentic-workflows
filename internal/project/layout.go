package project

import (
	"path/filepath"
	"sort"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
)

// Layout is the fixed, awf-given docs layout in typed form for Go consumers.
// These paths are not configurable through the project tree.
// templateMap projects it into the .layout template namespace (templates read a
// map, not unexported struct fields) and into the per-file ConfigHash. The
// mandatory-singleton paths are not struct fields: they derive from the catalog
// doc collection in templateMap (ADR-0061).
type Layout struct {
	DocsDir    string
	ADRDir     string
	IndexMd    string
	PlansDir   string
	Docs       map[string]string // catalog name -> output path (inv: layout-docs-full-catalog)
	DomainsDir string
}

func (p *Project) layout() Layout {
	d := config.DocsDir
	dec := d + "/decisions"
	// Docs maps every catalog document to its unconditional rendered output path.
	docs := map[string]string{}
	if p.Cat != nil {
		for name := range p.Cat.Docs {
			docs[name] = p.docOutPath(name)
		}
	}
	return Layout{
		DocsDir:    d,
		ADRDir:     dec,
		IndexMd:    dec + "/INDEX.md",
		PlansDir:   d + "/plans",
		Docs:       docs,
		DomainsDir: d + "/domains", // inv: domains-dir-given
	}
}

// templateMap projects the layout into the map the .layout template namespace and
// the per-file ConfigHash consume. The fixed directory/generated keys are set
// here; the mandatory-singleton keys (adrReadme, adrTemplate, plansReadme,
// workflowRef, docStandard, agentsMdStandard, workingWithAwf) derive from the
// catalog doc collection - each entry's TemplateKey at docsDir/Path - so the map
// reproduces the historical key set and values byte-for-byte (ADR-0061).
func (l Layout) templateMap() map[string]any {
	docs := map[string]any{}
	for k, v := range l.Docs {
		docs[k] = v
	}
	m := map[string]any{
		"docsDir":    l.DocsDir,
		"adrDir":     l.ADRDir,
		"indexMd":    l.IndexMd,
		"plansDir":   l.PlansDir,
		"docs":       docs,
		"domainsDir": l.DomainsDir,
	}
	for _, k := range catalog.SingletonKinds() {
		e := catalog.Standard.Docs[k]
		if e.AgentsDoc || e.TemplateKey == "" {
			continue
		}
		m[e.TemplateKey] = l.DocsDir + "/" + e.Path
	}
	return m
}

// docOutPath is the catalog-declared output path for a managed doc.
func (p *Project) docOutPath(name string) string {
	e := p.Cat.Docs[name]
	if e.AgentsDoc {
		return "AGENTS.md"
	}
	path := e.Path
	if path == "" {
		path = name + ".md"
	}
	return config.DocsDir + "/" + path
}

// decisionsDir is the absolute ADR decisions directory.
func (p *Project) decisionsDir() string {
	return filepath.Join(p.Root, config.DocsDir, "decisions")
}

// resolvedDocs builds the non-singleton Document-map entries for the agents-doc
// template from the full catalog, annotated with title and description.
func (p *Project) resolvedDocs() []map[string]any {
	out := []map[string]any{}
	var names []string
	for name, e := range p.Cat.Docs {
		if !e.AgentsDoc && e.Path == "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		spec := p.Cat.Docs[name]
		out = append(out, map[string]any{
			"name":  name,
			"title": spec.Title,
			"desc":  spec.Desc,
			"path":  p.docOutPath(name),
		})
	}
	return out
}

// documentMapDocs builds the AGENTS.md document-map entries for the mandatory
// DocumentMap docs from the catalog's title/desc, sorted by name (ADR-0062).
// Unlike resolvedDocs it is UNCONDITIONAL - a mandatory doc-map line renders
// from the complete catalog, matching the historically hardcoded lines.
func (p *Project) documentMapDocs() []map[string]any {
	d := config.DocsDir
	var names []string
	for name, e := range p.Cat.Docs {
		if e.DocumentMap {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	out := make([]map[string]any, 0, len(names))
	for _, name := range names {
		e := p.Cat.Docs[name]
		out = append(out, map[string]any{
			"title": e.Title,
			"desc":  e.Desc,
			"path":  d + "/" + e.Path,
		})
	}
	return out
}
