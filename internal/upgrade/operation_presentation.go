package upgrade

import "github.com/hypnotox/agentic-workflows/internal/presentation"

func upgradeMutation(sync presentation.Mutation, applied []string, changes []string) (presentation.Mutation, error) {
	migrationValues := make([]presentation.Value, len(applied))
	for i, name := range applied {
		value, err := presentation.Prose(name)
		if err != nil {
			return presentation.Mutation{}, err
		}
		migrationValues[i] = value
	}
	changeValues := make([]presentation.Value, len(changes))
	for i, change := range changes {
		value, err := presentation.Prose(change)
		if err != nil {
			return presentation.Mutation{}, err
		}
		changeValues[i] = value
	}
	prefix := make([]presentation.MutationChange, 0, 2)
	if len(migrationValues) > 0 {
		prefix = append(prefix, presentation.MutationChange{Label: "applied migrations", Values: migrationValues})
	}
	if len(changeValues) > 0 {
		prefix = append(prefix, presentation.MutationChange{Label: "migration changes", Values: changeValues})
	}
	sync.Changes = append(prefix, sync.Changes...)
	return sync, nil
}

type upgradeFailure struct {
	migration MigrationResult
	sync      presentation.Mutation
	cause     error
}

func newUpgradeFailure(migration MigrationResult, sync presentation.Mutation, cause error) error {
	migration.Planned = append([]string(nil), migration.Planned...)
	migration.Applied = append([]string(nil), migration.Applied...)
	migration.Changes = append([]string(nil), migration.Changes...)
	migration.Touched = append([]string(nil), migration.Touched...)
	migration.Pending = append([]string(nil), migration.Pending...)
	return upgradeFailure{migration: migration, sync: sync, cause: cause}
}

func (e upgradeFailure) Error() string { return e.cause.Error() }
func (e upgradeFailure) Unwrap() error { return e.cause }

func proseField(label, text string) (presentation.Field, error) {
	value, err := presentation.Prose(text)
	if err != nil {
		return presentation.Field{}, err
	}
	return presentation.NewField(label, value)
}

func literalField(label, text string) (presentation.Field, error) {
	value, err := presentation.Literal(text)
	if err != nil {
		return presentation.Field{}, err
	}
	return presentation.NewField(label, value)
}

func (e upgradeFailure) Diagnostic() (presentation.Diagnostic, error) {
	planned := make([]presentation.Field, 0, len(e.migration.Planned)+len(e.migration.Pending))
	for _, name := range e.migration.Planned {
		field, err := proseField("planned migration", name)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		planned = append(planned, field)
	}
	for _, name := range e.migration.Pending {
		field, err := literalField("pending path", name)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		planned = append(planned, field)
	}
	changed := make([]presentation.Field, 0, len(e.migration.Applied)+len(e.migration.Changes)+len(e.migration.Touched))
	for _, name := range e.migration.Applied {
		field, err := proseField("applied migration", name)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		changed = append(changed, field)
	}
	for _, change := range e.migration.Changes {
		label, destination := "migration", &changed
		if len(e.migration.Applied) == 0 && len(e.migration.Touched) == 0 {
			label, destination = "migration change", &planned
		}
		field, err := proseField(label, change)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		*destination = append(*destination, field)
	}
	for _, name := range e.migration.Touched {
		field, err := literalField("touched path", name)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		changed = append(changed, field)
	}
	for _, group := range e.sync.Changes {
		for _, value := range group.Values {
			field, err := presentation.NewField(group.Label, value)
			if err != nil {
				return presentation.Diagnostic{}, err
			}
			changed = append(changed, field)
		}
	}
	stepTexts := []string{"correct the reported cause", "run git status --short and git diff to inspect visible changes", "restore desired paths with Git if wanted", "rerun awf upgrade"}
	steps := make([]presentation.Value, len(stepTexts))
	for i, text := range stepTexts {
		value, err := presentation.Prose(text)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		steps[i] = value
	}
	return presentation.Diagnostic{Condition: "upgrade did not complete", State: "operation", Planned: planned, Changed: changed, Cause: e.cause.Error(), Steps: steps}, nil
}
