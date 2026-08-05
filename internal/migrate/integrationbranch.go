package migrate

import (
	"fmt"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// integrationBranchGeneration is the schema generation that materializes the
// required-explicit integrationBranch key.
const integrationBranchGeneration = 30

// applyIntegrationBranch ports schema 29 -> 30 (ADR-0202 Decision 6): the new
// integrationBranch key is required and has no in-code default, so an adopter's
// config must gain a visible `integrationBranch: main` line or fail its own
// validation on the next open. The value is written, never defaulted in code,
// which is what keeps ADR-0127's silent-default removal intact: a project whose
// integration branch is not main sees the wrong value and can correct it, rather
// than inheriting an invisible one. Idempotent - a config that already carries
// the key is left byte-identical, so a re-run never overwrites a corrected
// value - and the write is announced.
func applyIntegrationBranch(root string, out *Changes) error {
	if _, err := os.Stat(config.ConfigPath(root)); os.IsNotExist(err) {
		return nil // no config: nothing to write (idempotent re-run safe)
	}
	cfg, err := loadForMigration(root)
	if err != nil {
		return err
	}
	if cfg.IntegrationBranch != "" {
		return nil
	}
	return editConfig(root, func(src []byte) ([]byte, error) {
		b, err := config.SetString(src, "integrationBranch", "main")
		if err != nil { // coverage-ignore: loadForMigration already parsed this config, so SetString cannot error here
			return nil, err
		}
		fmt.Fprintln(out, "integration-branch-explicit: set integrationBranch: main")
		return b, nil
	})
}
