package migrate

import (
	"io"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"gopkg.in/yaml.v3"
)

// applyDropHooks ports schema 3 → 4: the hook kind is removed (ADR-0032), so the
// legacy `hooks:` enable *array* is stripped from .awf/config.yaml. A config
// with no `hooks:` key is left unchanged (idempotent), and the modern ADR-0048
// `hooks: {enabled: ...}` mapping is not this migration's shape - it survives a
// replay from a degraded lock. The edit routes through config.RemoveKey so
// config.yaml serialization stays owned by internal/config (ADR-0026).
func applyDropHooks(root string, _ io.Writer) error {
	return editConfig(root, func(src []byte) ([]byte, error) {
		// A typed-probe error on a parseable document means hooks is not the
		// legacy array (the modern mapping mis-types here) - leave it alone. A
		// genuinely malformed document falls through so RemoveKey surfaces its
		// parse error.
		var probe struct {
			Hooks []string `yaml:"hooks"`
		}
		if yaml.Unmarshal(src, &probe) != nil {
			var doc map[string]any
			if yaml.Unmarshal(src, &doc) == nil {
				return src, nil
			}
		}
		return config.RemoveKey(src, "hooks")
	})
}
