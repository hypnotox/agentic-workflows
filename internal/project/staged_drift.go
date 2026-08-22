package project

import (
	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/generatedcheck"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
)

// CheckStagedDriftResult compares one Publisher plan entirely within its prepared
// index universe and retains generated-output classification from its owner.
func CheckStagedDriftResult(prep *ContextPreparation, plan outputplan.Plan) (checkresult.Result, error) {
	indexed := map[string]bool{}
	for _, file := range prep.tree.List() {
		indexed[file.Path] = true
	}
	return generatedcheck.Staged(prep.State.Nested(), prep.lock, plan, prep.Reader, indexed)
}
