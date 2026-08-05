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

func upgradeMutation(sync presentation.Mutation, changes []migrate.Change) (presentation.Mutation, error) {
	if len(changes) == 0 {
		return sync, nil
	}
	values := make([]presentation.Value, len(changes))
	for i, change := range changes {
		value, err := presentation.Prose(change.Text)
		if err != nil {
			return presentation.Mutation{}, err
		}
		values[i] = value
	}
	sync.Changes = append([]presentation.MutationChange{{Label: "migrations", Values: values}}, sync.Changes...)
	return sync, nil
}
