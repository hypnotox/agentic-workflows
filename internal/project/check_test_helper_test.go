package project

import (
	"context"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// checkProject preserves the terse drift-only shape used by project tests while
// production consumers retain CheckReport and its tracking advisories.
func checkProject(p *ProjectState, ctx context.Context) ([]manifest.Drift, error) {
	report, err := checkReportProject(p, ctx)
	return report.Drift, err
}
