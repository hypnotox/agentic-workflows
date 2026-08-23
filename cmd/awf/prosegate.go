package main

import (
	"context"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/checkop"
)

// runProseGate invokes the resolved prose check operation.
func runProseGate(ctx context.Context, root string, stdout io.Writer) error {
	return runCheckOperation(ctx, root, stdout, checkop.RepositoryProse)
}
