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
	outcome, err := upgrade.Run(ctx, root, upgradeSyncMutation, gate, migrate.ProjectPresent, migrate.LiveSchemaRange, migrate.GateState, upgradeMigration, upgradeCurrentSchemaChange)
	if err != nil {
		return presentUpgradeRefusal(err)
	}
	return presentation.Render(stdout, outcome.Document)
}

// presentUpgradeRefusal is the command boundary for recovery guidance; semantic
// compatibility errors remain machine-classifiable below this boundary.
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
		mutations[i] = upgrade.FileMutation{Path: mutation.Path, Content: mutation.Content, Mode: mutation.Mode, Remove: mutation.Remove}
	}
	return upgrade.MigrationResult{Applied: applied, Changes: texts, Mutations: mutations}, err
}

func upgradeCurrentSchemaChange() string { return migrate.CurrentSchemaChange().Text }
