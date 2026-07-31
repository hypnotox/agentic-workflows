package project

import (
	"strings"

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
	bridgeTID     = "claude/CLAUDE.md.tmpl"
	bootstrapTID  = "bootstrap/awf-bootstrap.sh.tmpl"
	upgradeTID    = "bootstrap/awf-upgrade.sh.tmpl"
	runnerTID     = "runner/awf.tmpl"
	topicTID      = "topics/topic.md.tmpl"
	topicIndexTID = "topics/index.md.tmpl"

	// coOwnedRunnerTID is the legacy co-owned command-runner template id
	// (ADR-0101 shape). The prune backup matches it on the OUTGOING lock entry,
	// so the value stays this historic id no matter where the runner render unit
	// moves later (ADR-0156 Decision item 9).
	coOwnedRunnerTID = "runner/x.tmpl"

	// residentGitignoreTIDSuffix completes a resident root's template id.
	// internal/resident owns the closed root-name set and never spells a
	// template id; the identity of each root's one governed .gitignore is
	// core's, derived here from the name so a new root needs no second edit.
	residentGitignoreTIDSuffix = "/gitignore.tmpl"
)

// hookTID returns the embedded template id of a git-hook payload script
// (ADR-0048), derived from the payload name in hookNames.
func hookTID(name string) string { return "hooks/" + name + ".sh.tmpl" }

// residentGitignoreTID returns the template id that renders the one governed
// .gitignore of the named resident root (internal/resident owns the names).
func residentGitignoreTID(name string) string { return name + residentGitignoreTIDSuffix }

// isResidentGitignoreTID reports whether a template id is a resident root's
// governed .gitignore - the outputs whose provenance banner is a `#` comment.
func isResidentGitignoreTID(tid string) bool {
	return strings.HasSuffix(tid, residentGitignoreTIDSuffix)
}

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
