package project

import (
	"fmt"
	"sort"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/plan"
)

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
		allowed := map[string]bool{}
		for _, l := range p.ADRs {
			record, ok := corpus.ByIdentity(l.Identity())
			if !ok {
				drift = append(drift, manifest.Drift{Path: p.Path, Kind: "plan-reference", Detail: fmt.Sprintf("%s adrs %q: ADR not found", p.Filename, l.Identity())})
				continue
			}
			allowed[record.Identity()] = true
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
func resolvePlanDecisions(p plan.Plan, corpus adr.Corpus, refs []plan.DecisionRef, context bool) ([]plan.ResolvedDecision, error) {
	out := make([]plan.ResolvedDecision, 0, len(refs))
	for _, ref := range refs {
		record, ok := corpus.ByIdentity(ref.ADR)
		if !ok {
			return nil, fmt.Errorf("plan %s %s %q: ADR not found", p.Filename, ref.Kind, ref.Authored)
		}
		if context && record.IsContentAmendable() {
			return nil, fmt.Errorf("plan %s Context %q: requires frozen ADR", p.Filename, ref.Authored)
		}
		item, err := record.LookupDecision(ref.Selector)
		if err != nil {
			return nil, fmt.Errorf("plan %s %s %q: %w", p.Filename, ref.Kind, ref.Authored, err)
		}
		out = append(out, plan.ResolvedDecision{Key: item.Key, ADRIdentity: item.ADRIdentity, Title: item.Title, Status: item.Status, Markdown: item.Markdown})
	}
	return out, nil
}

func selectedRefs(p plan.Plan, selector string) (plan.Phase, plan.Task, error) {
	return plan.Select(p, selector)
}
