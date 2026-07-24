package migrate

import (
	"io"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"gopkg.in/yaml.v3"
)

// applyEnableRunner ports schema 17 → 18: the runner singleton becomes the pure
// awf wrapper `awf` and turns default-on (ADR-0156), enabled by default for
// ported configs so an upgraded project gets the new default. It writes
// `runner:\n  enabled: true` via config.SetMappingScalar so config.yaml
// serialization stays owned by internal/config (ADR-0026). A config absent on
// disk is a no-op (idempotent re-run safe), and a config that already carries a
// runner key made a choice - a replay from a degraded lock must not override a
// deliberate opt-out.
func applyEnableRunner(root string, _ io.Writer) error {
	return editConfig(root, func(src []byte) ([]byte, error) {
		var doc map[string]any
		if yaml.Unmarshal(src, &doc) == nil {
			if _, ok := doc["runner"]; ok {
				return src, nil
			}
		}
		return config.SetMappingScalar(src, "runner", "enabled", true)
	})
}
