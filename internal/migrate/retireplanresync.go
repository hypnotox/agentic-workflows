package migrate

import (
	"github.com/hypnotox/agentic-workflows/internal/config"
)

const retiredPlanResyncSkill = "reviewing-plan-resync"

// applyRetirePlanResync removes the retired standard skill selection before any
// current catalog consumer validates the config. The config editor owns YAML
// parsing and serialization; this migration only owns the retirement policy.
func applyRetirePlanResync(root string, out *Changes) error {
	return editConfig(root, out, func(src []byte, planned *Changes) ([]byte, error) {
		updated, removed, err := removePlanResyncSelection(src)
		if err != nil {
			return nil, err
		}
		if removed {
			planned.Add("retire-plan-resync: removed reviewing-plan-resync from skills")
		}
		return updated, nil
	})
}

func removePlanResyncSelection(src []byte) ([]byte, bool, error) {
	return config.RemoveArrayMember(src, "skills", retiredPlanResyncSkill)
}
