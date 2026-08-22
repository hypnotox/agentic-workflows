package currentstatecoord

import (
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/plancheck"
)

// ReadPlan selects one plan from the working filesystem and completes its
// plan-v2 authority projection. Parsing, selection, closure, and rendering
// remain owned by plan and plancheck.
func ReadPlan(root, name, selector string) ([]byte, error) {
	plansDir := filepath.Join(root, config.DocsDir, "plans")
	selected, err := plan.Resolve(plansDir, name)
	if err != nil {
		return nil, err
	}
	if selected.Format != "plan-v2" {
		return plan.RenderProjection(selected, selector)
	}
	phase, task, err := plancheck.Select(selected, selector)
	if err != nil {
		return nil, err
	}
	corpus, err := adr.LoadCorpus(filepath.Join(root, config.DocsDir, "decisions"))
	if err != nil {
		return nil, err
	}
	applying, context, err := plancheck.ResolveSelectedDecisions(selected, corpus, phase, task)
	if err != nil {
		return nil, err
	}
	return plan.RenderProjectionInput(plan.ProjectionInput{Plan: selected, Selector: selector, Applying: applying, Context: context})
}
