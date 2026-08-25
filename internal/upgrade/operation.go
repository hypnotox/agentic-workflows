package upgrade

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// Sync performs the terminal publication and returns all output facts proved
// before a possible failure. It is composed by the command because Publisher
// construction belongs at that boundary.
type Sync func(context.Context, string) (presentation.Mutation, error)

// Gate verifies that the selected binary may operate on the project. It is
// composed by the command because command execution owns the concrete gate.
type Gate func(context.Context, string) error

// ProjectPresent reports whether root contains an awf project.
type ProjectPresent func(string) bool

// AuthorityLockPath resolves the active migration authority lock for root.
type AuthorityLockPath func(string) string

// SchemaGate reports root's migration relation and schema generation.
type SchemaGate func(string) (string, int, error)

// LiveSchemaRange supplies the migrate-owned live compatibility bounds.
type LiveSchemaRange func() (floor, current int)

// Migration applies the migration sequence and returns its semantic results.
type Migration func(context.Context, string) (MigrationResult, error)

// CurrentSchemaChange describes stamping the current schema generation.
type CurrentSchemaChange func() string

// MigrationResult records applied migration steps and user-facing changes.
type MigrationResult struct {
	Applied []string
	Changes []string
}

// OperationOutcome is the semantic presentation result of an upgrade operation.
type OperationOutcome struct {
	Document presentation.Document
}

// RecoverOperation replays an interrupted upgrade transaction and maps its
// terminal evidence into the operation's presentation result.
func RecoverOperation(root string, present ProjectPresent) (OperationOutcome, error) {
	if !present(root) {
		return OperationOutcome{}, errors.New("not an awf project (run `awf init`)")
	}
	outcome, err := Recover(root)
	if err != nil {
		return OperationOutcome{}, newJournalFailure("recovery has not reached terminal state", outcome, err)
	}
	mutation, err := outcome.RecoveredMutation()
	if err != nil { // coverage-ignore: Recover validates journal evidence before producing its Outcome, so its terminal evidence always lowers to literals
		return OperationOutcome{}, err
	}
	document, err := mutation.Document()
	if err != nil { // coverage-ignore: RecoveredMutation supplies the fixed valid status and only validated journal literals
		return OperationOutcome{}, err
	}
	return OperationOutcome{Document: document}, nil
}

// Run executes the normal upgrade use case. Migration, authority and journal
// coordination live here; cmd/awf supplies only its concrete terminal sync and
// gate dependencies.
func Run(ctx context.Context, root string, sync Sync, gate Gate, present ProjectPresent, authorityPath AuthorityLockPath, liveSchemaRange LiveSchemaRange, schemaGate SchemaGate, migrate Migration, currentSchemaChange CurrentSchemaChange) (OperationOutcome, error) {
	if !present(root) {
		return OperationOutcome{}, errors.New("not an awf project (run `awf init`)")
	}
	lockPath := authorityPath(root)
	lock, found, err := manifest.LoadOptional(lockPath)
	if err != nil {
		return OperationOutcome{}, err
	}
	if !found {
		if _, err := os.Stat(config.ConfigPath(root)); err == nil {
			return OperationOutcome{}, &manifest.PartialAuthorityError{Config: true, Lock: false}
		}
		return OperationOutcome{}, errors.New("not an awf project")
	}
	if lockPath == config.LockPath(root) {
		if _, lockErr := os.Stat(lockPath); lockErr == nil {
			if _, err := os.Stat(config.ConfigPath(root)); os.IsNotExist(err) {
				return OperationOutcome{}, &manifest.PartialAuthorityError{Config: false, Lock: true}
			} else if err != nil { // coverage-ignore: after the lock proves this is a current authority root, config stat can fail only through a concurrent namespace, permission, or storage fault
				return OperationOutcome{}, err
			}
		}
	}
	floor, current := liveSchemaRange()
	if err := manifest.ValidateLive(lock, floor, current); err != nil {
		return OperationOutcome{}, err
	}
	state, _, err := schemaGate(root)
	if err != nil {
		return OperationOutcome{}, err
	}
	authority, err := lock.AuthorityState()
	if err != nil { // coverage-ignore: LoadOptional parses and validates the unchanged authority construction immediately above
		return OperationOutcome{}, fmt.Errorf("invalid authority: restore .awf/awf.lock from version control; if a cutover journal exists run `awf upgrade --recover`: %w", err)
	}
	switch authority {
	case manifest.AuthorityBridge:
		outcome, finalErr := FinalUpgrade(ctx, root, lock)
		if finalErr != nil {
			return OperationOutcome{}, newJournalFailure("upgrade has not reached terminal state", outcome, finalErr)
		}
		mutation, err := outcome.CompletedMutation()
		if err != nil { // coverage-ignore: FinalUpgrade constructs its terminal evidence from validated transaction operations
			return OperationOutcome{}, err
		}
		document, err := mutation.Document()
		if err != nil { // coverage-ignore: CompletedMutation supplies the fixed valid status and only validated journal literals
			return OperationOutcome{}, err
		}
		return OperationOutcome{Document: document}, nil
	case manifest.AuthorityPermanent:
	default: // coverage-ignore: AuthorityState returns only Bridge or Permanent when its validation succeeds
		return OperationOutcome{}, errors.New("invalid authority: restore .awf/awf.lock from version control")
	}
	migration, err := migrate(ctx, root)
	applied, changes := migration.Applied, migration.Changes
	if err != nil {
		return OperationOutcome{}, newUpgradeFailure(applied, changes, presentation.Mutation{}, err)
	}
	schemaCurrent := state == "ok"
	if lockPath != config.LockPath(root) {
		if _, found, err := manifest.LoadOptional(config.LockPath(root)); err != nil {
			return OperationOutcome{}, newUpgradeFailure(applied, changes, presentation.Mutation{}, err)
		} else if !found {
			lock.SchemaVersion = 14
			if err := lock.Save(config.LockPath(root)); err != nil {
				return OperationOutcome{}, newUpgradeFailure(applied, changes, presentation.Mutation{}, err)
			}
			changes = append(changes, "created and schema-stamped .awf/awf.lock")
			completion, completionErr := migrate(ctx, root)
			applied = append(applied, completion.Applied...)
			changes = append(changes, completion.Changes...)
			if completionErr != nil {
				return OperationOutcome{}, newUpgradeFailure(applied, changes, presentation.Mutation{}, completionErr)
			}
		}
	}
	if err := gate(ctx, root); err != nil {
		return OperationOutcome{}, newUpgradeFailure(applied, changes, presentation.Mutation{}, err)
	}
	syncMutation, err := sync(ctx, root)
	if err != nil {
		return OperationOutcome{}, newUpgradeFailure(applied, changes, syncMutation, err)
	}
	if schemaCurrent {
		changes = append(changes, currentSchemaChange())
	}
	mutation, err := upgradeMutation(syncMutation, applied, changes)
	if err != nil {
		return OperationOutcome{}, newUpgradeFailure(applied, changes, presentation.Mutation{}, err)
	}
	document, err := mutation.Document()
	if err != nil {
		return OperationOutcome{}, newUpgradeFailure(applied, changes, presentation.Mutation{}, err)
	}
	return OperationOutcome{Document: document}, nil
}
