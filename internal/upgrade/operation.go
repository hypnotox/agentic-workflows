package upgrade

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"

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

// SchemaGate reports root's migration relation and schema generation.
type SchemaGate func(string) (string, int, error)

// LiveSchemaRange supplies the migrate-owned live compatibility bounds.
type LiveSchemaRange func() (floor, current int)

// Migration applies the migration sequence and returns its semantic results.
type Migration func(context.Context, string) (MigrationResult, error)

// CurrentSchemaChange describes stamping the current schema generation.
type CurrentSchemaChange func() string

// FileMutation is one journal-owned replacement or removal.
type FileMutation struct {
	Path    string
	Content []byte
	Mode    os.FileMode
	Remove  bool
}

// MigrationResult records applied migration steps and user-facing changes.
type MigrationResult struct {
	Applied   []string
	Changes   []string
	Mutations []FileMutation
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

func currentConfigPresent(root string) (bool, error) {
	if _, err := os.Stat(config.ConfigPath(root)); err == nil {
		return true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("stat .awf/config.yaml: %w", err)
	}
	return false, nil
}

func requireCurrentConfig(root string) error {
	found, err := currentConfigPresent(root)
	if err != nil {
		return err
	}
	if !found {
		return &manifest.PartialAuthorityError{Config: false, Lock: true}
	}
	return nil
}

// commitMigration converts planned migration file replacements into the existing
// journal transaction. The replacement current-schema lock is deliberately the
// final operation, making rollback and --recover use the same safety boundary.
func commitMigration(root string, lock *manifest.Lock, current int, mutations []FileMutation) (Outcome, error) {
	return commitMigrationWith(root, lock, current, mutations, productionJournalOperation())
}

func commitMigrationWith(root string, lock *manifest.Lock, current int, mutations []FileMutation, operation journalOperation) (Outcome, error) {
	seen := make(map[string]bool, len(mutations))
	ops := make([]Operation, 0, len(mutations)+1)
	for _, mutation := range mutations {
		if mutation.Path == LockRel() || !safeRelPath(mutation.Path) || seen[mutation.Path] {
			return Outcome{}, fmt.Errorf("invalid planned migration path %q", mutation.Path)
		}
		if !mutation.Remove && mutation.Mode == 0 {
			return Outcome{}, fmt.Errorf("planned migration path %q has no mode", mutation.Path)
		}
		if mutation.Remove && (mutation.Mode != 0 || len(mutation.Content) != 0) {
			return Outcome{}, fmt.Errorf("planned removal %q carries replacement data", mutation.Path)
		}
		seen[mutation.Path] = true
		prior, err := imageOf(root, mutation.Path)
		if err != nil {
			return Outcome{}, fmt.Errorf("read planned migration path %s: %w", mutation.Path, err)
		}
		replacement := Image{Present: !mutation.Remove, Mode: uint32(mutation.Mode.Perm()), Content: mutation.Content}
		ops = append(ops, Operation{Path: mutation.Path, Prior: prior, Replacement: replacement})
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].Path < ops[j].Path })
	finalLock := lock.Clone()
	finalLock.SchemaVersion = current
	bytes, err := finalLock.Marshal()
	if err != nil {
		return Outcome{}, err
	}
	prior, err := imageOf(root, LockRel())
	if err != nil {
		return Outcome{}, err
	}
	ops = append(ops, Operation{Path: LockRel(), Prior: prior, Replacement: Image{Present: true, Mode: 0o644, Content: bytes}})
	return commitTransactionWith(root, ops, operation)
}

func reloadCurrentAuthority(root string, floor, current int) (*manifest.Lock, error) {
	lock, found, err := manifest.LoadLiveOptional(config.LockPath(root), floor, current)
	if err != nil {
		return nil, err
	}
	configFound, err := currentConfigPresent(root)
	if err != nil {
		return nil, err
	}
	if !found || !configFound {
		return nil, &manifest.PartialAuthorityError{Config: configFound, Lock: found}
	}
	return lock, nil
}

// Run executes the normal upgrade use case. Migration, authority and journal
// coordination live here; cmd/awf supplies only its concrete terminal sync and
// gate dependencies.
func Run(ctx context.Context, root string, sync Sync, gate Gate, present ProjectPresent, liveSchemaRange LiveSchemaRange, schemaGate SchemaGate, migrate Migration, currentSchemaChange CurrentSchemaChange) (OperationOutcome, error) {
	if !present(root) {
		return OperationOutcome{}, errors.New("not an awf project (run `awf init`)")
	}
	lockPath := config.LockPath(root)
	floor, current := liveSchemaRange()
	// A retired layout has neither current control file. Classify it before
	// loading authority so its removed representation is never decoded.
	configFound, configErr := currentConfigPresent(root)
	if configErr != nil {
		return OperationOutcome{}, configErr
	}
	_, lockStatErr := os.Stat(lockPath)
	lockFoundOnDisk := lockStatErr == nil
	if lockStatErr != nil && !errors.Is(lockStatErr, os.ErrNotExist) {
		return OperationOutcome{}, fmt.Errorf("stat .awf/awf.lock: %w", lockStatErr)
	}
	state := ""
	if !configFound && !lockFoundOnDisk && schemaGate != nil {
		var err error
		state, _, err = schemaGate(root)
		if err != nil {
			return OperationOutcome{}, err
		}
	}
	_, found, err := manifest.LoadLiveOptional(lockPath, floor, current)
	if err != nil {
		return OperationOutcome{}, err
	}
	if !found {
		configFound, statErr := currentConfigPresent(root)
		if statErr != nil {
			return OperationOutcome{}, statErr
		}
		if configFound {
			return OperationOutcome{}, &manifest.PartialAuthorityError{Config: true, Lock: false}
		}
		return OperationOutcome{}, errors.New("not an awf project")
	}
	if err := requireCurrentConfig(root); err != nil {
		return OperationOutcome{}, err
	}
	if schemaGate != nil {
		state, _, err = schemaGate(root)
		if err != nil {
			return OperationOutcome{}, err
		}
	}
	lock, err := reloadCurrentAuthority(root, floor, current)
	if err != nil {
		return OperationOutcome{}, err
	}
	authority, err := lock.AuthorityState()
	if err != nil { // coverage-ignore: LoadLiveOptional parses and validates the unchanged authority construction immediately above
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
	if len(migration.Mutations) > 0 || len(applied) > 0 {
		journalOutcome, journalErr := commitMigration(root, lock, current, migration.Mutations)
		if journalErr != nil {
			return OperationOutcome{}, newJournalFailure("migration has not reached terminal state", journalOutcome, journalErr)
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
