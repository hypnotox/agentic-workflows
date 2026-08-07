package migrate

import (
	"github.com/hypnotox/agentic-workflows/internal/config"
)

const retiredPlanResyncSkill = "reviewing-plan-resync"

// applyRetirePlanResync removes the retired standard skill selection before any
// current catalog consumer validates the config, then completes the generation-39
// selection-surface retirement for a tree stamped at 39 without having received
// that independently landed migration. The shared selection operation preserves
// its config and sidecar preflight and atomic-write boundaries.
func applyRetirePlanResync(root string, out *Changes) error {
	operation := productionDropSelectionOperation()
	edits, err := preflightSidecarLocal(root, operation)
	if err != nil {
		return err
	}
	if err := operation.configEditor.editConfig(root, out, func(src []byte, planned *Changes) ([]byte, error) {
		updated, removed, err := removePlanResyncSelection(src)
		if err != nil {
			return nil, err
		}
		if removed {
			planned.Add("retire-plan-resync: removed reviewing-plan-resync from skills")
		}
		return removeSelectionConfig(updated, planned, operation.removeKey)
	}); err != nil {
		return err
	}
	return writeSidecarLocal(edits, out, operation)
}

func removePlanResyncSelection(src []byte) ([]byte, bool, error) {
	return config.RemoveArrayMember(src, "skills", retiredPlanResyncSkill)
}
