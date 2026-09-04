package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/upgrade"
)

// leaseRetainer is the command-result handoff for a mutation lease. The
// command-selected writer carries it without giving model rendering to cmd.
type leaseRetainer interface{ retainLease(func() error) }

func runUpgrade(ctx context.Context, root string, stdout io.Writer) (returnErr error) {
	lease, err := filesystem.AcquireProjectLease(ctx, root, awfgit.ProjectResidentRoot(ctx, root))
	if err != nil {
		return err
	}
	if retained, ok := stdout.(leaseRetainer); ok {
		retained.retainLease(lease.Release)
	} else {
		defer func() { returnErr = errors.Join(returnErr, lease.Release()) }()
	}
	if err := guardMutationSession(ctx, root); err != nil {
		return err
	}
	sync := func(ctx context.Context, root string) (presentation.Mutation, error) {
		return upgradeSyncMutationLeased(ctx, root, lease)
	}
	outcome, err := upgrade.Run(ctx, root, sync, gate, migrate.ProjectPresent, migrate.LiveSchemaRange, migrate.GateState, upgradeMigration, upgradeCurrentSchemaChange)
	if err != nil {
		return presentUpgradeRefusal(err)
	}
	return presentation.Render(stdout, outcome.Document)
}

// upgradeSyncMutationLeased performs terminal publication under the lease held
// since before migration authority was read. The supported old lock remains in
// place until Publisher writes one complete current lock last.
func upgradeSyncMutationLeased(ctx context.Context, root string, lease *filesystem.Lease) (presentation.Mutation, error) {
	loader, err := newProjectLoader(root)
	if err != nil {
		return presentation.Mutation{}, err
	}
	session, err := loader.Load(ctx, root)
	if err != nil {
		return presentation.Mutation{}, err
	}
	result, syncErr := composePublisher(session).SyncUpgradeLeased(ctx, lease, migrate.LiveSchemaFloor)
	var mutation presentation.Mutation
	var mutationErr error
	if syncErr != nil {
		mutation, mutationErr = result.FailureMutation()
	} else {
		mutation, mutationErr = result.Mutation()
	}
	if mutationErr != nil {
		return presentation.Mutation{}, errors.Join(syncErr, mutationErr)
	}
	return mutation, syncErr
}

func presentUpgradeRefusal(err error) error {
	if errors.Is(err, manifest.ErrUnsupportedLiveSource) {
		return presentLiveSourceRefusal(err)
	}
	var partial *manifest.PartialAuthorityError
	if errors.As(err, &partial) {
		return fmt.Errorf("%w: restore the complete .awf/config.yaml and .awf/awf.lock control pair from version control", err)
	}
	return err
}

func upgradeMigration(ctx context.Context, root string) (upgrade.MigrationResult, error) {
	result, err := migrate.Apply(ctx, root)
	changes := make([]string, len(result.Changes))
	for i, change := range result.Changes {
		changes[i] = change.Text
	}
	return upgrade.MigrationResult{
		Planned: result.Planned,
		Applied: result.Applied,
		Changes: changes,
		Touched: result.Touched,
		Pending: result.Pending,
	}, err
}

func upgradeCurrentSchemaChange() string { return migrate.CurrentSchemaChange().Text }
