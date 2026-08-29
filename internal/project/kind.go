package project

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
)

// kindDescriptor resolves the per-kind facets the dispatch sites share. Facets are
// accessor funcs so one table absorbs the catalog map-vs-slice and adapter-vs-neutral
// path asymmetries (ADR-0027). A nil/"" facet means "absent" for that kind.
type kindDescriptor struct {
	Plural         string
	Singular       string
	freeformDomain bool                                            // freeform names, no catalog pool (domains)
	poolNames      func(*catalog.Catalog) []string                 // sorted catalog pool; nil for domains (no pool)
	sections       func(*catalog.Catalog, string) ([]string, bool) // declared sections + catalog presence
	outPath        func(t Target, prefix, name string) string      // rendered path; nil for neutral kinds
	templateID     func(*catalog.Catalog, string) string           // embedded template id
}

// kindDescriptors is the single ordered source of per-kind dispatch (inv:
// kind-dispatch-single-table), in `awf list` display order. It is also the sole
// enumeration of CLI-addressable artifact kinds and their dispatch facets.
var kindDescriptors = []kindDescriptor{
	{
		Plural: "skills", Singular: "skill",
		poolNames:  func(c *catalog.Catalog) []string { return slices.Sorted(maps.Keys(c.Skills)) },
		sections:   func(c *catalog.Catalog, n string) ([]string, bool) { s, ok := c.Skills[n]; return s.Sections, ok },
		outPath:    func(t Target, prefix, n string) string { return t.SkillPath(prefix, n) },
		templateID: func(_ *catalog.Catalog, n string) string { return fmt.Sprintf("skills/%s/SKILL.md.tmpl", n) },
	},
	{
		Plural: "agents", Singular: "agent",
		poolNames:  func(c *catalog.Catalog) []string { return slices.Sorted(maps.Keys(c.Agents)) },
		sections:   func(c *catalog.Catalog, n string) ([]string, bool) { a, ok := c.Agents[n]; return a.Sections, ok },
		outPath:    func(t Target, _, n string) string { return t.AgentPath(n) },
		templateID: func(_ *catalog.Catalog, n string) string { return fmt.Sprintf("agents/%s.md.tmpl", n) },
	},
	{
		Plural: "docs", Singular: "doc",
		poolNames:  func(c *catalog.Catalog) []string { return slices.Sorted(maps.Keys(c.Docs)) },
		sections:   func(c *catalog.Catalog, n string) ([]string, bool) { d, ok := c.Docs[n]; return d.Sections, ok },
		outPath:    nil,
		templateID: func(c *catalog.Catalog, n string) string { return c.Docs[n].TID },
	},
	{
		Plural: "domains", Singular: "domain", freeformDomain: true,
		poolNames:  nil, // freeform - no catalog pool
		sections:   func(c *catalog.Catalog, _ string) ([]string, bool) { return c.DomainDoc.Sections, false },
		outPath:    nil,
		templateID: func(*catalog.Catalog, string) string { return "domains/domain.md.tmpl" },
	},
}

func descriptorByPlural(kind string) (kindDescriptor, bool) {
	for _, d := range kindDescriptors {
		if d.Plural == kind {
			return d, true
		}
	}
	return kindDescriptor{}, false
}

func descriptorBySingular(kind string) (kindDescriptor, bool) {
	for _, d := range kindDescriptors {
		if d.Singular == kind {
			return d, true
		}
	}
	return kindDescriptor{}, false
}

// Kinds returns the singular CLI kind tokens in display order.
func Kinds() []string {
	out := make([]string, len(kindDescriptors))
	for i, d := range kindDescriptors {
		out[i] = d.Singular
	}
	return out
}

// PluralKind maps a singular CLI kind token to its descriptor plural.
func PluralKind(singular string) (string, bool) {
	d, ok := descriptorBySingular(singular)
	return d.Plural, ok
}

// CatalogNames returns the catalog pool for a singular CLI kind; ok is false for a
// kind with no catalog pool (domains).
func CatalogNames(cat *catalog.Catalog, singular string) ([]string, bool) {
	d, ok := descriptorBySingular(singular)
	if !ok || d.poolNames == nil {
		return nil, false
	}
	return d.poolNames(cat), true
}

// IsFreeformDomainKind reports whether the singular CLI kind is the freeform
// domains kind (no catalog pool).
func IsFreeformDomainKind(singular string) bool {
	d, ok := descriptorBySingular(singular)
	return ok && d.freeformDomain
}

// AuthoringTarget is the semantic resolution of one authorable part. SourcePath
// is project-relative and is either a convention part or, for Local, the
// configured document output whose synthetic body is the authored source.
type AuthoringTarget struct {
	Kind, Name, Part string
	SourcePath       string
	Local            bool
}

// ResolveAuthoringTarget resolves a singular kind, semantic artifact name, and
// declared part against the selected project catalog and configuration without
// widening ProjectState's compatibility facade.
// ResolveSidecarTarget resolves a capability-valid leaf and its configuration-owned path.
func ResolveSidecarTarget(s *ProjectState, cfg *config.Config, kind, name, field string) (AuthoringTarget, error) {
	if s == nil || s.state == nil || cfg == nil {
		return AuthoringTarget{}, fmt.Errorf("project: missing authoring authority")
	}
	d, ok := descriptorBySingular(kind)
	if !ok {
		return AuthoringTarget{}, fmt.Errorf("unknown artifact kind %q; expected one of %v", kind, Kinds())
	}
	if d.freeformDomain {
		if !slices.Contains(cfg.Domains, name) {
			return AuthoringTarget{}, fmt.Errorf("domain %q is not configured", name)
		}
	} else if _, found := d.sections(s.catalog(), name); !found {
		return AuthoringTarget{}, fmt.Errorf("%s %q is not present in the selected catalog", kind, name)
	}
	parts := strings.Split(field, ".")
	valid := false
	switch {
	case len(parts) == 2 && parts[0] == "data":
		valid = parts[1] != "" && kind != "domain"
	case len(parts) == 2 && parts[0] == "dataDefaults":
		valid = parts[1] != "" && kind != "domain"
	case len(parts) == 3 && parts[0] == "sections" && parts[2] == "drop":
		sections, _ := d.sections(s.catalog(), name)
		valid = slices.Contains(sections, parts[1]) && kind != "domain"
	case len(parts) == 1 && parts[0] == "paths":
		valid = kind == "domain"
	}
	if !valid {
		return AuthoringTarget{}, fmt.Errorf("sidecar field %q is not authorable for %s %q", field, kind, name)
	}
	sidecarKind, sidecarName := d.Plural, name
	if kind == "doc" && s.catalog().Docs[name].Mandatory {
		sidecarKind, sidecarName = name, ""
	}
	rel := filepath.ToSlash(filepath.Join(config.DirName, cfg.SidecarPath(sidecarKind, sidecarName)))
	return AuthoringTarget{Kind: kind, Name: name, Part: field, SourcePath: rel}, nil
}

func ResolveAuthoringTarget(s *ProjectState, cfg *config.Config, kind, name, part string) (AuthoringTarget, error) {
	if s == nil || s.state == nil || cfg == nil {
		return AuthoringTarget{}, fmt.Errorf("project: missing authoring authority")
	}
	descriptor, ok := descriptorBySingular(kind)
	if !ok {
		return AuthoringTarget{}, fmt.Errorf("unknown artifact kind %q; expected one of %v", kind, Kinds())
	}
	if kind == "doc" {
		for _, local := range cfg.NormalizedLocalDocs() {
			if local.Name != name {
				continue
			}
			if part != "body" {
				return AuthoringTarget{}, fmt.Errorf("configured local document %q exposes only part body", name)
			}
			return AuthoringTarget{Kind: kind, Name: name, Part: part, SourcePath: filepath.ToSlash(filepath.Join(config.DocsDir, name+".md")), Local: true}, nil
		}
	}
	if descriptor.freeformDomain {
		if !slices.Contains(cfg.Domains, name) {
			return AuthoringTarget{}, fmt.Errorf("domain %q is not configured", name)
		}
	} else if _, found := descriptor.sections(s.catalog(), name); !found {
		return AuthoringTarget{}, fmt.Errorf("%s %q is not present in the selected catalog", kind, name)
	}
	sections, _ := descriptor.sections(s.catalog(), name)
	if !slices.Contains(sections, part) {
		return AuthoringTarget{}, fmt.Errorf("part %q is not declared for %s %q", part, kind, name)
	}
	absolute := cfg.PartPath(descriptor.Plural, name, part)
	relative, err := filepath.Rel(s.Root(), absolute)
	if err != nil {
		return AuthoringTarget{}, fmt.Errorf("resolve convention part path: %w", err)
	}
	return AuthoringTarget{Kind: kind, Name: name, Part: part, SourcePath: filepath.ToSlash(relative)}, nil
}
