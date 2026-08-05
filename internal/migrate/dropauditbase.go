package migrate

import (
	"bytes"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// applyDropAuditBase ports schema 10 -> 11: the audit.baseBranch key is removed
// (ADR-0127), so awf holds no opinion about which branch an adopter integrates
// into. config.yaml is strict-parsed, so a surviving key would hard-fail on the
// new binary rather than warn. Unlike the silent applyDropHooks precedent this
// collects the removal fact: deleting a value an adopter deliberately set must
// remain available to terminal-owner presentation rather than git archaeology. The
// edit routes through config.RemoveMappingKey so config.yaml serialization stays
// owned by internal/config (ADR-0026); RemoveKey cannot be used, since it walks
// only top-level entries and baseBranch is nested under audit.
func applyDropAuditBase(root string, w *Changes) error {
	return editConfig(root, w, func(src []byte, planned *Changes) ([]byte, error) {
		out, err := config.RemoveMappingKey(src, "audit", "baseBranch")
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(out, src) {
			planned.Add("drop-audit-base: removed audit.baseBranch")
		}
		return out, nil
	})
}
