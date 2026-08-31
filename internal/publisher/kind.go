package publisher

import (
	"fmt"
	"maps"
	"slices"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
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
	templateID     func(*catalog.Catalog, string) string           // embedded template id
}

// kindDescriptors lists CLI-addressable artifact kinds and the facets used by
// publication dispatch, in `awf list` display order.
var kindDescriptors = []kindDescriptor{
	{
		Plural: "skills", Singular: "skill",
		poolNames:  func(c *catalog.Catalog) []string { return slices.Sorted(maps.Keys(c.Skills)) },
		sections:   func(c *catalog.Catalog, n string) ([]string, bool) { s, ok := c.Skills[n]; return s.Sections, ok },
		templateID: func(_ *catalog.Catalog, n string) string { return fmt.Sprintf("skills/%s/SKILL.md.tmpl", n) },
	},
	{
		Plural: "agents", Singular: "agent",
		poolNames:  func(c *catalog.Catalog) []string { return slices.Sorted(maps.Keys(c.Agents)) },
		sections:   func(c *catalog.Catalog, n string) ([]string, bool) { a, ok := c.Agents[n]; return a.Sections, ok },
		templateID: func(_ *catalog.Catalog, n string) string { return fmt.Sprintf("agents/%s.md.tmpl", n) },
	},
	{
		Plural: "docs", Singular: "doc",
		poolNames:  func(c *catalog.Catalog) []string { return slices.Sorted(maps.Keys(c.Docs)) },
		sections:   func(c *catalog.Catalog, n string) ([]string, bool) { d, ok := c.Docs[n]; return d.Sections, ok },
		templateID: func(c *catalog.Catalog, n string) string { return c.Docs[n].TID },
	},
	{
		Plural: "domains", Singular: "domain", freeformDomain: true,
		poolNames:  nil, // freeform - no catalog pool
		sections:   func(c *catalog.Catalog, _ string) ([]string, bool) { return c.DomainDoc.Sections, false },
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

// mustDescriptor returns the descriptor for a plural kind known to exist at the
// call site (static kind literals in renderAllBase).
func mustDescriptor(kind string) kindDescriptor {
	d, _ := descriptorByPlural(kind)
	return d
}
