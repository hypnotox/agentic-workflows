package main

import (
	"context"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/checkop"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// runCheckOperation composes one parsed check leaf with the CLI renderer and
// exit protocol.
func runCheckOperation(ctx context.Context, root string, stdout io.Writer, leaf checkop.Leaf) error {
	outcome, err := checkop.Run(ctx, root, leaf)
	if err != nil {
		return err
	}
	return renderCheckOutcome(stdout, outcome)
}

func renderCheckOutcome(stdout io.Writer, outcome checkop.Outcome) error {
	if err := presentation.Render(stdout, outcome.Document); err != nil {
		return err
	}
	if outcome.Failure != nil {
		return &producedReportError{outcome.Failure}
	}
	return nil
}

func runCheck(ctx context.Context, root string, stdout io.Writer) error {
	return runCheckOperation(ctx, root, stdout, checkop.Check)
}
