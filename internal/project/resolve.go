package project

import (
	"slices"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

// PlanOp is one enable-array change in a resolver plan (ADR-0081 Decision 2).
// RequiredBy carries provenance: the artifact demanding the op ("" for the
// node the user named).
type PlanOp struct {
	Node       catalog.Node
	Add        bool
	RequiredBy string
}

// ResolveAdd plans enabling (kind, name): the node plus its missing forward
// closure. An already-enabled dependency is skipped along with its subtree —
// the open-time validation invariant guarantees enabled implies closed.
// invariant: add-applies-closure-plan
func (p *Project) ResolveAdd(kind, name string) []PlanOp {
	type item struct {
		n  catalog.Node
		by string
	}
	seed := catalog.Node{Kind: kind, Name: name}
	seen := map[catalog.Node]bool{seed: true}
	queue := []item{{seed, ""}}
	var plan []PlanOp
	for len(queue) > 0 {
		it := queue[0]
		queue = queue[1:]
		if it.n != seed && p.nodeEnabled(it.n) {
			continue
		}
		plan = append(plan, PlanOp{Node: it.n, Add: true, RequiredBy: it.by})
		for _, r := range catalog.RequiresOf(p.Cat, it.n) {
			if !seen[r] {
				seen[r] = true
				queue = append(queue, item{r, it.n.Name})
			}
		}
	}
	return plan
}

// ResolveRemove plans disabling (kind, name): the node plus every enabled,
// non-local artifact that transitively requires it (reverse closure, fixed
// point over direct edges). Local-sidecar artifacts have no catalog edges
// demanded of them, mirroring the validator's skip.
// invariant: remove-refuses-dependents
func (p *Project) ResolveRemove(kind, name string) []PlanOp {
	target := catalog.Node{Kind: kind, Name: name}
	removed := map[catalog.Node]bool{target: true}
	plan := []PlanOp{{Node: target, Add: false}}
	for changed := true; changed; {
		changed = false
		for _, n := range p.enabledGraphNodes() {
			if removed[n] {
				continue
			}
			for _, r := range catalog.RequiresOf(p.Cat, n) {
				if removed[r] {
					removed[n] = true
					plan = append(plan, PlanOp{Node: n, Add: false, RequiredBy: r.Name})
					changed = true
					break
				}
			}
		}
	}
	return plan
}

// enabledGraphNodes returns the enabled skills and agents that carry catalog
// edges — non-local only, mirroring validate's skip. Docs are sinks and
// never depend on anything.
func (p *Project) enabledGraphNodes() []catalog.Node {
	var out []catalog.Node
	for _, name := range slices.Sorted(slices.Values(p.Cfg.Skills)) {
		if sc, err := p.Cfg.Sidecar("skills", name); err == nil && !sc.Local {
			out = append(out, catalog.Node{Kind: "skill", Name: name})
		}
	}
	for _, name := range slices.Sorted(slices.Values(p.Cfg.Agents)) {
		if sc, err := p.Cfg.Sidecar("agents", name); err == nil && !sc.Local {
			out = append(out, catalog.Node{Kind: "agent", Name: name})
		}
	}
	return out
}
