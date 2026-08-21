package main

import (
	"context"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/project"
)

func runReadPlan(ctx context.Context, root string, args []string, stdout io.Writer) error {
	if len(args) != 2 {
		return &usageErr{"usage: awf read plan <plan> <P[.T]>"}
	}
	state, _, _, err := openProjectOperation(ctx, root)
	if err != nil {
		return err
	}
	projection, err := project.ReadPlan(state.Root(), args[0], args[1])
	if err != nil {
		return err
	}
	_, err = stdout.Write(projection)
	return err
}
