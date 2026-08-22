package project

import (
	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/generatedcheck"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
)

// CheckStagedDrift compares one Publisher plan entirely within its prepared index universe.
func CheckStagedDrift(prep *ContextPreparation, plan outputplan.Plan) ([]manifest.Drift, error) {
	indexed := map[string]bool{}
	for _, file := range prep.tree.List() {
		indexed[file.Path] = true
	}
	result, err := generatedcheck.Staged(prep.State.Nested(), prep.lock, plan, prep.Reader, indexed)
	if err != nil { // coverage-ignore: ContextPreparation already opened the immutable index tree
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
