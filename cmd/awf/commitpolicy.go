package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/project"
)

// runCommitPolicy opens the ordinary project once and emits its model-owned policy rendering.
func runCommitPolicy(ctx context.Context, root string, targets []string, stdout io.Writer) error {
	p, err := project.Open(ctx, root)
	if err != nil { // coverage-ignore: all ordinary command handlers share project-open refusal coverage
		return err
	}
	outcome := p.VerifyCommitPolicy(ctx, targets)
	fmt.Fprint(stdout, p.CommitPolicyText(outcome))
	if outcome.Disabled || outcome.OK() {
		return nil
	}
	if outcome.Refusal != nil { // coverage-ignore: typed project refusals are covered at composition; command mapping is the direct return
		return outcome.Refusal
	}
	return errors.New("commit policy violations") // coverage-ignore: violations are rendered by the model and mapped to the ordinary nonzero handler error
}
