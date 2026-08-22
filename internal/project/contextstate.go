package project

import (
	"context"

	"github.com/hypnotox/agentic-workflows/internal/currentstatecoord"
)

// ContextPreparation is the coordinator-selected staged universe used by the
// project-owned generated-output drift comparison. The project comparison does
// not select, parse, or cache another universe.
type ContextPreparation = currentstatecoord.ContextPreparation

// PrepareStagedContextState retains the project compatibility entry point for
// staged generated-output drift while delegating its authority preparation.
func PrepareStagedContextState(ctx context.Context, root string) (*ContextPreparation, error) {
	return currentstatecoord.PrepareStagedContext(ctx, root)
}
