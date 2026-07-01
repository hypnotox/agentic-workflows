package project

import "github.com/hypnotox/agentic-workflows/internal/catalog"

// singletonSpec is one plain (neutral, non-agents-doc) always-on singleton's
// render/validate identity: a kind name, its embedded template id, and
// accessors for its fixed output path and catalog sections. plainSingletons is
// the single source of truth both RenderAll (via renderKind) and
// validateAgainstCatalog range over — adding a 7th plain singleton means
// appending one entry here, not hand-editing two separate loops (ADR-0043).
// invariant: plain-singleton-via-renderkind
type singletonSpec struct {
	kind     string
	tid      string
	outPath  func(Layout) string
	sections func(*catalog.Catalog) []string
}

var plainSingletons = []singletonSpec{
	{
		kind: "adr-readme", tid: "adr-readme/README.md.tmpl",
		outPath:  func(l Layout) string { return l.AdrReadme },
		sections: func(c *catalog.Catalog) []string { return c.Singletons["adr-readme"].Sections },
	},
	{
		kind: "adr-template", tid: "adr-template/template.md.tmpl",
		outPath:  func(l Layout) string { return l.AdrTemplate },
		sections: func(c *catalog.Catalog) []string { return c.Singletons["adr-template"].Sections },
	},
	{
		kind: "plans-readme", tid: "plans-readme/README.md.tmpl",
		outPath:  func(l Layout) string { return l.PlansReadme },
		sections: func(c *catalog.Catalog) []string { return c.Singletons["plans-readme"].Sections },
	},
	{
		kind: "workflow", tid: "docs/workflow.md.tmpl",
		outPath:  func(l Layout) string { return l.WorkflowRef },
		sections: func(c *catalog.Catalog) []string { return c.Singletons["workflow"].Sections },
	},
	{
		kind: "doc-standard", tid: "docs/doc-standard.md.tmpl",
		outPath:  func(l Layout) string { return l.DocStandard },
		sections: func(c *catalog.Catalog) []string { return c.Singletons["doc-standard"].Sections },
	},
	{
		kind: "agents-md-standard", tid: "docs/agents-md-standard.md.tmpl",
		outPath:  func(l Layout) string { return l.AgentsMdStandard },
		sections: func(c *catalog.Catalog) []string { return c.Singletons["agents-md-standard"].Sections },
	},
}
