// Package contextinput owns the neutral immutable semantic input consumed by context queries.
package contextinput

import (
	"maps"
	"slices"
	"sort"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

// Layout is the fixed documentation layout needed to project context results.
type Layout struct {
	DocsDir, ADRDir, IndexMd, PlansDir, DomainsDir string
	Docs, Singletons                               map[string]string
}

// PlanReference identifies one plan linked to an ADR.
type PlanReference struct{ Path, Filename string }

// PlanContext carries parsed plans and their resolved reverse ADR links.
type PlanContext struct {
	Plans []plan.Plan
	byADR map[string][]PlanReference
}

// NewPlanContext builds a defensive reverse link projection from already parsed plans.
func NewPlanContext(plans []plan.Plan, corpus adr.Corpus) PlanContext {
	links := map[string][]PlanReference{}
	seen := map[string]map[string]bool{}
	for _, p := range plans {
		if p.Format != "plan-v2" {
			continue
		}
		for _, link := range p.ADRs {
			record, ok := corpus.ByIdentity(link.Identity())
			if !ok {
				continue
			}
			identity := record.Identity()
			if seen[identity] == nil {
				seen[identity] = map[string]bool{}
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
	return PlanContext{Plans: clonePlans(plans), byADR: links}
}

// LinkedPlans returns a defensive path projection.
func (c PlanContext) LinkedPlans(identity string) []string {
	refs := c.byADR[identity]
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		out = append(out, ref.Path)
	}
	return out
}

// Input is the complete, immutable semantic universe consumed by contextq.
type Input struct {
	Layout        Layout
	Loaded        currentstate.Loaded
	PlanState     PlanContext
	Tree          *snapshot.Tree
	Lock          *manifest.Lock
	Declarations  []outputplan.Declaration
	Eligible      []string
	ContextIgnore []string
}

// New defensively retains one selected semantic universe.
func New(layout Layout, loaded currentstate.Loaded, plans PlanContext, tree *snapshot.Tree, lock *manifest.Lock, declarations []outputplan.Declaration, eligible, ignores []string) Input {
	return Input{Layout: cloneLayout(layout), Loaded: cloneLoaded(loaded), PlanState: PlanContext{Plans: clonePlans(plans.Plans), byADR: cloneLinks(plans.byADR)}, Tree: tree, Lock: cloneLock(lock), Declarations: slices.Clone(declarations), Eligible: slices.Clone(eligible), ContextIgnore: slices.Clone(ignores)}
}

// Clone returns an independent semantic input. The selected Tree is deliberately
// shared because snapshot.Tree exposes immutable values only.
func (v Input) Clone() Input {
	return New(v.Layout, v.Loaded, v.PlanState, v.Tree, v.Lock, v.Declarations, v.Eligible, v.ContextIgnore)
}
func cloneLayout(v Layout) Layout {
	v.Docs = maps.Clone(v.Docs)
	v.Singletons = maps.Clone(v.Singletons)
	return v
}
func cloneLock(in *manifest.Lock) *manifest.Lock {
	if in == nil {
		return nil
	}
	out := *in
	out.Files = maps.Clone(in.Files)
	if in.BridgeAttestation != nil {
		bridge := *in.BridgeAttestation
		bridge.LegacyADRGaps = slices.Clone(in.BridgeAttestation.LegacyADRGaps)
		out.BridgeAttestation = &bridge
	}
	return &out
}
func cloneLinks(in map[string][]PlanReference) map[string][]PlanReference {
	out := map[string][]PlanReference{}
	for k, v := range in {
		out[k] = slices.Clone(v)
	}
	return out
}
func cloneLoaded(v currentstate.Loaded) currentstate.Loaded {
	out := v
	out.ADRs = slices.Clone(v.ADRs)
	out.Corpus = v.Corpus.Clone()
	out.Topics = v.Topics.Clone()
	out.Sources = map[string][]byte{}
	for k, b := range v.Sources {
		out.Sources[k] = slices.Clone(b)
	}
	return out
}
func clonePlans(in []plan.Plan) []plan.Plan {
	out := slices.Clone(in)
	for i := range out {
		out[i].ADRs = slices.Clone(out[i].ADRs)
		out[i].Source = slices.Clone(out[i].Source)
		out[i].Phases = slices.Clone(out[i].Phases)
		for phase := range out[i].Phases {
			out[i].Phases[phase].Tasks = slices.Clone(out[i].Phases[phase].Tasks)
			out[i].Phases[phase].Advances = slices.Clone(out[i].Phases[phase].Advances)
			out[i].Phases[phase].Completes = slices.Clone(out[i].Phases[phase].Completes)
			for task := range out[i].Phases[phase].Tasks {
				fields := &out[i].Phases[phase].Tasks[task].Fields
				fields.Paths = slices.Clone(fields.Paths)
				fields.Applying = slices.Clone(fields.Applying)
				fields.Context = slices.Clone(fields.Context)
			}
		}
		out[i].DoD = slices.Clone(out[i].DoD)
		out[i].CommitSubjects = slices.Clone(out[i].CommitSubjects)
	}
	return out
}
