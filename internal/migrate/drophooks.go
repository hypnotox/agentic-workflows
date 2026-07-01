package migrate

import (
	"github.com/hypnotox/agentic-workflows/internal/config"
)

// applyDropHooks ports schema 3 → 4: the hook kind is removed (ADR-0032), so the
// `hooks:` enable array is stripped from .awf/config.yaml. A config with no
// `hooks:` key is left unchanged (idempotent). The edit routes through
// config.RemoveKey so config.yaml serialization stays owned by internal/config
// (ADR-0026).
func applyDropHooks(root string) error {
	return editConfig(root, func(src []byte) ([]byte, error) {
		return config.RemoveKey(src, "hooks")
	})
}
