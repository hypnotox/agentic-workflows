package upgrade

import (
	"context"
	"errors"
	"fmt"

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

// ProjectPresent, AuthorityLockPath, SchemaGate, and Migration are the focused
// migration-owner inputs required because migrate already owns lower upgrade
// mechanisms. cmd/awf supplies their concrete implementations.
type ProjectPresent func(string) bool
type AuthorityLockPath func(string) string
type SchemaGate func(string) (string, int, error)
type SchemaAheadError func(int) error
type Migration func(context.Context, string) (MigrationResult, error)
type CurrentSchemaChange func() string

type MigrationResult struct {
	Applied []string
	Changes []string
}

// Outcome is the semantic presentation result of an upgrade operation.
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
func Run(ctx context.Context, root string, sync Sync, gate Gate, present ProjectPresent, authorityPath AuthorityLockPath, schemaGate SchemaGate, schemaAhead SchemaAheadError, migrate Migration, currentSchemaChange CurrentSchemaChange) (OperationOutcome, error) {
	if !present(root) {
		return OperationOutcome{}, errors.New("not an awf project (run `awf init`)")
	}
	lockPath := authorityPath(root)
	lock, found, err := manifest.LoadOptional(lockPath)
	if err != nil {
		return OperationOutcome{}, err
	}
	if !found {
		return OperationOutcome{}, errors.New("pre-tracking authority: use the bridge release to attest before upgrading")
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
	state, generation, err := schemaGate(root)
	if err != nil {
		return OperationOutcome{}, err
	}
	if state == "ahead" {
		return OperationOutcome{}, schemaAhead(generation)
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
			if err := lock.Save(config.LockPath(root)); err != nil { // coverage-ignore: the absent owned lock's parent was successfully read immediately above; a save failure requires a concurrent filesystem or storage fault
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
