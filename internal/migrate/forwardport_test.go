package migrate

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// retiredConfigKeys is every config key a historical migration once wrote (or an
// adopter once legitimately set) that the current schema no longer declares.
//
// MAINTENANCE OBLIGATION: a migration that retires a config key adds the key
// here and to retiredKeyRemovals. That is the whole point of this test, so do
// not skip it.
var retiredConfigKeys = []struct {
	name     string
	fragment string
}{
	{"workflowTelemetry", "workflowTelemetry:\n  enabled: true\n"},
	{"topicCoverage", "currentState:\n  topicCoverage: error\n"},
	{"topicFanout", "currentState:\n  topicFanout: warn\n"},
	{"maxClaimsPerTopic", "currentState:\n  maxClaimsPerTopic: 20\n"},
}

// TestConfigForCurrentSchemaParsesEveryRetiredKey is the deterministic backstop
// for a failure the per-key forward-port tests structurally cannot catch: they
// each assert their own key, so nothing goes red when a NEW key-removing
// migration ships without its ConfigForCurrentSchema branch.
//
// The consequence of missing that branch is remote from its cause. The staged
// check loads the BEFORE side from HEAD, which still carries the key, and
// forward-ports it through ConfigForCurrentSchema for the current strict parser.
// Without a branch the key survives and parsing fails, so the phase-closing
// `awf check --staged` breaks on the very commit that removes the key, while the
// migration's own test passes throughout because it operates on fixture bytes.
//
// docs/pitfalls.md has recorded this since ADR-0183 and it recurred anyway at
// ADR-0194, which is why the rule now lives in a test rather than only in prose.
func TestConfigForCurrentSchemaParsesEveryRetiredKey(t *testing.T) {
	const base = "prefix: example\nskills: []\nagents: []\n"
	for _, tc := range retiredConfigKeys {
		t.Run(tc.name, func(t *testing.T) {
			src := []byte(base + tc.fragment)
			// The current strict parser must reject it, or the fragment is stale
			// and this subtest would pass without proving anything.
			if _, err := config.Parse("staged/.awf", src); err == nil {
				t.Fatalf("%q is still accepted by the current schema; it does not belong in retiredConfigKeys", tc.fragment)
			}
			// Every generation, not only those before the removal. A stamped
			// generation is not proof the removal ever ran: two branches
			// allocating generations concurrently can leave a tree stamped past
			// a removal it never applied, which is how ADR-0202's integration
			// produced a config carrying a retired key at a later stamp.
			for from := 1; from <= Current(); from++ {
				ported, err := ConfigForCurrentSchema(src, from)
				if err != nil {
					t.Fatalf("from generation %d: %v", from, err)
				}
				if _, err := config.Parse("staged/.awf", ported); err != nil {
					t.Fatalf("from generation %d: forward-ported config does not parse: %v\nport produced %q", from, err, ported)
				}
			}
		})
	}
}
