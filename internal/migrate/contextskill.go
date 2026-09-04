package migrate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
)

const (
	contextSkillGeneration = 51
	contextSkillMigration  = "rename repository-context skill to context"
)

var contextSkillSources = []struct {
	old string
	new string
}{
	{old: ".awf/skills/repository-context.yaml", new: ".awf/skills/context.yaml"},
	{old: ".awf/skills/parts/repository-context/orient.md", new: ".awf/skills/parts/context/orient.md"},
	{old: ".awf/skills/parts/repository-context/explore.md", new: ".awf/skills/parts/context/explore.md"},
	{old: ".awf/skills/parts/repository-context/challenge.md", new: ".awf/skills/parts/context/challenge.md"},
}

func renameRepositoryContextSkill(ctx context.Context, tree *proposedTree, changes *Changes) ([]fileMutation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var planned []fileMutation
	for _, source := range contextSkillSources {
		oldContent, oldMode, err := tree.Read(source.old)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", source.old, err)
		}

		newContent, newMode, newErr := tree.Read(source.new)
		switch {
		case errors.Is(newErr, fs.ErrNotExist):
			planned = append(planned,
				fileMutation{Path: source.new, Content: oldContent, Mode: oldMode},
				fileMutation{Path: source.old, Remove: true},
			)
		case newErr != nil:
			return nil, fmt.Errorf("read %s: %w", source.new, newErr)
		case bytes.Equal(oldContent, newContent) && oldMode == newMode:
			planned = append(planned, fileMutation{Path: source.old, Remove: true})
		default:
			return nil, fmt.Errorf("cannot migrate %s: %s already exists with different content or mode; reconcile the two authored sources and retry", source.old, source.new)
		}
	}
	changes.items = append(changes.items, Change{Text: "renamed the repository-context skill to context"})
	return planned, nil
}
