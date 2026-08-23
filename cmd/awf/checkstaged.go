package main

import (
	"context"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/checkop"
)

func runCheckStaged(ctx context.Context, root string, stdout io.Writer) error {
	return runCheckOperation(ctx, root, stdout, checkop.Staged)
}

func runCheckStagedState(ctx context.Context, root string, stdout io.Writer) error {
	return runCheckOperation(ctx, root, stdout, checkop.StagedState)
}

func runCheckStagedDrift(ctx context.Context, root string, stdout io.Writer) error {
	return runCheckOperation(ctx, root, stdout, checkop.StagedDrift)
}
