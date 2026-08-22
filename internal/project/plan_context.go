package project

import (
	"sort"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

// PlanReference identifies one plan linked to an ADR. Paths are repository-relative.
type PlanReference struct {
	Path, Filename string
}

// PlanContext carries the parsed plan snapshot and its resolved reverse ADR links.
// It is assembled once by project with the same immutable tree as the ADR corpus.
type PlanContext struct {
	Plans []plan.Plan
	byADR map[string][]PlanReference
}

// LinkedPlans returns the normalized repository-relative paths linked to identity.
func (c PlanContext) LinkedPlans(identity string) []string {
	refs := c.byADR[identity]
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		out = append(out, ref.Path)
	}
	return out
}

func planContext(plans []plan.Plan, corpus adr.Corpus) PlanContext {
	links := make(map[string][]PlanReference)
	seen := make(map[string]map[string]bool)
	for _, p := range plans {
		if p.Format != "plan-v2" {
			continue
		}
		for _, link := range p.ADRs {
			record, ok := corpus.ByIdentity(link.Identity())
			if !ok {
				continue // existing plan-reference validation blocks unresolved links
			}
			identity := record.Identity()
			if seen[identity] == nil {
				seen[identity] = make(map[string]bool)
			}
			if seen[identity][p.Path] {
				continue
			}
			seen[identity][p.Path] = true
			links[identity] = append(links[identity], PlanReference{Path: p.Path, Filename: p.Filename})
		}
	}
	for identity := range links {
		sort.Slice(links[identity], func(i, j int) bool { return links[identity][i].Path < links[identity][j].Path })
	}
	return PlanContext{Plans: plans, byADR: links}
}

func planContextFromTree(tree *snapshot.Tree, docsDir string, corpus adr.Corpus) (PlanContext, error) {
	plans, _, err := plansFromTree(tree, docsDir)
	if err != nil { // coverage-ignore: plansFromTree converts every plan parse failure into drift rather than an error
		return PlanContext{}, err
	}
	return planContext(plans, corpus), nil
}

// planArtifactReport validates plan-v2 references from one already parsed plan set.
