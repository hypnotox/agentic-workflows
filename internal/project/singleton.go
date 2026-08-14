package project

import (
	"maps"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/resident"
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

// plainSingletons is derived from the project-owned catalog view (ADR-0061
// inv: unified-doc-model): one entry per Mandatory non-agents-doc doc, with
// tid / output path / sections read from that DocEntry. There is no hand-authored
// table - adding a mandatory doc is one DocEntry and this loop picks it up.
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

// conditionalUnit is the bounded shared declaration for config-tree outputs.
// It deliberately excludes policy, encoding, and data construction: those stay
// explicit at the render seam, while selection, path, identity, kind, and
// fixed section facts cannot drift between declaration and dispatch.
type conditionalUnit struct {
	enabled  func(*config.Config) bool
	path     string
	tid      string
	kind     string
	sections []string
}

// liveTemplateEncoders derives every embedded identity that can participate in
// render authority together with its declared representation. Recognition-only
// identities are intentionally absent. This keeps structural parsing and the
// exhaustive census on the same declaration owners as output planning.
func (p *Project) liveTemplateEncoders() map[string]AgentDialect {
	encoders := map[string]AgentDialect{topicTID: MarkdownAgentDialect, topicIndexTID: MarkdownAgentDialect, pitfallEntryTID: MarkdownAgentDialect}
	if len(p.Cfg.LocalDocs) != 0 {
		encoders[localDocTID] = MarkdownAgentDialect
	}
	for _, descriptor := range kindDescriptors {
		if descriptor.freeformDomain {
			encoders[descriptor.tid("")] = MarkdownAgentDialect
		}
	}
	for name := range p.catalog().Skills {
		encoders[p.skillTID(name)] = MarkdownAgentDialect
	}
	for name := range p.catalog().Agents {
		encoders[p.agentTID(name)] = MarkdownAgentDialect
	}
	for _, entry := range p.catalog().Docs {
		encoders[entry.TID] = MarkdownAgentDialect
	}
	for _, target := range p.Targets {
		if target.BridgeTemplate != "" {
			encoders[target.BridgeTemplate] = MarkdownAgentDialect
		}
		for _, output := range target.Outputs {
			encoders[output.TemplateID] = output.Encoder
		}
	}
	for _, unit := range conditionalUnits() {
		encoders[unit.tid] = PlainAgentDialect
	}
	for _, root := range resident.RootNames() {
		encoders[residentGitignoreTID(root)] = PlainAgentDialect
	}
	return encoders
}

// liveTemplateIDs is the identity-only projection used by completeness checks.
func (p *Project) liveTemplateIDs() map[string]bool {
	ids := map[string]bool{}
	for tid := range p.liveTemplateEncoders() {
		ids[tid] = true
	}
	return ids
}

func conditionalUnits() []conditionalUnit {
	units := []conditionalUnit{
		{func(c *config.Config) bool { return c.Bootstrap != nil && c.Bootstrap.Enabled }, config.DirName + "/bootstrap.sh", bootstrapTID, "bootstrap", nil},
		{func(c *config.Config) bool { return c.Bootstrap != nil && c.Bootstrap.Enabled }, config.DirName + "/upgrade.sh", upgradeTID, "bootstrap", nil},
		{func(*config.Config) bool { return true }, "awf", runnerTID, "runner", runnerSections},
	}
	for _, name := range hookNames {
		units = append(units, conditionalUnit{func(*config.Config) bool { return true }, config.DirName + "/hooks/" + name + ".sh", hookTID(name), "hooks", nil})
	}
	return units
}
