package main

import (
	"context"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
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
	document, outcome, err := project.VerifyCommitPolicyAt(ctx, root, targets)
	if err != nil { // coverage-ignore: project and policy validation constrain every mapped outcome to the fixed presentation grammar
		return err
	}
	if err := presentation.Render(stdout, document); err != nil {
		return err
	}
	if outcome.Disabled || outcome.OK() {
		return nil
	}
	if outcome.Refusal != nil {
		return &producedReportError{&renderedCommitPolicyError{kind: commitPolicyRefusalExit}}
	}
	return &producedReportError{&renderedCommitPolicyError{kind: commitPolicyViolationExit}}
}
