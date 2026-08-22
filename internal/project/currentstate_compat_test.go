package project

import (
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/severity"
)

// Findings preserves the legacy test-fixture projection while production
// consumers use the owner-classified Result boundary.
func (r CurrentStateReport) Findings() []string {
	var out []string
	for _, finding := range r.Static {
		out = append(out, finding.Message)
	}
	for _, coverage := range r.Coverage {
		if coverage.Severity == severity.Error {
			out = append(out, coverage.Message())
		}
	}
	for _, drift := range r.PlanDrift {
		out = append(out, fmt.Sprintf("%s %s: %s", drift.Kind, drift.Path, drift.Detail))
	}
	return out
}
