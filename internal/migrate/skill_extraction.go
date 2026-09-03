package migrate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
)

const (
	skillExtractionGeneration = 52
	skillExtractionMigration  = "extract generic skills and rename AWF skills"
)

type skillSourceMigration struct {
	old, new string
	sections []string
}

var retainedSkillSources = []skillSourceMigration{
	{old: "effort-workflow", new: "awf-effort", sections: []string{"continuity-and-resident", "execution-and-checkpoints", "integration-and-recovery", "close"}},
	{old: "current-state", new: "awf-topics", sections: []string{"claims"}},
	{old: "decision-records", new: "awf-decisions", sections: []string{"format"}},
	{old: "using-awf", new: "awf-maintenance", sections: []string{"generated-documents", "upgrades"}},
}

type removedArtifactSource struct {
	name     string
	sections []string
}

var removedSkillSources = []removedArtifactSource{
	{name: "context", sections: []string{"orient", "explore", "challenge"}},
	{name: "brainstorming", sections: []string{"procedure"}},
	{name: "debugging", sections: []string{"oracle-and-handoff"}},
	{name: "implementing", sections: []string{"ownership", "procedure", "review-handoff"}},
	{name: "planning", sections: []string{"shape"}},
	{name: "reviewing", sections: []string{"brief"}},
	{name: "refactor-scope", sections: []string{"inventory"}},
}

var removedAgentSources = []removedArtifactSource{
	{name: "explorer", sections: []string{"scope", "report"}},
	{name: "premise-checker", sections: []string{"procedure", "report"}},
	{name: "implementer", sections: []string{"authority", "work", "receipt"}},
	{name: "reviewer", sections: []string{"review", "report"}},
}

func artifactSourcePaths(kind, name string, sections []string) []string {
	paths := []string{fmt.Sprintf(".awf/%s/%s.yaml", kind, name)}
	for _, section := range sections {
		paths = append(paths, fmt.Sprintf(".awf/%s/parts/%s/%s.md", kind, name, section))
	}
	return paths
}

func skillSourcePaths(name string, sections []string) []string {
	return artifactSourcePaths("skills", name, sections)
}

func migrateExtractedSkills(ctx context.Context, tree *ProposedTree, changes *Changes) ([]FileMutation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var planned []FileMutation
	for _, migration := range retainedSkillSources {
		oldPaths := skillSourcePaths(migration.old, migration.sections)
		newPaths := skillSourcePaths(migration.new, migration.sections)
		for i, oldPath := range oldPaths {
			moves, err := planAuthoredMove(ctx, tree, oldPath, newPaths[i])
			if err != nil {
				return nil, err
			}
			planned = append(planned, moves...)
		}
	}
	for _, family := range []struct {
		kind    string
		sources []removedArtifactSource
	}{
		{kind: "skills", sources: removedSkillSources},
		{kind: "agents", sources: removedAgentSources},
	} {
		for _, removed := range family.sources {
			for _, source := range artifactSourcePaths(family.kind, removed.name, removed.sections) {
				moves, err := planAuthoredBackup(ctx, tree, source, planned)
				if err != nil {
					return nil, err
				}
				planned = append(planned, moves...)
			}
		}
	}
	changes.items = append(changes.items, Change{Text: "renamed AWF skills and preserved extracted generic skill and role overrides as .awf-bak files"})
	return planned, nil
}

func planAuthoredMove(ctx context.Context, tree *ProposedTree, oldPath, newPath string) ([]FileMutation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	oldContent, oldMode, err := tree.Read(oldPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", oldPath, err)
	}
	newContent, newMode, newErr := tree.Read(newPath)
	switch {
	case errors.Is(newErr, fs.ErrNotExist):
		return []FileMutation{
			{Path: newPath, Content: oldContent, Mode: oldMode},
			{Path: oldPath, Remove: true},
		}, nil
	case newErr != nil:
		return nil, fmt.Errorf("read %s: %w", newPath, newErr)
	case bytes.Equal(oldContent, newContent) && oldMode == newMode:
		return []FileMutation{{Path: oldPath, Remove: true}}, nil
	default:
		return nil, fmt.Errorf("cannot migrate %s: %s already exists with different content or mode; reconcile the two authored sources and retry", oldPath, newPath)
	}
}

func planAuthoredBackup(ctx context.Context, tree *ProposedTree, source string, planned []FileMutation) ([]FileMutation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	content, mode, err := tree.Read(source)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", source, err)
	}
	occupied := make(map[string]bool, len(planned))
	for _, mutation := range planned {
		if !mutation.Remove {
			occupied[mutation.Path] = true
		}
	}
	for n := 0; ; n++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		destination := source + ".awf-bak"
		if n > 0 {
			destination = fmt.Sprintf("%s.%d", destination, n)
		}
		if occupied[destination] {
			continue
		}
		_, _, readErr := tree.Read(destination)
		if readErr == nil {
			continue
		}
		if !errors.Is(readErr, fs.ErrNotExist) {
			return nil, fmt.Errorf("inspect recovery destination %s: %w", destination, readErr)
		}
		return []FileMutation{
			{Path: destination, Content: content, Mode: mode},
			{Path: source, Remove: true},
		}, nil
	}
}
