package main

import (
	"context"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/project"
)

type commitPolicyExit string

const (
	commitPolicyViolationExit commitPolicyExit = "violations"
	commitPolicyRefusalExit   commitPolicyExit = "operational-refusal"
)

type renderedCommitPolicyError struct{ kind commitPolicyExit }

func (e *renderedCommitPolicyError) Error() string { return "commit policy " + string(e.kind) }

// runCommitPolicy resolves the invoking worktree once and emits only model-owned policy rendering.
func runCommitPolicy(ctx context.Context, root string, targets []string, stdout io.Writer) error {
	text, outcome := project.VerifyCommitPolicyAt(ctx, root, targets)
	fmt.Fprint(stdout, text)
	if outcome.Disabled || outcome.OK() {
		return nil
	}
	if outcome.Refusal != nil {
		return &renderedCommitPolicyError{kind: commitPolicyRefusalExit}
	}
	return &renderedCommitPolicyError{kind: commitPolicyViolationExit}
}
