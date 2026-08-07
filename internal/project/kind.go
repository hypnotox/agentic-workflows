package project

import (
	"fmt"
	"maps"
	"slices"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
)

// kindDescriptor resolves the per-kind facets the dispatch sites share. Facets are
// accessor funcs so one table absorbs the catalog map-vs-slice and adapter-vs-neutral
// path asymmetries (ADR-0027). A nil/"" facet means "absent" for that kind.
type kindDescriptor struct {
	Plural         string
	Singular       string
	graphKind      bool                                            // the ADR-0081 resolver plans over this kind
	freeformDomain bool                                            // freeform names, no catalog pool (domains)
	enable         func(*config.Config) []string                   // config enable array
	poolNames      func(*catalog.Catalog) []string                 // sorted catalog pool; nil for domains (no pool)
	sections       func(*catalog.Catalog, string) ([]string, bool) // declared sections + catalog presence
	outPath        func(t Target, prefix, name string) string      // rendered path; nil for neutral kinds
	tid            func(name string) string                        // embedded template id
	// baseTID is the shared base template every synthesized project-local
	// artifact of this kind renders from (ADR-0068); "" for a kind with no
	// base template (domains).
	baseTID string
}

// kindDescriptors is the single ordered source of per-kind dispatch (inv:
// kind-dispatch-single-table), in `awf list` display order. It is also the sole
// enumeration of CLI-addressable kinds (ADR-0024 inv: cli-config-kinds, re-homed here).
var kindDescriptors = []kindDescriptor{
	{
		Plural: "skills", Singular: "skill", graphKind: true,
		enable:    func(c *config.Config) []string { return c.Skills },
		poolNames: func(c *catalog.Catalog) []string { return slices.Sorted(maps.Keys(c.Skills)) },
		sections:  func(c *catalog.Catalog, n string) ([]string, bool) { s, ok := c.Skills[n]; return s.Sections, ok },
		outPath:   func(t Target, prefix, n string) string { return t.SkillPath(prefix, n) },
		tid:       func(n string) string { return fmt.Sprintf("skills/%s/SKILL.md.tmpl", n) },
		baseTID:   "skills/_base/SKILL.md.tmpl",
	},
	{
		Plural: "agents", Singular: "agent", graphKind: true,
		enable:    func(c *config.Config) []string { return c.Agents },
		poolNames: func(c *catalog.Catalog) []string { return slices.Sorted(maps.Keys(c.Agents)) },
		sections:  func(c *catalog.Catalog, n string) ([]string, bool) { a, ok := c.Agents[n]; return a.Sections, ok },
		outPath:   func(t Target, _, n string) string { return t.AgentPath(n) },
		tid:       func(n string) string { return fmt.Sprintf("agents/%s.md.tmpl", n) },
		baseTID:   "agents/_base.md.tmpl",
	},
	{
		Plural: "docs", Singular: "doc", graphKind: true,
		enable:    func(c *config.Config) []string { return c.Docs },
		poolNames: catalog.NonMandatoryDocNames,
		sections:  func(c *catalog.Catalog, n string) ([]string, bool) { d, ok := c.Docs[n]; return d.Sections, ok },
		outPath:   nil,
		// Read the entry's TID: merged-in singletons render from non-docs/ templates.
		tid:     func(n string) string { return catalog.Standard.Docs[n].TID },
		baseTID: "docs/_base.md.tmpl",
	},
	{
		Plural: "domains", Singular: "domain", freeformDomain: true,
		enable:    func(c *config.Config) []string { return c.Domains },
		poolNames: nil, // freeform - no catalog pool
		sections:  func(c *catalog.Catalog, _ string) ([]string, bool) { return c.DomainDoc.Sections, false },
		outPath:   nil,
		tid:       func(string) string { return "domains/domain.md.tmpl" },
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

// mustDescriptor returns the descriptor for a plural kind known to exist at the
// call site (static kind literals in renderAllBase).
func mustDescriptor(kind string) kindDescriptor {
	d, _ := descriptorByPlural(kind)
	return d
}

// baseTID returns the shared base template id a synthesized project-local
// artifact of the given plural kind renders from, "" for a kind without one.
// It is the table's declaration, never a second spelling at a call site.
func baseTID(kind string) string { return mustDescriptor(kind).baseTID }

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

// PluralKind maps a singular CLI kind token to its config enable-array key.
func PluralKind(singular string) (string, bool) {
	d, ok := descriptorBySingular(singular)
	return d.Plural, ok
}

// EnabledNames returns the config enable array for a singular CLI kind.
func EnabledNames(c *config.Config, singular string) ([]string, bool) {
	d, ok := descriptorBySingular(singular)
	if !ok {
		return nil, false
	}
	return d.enable(c), true
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

// IsGraphKind reports whether the singular CLI kind is one the ADR-0081
// dependency resolver plans over (a catalog-artifact kind).
func IsGraphKind(singular string) bool {
	d, ok := descriptorBySingular(singular)
	return ok && d.graphKind
}

// IsFreeformDomainKind reports whether the singular CLI kind is the freeform
// domains kind (no catalog pool).
func IsFreeformDomainKind(singular string) bool {
	d, ok := descriptorBySingular(singular)
	return ok && d.freeformDomain
}

// IsDocKind reports whether the singular CLI kind is the docs kind.
func IsDocKind(singular string) bool {
	d, ok := descriptorBySingular(singular)
	return ok && d.Plural == "docs"
}

// IsSkillKind reports whether the singular CLI kind is the skills kind.
func IsSkillKind(singular string) bool {
	d, ok := descriptorBySingular(singular)
	return ok && d.Plural == "skills"
}
