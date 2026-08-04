package project

import (
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// SyncMutation maps the completed sync outcome into presentation-owned syntax.
// Backup ownership and output provenance stay semantic facts of this package.
func SyncMutation(backups []Backup, changes []Change, pruned []string) (presentation.Mutation, error) {
	groups := make([]presentation.MutationChange, 0, 3)
	if len(backups) > 0 {
		values := make([]presentation.Value, 0, len(backups))
		for _, backup := range backups {
			value, err := presentation.Prose(fmt.Sprintf("%s to %s", backup.Path, backup.Bak))
			if err != nil { // coverage-ignore: typed result values and fixed presentation grammar are validated before this mapping
				return presentation.Mutation{}, err
			}
			values = append(values, value)
		}
		groups = append(groups, presentation.MutationChange{Label: "backups", Values: values})
	}
	if len(changes) > 0 {
		values := make([]presentation.Value, 0, len(changes))
		for _, change := range changes {
			text := change.Path
			if change.Cause == "added" {
				text = "added " + text
			} else {
				text = fmt.Sprintf("changed %s (%s)", text, change.Cause)
			}
			value, err := presentation.Prose(text)
			if err != nil { // coverage-ignore: typed result values and fixed presentation grammar are validated before this mapping
				return presentation.Mutation{}, err
			}
			values = append(values, value)
		}
		groups = append(groups, presentation.MutationChange{Label: "outputs", Values: values})
	}
	if len(pruned) > 0 {
		values := make([]presentation.Value, 0, len(pruned))
		for _, path := range pruned {
			value, err := presentation.Prose(path)
			if err != nil { // coverage-ignore: typed result values and fixed presentation grammar are validated before this mapping
				return presentation.Mutation{}, err
			}
			values = append(values, value)
		}
		groups = append(groups, presentation.MutationChange{Label: "pruned", Values: values})
	}
	notes := []presentation.Value{}
	for _, backup := range backups {
		if backup.Index {
			value, err := presentation.Prose("awf now generates " + backup.Path + "; retire any external generator for it")
			if err != nil { // coverage-ignore: typed result values and fixed presentation grammar are validated before this mapping
				return presentation.Mutation{}, err
			}
			notes = append(notes, value)
		}
	}
	next, err := presentation.Prose("continue with the rendered project state")
	if err != nil { // coverage-ignore: fixed nonempty completion action always validates as prose
		return presentation.Mutation{}, err
	}
	return presentation.Mutation{Status: "completed", Changes: groups, Notes: notes, NextActions: []presentation.Value{next}}, nil
}
