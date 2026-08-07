package project

import (
	"fmt"
	"sort"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
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
func planArtifactReport(plans []plan.Plan, corpus adr.Corpus) ([]manifest.Drift, []string) {
	var drift []manifest.Drift
	for _, p := range plans {
		if p.Format != "plan-v2" {
			continue
		}
		// Assignment and membership belong to one plan, never the corpus-wide
		// traversal. Resolve every declared link even if no task references it.
		assigned := map[string]bool{}
		allowed, missingLinks := planAllowedADRIdentities(p, corpus)
		for _, identity := range missingLinks {
			drift = append(drift, manifest.Drift{Path: p.Path, Kind: "plan-reference", Detail: fmt.Sprintf("%s adrs %q: ADR not found", p.Filename, identity)})
		}
		for _, ph := range p.Phases {
			for _, task := range ph.Tasks {
				for _, ref := range append(append([]plan.DecisionRef{}, task.Fields.Applying...), task.Fields.Context...) {
					record, ok := corpus.ByIdentity(ref.ADR)
					where := fmt.Sprintf("%s task %d.%d %s %q", p.Filename, ph.Number, task.Number, ref.Kind, ref.Authored)
					if !ok {
						drift = append(drift, manifest.Drift{Path: p.Path, Kind: "plan-reference", Detail: where + ": ADR not found"})
						continue
					}
					if ref.Kind == "Applying" && !allowed[record.Identity()] {
						drift = append(drift, manifest.Drift{Path: p.Path, Kind: "plan-reference", Detail: where + ": Applying ADR is absent from adrs"})
						continue
					}
					if ref.Kind == "Context" && record.IsContentAmendable() {
						drift = append(drift, manifest.Drift{Path: p.Path, Kind: "plan-reference", Detail: where + ": Context requires frozen ADR"})
						continue
					}
					item, err := record.LookupDecision(ref.Selector)
					if err != nil {
						drift = append(drift, manifest.Drift{Path: p.Path, Kind: "plan-reference", Detail: where + ": " + err.Error()})
						continue
					}
					if ref.Kind == "Applying" {
						assigned[item.Key] = true
					}
				}
			}
		}
		if p.IsProposed() {
			if len(p.ADRs) > 0 {
				for _, ph := range p.Phases {
					for _, task := range ph.Tasks {
						if task.Fields.Kind != plan.TaskSpike && len(task.Fields.Applying) == 0 {
							driftNote(&drift, p, ph.Number, task.Number)
						}
					}
				}
				for _, link := range p.ADRs {
					if record, ok := corpus.ByIdentity(link.Identity()); ok {
						for _, item := range record.Decisions() {
							if !assigned[item.Key] {
								drift = append(drift, manifest.Drift{Path: p.Path, Kind: "plan-advisory", Detail: fmt.Sprintf("%s Decision %s has no Applying assignment", p.Filename, item.Key)})
							}
						}
					}
				}
			}
			completed := map[string]bool{}
			advanced := map[string]bool{}
			for _, ph := range p.Phases {
				for _, slug := range ph.Advances {
					advanced[slug] = true
				}
				for _, slug := range ph.Completes {
					completed[slug] = true
				}
			}
			for _, item := range p.DoD {
				if !advanced[item.Slug] && !completed[item.Slug] {
					drift = append(drift, manifest.Drift{Path: p.Path, Kind: "plan-advisory", Detail: fmt.Sprintf("%s DoD %s has no outcome assignment", p.Filename, item.Slug)})
				} else if advanced[item.Slug] && !completed[item.Slug] {
					drift = append(drift, manifest.Drift{Path: p.Path, Kind: "plan-advisory", Detail: fmt.Sprintf("%s DoD %s is advanced but has no Completes owner", p.Filename, item.Slug)})
				}
			}
		}
	}
	// advisory markers are encoded separately to avoid blocking success.
	var notes []string
	for _, d := range drift {
		if d.Kind == "plan-advisory" {
			notes = append(notes, d.Detail)
		}
	}
	drift = filterPlanAdvisories(drift)
	sort.Strings(notes)
	return drift, notes
}
func driftNote(drift *[]manifest.Drift, p plan.Plan, phase, task int) {
	*drift = append(*drift, manifest.Drift{Path: p.Path, Kind: "plan-advisory", Detail: fmt.Sprintf("%s task %d.%d has no Applying assignment", p.Filename, phase, task)})
}
func filterPlanAdvisories(in []manifest.Drift) []manifest.Drift {
	out := in[:0]
	for _, d := range in {
		if d.Kind != "plan-advisory" {
			out = append(out, d)
		}
	}
	return out
}
func planAllowedADRIdentities(p plan.Plan, corpus adr.Corpus) (map[string]bool, []string) {
	allowed := make(map[string]bool, len(p.ADRs))
	var missing []string
	for _, link := range p.ADRs {
		record, ok := corpus.ByIdentity(link.Identity())
		if !ok {
			missing = append(missing, link.Identity())
			continue
		}
		allowed[record.Identity()] = true
	}
	return allowed, missing
}

type selectedPlanDecisionRef struct {
	ref         plan.DecisionRef
	phase, task int
}

func resolveSelectedPlanDecisions(p plan.Plan, corpus adr.Corpus, phase plan.Phase, task plan.Task) ([]plan.ResolvedDecision, []plan.ResolvedDecision, error) {
	var refs []selectedPlanDecisionRef
	appendTask := func(candidate plan.Task) {
		for _, ref := range candidate.Fields.Applying {
			refs = append(refs, selectedPlanDecisionRef{ref: ref, phase: phase.Number, task: candidate.Number})
		}
		for _, ref := range candidate.Fields.Context {
			refs = append(refs, selectedPlanDecisionRef{ref: ref, phase: phase.Number, task: candidate.Number})
		}
	}
	if task.Number != 0 {
		appendTask(task)
	} else {
		for _, candidate := range phase.Tasks {
			appendTask(candidate)
		}
	}

	allowed, _ := planAllowedADRIdentities(p, corpus)
	order := make([]string, 0, len(refs))
	resolved := make(map[string]plan.ResolvedDecision, len(refs))
	applyingKeys := make(map[string]bool, len(refs))
	for _, located := range refs {
		ref := located.ref
		prefix := fmt.Sprintf("plan %s task %d.%d %s %q", p.Filename, located.phase, located.task, ref.Kind, ref.Authored)
		record, ok := corpus.ByIdentity(ref.ADR)
		if !ok {
			return nil, nil, fmt.Errorf("%s: ADR not found", prefix)
		}
		if ref.Kind == "Applying" && !allowed[record.Identity()] {
			return nil, nil, fmt.Errorf("%s: Applying ADR is absent from adrs", prefix)
		}
		if ref.Kind == "Context" && record.IsContentAmendable() {
			return nil, nil, fmt.Errorf("%s: requires frozen ADR", prefix)
		}
		item, err := record.LookupDecision(ref.Selector)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", prefix, err)
		}
		if _, seen := resolved[item.Key]; !seen {
			order = append(order, item.Key)
			resolved[item.Key] = plan.ResolvedDecision{Key: item.Key, ADRIdentity: item.ADRIdentity, Title: item.Title, Status: item.Status, Markdown: item.Markdown}
		}
		if ref.Kind == "Applying" {
			applyingKeys[item.Key] = true
		}
	}

	var applying, context []plan.ResolvedDecision
	for _, key := range order {
		if applyingKeys[key] {
			applying = append(applying, resolved[key])
		} else {
			context = append(context, resolved[key])
		}
	}
	return applying, context, nil
}

func selectedRefs(p plan.Plan, selector string) (plan.Phase, plan.Task, error) {
	return plan.Select(p, selector)
}
