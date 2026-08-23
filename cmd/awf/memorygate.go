package main

import (
	"context"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/checkop"
)

// runMemoryGate invokes the resolved memory check operation.
func runMemoryGate(ctx context.Context, root string, stdout io.Writer) error {
	return runCheckOperation(ctx, root, stdout, checkop.RepositoryMemory)
}
