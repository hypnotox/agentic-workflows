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
	// ADR-0194 retired currentState.maxClaimsPerTopic, but the generation-16
	// migration that writes it is retained, so a tree upgrading from before 16
	// carries the key through every intervening generation until 28 removes it.
	// A migration in between (generation 23 closes an enabled set) parses here
	// and would hard-fail on a key the current schema no longer declares. Strip
	// it for the same reason the invariants block is stripped above: a migration
	// reads a config at its historical shape, not at the current schema's.
	src, err = config.RemoveMappingKey(src, "currentState", "maxClaimsPerTopic")
	if err != nil { // coverage-ignore: RemoveKey's parse above already rejected any non-mapping YAML, so this removal cannot reach a parse error
		return nil, err
	}
	// Generation 38 removes the gate and audit settings from the live shape,
	// while older frozen steps can still create or inspect their historical
	// representation. Strip them only from this typed analysis view; the bytes
	// the frozen step edits remain unchanged until generation 38 applies.
	for _, retired := range gateAuditRetiredKeys {
		if retired.parent == "" {
			src, err = config.RemoveKey(src, retired.key)
		} else {
			src, err = config.RemoveMappingKey(src, retired.parent, retired.key)
		}
		if err != nil { // coverage-ignore: the first removal parses the whole mapping, and each successful removal re-encodes valid YAML before the next iteration
			return nil, err
		}
	}
	cfg, err := config.Parse(config.RootDir(root), src)
	if err != nil { // coverage-ignore: RemoveKey's parse above already rejected any non-mapping YAML, and no schema-valid mapping reaching a migration fails strict decode
		return nil, err
	}
	return cfg, nil
}

// configEditor owns one config edit's volatile write dependency. Each migration
// creates its own production editor; tests compose a faulting editor into the
// operation they exercise rather than replacing package state.
type configEditor struct {
	writeAtomic func(string, []byte) error
}

func productionConfigEditor() configEditor {
	return configEditor{writeAtomic: manifest.WriteFileAtomic}
}

// editConfig applies mutate to the project's config.yaml, routing serialization
// through internal/config (ADR-0026). A config absent on disk is a no-op
// (idempotent re-run safe) - the shared skeleton of the scalar-edit migrations.
func (e configEditor) editConfig(root string, out *Changes, mutate func(src []byte, planned *Changes) ([]byte, error)) error {
	cfgPath := config.ConfigPath(root)
	src, err := os.ReadFile(cfgPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil { // coverage-ignore: ReadFile faults only on a permission error that the test root bypasses
		return err
	}
	planned := &Changes{}
	updated, err := mutate(src, planned)
	if err != nil {
		return err
	}
	// touches-state: config/migrations-and-locks:lock-atomic-save - atomic temp-file+rename write site; proof in manifest_test.go
	if err := e.writeAtomic(cfgPath, updated); err != nil {
		return err
	}
	for _, change := range planned.Items() {
		out.Add(change.Text)
	}
	return nil
}

func editConfig(root string, out *Changes, mutate func(src []byte, planned *Changes) ([]byte, error)) error {
	return productionConfigEditor().editConfig(root, out, mutate)
}
