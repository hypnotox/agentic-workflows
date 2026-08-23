package main

import (
	"context"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/checkop"
)

func runCheckRepo(ctx context.Context, root string, stdout io.Writer) error {
	return runCheckOperation(ctx, root, stdout, checkop.Repository)
}

func runCheckDrift(ctx context.Context, root string, stdout io.Writer) error {
	return runCheckOperation(ctx, root, stdout, checkop.RepositoryDrift)
}

func runCheckState(ctx context.Context, root string, stdout io.Writer) error {
	return runCheckOperation(ctx, root, stdout, checkop.RepositoryState)
}
