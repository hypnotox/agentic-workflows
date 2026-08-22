package project

import (
	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
)

// CheckStagedDrift preserves legacy test fixtures while production consumers
// use the owner-classified semantic result directly.
func CheckStagedDrift(prep *ContextPreparation, plan outputplan.Plan) ([]manifest.Drift, error) {
	result, err := CheckStagedDriftResult(prep, plan)
	if err != nil {
		return nil, err
	}
	return stagedCompatibilityDrift(result), nil
}

func stagedCompatibilityDrift(result checkresult.Result) []manifest.Drift {
	findings := result.Findings()
	drift := make([]manifest.Drift, len(findings))
	for i, finding := range findings {
		drift[i] = manifest.Drift{Path: finding.Evidence.Path, Kind: finding.Evidence.Kind, Detail: finding.Evidence.Detail}
	}
	return drift
}
