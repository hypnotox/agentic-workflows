package project

import (
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/plan"
)

// ReadPlan resolves an exact plan filename or stem beneath the configured plans
// directory and returns internal/plan's executable projection unchanged.
func readPlan(root, name, selector string) ([]byte, error) {
	plansDir := filepath.Join(root, config.DocsDir, "plans")
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
	corpus, err := adr.LoadCorpus(decisionsDir(root))
	if err != nil {
		return nil, err
	}
	applying, context, err := resolveSelectedPlanDecisions(selected, corpus, phase, task)
	if err != nil {
		return nil, err
	}
	return plan.RenderProjectionInput(plan.ProjectionInput{Plan: selected, Selector: selector, Applying: applying, Context: context})
}
