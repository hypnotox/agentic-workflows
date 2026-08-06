package migrate

import (
	"errors"
	"os"
	"slices"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// GroundingSkillCollisionError reports that a project-local grounding skill
// occupies the standard name required by the guarded schema-37 backfill.
type GroundingSkillCollisionError struct {
	Path string
}

func (e *GroundingSkillCollisionError) Error() string {
	return "project-local grounding occupies standard skill name: " + e.Path
}

// Diagnostic maps this migration-owned refusal to the central actionable
// presentation model. priorChanges are facts proved by earlier migrations.
func (e *GroundingSkillCollisionError) Diagnostic(priorChanges []Change) (presentation.Diagnostic, error) {
	changed := make([]presentation.Field, 0, len(priorChanges))
	for _, change := range priorChanges {
		if _, err := presentation.Prose(change.Text); err != nil {
			return presentation.Diagnostic{}, err
		}
		value, err := presentation.Prose("change: " + change.Text)
		if err != nil { // coverage-ignore: the same change text was validated above and adding an ASCII prefix cannot invalidate it
			return presentation.Diagnostic{}, err
		}
		field, err := presentation.NewField("migration", value)
		if err != nil { // coverage-ignore: the fixed label is grammar-valid and value was validated immediately above
			return presentation.Diagnostic{}, err
		}
		changed = append(changed, field)
	}
	steps := []string{
		"rename the local grounding artifact at " + e.Path + " and update its selected name, or remove the local override to adopt standard grounding",
		"rerun awf upgrade",
	}
	if len(changed) > 0 {
		steps = append([]string{"inspect the listed changed axes"}, steps...)
	}
	values := make([]presentation.Value, 0, len(steps))
	for _, step := range steps {
		value, err := presentation.Prose(step)
		if err != nil { // coverage-ignore: recovery steps are closed nonempty ASCII strings assembled only from the recorded path
			return presentation.Diagnostic{}, err
		}
		values = append(values, value)
	}
	return presentation.Diagnostic{
		Condition: "project-local grounding at " + e.Path + " occupies the standard name",
		State:     "operation",
		Changed:   changed,
		Steps:     values,
	}, nil
}

// applyGroundingSkillBackfill ports schema 36 -> 37. It adds standard grounding
// only for standard brainstorming, preserving project-local workflow semantics.
func applyGroundingSkillBackfill(root string, out *Changes) error {
	return applyGroundingSkillBackfillWith(root, out, productionConfigEditor())
}

func applyGroundingSkillBackfillWith(root string, out *Changes, editor configEditor) error {
	if _, err := os.Stat(config.ConfigPath(root)); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	cfg, err := loadForMigration(root)
	if err != nil {
		return err
	}
	brainstorming, err := cfg.Sidecar("skills", "brainstorming")
	if err != nil {
		return err
	}
	if brainstorming.Local || !slices.Contains(cfg.Skills, "brainstorming") {
		return nil
	}
	grounding, err := cfg.Sidecar("skills", "grounding")
	if err != nil {
		return err
	}
	if grounding.Local {
		return &GroundingSkillCollisionError{Path: "skills/grounding.yaml"}
	}
	if slices.Contains(cfg.Skills, "grounding") {
		return nil
	}
	return editor.editConfig(root, out, func(src []byte, planned *Changes) ([]byte, error) {
		updated, err := config.SetArrayMember(src, "skills", "grounding", true)
		if err != nil { // coverage-ignore: loadForMigration parsed these unchanged bytes and proved a top-level skills array
			return nil, err
		}
		if string(updated) != string(src) {
			planned.Add("grounding-skill-backfill: enabled skill grounding (standard brainstorming is enabled)")
		}
		return updated, nil
	})
}
