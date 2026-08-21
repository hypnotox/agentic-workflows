package project

import (
	"maps"
	"slices"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

// Template-ID declarations for every render unit that has no catalog DocEntry:
// the adapter bridge, the config-tree singletons (the bootstrap pair, the
// git-hook payloads, the awf wrapper), the resident-root gitignores, the topic
// doc pair, and the retired co-owned runner the prune backup still matches.
// Together with the catalog's own DocEntry TIDs (internal/catalog/standard.go),
// the kind-descriptor table (kind.go), and the target declaration table
// (target.go) these are the only production spellings of a template id
// (inv: template-id-single-derivation).
const (
	// targetBridgeKind is the neutral render identity for every descriptor-owned
	// bridge. It is not a target name and therefore has no sidecar namespace.
	targetBridgeKind = "target-bridge"
	bridgeTID        = "claude/CLAUDE.md.tmpl"
	bootstrapTID     = "bootstrap/awf-bootstrap.sh.tmpl"
	upgradeTID       = "bootstrap/awf-upgrade.sh.tmpl"
	runnerTID        = "runner/awf.tmpl"
	topicTID         = "topics/topic.md.tmpl"
	topicIndexTID    = "topics/index.md.tmpl"
	pitfallEntryTID  = "pitfalls/entry.md.tmpl"
	localDocTID      = "docs/local.md.tmpl"

	// coOwnedRunnerTID is the legacy co-owned command-runner template id
	// (ADR-0101 shape). The prune backup matches it on the OUTGOING lock entry,
	// so the value stays this historic id no matter where the runner render unit
	// moves later (ADR-0156 Decision item 9).
	coOwnedRunnerTID = "runner/x.tmpl"
)

// hookTID returns the embedded template id of a git-hook payload script
// (ADR-0048), derived from the payload name in hookNames.
func hookTID(name string) string { return "hooks/" + name + ".sh.tmpl" }

// isResidentGitignoreTID reports whether a template id is a resident root's
// governed .gitignore - the outputs whose provenance banner is a `#` comment.
// singletonSpec is one plain (neutral, non-agents-doc) always-on singleton's
// render/validate identity: a kind name, its embedded template id, and accessors
// for its fixed output path and catalog sections. plainSingletons is the single
// source of truth both renderAllBase (via renderKind) and validateAgainstCatalog
// range over.
type singletonSpec struct {
	kind     string
	tid      string
	outPath  func(Layout) string
	sections func(*catalog.Catalog) []string
}

// plainSingletons is derived from the project-owned catalog view (ADR-0061
// inv: unified-doc-model): one entry per Path-bearing, non-agent, non-generated
// structural doc, with tid, output path, and sections read from that DocEntry.
// There is no hand-authored singleton table.
func plainSingletons(cat *catalog.Catalog) []singletonSpec {
	var out []singletonSpec
	for _, k := range slices.Sorted(maps.Keys(cat.Docs)) {
		e := cat.Docs[k]
		if !e.AgentsDoc && e.Path == "" {
			continue
		}
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

var hookNames = []string{"pre-commit", "commit-msg", "pre-push", "pre-merge-commit", "reference-transaction"}
var runnerSections = []string{"runner-body"}
