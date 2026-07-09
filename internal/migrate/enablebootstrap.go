package migrate

import (
	"io"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"gopkg.in/yaml.v3"
)

// applyEnableBootstrap ports schema 4 → 5: the self-pinning bootstrap artifact is
// added (ADR-0040), enabled by default for ported configs so an upgraded project
// gets the new default. It writes `bootstrap:\n  enabled: true` via
// config.SetMappingScalar so config.yaml serialization stays owned by
// internal/config (ADR-0026). A config absent on disk is a no-op (idempotent
// re-run safe), and a config that already carries a bootstrap key made a choice —
// a replay from a degraded lock must not override a deliberate opt-out.
func applyEnableBootstrap(root string, _ io.Writer) error {
	return editConfig(root, func(src []byte) ([]byte, error) {
		var doc map[string]any
		if yaml.Unmarshal(src, &doc) == nil {
			if _, ok := doc["bootstrap"]; ok {
				return src, nil
			}
		}
		return config.SetMappingScalar(src, "bootstrap", "enabled", true)
	})
}
