package project

import (
	"context"
	"github.com/hypnotox/agentic-workflows/internal/currentstatecoord"
)

// OutputPreparation is the coordinator-selected index universe used by
// project-owned generated-output drift comparison.
type OutputPreparation = currentstatecoord.OutputPreparation

func PrepareStagedOutputState(ctx context.Context, root string) (*OutputPreparation, error) {
	return currentstatecoord.PrepareStagedOutput(ctx, root)
}
