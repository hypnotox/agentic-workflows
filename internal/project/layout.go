package project

import (
	"path/filepath"

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
	Singletons map[string]string // template key -> output path
	DomainsDir string
}

func layout(p renderInputs) Layout {
	d := config.DocsDir
	dec := d + "/decisions"
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
		ADRDir:     dec,
		IndexMd:    dec + "/INDEX.md",
		PlansDir:   d + "/plans",
		Docs:       docs,
		Singletons: singletons,
		DomainsDir: d + "/domains", // inv: domains-dir-given
	}
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

// decisionsDir is the absolute ADR decisions directory.
func decisionsDir(root string) string {
	return filepath.Join(root, config.DocsDir, "decisions")
}
