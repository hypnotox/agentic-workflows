package upgrade

import (
	"errors"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

type journalFailure struct {
	condition string
	outcome   Outcome
	cause     error
}

func newJournalFailure(condition string, outcome Outcome, cause error) error {
	return journalFailure{condition: condition, outcome: outcome, cause: cause}
}

func (e journalFailure) Error() string { return e.cause.Error() }
func (e journalFailure) Unwrap() error { return e.cause }
func (e journalFailure) Diagnostic() (presentation.Diagnostic, error) {
	return e.outcome.FailureDiagnostic(e.condition, e.cause)
}

func upgradeMutation(sync presentation.Mutation, applied []string, changes []string) (presentation.Mutation, error) {
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
		value, err := presentation.Prose(change)
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

// migrationDiagnostic is the narrow migration-to-upgrade presentation bridge
// for migration-owned special diagnostics.
type migrationDiagnostic interface {
	UpgradeDiagnostic([]string) (presentation.Diagnostic, error)
}

type upgradeFailure struct {
	planning bool
	planned  []string
	applied  []string
	changes  []string
	sync     presentation.Mutation
	cause    error
}

func newUpgradePlanningFailure(planned []string, changes []string, cause error) error {
	return upgradeFailure{planning: true, planned: append([]string(nil), planned...), changes: append([]string(nil), changes...), cause: cause}
}

func newUpgradeFailure(applied []string, changes []string, sync presentation.Mutation, cause error) error {
	return upgradeFailure{applied: append([]string(nil), applied...), changes: append([]string(nil), changes...), sync: sync, cause: cause}
}

func (e upgradeFailure) Error() string { return e.cause.Error() }
func (e upgradeFailure) Unwrap() error { return e.cause }
func (e upgradeFailure) Diagnostic() (presentation.Diagnostic, error) {
	var special migrationDiagnostic
	if errors.As(e.cause, &special) {
		return special.UpgradeDiagnostic(e.changes)
	}
	planned := make([]presentation.Field, 0, len(e.planned)+len(e.changes))
	for _, name := range e.planned {
		value, err := presentation.Prose("planned: " + name)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		field, err := presentation.NewField("planned migration", value)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		planned = append(planned, field)
	}
	changed := make([]presentation.Field, 0, len(e.applied)+len(e.changes))
	for _, name := range e.applied {
		value, err := presentation.Prose("applied: " + name)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		field, err := presentation.NewField("applied migration", value)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		changed = append(changed, field)
	}
	for _, change := range e.changes {
		if _, err := presentation.Prose(change); err != nil {
			return presentation.Diagnostic{}, err
		}
		prefix, label := "change: ", "migration"
		destination := &changed
		if e.planning {
			prefix, label, destination = "planned: ", "migration change", &planned
		}
		value, _ := presentation.Prose(prefix + change)
		field, err := presentation.NewField(label, value)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		*destination = append(*destination, field)
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
	stepTexts := []string{"correct the reported cause and retry"}
	if len(changed) > 0 {
		stepTexts = []string{"run awf upgrade --recover if an upgrade journal exists", "inspect the listed changed axes", "restore the project from version control if recovery cannot complete"}
	}
	steps := make([]presentation.Value, 0, len(stepTexts))
	for _, text := range stepTexts {
		value, err := presentation.Prose(text)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		steps = append(steps, value)
	}
	return presentation.Diagnostic{Condition: "upgrade has not reached terminal sync", State: "operation", Planned: planned, Changed: changed, Cause: e.cause.Error(), Steps: steps}, nil
}
