package project

import "github.com/hypnotox/agentic-workflows/internal/config"

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
