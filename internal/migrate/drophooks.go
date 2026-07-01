package migrate

import (
	"os"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// applyDropHooks ports schema 3 → 4: the hook kind is removed (ADR-0032), so the
// `hooks:` enable array is stripped from .awf/config.yaml. A config with no
// `hooks:` key is left unchanged (idempotent). The edit routes through
// config.RemoveKey so config.yaml serialization stays owned by internal/config
// (ADR-0026).
func applyDropHooks(root string) error {
	cfgPath := config.ConfigPath(root)
	src, err := os.ReadFile(cfgPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil { // coverage-ignore: ReadFile faults only on a permission error that the test root bypasses
		return err
	}
	out, err := config.RemoveKey(src, "hooks")
	if err != nil {
		return err
	}
	return os.WriteFile(cfgPath, out, 0o644)
}
