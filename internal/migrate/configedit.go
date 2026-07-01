package migrate

import (
	"os"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// editConfig applies mutate to the project's config.yaml, routing serialization
// through internal/config (ADR-0026). A config absent on disk is a no-op
// (idempotent re-run safe) — the shared skeleton of the scalar-edit migrations.
func editConfig(root string, mutate func(src []byte) ([]byte, error)) error {
	cfgPath := config.ConfigPath(root)
	src, err := os.ReadFile(cfgPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil { // coverage-ignore: ReadFile faults only on a permission error that the test root bypasses
		return err
	}
	out, err := mutate(src)
	if err != nil {
		return err
	}
	return os.WriteFile(cfgPath, out, 0o644)
}
