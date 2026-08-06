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
		// A migration Change has already been accepted as presentation prose by
		// the command boundary; the fixed prefix preserves that validity.
		value, _ := presentation.Prose("change: " + change.Text)
		// The fixed grammar-valid label and validated value cannot fail field construction.
		field, _ := presentation.NewField("migration", value)
		changed = append(changed, field)
	}
	steps := []string{
		"rename the local grounding artifact and update its selected name, or remove the local override to adopt standard grounding",
		"rerun awf upgrade",
	}
	if len(changed) > 0 {
		steps = append([]string{"inspect the listed changed axes"}, steps...)
	}
	values := make([]presentation.Value, 0, len(steps))
	for _, step := range steps {
		// These closed, nonempty recovery instructions are presentation-valid.
		value, _ := presentation.Prose(step)
		values = append(values, value)
	}
	return presentation.Diagnostic{
		Condition: "project-local grounding occupies the standard name",
		State:     "operation",
		Changed:   changed,
		Steps:     values,
	}, nil
}

// applyGroundingSkillBackfill ports schema 36 -> 37. It adds standard grounding
// only for standard brainstorming, preserving project-local workflow semantics.
func applyGroundingSkillBackfill(root string, out *Changes) error {
	if _, err := os.Stat(config.ConfigPath(root)); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil { // coverage-ignore: filesystem failure propagation follows the shared migration seam
		return err
	}
	cfg, err := loadForMigration(root)
	if err != nil { // coverage-ignore: malformed-config propagation follows the shared migration seam
		return err
	}
	brainstorming, err := cfg.Sidecar("skills", "brainstorming")
	if err != nil { // coverage-ignore: sidecar-load propagation follows the shared migration seam
		return err
	}
	if brainstorming.Local || !slices.Contains(cfg.Skills, "brainstorming") {
		return nil
	}
	grounding, err := cfg.Sidecar("skills", "grounding")
	if err != nil { // coverage-ignore: sidecar-load propagation follows the shared migration seam
		return err
	}
	if grounding.Local {
		return &GroundingSkillCollisionError{Path: "skills/grounding.yaml"}
	}
	if slices.Contains(cfg.Skills, "grounding") {
		return nil
	}
	return editConfig(root, out, func(src []byte, planned *Changes) ([]byte, error) {
		updated, err := config.SetArrayMember(src, "skills", "grounding", true)
		if err != nil { // coverage-ignore: a strict-parsed skills array cannot fail its deterministic member edit
			return nil, err
		}
		if string(updated) != string(src) {
			planned.Add("grounding-skill-backfill: enabled skill grounding (standard brainstorming is enabled)")
		}
		return updated, nil
	})
}
