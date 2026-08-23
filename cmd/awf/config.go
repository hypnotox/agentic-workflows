package main

import (
	"context"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/configop"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// runConfig composes and invokes the configuration-reference operation, then
// renders its Publisher-owned document to the selected command stream.
func runConfig(ctx context.Context, cwd, key string, stdout io.Writer) error {
	document, err := configop.Run(ctx, cwd, key, newProjectLoader, gate)
	if err != nil {
		return err
	}
	return presentation.Render(stdout, document)
}
