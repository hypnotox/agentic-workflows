package migrate

import (
	"github.com/hypnotox/agentic-workflows/internal/config"
)

// applyEnableBootstrap ports schema 4 → 5: the self-pinning bootstrap artifact is
// added (ADR-0040), enabled by default for ported configs so an upgraded project
// gets the new default. It writes `bootstrap:\n  enabled: true` via
// config.SetMappingScalar so config.yaml serialization stays owned by
// internal/config (ADR-0026). A config absent on disk is a no-op (idempotent
// re-run safe); a config that already carries bootstrap.enabled is overwritten to
// true (the upgrade default).
func applyEnableBootstrap(root string) error {
	return editConfig(root, func(src []byte) ([]byte, error) {
		return config.SetMappingScalar(src, "bootstrap", "enabled", true)
	})
}
