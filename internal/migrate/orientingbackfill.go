package migrate

import (
	"os"
	"slices"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// applyOrientingSkillBackfill ports schema 25 -> 26 (ADR-0187): the orienting
// skill becomes the single home of the orientation procedure and the shrunk
// brainstorming template invokes it by name, a prose reference no structural
// edge backs (requires-skills-exact forces RequiresSkills empty, and orienting
// declares no agent or doc requirement), so applyCloseEnabledSet cannot reach
// it. Any config with brainstorming enabled gains orienting; a config without
// brainstorming is untouched. Idempotent; the addition is announced.
func applyOrientingSkillBackfill(root string, out *Changes) error {
	if _, err := os.Stat(config.ConfigPath(root)); os.IsNotExist(err) {
		return nil // no config: nothing to backfill (idempotent re-run safe)
	}
	cfg, err := loadForMigration(root)
	if err != nil {
		return err
	}
	if !slices.Contains(cfg.Skills, "brainstorming") || slices.Contains(cfg.Skills, "orienting") {
		return nil
	}
	return editConfig(root, func(src []byte) ([]byte, error) {
		b, err := config.SetArrayMember(src, "skills", "orienting", true)
		if err != nil { // coverage-ignore: config.Load already parsed this config, so SetArrayMember cannot error here
			return nil, err
		}
		out.Add("orienting-skill-backfill: enabled skill orienting (brainstorming is enabled)")
		return b, nil
	})
}
