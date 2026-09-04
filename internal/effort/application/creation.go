package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/effort"
	"github.com/hypnotox/agentic-workflows/internal/worktree"
)

// CreationError describes a failed default managed-worktree creation. The
// newly published effort resident is always retained for retry or inspection.
type CreationError struct {
	Message         string
	Condition       string
	ChangedEffort   bool
	ChangedTopology bool
	Topology        worktree.TopologyEffects
	ManagedPath     string
	ManagedBranch   string
	Cause           error
	Steps           []string
}

func (e *CreationError) Error() string { return e.Message }
func (e *CreationError) Unwrap() error { return e.Cause }

func (a *app) newEffort(ctx context.Context, input effort.NewInput, base string) (effort.Record, worktree.Result, error) {
	record, err := a.service.New(ctx, input)
	if err != nil {
		return effort.Record{}, worktree.Result{}, err
	}
	result, addErr := a.addWorktree(ctx, record.Slug, base)
	if addErr == nil {
		return record, result, nil
	}
	return record, result, a.creationAddFailure(record, addErr)
}

func (a *app) creationAddFailure(record effort.Record, addErr error) error {
	effects := topologyFromError(addErr)
	var path, branch string
	var refusal *worktree.RefusalError
	if errors.As(addErr, &refusal) {
		path, branch = refusal.ManagedPath, refusal.ManagedBranch
	}
	return &CreationError{
		Message:         fmt.Sprintf("worktree creation failed: %v; effort %s retained; next action: inspect the named Git topology and retry `awf effort worktree add %s`", addErr, record.Slug, record.Slug),
		Condition:       "managed worktree creation failed and the effort resident was retained",
		ChangedEffort:   true,
		ChangedTopology: effects.Changed(),
		Topology:        effects,
		ManagedPath:     path,
		ManagedBranch:   branch,
		Cause:           addErr,
		Steps: []string{
			"inspect the named managed path, registration, and branch with native Git",
			fmt.Sprintf("retry `awf effort worktree add %s`", record.Slug),
			fmt.Sprintf("inspect `.awf/efforts/%s`", record.Slug),
		},
	}
}

func topologyFromError(err error) worktree.TopologyEffects {
	var refusal *worktree.RefusalError
	if errors.As(err, &refusal) {
		return refusal.Topology
	}
	return worktree.TopologyEffects{}
}
