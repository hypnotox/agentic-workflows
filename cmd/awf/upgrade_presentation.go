package main

import (
	"context"

	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

var upgradeProjectSyncReport = func(ctx context.Context, p *project.Project) ([]project.Backup, []project.Change, []string, error) {
	return p.SyncReport(ctx)
}

// upgradeSyncMutation performs the terminal sync but leaves rendering to the
// upgrade owner, so migration facts and sync changes become one mutation.
func upgradeSyncMutation(ctx context.Context, root string) (presentation.Mutation, error) {
	loader, err := newProjectLoader(root)
	if err != nil {
		return presentation.Mutation{}, err
	}
	p, err := loader.Open(ctx, root)
	if err != nil {
		return presentation.Mutation{}, err
	}
	backups, changes, pruned, err := upgradeProjectSyncReport(ctx, p)
	if err != nil {
		return presentation.Mutation{}, err
	}
	return project.SyncMutation(backups, changes, pruned)
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
	cause   error
}

func newUpgradeFailure(applied []string, changes []migrate.Change, cause error) error {
	return upgradeFailure{
		applied: append([]string(nil), applied...),
		changes: append([]migrate.Change(nil), changes...),
		cause:   cause,
	}
}

func (e upgradeFailure) Error() string { return e.cause.Error() }
func (e upgradeFailure) Unwrap() error { return e.cause }

func (e upgradeFailure) Diagnostic() (presentation.Diagnostic, error) {
	changed := make([]presentation.Field, 0, len(e.applied)+len(e.changes))
	for _, name := range e.applied {
		value, err := presentation.Prose("applied: " + name)
		if err != nil { // coverage-ignore: the fixed applied prefix keeps this diagnostic field nonempty
			return presentation.Diagnostic{}, err
		}
		field, err := presentation.NewField("migration", value)
		if err != nil { // coverage-ignore: the fixed grammar-valid migration label receives the validated Prose value
			return presentation.Diagnostic{}, err
		}
		changed = append(changed, field)
	}
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
	steps := []presentation.Value{}
	for _, step := range []string{"inspect the changed migration axes", "run awf upgrade --recover if an upgrade journal exists", "restore the project from version control if recovery cannot complete"} {
		value, err := presentation.Prose(step)
		if err != nil { // coverage-ignore: every closed recovery-step literal is nonempty and Prose-normalized
			return presentation.Diagnostic{}, err
		}
		steps = append(steps, value)
	}
	return presentation.Diagnostic{Condition: "upgrade did not reach terminal sync", State: "partial mutation", Changed: changed, Cause: e.cause.Error(), Steps: steps}, nil
}
