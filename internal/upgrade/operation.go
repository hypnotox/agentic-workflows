package upgrade

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
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

// ProjectPresent reports whether root contains an awf project and preserves
// inspection failures rather than treating an unreadable authority path as absent.
type ProjectPresent func(string) (bool, error)

// SchemaGate reports root's migration relation and schema generation.
type SchemaGate func(string) (string, int, error)

// LiveSchemaRange supplies the migrate-owned live compatibility bounds.
type LiveSchemaRange func() (floor, current int)

// Migration applies the migration sequence and returns its semantic results.
type Migration func(context.Context, string) (MigrationResult, error)

// CurrentSchemaChange describes stamping the current schema generation.
type CurrentSchemaChange func() string

// FileMutation is one journal-owned replacement or removal with the exact
// planning preimage that must still be present before any journal is written.
type FileMutation struct {
	Path            string
	Expected        Image
	ExpectedEntries []string
	Content         []byte
	Mode            os.FileMode
	Remove          bool
	EmptyDirectory  bool
}

// MigrationResult records planned migration steps, user-facing changes, and mutations.
type MigrationResult struct {
	Planned   []string
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
	found, err := present(root)
	if err != nil {
		return OperationOutcome{}, err
	}
	if !found {
		return OperationOutcome{}, errors.New("not an awf project (run `awf init`)")
	}
	outcome, err := Recover(root)
	if err != nil {
		return OperationOutcome{}, newJournalFailure("recovery has not reached terminal state", outcome, err)
	}
	mutation, err := outcome.RecoveredMutation()
	if err != nil {
		return OperationOutcome{}, err
	}
	document, err := mutation.Document()
	if err != nil {
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

func currentLockPresent(root string) (present bool, err error) {
	err = withJournalFilesystem(root, func(files *filesystem.Handle) error {
		_, infoErr := files.LinkInfo(LockRel())
		if errors.Is(infoErr, os.ErrNotExist) {
			return nil
		}
		if infoErr != nil {
			return infoErr
		}
		present = true
		return nil
	})
	return present, err
}

// commitMigration converts planned migration file replacements into the existing
// journal transaction. The replacement current-schema lock is deliberately the
// final operation, making rollback and --recover use the same safety boundary.
func commitMigration(root string, lock *manifest.Lock, lockExpected Image, current int, mutations []FileMutation) (Outcome, error) {
	return commitMigrationWith(root, lock, lockExpected, current, mutations, productionJournalOperation())
}

func commitMigrationWith(root string, lock *manifest.Lock, lockExpected Image, current int, mutations []FileMutation, operation journalOperation) (outcome Outcome, err error) {
	err = withBoundJournalOperation(root, operation, func(bound journalOperation) error {
		var runErr error
		outcome, runErr = commitMigrationBound(root, lock, lockExpected, current, mutations, bound)
		return runErr
	})
	return outcome, err
}

func commitMigrationBound(root string, lock *manifest.Lock, lockExpected Image, current int, mutations []FileMutation, operation journalOperation) (Outcome, error) {
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
		if mutation.EmptyDirectory && (!mutation.Remove || !mutation.Expected.Present || len(mutation.Expected.Content) != 0) {
			return Outcome{}, fmt.Errorf("planned empty-directory prune %q has an invalid image", mutation.Path)
		}
		if !mutation.EmptyDirectory && len(mutation.ExpectedEntries) != 0 {
			return Outcome{}, fmt.Errorf("planned file mutation %q carries directory entries", mutation.Path)
		}
		if err := validateImage(mutation.Expected); err != nil {
			return Outcome{}, fmt.Errorf("planned migration path %q expected image: %w", mutation.Path, err)
		}
		seen[mutation.Path] = true
		if mutation.EmptyDirectory {
			entries, readErr := operation.captureDirectoryExpectation(root, mutation.Path)
			if readErr != nil {
				return Outcome{}, fmt.Errorf("capture planned migration directory %s: %w", mutation.Path, readErr)
			}
			names := make([]string, len(entries))
			for i, entry := range entries {
				names[i] = entry.Name()
			}
			sort.Strings(names)
			expected := operation.state.directoryExpectations[mutation.Path].apply
			if expected.Mode().Perm() != os.FileMode(mutation.Expected.Mode).Perm() || !slices.Equal(names, mutation.ExpectedEntries) {
				return Outcome{}, fmt.Errorf("planned migration path %s changed after planning", mutation.Path)
			}
			ops = append(ops, Operation{Path: mutation.Path, Kind: KindEmptyDirectory, Prior: mutation.Expected, Replacement: Image{}, ExpectedEntries: append([]string(nil), mutation.ExpectedEntries...)})
			continue
		}
		info, statErr := operation.lstat(root, mutation.Path)
		switch {
		case errors.Is(statErr, os.ErrNotExist):
			if mutation.Expected.Present {
				return Outcome{}, fmt.Errorf("planned migration path %s changed after planning: expected present", mutation.Path)
			}
		case statErr != nil:
			return Outcome{}, fmt.Errorf("inspect planned migration path %s: %w", mutation.Path, statErr)
		case info.Mode()&os.ModeSymlink != 0:
			return Outcome{}, fmt.Errorf("planned migration path %s changed after planning: final symlinks are unsupported", mutation.Path)
		case !info.Mode().IsRegular():
			return Outcome{}, fmt.Errorf("planned migration path %s changed after planning: final entry is not a regular file", mutation.Path)
		case !mutation.Expected.Present:
			return Outcome{}, fmt.Errorf("planned migration path %s changed after planning: expected absent", mutation.Path)
		}
		prior, err := operation.imageOf(root, mutation.Path)
		if err != nil {
			return Outcome{}, fmt.Errorf("read planned migration path %s: %w", mutation.Path, err)
		}
		if !imagesEqual(prior, mutation.Expected) {
			return Outcome{}, fmt.Errorf("planned migration path %s changed after planning", mutation.Path)
		}
		replacement := Image{Present: !mutation.Remove, Mode: uint32(mutation.Mode.Perm()), Content: mutation.Content}
		ops = append(ops, Operation{Path: mutation.Path, Prior: prior, Replacement: replacement})
	}
	sort.Slice(ops, func(i, j int) bool { return operationLess(ops[i], ops[j]) })
	finalLock := lock.Clone()
	finalLock.SchemaVersion = current
	bytes, err := finalLock.Marshal()
	if err != nil {
		return Outcome{}, err
	}
	if err := validateImage(lockExpected); err != nil {
		return Outcome{}, fmt.Errorf("planned authority lock expected image is invalid: %w", err)
	}
	if !lockExpected.Present {
		return Outcome{}, errors.New("planned authority lock expected image is invalid: lock must be present")
	}
	info, err := operation.lstat(root, LockRel())
	if err != nil {
		return Outcome{}, fmt.Errorf("inspect planned authority lock: %w", err)
	}
	if !info.Mode().IsRegular() {
		return Outcome{}, errors.New("planned authority lock changed after planning: expected a regular file")
	}
	prior, err := operation.imageOf(root, LockRel())
	if err != nil {
		return Outcome{}, fmt.Errorf("read planned authority lock: %w", err)
	}
	if !imagesEqual(prior, lockExpected) {
		return Outcome{}, errors.New("planned authority lock changed after planning")
	}
	ops = append(ops, Operation{Path: LockRel(), Prior: lockExpected, Replacement: Image{Present: true, Mode: 0o644, Content: bytes}})
	return commitTransactionBound(root, ops, operation)
}

func reloadCurrentAuthority(root string, floor, current int) (*manifest.Lock, Image, error) {
	live, found, err := manifest.LoadLiveFileOptional(root, LockRel(), floor, current)
	if err != nil {
		return nil, Image{}, err
	}
	configFound, err := currentConfigPresent(root)
	if err != nil {
		return nil, Image{}, err
	}
	if !found {
		return nil, Image{}, &manifest.PartialAuthorityError{Config: configFound, Lock: false}
	}
	if !configFound {
		return nil, Image{}, &manifest.PartialAuthorityError{Config: false, Lock: true}
	}
	lockImage := Image{Present: true, Mode: uint32(live.Mode.Perm()), Content: live.Content}
	return live.Lock, lockImage, nil
}

// Run executes the normal upgrade use case. Migration, authority and journal
// coordination live here; cmd/awf supplies only its concrete terminal sync and
// gate dependencies.
func Run(ctx context.Context, root string, sync Sync, gate Gate, present ProjectPresent, liveSchemaRange LiveSchemaRange, schemaGate SchemaGate, migrate Migration, currentSchemaChange CurrentSchemaChange) (OperationOutcome, error) {
	projectFound, err := present(root)
	if err != nil {
		return OperationOutcome{}, err
	}
	if !projectFound {
		return OperationOutcome{}, errors.New("not an awf project (run `awf init`)")
	}
	floor, current := liveSchemaRange()
	// A retired layout has neither current control file. Classify it before
	// loading authority so its removed representation is never decoded.
	configFound, configErr := currentConfigPresent(root)
	if configErr != nil {
		return OperationOutcome{}, configErr
	}
	lockFoundOnDisk, lockInspectErr := currentLockPresent(root)
	if lockInspectErr != nil {
		return OperationOutcome{}, fmt.Errorf("inspect .awf/awf.lock: %w", lockInspectErr)
	}
	state := ""
	if !configFound && !lockFoundOnDisk && schemaGate != nil {
		var err error
		state, _, err = schemaGate(root)
		if err != nil {
			return OperationOutcome{}, err
		}
	}
	if !lockFoundOnDisk {
		configFound, statErr := currentConfigPresent(root)
		if statErr != nil {
			return OperationOutcome{}, statErr
		}
		if configFound {
			return OperationOutcome{}, &manifest.PartialAuthorityError{Config: true, Lock: false}
		}
		return OperationOutcome{}, errors.New("not an awf project")
	}
	if _, _, err := reloadCurrentAuthority(root, floor, current); err != nil {
		return OperationOutcome{}, err
	}
	if schemaGate != nil {
		state, _, err = schemaGate(root)
		if err != nil {
			return OperationOutcome{}, err
		}
	}
	lock, lockExpected, err := reloadCurrentAuthority(root, floor, current)
	if err != nil {
		return OperationOutcome{}, err
	}
	if _, err := lock.AuthorityState(); err != nil {
		return OperationOutcome{}, fmt.Errorf("invalid authority: restore .awf/awf.lock from version control; if a current-state-upgrade journal exists run `awf upgrade --recover`: %w", err)
	}
	migration, err := migrate(ctx, root)
	planned, applied, changes := migration.Planned, []string(nil), migration.Changes
	if err != nil {
		return OperationOutcome{}, newUpgradePlanningFailure(planned, changes, err)
	}
	schemaCurrent := state == "ok"
	if len(migration.Mutations) > 0 || len(planned) > 0 {
		journalOutcome, journalErr := commitMigration(root, lock, lockExpected, current, migration.Mutations)
		if journalErr != nil {
			return OperationOutcome{}, newJournalFailure("migration has not reached terminal state", journalOutcome, journalErr)
		}
		applied = planned
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
