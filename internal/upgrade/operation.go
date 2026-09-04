package upgrade

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

const (
	lockRel             = config.DirName + "/awf.lock"
	legacyUpgradeMarker = config.DirName + "/current-state-upgrade.journal"
)

// Sync performs the terminal publication and returns all output facts proved
// before a possible failure. Publisher owns writing the final current lock.
type Sync func(context.Context, string) (presentation.Mutation, error)

// Gate verifies the complete synchronized project.
type Gate func(context.Context, string) error

type ProjectPresent func(string) (bool, error)
type SchemaGate func(string) (string, int, error)
type LiveSchemaRange func() (floor, current int)

// Migration plans, preflights, and applies the live migration bridge.
type Migration func(context.Context, string) (MigrationResult, error)

// MigrationResult contains semantic migration evidence and simple visible path
// lists. Upgrade intentionally knows no file-image or operation representation.
type MigrationResult struct {
	Planned []string
	Applied []string
	Changes []string
	Touched []string
	Pending []string
}

type CurrentSchemaChange func() string

type OperationOutcome struct {
	Document presentation.Document
}

func currentConfigPresent(root string) (bool, error) {
	if _, err := os.Stat(config.ConfigPath(root)); err == nil {
		return true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("stat .awf/config.yaml: %w", err)
	}
	return false, nil
}

func currentLockPresent(root string) (present bool, returnErr error) {
	files, err := filesystem.Open(root)
	if err != nil {
		return false, err
	}
	defer func() { returnErr = errors.Join(returnErr, files.Close()) }()
	_, err = files.LinkInfo(lockRel)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func refuseLegacyMarker(root string) error {
	_, err := os.Lstat(filepath.Join(root, filepath.FromSlash(legacyUpgradeMarker)))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect legacy upgrade journal %s: %w", legacyUpgradeMarker, err)
	}
	return fmt.Errorf("legacy upgrade journal %s exists; inspect and resolve it with the last journal-capable awf binary or Git, remove it, then rerun `awf upgrade`", legacyUpgradeMarker)
}

func reloadCurrentAuthority(root string, floor, current int) (*manifest.Lock, error) {
	live, found, err := manifest.LoadLiveFileOptional(root, lockRel, floor, current)
	if err != nil {
		return nil, err
	}
	configFound, err := currentConfigPresent(root)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, &manifest.PartialAuthorityError{Config: configFound, Lock: false}
	}
	if !configFound {
		return nil, &manifest.PartialAuthorityError{Config: false, Lock: true}
	}
	return live.Lock, nil
}

// Run sequences authority validation, the live migration, terminal sync, and
// the ordinary post-sync gate. The migration never writes the schema lock.
func Run(ctx context.Context, root string, sync Sync, gate Gate, present ProjectPresent, liveSchemaRange LiveSchemaRange, schemaGate SchemaGate, migrate Migration, currentSchemaChange CurrentSchemaChange) (OperationOutcome, error) {
	projectFound, err := present(root)
	if err != nil {
		return OperationOutcome{}, err
	}
	if !projectFound {
		return OperationOutcome{}, errors.New("not an awf project (run `awf init`)")
	}
	// This check precedes schema authority dispatch and every mutating dependency.
	if err := refuseLegacyMarker(root); err != nil {
		return OperationOutcome{}, err
	}

	floor, current := liveSchemaRange()
	configFound, err := currentConfigPresent(root)
	if err != nil {
		return OperationOutcome{}, err
	}
	lockFound, err := currentLockPresent(root)
	if err != nil {
		return OperationOutcome{}, fmt.Errorf("inspect %s: %w", lockRel, err)
	}
	state := ""
	if !configFound && !lockFound && schemaGate != nil {
		state, _, err = schemaGate(root)
		if err != nil {
			return OperationOutcome{}, err
		}
	}
	if !lockFound {
		if configFound {
			return OperationOutcome{}, &manifest.PartialAuthorityError{Config: true, Lock: false}
		}
		return OperationOutcome{}, errors.New("not an awf project")
	}
	if _, err := reloadCurrentAuthority(root, floor, current); err != nil {
		return OperationOutcome{}, err
	}
	if schemaGate != nil {
		state, _, err = schemaGate(root)
		if err != nil {
			return OperationOutcome{}, err
		}
	}
	if _, err := reloadCurrentAuthority(root, floor, current); err != nil {
		return OperationOutcome{}, err
	}

	migration, migrationErr := migrate(ctx, root)
	if migrationErr != nil {
		return OperationOutcome{}, newUpgradeFailure(migration, presentation.Mutation{}, migrationErr)
	}
	syncMutation, syncErr := sync(ctx, root)
	if syncErr != nil {
		return OperationOutcome{}, newUpgradeFailure(migration, syncMutation, syncErr)
	}
	if gateErr := gate(ctx, root); gateErr != nil {
		return OperationOutcome{}, newUpgradeFailure(migration, syncMutation, gateErr)
	}
	changes := append([]string(nil), migration.Changes...)
	if state == "ok" && currentSchemaChange != nil {
		changes = append(changes, currentSchemaChange())
	}
	mutation, err := upgradeMutation(syncMutation, migration.Applied, changes)
	if err != nil {
		return OperationOutcome{}, newUpgradeFailure(migration, syncMutation, err)
	}
	document, err := mutation.Document()
	if err != nil {
		return OperationOutcome{}, newUpgradeFailure(migration, syncMutation, err)
	}
	return OperationOutcome{Document: document}, nil
}
