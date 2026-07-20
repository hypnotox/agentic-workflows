package migrate

import (
	"os"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// loadForMigration parses the project config for a migration's analysis, first
// stripping the retired top-level `invariants` block. That block was valid in the
// schemas these pre-cutover migrations run against but is absent from the current
// strict config.Config, so a plain config.Load would reject a config the migration
// must still read; the current-state-topic-substrate migration (schema 14) removes
// the block from the file itself. Callers os.Stat the config first, so a missing
// file never reaches here.
func loadForMigration(root string) (*config.Config, error) {
	src, err := os.ReadFile(config.ConfigPath(root))
	if err != nil { // coverage-ignore: every caller os.Stats the config first; only a race between the stat and this read faults
		return nil, err
	}
	src, err = config.RemoveKey(src, "invariants")
	if err != nil {
		return nil, err
	}
	cfg, err := config.Parse(config.RootDir(root), src)
	if err != nil { // coverage-ignore: RemoveKey's parse above already rejected any non-mapping YAML, and no schema-valid mapping reaching a migration fails strict decode
		return nil, err
	}
	return cfg, nil
}

// editConfig applies mutate to the project's config.yaml, routing serialization
// through internal/config (ADR-0026). A config absent on disk is a no-op
// (idempotent re-run safe) - the shared skeleton of the scalar-edit migrations.
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
	// touches-state: config/migrations-and-locks:lock-atomic-save - atomic temp-file+rename write site; proof in manifest_test.go
	return manifest.WriteFileAtomic(cfgPath, out)
}
