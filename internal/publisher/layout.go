package publisher

import (
	"sort"
	"strings"

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
	Docs       map[string]string // catalog name -> output path (inv: layout-docs-full-catalog)
	Singletons map[string]string // template key -> output path
	DomainsDir string
}

func layout(p renderInputs) Layout {
	d := config.DocsDir
	// Docs maps every catalog document to its unconditional rendered output path.
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
	return Layout{
		DocsDir:    d,
		Docs:       docs,
		Singletons: singletons,
		DomainsDir: d + "/domains", // inv: domains-dir-given
	}
}

// templateMap projects the layout into the map the .layout template namespace and
// the per-file ConfigHash consume. The fixed directory/generated keys are set
// here; mandatory singleton keys derive from the
// catalog doc collection - each entry's TemplateKey at docsDir/Path - so the map
// reproduces the historical key set and values byte-for-byte (ADR-0061).
func (l Layout) templateMap() map[string]any {
	docs := map[string]any{}
	for k, v := range l.Docs {
		docs[k] = v
	}
	m := map[string]any{
		"docsDir":    l.DocsDir,
		"docs":       docs,
		"domainsDir": l.DomainsDir,
	}
	for key, output := range l.Singletons {
		m[key] = output
	}
	return m
}

// docOutPath is the catalog-declared output path for a managed doc.
func docOutPath(p renderInputs, name string) string {
	e := projectCatalog(p).Docs[name]
	if e.AgentsDoc {
		return "AGENTS.md"
	}
	path := e.Path
	if path == "" {
		path = name + ".md"
	}
	return config.DocsDir + "/" + path
}

// resolvedDocs builds the non-singleton Document-map entries for the agents-doc
// template from the full catalog, annotated with title and description.
func resolvedDocs(p renderInputs) []map[string]any {
	out := []map[string]any{}
	var names []string
	for name, e := range projectCatalog(p).Docs {
		if !e.AgentsDoc && e.Path == "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		spec := projectCatalog(p).Docs[name]
		out = append(out, map[string]any{
			"name":  name,
			"title": spec.Title,
			"desc":  spec.Desc,
			"path":  docOutPath(p, name),
		})
	}
	return out
}

// documentMapDocs builds the catalog portion of the AGENTS.md map. Layout.Docs
// remains catalog-only; local documents are a separate output family.
func localDocsProjection(docs []config.LocalDoc) string {
	projection := make([]string, 0, len(docs)*3)
	for _, doc := range docs {
		projection = append(projection, doc.Name, doc.Title, doc.Description)
	}
	return strings.Join(projection, "\x00")
}

func documentMapDocs(p renderInputs) []map[string]any {
	d := config.DocsDir
	var names []string
	for name, e := range projectCatalog(p).Docs {
		if e.DocumentMap {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	out := make([]map[string]any, 0, len(names))
	for _, name := range names {
		e := projectCatalog(p).Docs[name]
		out = append(out, map[string]any{
			"name": name, "title": e.Title, "desc": e.Desc, "path": d + "/" + e.Path,
		})
	}
	return out
}

// localDocumentMapDocs projects configured local documents in normalized name
// order for the explicit agent-guide document-map union.
func localDocumentMapDocs(p renderInputs) []map[string]any {
	locals := p.cfg.NormalizedLocalDocs()
	out := make([]map[string]any, 0, len(locals))
	for _, local := range locals {
		out = append(out, map[string]any{
			"name": local.Name, "title": local.Title, "desc": local.Description,
			"path": config.DocsDir + "/" + local.Name + ".md",
		})
	}
	return out
}
