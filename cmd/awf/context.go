package main

import (
	"context"
	"errors"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/contextdelivery"
	"github.com/hypnotox/agentic-workflows/internal/contextop"
)

func runContext(ctx context.Context, cwd string, paths []string, staged bool, rng string, uncovered, full bool, shows []string, stdout io.Writer) error {
	return runContextWithDelivery(ctx, cwd, paths, staged, rng, uncovered, full, shows, stdout, contextdelivery.Deliver)
}

func runContextWithDelivery(ctx context.Context, cwd string, paths []string, staged bool, rng string, uncovered, full bool, shows []string, stdout io.Writer, deliver func([]byte, string, io.Writer) error) error {
	rendered, err := contextop.Run(ctx, cwd, contextop.Input{Paths: paths, Staged: staged, Range: rng, Uncovered: uncovered, Full: full, Shows: shows}, openProjectOperation, gate, gateStaged)
	if err != nil {
		var usage *contextop.UsageError
		if errors.As(err, &usage) {
			return &usageErr{msg: usage.Message}
		}
		return err
	}
	return deliver(rendered, cwd, stdout)
}
