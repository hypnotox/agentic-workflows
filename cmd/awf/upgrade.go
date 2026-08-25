package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/upgrade"
)

// runUpgradeFlags retains CLI mode selection and renders the semantic result on
// the command-selected stream.
func runUpgradeFlags(ctx context.Context, root string, recoverMode bool, stdout io.Writer) error {
	if recoverMode {
		return runRecover(root, stdout)
	}
	return runUpgrade(ctx, root, stdout)
}

func runRecover(root string, stdout io.Writer) error {
	outcome, err := upgrade.RecoverOperation(root, migrate.ProjectPresent)
	if err != nil {
		return err
	}
	return presentation.Render(stdout, outcome.Document)
}

func runUpgrade(ctx context.Context, root string, stdout io.Writer) error {
	outcome, err := upgrade.Run(ctx, root, upgradeSyncMutation, gate, migrate.ProjectPresent, migrate.AuthorityLockPath, migrate.LiveSchemaRange, migrate.GateState, schemaAheadError, upgradeMigration, upgradeCurrentSchemaChange)
	if err != nil {
		return presentUpgradeRefusal(err)
	}
	return presentation.Render(stdout, outcome.Document)
}

// presentUpgradeRefusal is the command boundary for recovery guidance; semantic
// compatibility errors remain machine-classifiable below this boundary.
func presentUpgradeRefusal(err error) error {
	var live *manifest.LiveSourceError
	if errors.As(err, &live) {
		if live.Schema < live.Floor {
			return fmt.Errorf("%w: use a release that supports schema %d, then upgrade to schema %d", err, live.Schema, live.Floor)
		}
		return fmt.Errorf("%w: update your pinned awf to a supporting release for schema %d", err, live.Schema)
	}
	var partial *manifest.PartialAuthorityError
	if errors.As(err, &partial) {
		return fmt.Errorf("%w: restore the complete .awf/config.yaml and .awf/awf.lock control pair from version control", err)
	}
	return err
}

func upgradeMigration(ctx context.Context, root string) (upgrade.MigrationResult, error) {
	applied, changes, err := migrate.Upgrade(ctx, root)
	texts := make([]string, len(changes))
	for i, change := range changes {
		texts[i] = change.Text
	}
	if err != nil {
		var collision *migrate.GroundingSkillCollisionError
		if errors.As(err, &collision) {
			err = upgradeGroundingCollision{collision}
		}
	}
	return upgrade.MigrationResult{Applied: applied, Changes: texts}, err
}

type upgradeGroundingCollision struct {
	cause *migrate.GroundingSkillCollisionError
}

func (e upgradeGroundingCollision) Error() string { return e.cause.Error() }
func (e upgradeGroundingCollision) Unwrap() error { return e.cause }
func (e upgradeGroundingCollision) UpgradeDiagnostic(changes []string) (presentation.Diagnostic, error) {
	migrationChanges := make([]migrate.Change, len(changes))
	for i, change := range changes {
		migrationChanges[i] = migrate.Change{Text: change}
	}
	return e.cause.Diagnostic(migrationChanges)
}

func upgradeCurrentSchemaChange() string { return migrate.CurrentSchemaChange().Text }
