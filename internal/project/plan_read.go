package project

import (
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/plan"
)

// ReadPlan resolves an exact plan filename or stem beneath the configured plans
// directory and returns internal/plan's executable projection unchanged.
func (p *Project) ReadPlan(name, selector string) ([]byte, error) {
	plansDir := filepath.Join(p.Root, p.Cfg.DocsDir, "plans")
	selected, err := plan.Resolve(plansDir, name)
	if err != nil {
		return nil, err
	}
	if selected.Format != "plan-v2" {
		return plan.RenderProjection(selected, selector)
	}
	phase, task, err := selectedRefs(selected, selector)
	if err != nil {
		return nil, err
	}
	corpus, _, _, err := p.deriveOperationState()
	if err != nil {
		return nil, err
	}
	refs := task.Fields
	if task.Number == 0 {
		for _, candidate := range phase.Tasks {
			refs.Applying = append(refs.Applying, candidate.Fields.Applying...)
			refs.Context = append(refs.Context, candidate.Fields.Context...)
		}
	}
	applying, err := resolvePlanDecisions(selected, corpus, refs.Applying, false)
	if err != nil {
		return nil, err
	}
	context, err := resolvePlanDecisions(selected, corpus, refs.Context, true)
	if err != nil {
		return nil, err
	}
	_ = phase
	return plan.RenderProjectionInput(plan.ProjectionInput{Plan: selected, Selector: selector, Applying: applying, Context: context})
}
