package project

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/artifactregistry"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
)

// kindDescriptor is the project adapter for one canonical registry kind.
type kindDescriptor struct {
	Plural         string
	Singular       string
	freeformDomain bool
	poolNames      func(*catalog.Catalog) []string
	sections       func(*catalog.Catalog, string) ([]string, bool)
	outPath        func(t artifactregistry.Target, prefix, name string) string
	templateID     func(*catalog.Catalog, string) string
}

func projectKindDescriptor(kind artifactregistry.Kind) kindDescriptor {
	descriptor := kindDescriptor{
		Plural: kind.Plural, Singular: kind.Singular,
		freeformDomain: kind.Cardinality == artifactregistry.CardinalityFreeform,
		sections: func(cat *catalog.Catalog, name string) ([]string, bool) {
			return artifactregistry.Sections(cat, kind.Plural, name)
		},
		templateID: func(cat *catalog.Catalog, name string) string {
			return artifactregistry.TemplateID(cat, kind.Plural, name)
		},
	}
	if kind.Cardinality != artifactregistry.CardinalityFreeform {
		descriptor.poolNames = func(cat *catalog.Catalog) []string {
			names, _ := artifactregistry.Names(cat, kind.Plural)
			return names
		}
	}
	if kind.Targeting == artifactregistry.TargetAdapter {
		descriptor.outPath = func(target artifactregistry.Target, prefix, name string) string {
			return artifactregistry.OutputPath(catlessCatalog, target, prefix, kind.Plural, name)
		}
	}
	return descriptor
}

// catlessCatalog is safe for target-scoped path projections, which do not read catalog data.
var catlessCatalog = &catalog.Catalog{}

func allKindDescriptors() []kindDescriptor {
	kinds := artifactregistry.Kinds()
	out := make([]kindDescriptor, len(kinds))
	for i, kind := range kinds {
		out[i] = projectKindDescriptor(kind)
	}
	return out
}

func descriptorByPlural(kind string) (kindDescriptor, bool) {
	declaration, ok := artifactregistry.KindByPlural(kind)
	if !ok {
		return kindDescriptor{}, false
	}
	return projectKindDescriptor(declaration), true
}

func descriptorBySingular(kind string) (kindDescriptor, bool) {
	declaration, ok := artifactregistry.KindBySingular(kind)
	if !ok {
		return kindDescriptor{}, false
	}
	return projectKindDescriptor(declaration), true
}

// Kinds returns the singular CLI kind tokens in display order.
func Kinds() []string {
	kinds := artifactregistry.Kinds()
	out := make([]string, len(kinds))
	for i, kind := range kinds {
		out[i] = kind.Singular
	}
	return out
}

// PluralKind maps a singular CLI kind token to its descriptor plural.
func PluralKind(singular string) (string, bool) {
	d, ok := artifactregistry.KindBySingular(singular)
	return d.Plural, ok
}

// CatalogNames returns the catalog pool for a singular CLI kind.
func CatalogNames(cat *catalog.Catalog, singular string) ([]string, bool) {
	d, ok := artifactregistry.KindBySingular(singular)
	if !ok {
		return nil, false
	}
	return artifactregistry.Names(cat, d.Plural)
}

// IsFreeformDomainKind reports whether the kind has freeform cardinality.
func IsFreeformDomainKind(singular string) bool {
	d, ok := artifactregistry.KindBySingular(singular)
	return ok && d.Cardinality == artifactregistry.CardinalityFreeform
}

// AuthoringTarget is the semantic resolution of one authorable part.
type AuthoringTarget struct {
	Kind, Name, Part string
	SourcePath       string
	Local            bool
}

// ResolveSidecarTarget resolves a capability-valid leaf and its configuration-owned path.
func ResolveSidecarTarget(s *Session, cfg *config.Config, kind, name, field string) (AuthoringTarget, error) {
	if s == nil || s.selected == nil || cfg == nil {
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

func ResolveAuthoringTarget(s *Session, cfg *config.Config, kind, name, part string) (AuthoringTarget, error) {
	if s == nil || s.selected == nil || cfg == nil {
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
