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

// runUpgradeFlags retains CLI mode selection and renders the semantic result on
// the command-selected stream.
func runUpgradeFlags(ctx context.Context, root string, recoverMode bool, stdout io.Writer) error {
	if recoverMode {
		return runRecover(ctx, root, stdout)
	}
	return runUpgrade(ctx, root, stdout)
}

func runRecover(ctx context.Context, root string, stdout io.Writer) (returnErr error) {
	lease, err := filesystem.AcquireTrackedLease(ctx, root)
	if err != nil {
		return err
	}
	if retained, ok := stdout.(leaseRetainer); ok {
		retained.retainLease(lease.Release)
	} else {
		defer func() { returnErr = errors.Join(returnErr, lease.Release()) }()
	}
	outcome, err := upgrade.RecoverOperation(root, migrate.ProjectPresent)
	if err != nil {
		return err
	}
	return presentation.Render(stdout, outcome.Document)
}

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
	// The terminal publisher shares this operation's lease rather than acquiring
	// a nested transaction after migration has already observed authority.
	sync := func(ctx context.Context, root string) (presentation.Mutation, error) {
		return upgradeSyncMutationLeased(ctx, root, lease)
	}
	outcome, err := upgrade.Run(ctx, root, sync, gate, migrate.ProjectPresent, migrate.LiveSchemaRange, migrate.GateState, upgradeMigration, upgradeCurrentSchemaChange)
	if err != nil {
		return presentUpgradeRefusal(err)
	}
	return presentation.Render(stdout, outcome.Document)
}

// presentUpgradeRefusal is the command boundary for recovery guidance; semantic
// compatibility errors remain machine-classifiable below this boundary.
// upgradeSyncMutationLeased is the normal-upgrade terminal publication under
// the lease acquired before authority and journal planning. Recovery remains
// journal-owned and does not use Publisher.
func upgradeSyncMutationLeased(ctx context.Context, root string, lease *filesystem.Lease) (presentation.Mutation, error) {
	loader, err := newProjectLoader(root)
	if err != nil {
		return presentation.Mutation{}, err
	}
	session, err := loader.Load(ctx, root)
	if err != nil {
		return presentation.Mutation{}, err
	}
	result, syncErr := composePublisher(session).SyncLeased(ctx, lease)
	if syncErr != nil {
		mutation, mutationErr := result.PartialMutation()
		if mutationErr != nil {
			return presentation.Mutation{}, mutationErr
		}
		return mutation, syncErr
	}
	return result.Mutation()
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
	applied, changes, planned, err := migrate.Build(ctx, root)
	texts := make([]string, len(changes))
	for i, change := range changes {
		texts[i] = change.Text
	}
	mutations := make([]upgrade.FileMutation, len(planned))
	for i, mutation := range planned {
		mutations[i] = upgrade.FileMutation{
			Path:     mutation.Path,
			Expected: upgrade.Image{Present: mutation.Expected.Present, Mode: uint32(mutation.Expected.Mode.Perm()), Content: mutation.Expected.Content},
			Content:  mutation.Content, Mode: mutation.Mode, Remove: mutation.Remove,
		}
	}
	return upgrade.MigrationResult{Planned: applied, Changes: texts, Mutations: mutations}, err
}

func upgradeCurrentSchemaChange() string { return migrate.CurrentSchemaChange().Text }
