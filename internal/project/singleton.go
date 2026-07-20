package project

import "github.com/hypnotox/agentic-workflows/internal/catalog"

// singletonSpec is one plain (neutral, non-agents-doc) always-on singleton's
// render/validate identity: a kind name, its embedded template id, and accessors
// for its fixed output path and catalog sections. plainSingletons is the single
// source of truth both RenderAll (via renderKind) and validateAgainstCatalog
// range over.
type singletonSpec struct {
	kind     string
	tid      string
	outPath  func(Layout) string
	sections func(*catalog.Catalog) []string
}

// plainSingletons is derived from the catalog (ADR-0061 inv: unified-doc-model):
// one entry per Mandatory non-agents-doc doc, with tid / output path / sections
// read from that DocEntry. There is no hand-authored table - adding a mandatory
// doc is one DocEntry and this loop picks it up, so a new plain singleton cannot
// be dropped from the render/validate set by a forgotten table edit.
var plainSingletons = buildPlainSingletons()

func buildPlainSingletons() []singletonSpec {
	var out []singletonSpec
	for _, k := range catalog.SingletonKinds() {
		e := catalog.Standard.Docs[k]
		if e.AgentsDoc || e.Generated {
			continue
		}
		out = append(out, singletonSpec{
			kind:     k,
			tid:      e.TID,
			outPath:  func(l Layout) string { return l.DocsDir + "/" + e.Path },
			sections: func(*catalog.Catalog) []string { return e.Sections },
		})
	}
	return out
}
