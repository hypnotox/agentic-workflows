package main

import (
	"context"
	"errors"
	"io"

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
	outcome, err := upgrade.Run(ctx, root, upgradeSyncMutation, gate, migrate.ProjectPresent, migrate.AuthorityLockPath, migrate.GateState, schemaAheadError, upgradeMigration, upgradeCurrentSchemaChange)
	if err != nil {
		return err
	}
	return presentation.Render(stdout, outcome.Document)
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
