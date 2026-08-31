package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/effort"
	"github.com/hypnotox/agentic-workflows/internal/worktree"
)

// CreationError describes a failed default managed-worktree creation and its
// identity-bound effort rollback outcome.
type CreationError struct {
	Message         string
	Condition       string
	ChangedEffort   bool
	ChangedTopology bool
	Topology        worktree.TopologyEffects
	ManagedPath     string
	ManagedBranch   string
	Cause           error
	RollbackCause   error
	Steps           []string
}

func (e *CreationError) Error() string { return e.Message }

func (e *CreationError) Unwrap() []error {
	if e.Cause == nil && e.RollbackCause == nil {
		return nil
	}
	if e.RollbackCause == nil {
		return []error{e.Cause}
	}
	return []error{e.Cause, e.RollbackCause}
}

func (a *app) newEffort(ctx context.Context, input effort.NewInput, base string) (effort.Record, worktree.Result, error) {
	record, err := a.service.New(ctx, input)
	if err != nil {
		return effort.Record{}, worktree.Result{}, err
	}
	result, addErr := a.addWorktree(ctx, record.Slug, base)
	if addErr == nil {
		return record, result, nil
	}
	return effort.Record{}, worktree.Result{}, a.rollbackCreation(ctx, record, addErr)
}

// rollbackCreation composes one failed creation from structured rollback and
// managed-topology observations, never from error prose.
func (a *app) rollbackCreation(ctx context.Context, record effort.Record, addErr error) error {
	slug := record.Slug
	before := a.manager.Observe(ctx, slug)
	effects := mergeTopology(topologyFromError(addErr), before.Topology)
	rollbackResult, rollbackErr := a.service.RollbackCreation(ctx, record)
	after := a.manager.Observe(ctx, slug)
	effects = mergeTopology(effects, after.Topology)
	path := before.Path
	if path == "" {
		path = after.Path
	}
	managedBranch := before.Branch
	if managedBranch == "" {
		managedBranch = after.Branch
	}
	changedTopology := effects.Changed()
	switch {
	case rollbackErr == nil:
		return &CreationError{
			Message:   fmt.Sprintf("worktree creation failed: %v; effort %s rolled back; next action: retry `awf effort new --slug %q %q`", addErr, slug, record.Slug, record.Title),
			Condition: "managed worktree creation failed and the effort was rolled back", ChangedTopology: changedTopology, Topology: effects, ManagedPath: path, ManagedBranch: managedBranch, Cause: addErr,
			Steps: []string{"fix the reported cause", fmt.Sprintf("retry `awf effort new --slug %q %q`", record.Slug, record.Title)},
		}
	case errors.Is(rollbackErr, effort.ErrManagedTopologyPresent):
		return &CreationError{
			Message:   fmt.Sprintf("worktree creation failed: %v; effort %s retained: managed topology remains; next action: inspect `git worktree list --porcelain`", addErr, slug),
			Condition: "managed worktree creation failed and topology remains", ChangedEffort: true, ChangedTopology: changedTopology, Topology: effects, ManagedPath: path, ManagedBranch: managedBranch, Cause: addErr, RollbackCause: rollbackErr,
			Steps: []string{"inspect `git worktree list --porcelain`", fmt.Sprintf("clean up with native Git or `awf effort worktree remove %s`", slug), fmt.Sprintf("retry `awf effort worktree add %s` or finish the effort", slug)},
		}
	case rollbackResult.Removed:
		reservation := ".awf/efforts/.finishing-" + record.ID + "-" + slug
		return &CreationError{
			Message:   fmt.Sprintf("worktree creation failed: %v; effort %s deletion completed with parent durability uncertainty: %v; next action: verify the active resident and %s are absent", addErr, slug, rollbackErr, reservation),
			Condition: "managed worktree creation failed after effort deletion with durability uncertainty", ChangedTopology: changedTopology, Topology: effects, ManagedPath: path, ManagedBranch: managedBranch, Cause: addErr, RollbackCause: rollbackErr,
			Steps: []string{fmt.Sprintf("verify `.awf/efforts/%s` is absent", slug), fmt.Sprintf("verify `%s` is absent", reservation), "retry effort creation only after both paths are absent"},
		}
	case rollbackResult.ResiduePath != "":
		reservation := rollbackResult.ReservationPath
		return &CreationError{
			Message:   fmt.Sprintf("worktree creation failed: %v; effort %s deletion retained identity-bound entries across reservation path %s and cleanup residue %s: %v", addErr, slug, reservation, rollbackResult.ResiduePath, rollbackErr),
			Condition: "managed worktree creation failed and effort deletion retained identity-bound reservation and cleanup paths requiring inspection", ChangedEffort: true, ChangedTopology: changedTopology, Topology: effects, ManagedPath: path, ManagedBranch: managedBranch, Cause: addErr, RollbackCause: rollbackErr,
			Steps: []string{"inspect `" + reservation + "`", "inspect `" + rollbackResult.ResiduePath + "`", "remove only paths whose identities are verified", "retry effort creation only after the active resident, reservation path, and cleanup residue are absent"},
		}
	case rollbackResult.Reserved:
		reservation := rollbackResult.ReservationPath
		if reservation == "" {
			reservation = ".awf/efforts/.finishing-" + record.ID + "-" + slug
		}
		return &CreationError{
			Message:   fmt.Sprintf("worktree creation failed: %v; effort %s deletion rollback was interrupted: %v; next action: inspect the finishing reservation at %s", addErr, slug, rollbackErr, reservation),
			Condition: "managed worktree creation failed and effort deletion rollback was interrupted", ChangedEffort: true, ChangedTopology: changedTopology, Topology: effects, ManagedPath: path, ManagedBranch: managedBranch, Cause: addErr, RollbackCause: rollbackErr,
			Steps: []string{"inspect `" + reservation + "`", "complete safe manual cleanup only after verifying its immutable identity"},
		}
	default:
		return &CreationError{
			Message:   fmt.Sprintf("worktree creation failed: %v; effort %s retained: rollback failed: %v; next action: retry `awf effort worktree add %s` or inspect the resident", addErr, slug, rollbackErr, slug),
			Condition: "managed worktree creation failed and effort rollback failed", ChangedEffort: true, ChangedTopology: changedTopology, Topology: effects, ManagedPath: path, ManagedBranch: managedBranch, Cause: addErr, RollbackCause: rollbackErr,
			Steps: []string{"resolve the rollback failure", fmt.Sprintf("retry `awf effort worktree add %s` or `awf effort finish %s`", slug, slug)},
		}
	}
}

func topologyFromError(err error) worktree.TopologyEffects {
	var refusal *worktree.RefusalError
	if errors.As(err, &refusal) {
		return refusal.Topology
	}
	return worktree.TopologyEffects{}
}

func mergeTopology(left, right worktree.TopologyEffects) worktree.TopologyEffects {
	return worktree.TopologyEffects{
		ManagedPath:     left.ManagedPath || right.ManagedPath,
		GitRegistration: left.GitRegistration || right.GitRegistration,
		Branch:          left.Branch || right.Branch,
		ReceivingHEAD:   left.ReceivingHEAD || right.ReceivingHEAD,
		Uncertain:       left.Uncertain || right.Uncertain,
	}
}
