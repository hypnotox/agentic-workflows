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
	return plan.RenderProjection(selected, selector)
}
