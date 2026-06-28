package project

import (
	"path/filepath"
	"sort"
	"strings"
)

// Layout is the fixed, awf-given docs layout derived from cfg.DocsDir, in typed
// form for Go consumers. These paths are not configurable through vars.
// templateMap projects it into the .layout template namespace (templates read a
// map, not unexported struct fields) and into the per-file ConfigHash.
type Layout struct {
	DocsDir     string
	ADRDir      string
	ActiveMd    string
	AdrReadme   string
	AdrTemplate string
	PlansDir    string
	PlansReadme string
	Docs        map[string]string // name -> output path; present iff enabled (inv: layout-docs-enabled-only)
	WorkflowRef string
	DomainsDir  string
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
	// WorkflowRef is the workflow doc's path when enabled, else AGENTS.md, so the
	// ~always-cited workflow reference always resolves (inv: workflow-ref-fallback).
	workflowRef := "AGENTS.md"
	if wp, ok := docs["workflow"]; ok {
		workflowRef = wp
	}
	return Layout{
		DocsDir:     d,
		ADRDir:      dec,
		ActiveMd:    dec + "/ACTIVE.md",
		AdrReadme:   dec + "/README.md",
		AdrTemplate: dec + "/template.md",
		PlansDir:    d + "/plans",
		PlansReadme: d + "/plans/README.md",
		Docs:        docs,
		WorkflowRef: workflowRef,
		DomainsDir:  d + "/domains", // inv: domains-dir-given
	}
}

// templateMap projects the layout into the map the .layout template namespace and
// the per-file ConfigHash consume, reproducing the historical layout() map exactly
// so the ConfigHash stays byte-identical (no drift).
func (l Layout) templateMap() map[string]any {
	docs := map[string]any{}
	for k, v := range l.Docs {
		docs[k] = v
	}
	return map[string]any{
		"docsDir":     l.DocsDir,
		"adrDir":      l.ADRDir,
		"activeMd":    l.ActiveMd,
		"adrReadme":   l.AdrReadme,
		"adrTemplate": l.AdrTemplate,
		"plansDir":    l.PlansDir,
		"plansReadme": l.PlansReadme,
		"docs":        docs,
		"workflowRef": l.WorkflowRef,
		"domainsDir":  l.DomainsDir,
	}
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
