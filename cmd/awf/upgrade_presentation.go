package main

import (
	"context"

	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/upgrade"
)

type upgradeSyncDependencies struct {
	projectSyncReport func(context.Context, *project.Project) ([]project.Backup, []project.Change, []string, error)
}

func productionUpgradeSyncDependencies() upgradeSyncDependencies {
	return upgradeSyncDependencies{projectSyncReport: func(ctx context.Context, p *project.Project) ([]project.Backup, []project.Change, []string, error) {
		return p.SyncReport(ctx)
	}}
}

// upgradeSyncOutcome preserves every proven SyncReport axis even if the
// terminal write fails, leaving the command owner to present it once.
type upgradeSyncOutcome struct {
	mutation presentation.Mutation
}

// upgradeSyncMutation performs the terminal sync but leaves rendering to the
// upgrade owner, so migration facts and sync changes become one mutation.
func upgradeSyncMutation(ctx context.Context, root string) (upgradeSyncOutcome, error) {
	return upgradeSyncMutationWith(ctx, root, productionUpgradeSyncDependencies())
}

func upgradeSyncMutationWith(ctx context.Context, root string, dependencies upgradeSyncDependencies) (upgradeSyncOutcome, error) {
	loader, err := newProjectLoader(root)
	if err != nil {
		return upgradeSyncOutcome{}, err
	}
	p, err := loader.Open(ctx, root)
	if err != nil {
		return upgradeSyncOutcome{}, err
	}
	backups, changes, pruned, syncErr := dependencies.projectSyncReport(ctx, p)
	mutation, mutationErr := project.SyncMutation(backups, changes, pruned)
	if mutationErr != nil {
		return upgradeSyncOutcome{}, mutationErr
	}
	return upgradeSyncOutcome{mutation: mutation}, syncErr
}

type journalFailure struct {
	condition string
	outcome   upgrade.Outcome
	cause     error
}

func newJournalFailure(condition string, outcome upgrade.Outcome, cause error) error {
	return journalFailure{condition: condition, outcome: outcome, cause: cause}
}

func (e journalFailure) Error() string { return e.cause.Error() }
func (e journalFailure) Unwrap() error { return e.cause }

func (e journalFailure) Diagnostic() (presentation.Diagnostic, error) {
	return e.outcome.FailureDiagnostic(e.condition, e.cause)
}

func upgradeMutation(sync presentation.Mutation, applied []string, changes []migrate.Change) (presentation.Mutation, error) {
	migrationValues := make([]presentation.Value, len(applied))
	for i, name := range applied {
		value, err := presentation.Prose(name)
		if err != nil {
			return presentation.Mutation{}, err
		}
		migrationValues[i] = value
	}
	values := make([]presentation.Value, len(changes))
	for i, change := range changes {
		value, err := presentation.Prose(change.Text)
		if err != nil {
			return presentation.Mutation{}, err
		}
		values[i] = value
	}
	migrationChanges := make([]presentation.MutationChange, 0, 2)
	if len(migrationValues) > 0 {
		migrationChanges = append(migrationChanges, presentation.MutationChange{Label: "applied migrations", Values: migrationValues})
	}
	if len(values) > 0 {
		migrationChanges = append(migrationChanges, presentation.MutationChange{Label: "migration changes", Values: values})
	}
	sync.Changes = append(migrationChanges, sync.Changes...)
	return sync, nil
}

type upgradeFailure struct {
	applied []string
	changes []migrate.Change
	sync    presentation.Mutation
	cause   error
}

func newUpgradeFailure(applied []string, changes []migrate.Change, cause error) error {
	return newUpgradeFailureWithSync(applied, changes, presentation.Mutation{}, cause)
}

func newUpgradeFailureWithSync(applied []string, changes []migrate.Change, sync presentation.Mutation, cause error) error {
	return upgradeFailure{applied: append([]string(nil), applied...), changes: append([]migrate.Change(nil), changes...), sync: sync, cause: cause}
}

func (e upgradeFailure) Error() string { return e.cause.Error() }
func (e upgradeFailure) Unwrap() error { return e.cause }

func (e upgradeFailure) Diagnostic() (presentation.Diagnostic, error) {
	changed := make([]presentation.Field, 0, len(e.changes))
	for _, change := range e.changes {
		if _, err := presentation.Prose(change.Text); err != nil {
			return presentation.Diagnostic{}, err
		}
		// The preceding raw change validation proves this fixed prefix remains
		// presentation-valid.
		value, _ := presentation.Prose("change: " + change.Text)
		field, err := presentation.NewField("migration", value)
		if err != nil { // coverage-ignore: the fixed grammar-valid migration label receives the validated Prose value
			return presentation.Diagnostic{}, err
		}
		changed = append(changed, field)
	}
	for _, group := range e.sync.Changes {
		for _, value := range group.Values {
			field, err := presentation.NewField(group.Label, value)
			if err != nil { // coverage-ignore: sync values are validated and the fixed label is grammar-valid
				return presentation.Diagnostic{}, err
			}
			changed = append(changed, field)
		}
	}
	stepTexts := []string{"correct the reported cause and retry"}
	if len(changed) > 0 {
		stepTexts = []string{
			"run awf upgrade --recover if an upgrade journal exists",
			"inspect the listed changed axes",
			"restore the project from version control if recovery cannot complete",
		}
	}
	steps := make([]presentation.Value, 0, len(stepTexts))
	for _, text := range stepTexts {
		value, err := presentation.Prose(text)
		if err != nil { // coverage-ignore: every closed recovery-step literal is nonempty and Prose-normalized
			return presentation.Diagnostic{}, err
		}
		steps = append(steps, value)
	}
	return presentation.Diagnostic{Condition: "upgrade has not reached terminal sync", State: "operation", Changed: changed, Cause: e.cause.Error(), Steps: steps}, nil
}
