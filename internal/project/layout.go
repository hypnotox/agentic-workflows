package project

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

// Layout is the fixed, awf-given docs layout derived from cfg.DocsDir, in typed
// form for Go consumers. These paths are not configurable through vars.
// templateMap projects it into the .layout template namespace (templates read a
// map, not unexported struct fields) and into the per-file ConfigHash. The
// mandatory-singleton paths are not struct fields: they derive from the catalog
// doc collection in templateMap (ADR-0061).
type Layout struct {
	DocsDir    string
	ADRDir     string
	ActiveMd   string
	IndexMd    string
	PlansDir   string
	Docs       map[string]string // name -> output path; present iff enabled (inv: layout-docs-enabled-only)
	DomainsDir string
}

func (p *Project) layout() Layout {
	d := strings.TrimRight(p.Cfg.DocsDir, "/")
	dec := d + "/decisions"
	// Docs maps every enabled doc name to its output path. Local docs are included:
	// the file still exists at that path and is citable.
	docs := map[string]string{}
	for _, name := range p.Cfg.Docs {
		docs[name] = p.docOutPath(name)
	}
	return Layout{
		DocsDir:    d,
		ADRDir:     dec,
		ActiveMd:   dec + "/ACTIVE.md",
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
		"activeMd":   l.ActiveMd,
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

// docOutPath is the output path for a managed doc, rooted at docsDir.
func (p *Project) docOutPath(name string) string {
	return strings.TrimRight(p.Cfg.DocsDir, "/") + "/" + name + ".md"
}

// decisionsDir is the absolute ADR decisions directory.
func (p *Project) decisionsDir() string {
	return filepath.Join(p.Root, p.Cfg.DocsDir, "decisions")
}

// resolvedDocs builds the Document-map entries for the agents-doc template from
// the docs declared in config, annotated with the catalog's title/desc. Local
// docs are excluded.
func (p *Project) resolvedDocs() ([]map[string]any, error) {
	out := []map[string]any{}
	names := append([]string(nil), p.Cfg.Docs...)
	sort.Strings(names)
	for _, name := range names {
		sc, err := p.Cfg.Sidecar("docs", name)
		if err != nil {
			return nil, err
		}
		if sc.Local {
			continue
		}
		spec := p.Cat.Docs[name]
		out = append(out, map[string]any{
			"name":  name,
			"title": spec.Title,
			"desc":  spec.Desc,
			"path":  p.docOutPath(name),
		})
	}
	return out, nil
}

// documentMapDocs builds the AGENTS.md document-map entries for the mandatory
// DocumentMap docs from the catalog's title/desc, sorted by name (ADR-0062).
// Unlike resolvedDocs it is UNCONDITIONAL - a mandatory doc-map line renders
// regardless of a local: sidecar, matching the historically hardcoded lines.
func (p *Project) documentMapDocs() []map[string]any {
	d := strings.TrimRight(p.Cfg.DocsDir, "/")
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
