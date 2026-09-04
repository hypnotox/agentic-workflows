package migrate

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

const (
	workflowSurfaceGeneration = 53
	workflowSurfaceMigration  = "retire duplicate code-design guide and rename advanced workflow section"
)

var maintainableCodeDesignSections = []string{
	"decision-posture", "contextual-heuristics", "semantic-modeling", "readability",
	"boundaries-and-dependencies", "pattern-toolbox", "preparatory-refactoring", "failure-modes",
}

func singletonSourcePaths(name string, sections []string) []string {
	paths := []string{fmt.Sprintf(".awf/%s.yaml", name)}
	for _, section := range sections {
		paths = append(paths, fmt.Sprintf(".awf/parts/%s/%s.md", name, section))
	}
	return paths
}

func retireWorkflowSurface(ctx context.Context, tree *ProposedTree, changes *Changes) ([]FileMutation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var planned []FileMutation
	backup := func(source string) error {
		moves, err := planAuthoredBackup(ctx, tree, source, planned)
		if err != nil {
			return err
		}
		planned = append(planned, moves...)
		for _, mutation := range moves {
			if !mutation.Remove {
				changes.items = append(changes.items, Change{Text: fmt.Sprintf("preserved retired authored source at %s; review its content, then delete the backup when no longer needed and remove its retired parent directory if empty", mutation.Path)})
			}
		}
		return nil
	}

	for _, source := range singletonSourcePaths("maintainable-code-design", maintainableCodeDesignSections) {
		if err := backup(source); err != nil {
			return nil, err
		}
	}
	if err := backup(".awf/parts/working-with-awf/model-selection.md"); err != nil {
		return nil, err
	}

	const workingSidecar = ".awf/working-with-awf.yaml"
	content, mode, err := tree.Read(workingSidecar)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, fmt.Errorf("read %s: %w", workingSidecar, err)
	default:
		edited, present, changed, editErr := config.EditSidecar(content, config.SidecarEdit{Field: "sections.model-selection", Mode: "reset"})
		if editErr != nil {
			return nil, fmt.Errorf("remove retired section setting from %s: %w", workingSidecar, editErr)
		}
		if changed {
			if err := backup(workingSidecar); err != nil {
				return nil, err
			}
			if present {
				planned = append(planned, FileMutation{Path: workingSidecar, Content: edited, Mode: mode})
			}
		}
	}

	obsoleteDirs := []string{
		".awf/skills/parts/effort-workflow",
		".awf/skills/parts/current-state",
		".awf/skills/parts/decision-records",
		".awf/skills/parts/using-awf",
		".awf/parts/maintainable-code-design",
	}
	for _, source := range removedSkillSources {
		obsoleteDirs = append(obsoleteDirs, ".awf/skills/parts/"+source.name)
	}
	for _, source := range removedAgentSources {
		obsoleteDirs = append(obsoleteDirs, ".awf/agents/parts/"+source.name)
	}
	obsoleteDirs = append(obsoleteDirs, ".awf/agents/parts", ".awf/agents")
	for _, dir := range obsoleteDirs {
		prune, ok, pruneErr := tree.PlanEmptyDirectory(dir, planned)
		if pruneErr != nil {
			return nil, fmt.Errorf("plan obsolete directory cleanup %s: %w", dir, pruneErr)
		}
		if !ok {
			continue
		}
		planned = append(planned, prune)
		changes.items = append(changes.items, Change{Text: fmt.Sprintf("removed empty obsolete source directory %s", filepath.ToSlash(dir))})
	}

	changes.items = append(changes.items, Change{Text: "retired AWF's duplicate maintainable-code-design document and renamed working-with-awf section model-selection to advanced-workflow; obsolete section content was not applied to the new section"})
	return planned, nil
}
