package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/upgrade"
)

// runUpgradeFlags routes the two current-state upgrade modes. Plain upgrade
// either consumes a sealed bridge attestation (the final cutover) or, with no
// attestation, migrates the schema and syncs. --recover replays the journal
// recovery table. Attestation, readiness, and their reporting live only in the
// preceding bridge release; the current-state binary consumes seals, it never
// produces them.
func runUpgradeFlags(ctx context.Context, root string, recoverMode bool, stdout io.Writer) error {
	if recoverMode {
		return runRecover(root, stdout)
	}
	return runUpgrade(ctx, root, stdout)
}

// runRecover replays the current-state upgrade journal recovery table. It never
// runs project tests or gates; terminal evidence is rendered only after recovery.
func runRecover(root string, stdout io.Writer) error {
	if !migrate.ProjectPresent(root) {
		return errors.New("not an awf project (run `awf init`)")
	}
	outcome, err := upgradeRecover(root)
	if err != nil {
		return newJournalFailure("recovery has not reached terminal state", outcome, err)
	}
	mutation, err := outcome.RecoveredMutation()
	if err != nil { // coverage-ignore: Outcome formats evidence with fixed valid syntax
		return err
	}
	document, err := mutation.Document()
	if err != nil { // coverage-ignore: Outcome supplies a fixed valid status and validated values
		return err
	}
	return presentation.Render(stdout, document)
}

var (
	upgradeSync         = upgradeSyncMutation
	upgradeRecover      = upgrade.Recover
	upgradeFinal        = upgrade.FinalUpgrade
	upgradeMigrate      = migrate.Upgrade
	upgradeGate         = gate
	upgradeLoadOptional = manifest.LoadOptional
	upgradeSaveLock     = func(lock *manifest.Lock, path string) error { return lock.Save(path) }
)

// runUpgrade consumes a sealed attestation when the lock carries one: the final
// current-state cutover verifies only the sealed facts and journals the approval
// deletion plus the replacement lock (discarding the sealed routing payload). With no
// attestation it applies every registered migration past the project's current
// schema generation, then always runs a normal sync - even when no migration
// applies - so a same-schema binary bump still re-renders every managed file and
// re-pins the bootstrap (ADR-0085 Decision 4). Truthful edge states
// (ADR-0076 Decision 4): no config layout at all → the awf init hint; a tree
// whose schema is ahead of this binary → the version-gate guidance.
func runUpgrade(ctx context.Context, root string, stdout io.Writer) error {
	if !migrate.ProjectPresent(root) {
		return errors.New("not an awf project (run `awf init`)")
	}
	authorityPath := authorityLockPath(root)
	lock, found, err := manifest.LoadOptional(authorityPath)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("pre-tracking authority: use the bridge release to attest before upgrading")
	}
	authority, err := lock.AuthorityState()
	if err != nil { // coverage-ignore: LoadOptional parsed and validated this unchanged lock
		return fmt.Errorf("invalid authority: restore .awf/awf.lock from version control; if a cutover journal exists run `awf upgrade --recover`: %w", err)
	}
	switch authority {
	case manifest.AuthorityBridge:
		outcome, finalErr := upgradeFinal(ctx, root, lock)
		if finalErr != nil {
			return newJournalFailure("upgrade has not reached terminal state", outcome, finalErr)
		}
		mutation, err := outcome.CompletedMutation()
		if err != nil { // coverage-ignore: Outcome formats evidence with fixed valid syntax
			return err
		}
		document, err := mutation.Document()
		if err != nil { // coverage-ignore: Outcome supplies a fixed valid status and validated values
			return err
		}
		return presentation.Render(stdout, document)
	case manifest.AuthorityPermanent:
		// Continue with ordinary schema migration and sync.
	default: // coverage-ignore: AuthorityState returns only the closed enum values
		return errors.New("invalid authority: restore .awf/awf.lock from version control")
	}
	state, gen, err := migrate.GateState(root)
	if err != nil {
		return err
	}
	if state == "ahead" {
		return schemaAheadError(gen)
	}
	applied, changes, err := upgradeMigrate(ctx, root)
	if err != nil {
		return newUpgradeFailure(applied, changes, err)
	}
	_ = gen
	if authorityPath != config.LockPath(root) {
		if _, found, err := upgradeLoadOptional(config.LockPath(root)); err != nil {
			return newUpgradeFailure(applied, changes, err)
		} else if !found {
			lock.SchemaVersion = 14
			if err := upgradeSaveLock(lock, config.LockPath(root)); err != nil {
				return newUpgradeFailure(applied, changes, err)
			}
			changes = append(changes, migrate.Change{Text: "created and schema-stamped .awf/awf.lock"})
			completionApplied, completionChanges, completionErr := upgradeMigrate(ctx, root)
			applied = append(applied, completionApplied...)
			changes = append(changes, completionChanges...)
			if completionErr != nil {
				return newUpgradeFailure(applied, changes, completionErr)
			}
		}
	}
	// Gate before the terminal sync: migration brings the schema current, but a
	// binary behind the lock's awfVersion (version axis, schema equal) must still
	// refuse rather than re-stamp a downgraded version. runSync no longer self-gates,
	// so upgrade re-asserts it here (schema-ahead is already refused above).
	if err := upgradeGate(ctx, root); err != nil {
		return newUpgradeFailure(applied, changes, err)
	}
	sync, err := upgradeSync(ctx, root)
	if err != nil {
		return newUpgradeFailureWithSync(applied, changes, sync.mutation, err)
	}
	mutation, err := upgradeMutation(sync.mutation, applied, changes)
	if err != nil { // coverage-ignore: registered migration descriptions are validated fixed prose
		return newUpgradeFailure(applied, changes, err)
	}
	document, err := mutation.Document()
	if err != nil { // coverage-ignore: typed sync and migration mutations compose only grammar-valid presentation values
		return newUpgradeFailure(applied, changes, err)
	}
	return presentation.Render(stdout, document)
}
