package migrate

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// retiredConfigKeys is every config key a historical migration once wrote (or an
// adopter once legitimately set) that the current schema no longer declares,
// paired with the generation whose migration removed it.
//
// MAINTENANCE OBLIGATION: a migration that retires a config key adds the key
// here with its own generation. That is the whole point of this test, so do not
// skip it.
var retiredConfigKeys = []struct {
	name      string
	fragment  string
	removedAt int
}{
	{"workflowTelemetry", "workflowTelemetry:\n  enabled: true\n", 20},
	{"topicCoverage", "currentState:\n  topicCoverage: error\n", 25},
	{"topicFanout", "currentState:\n  topicFanout: warn\n", 25},
	{"maxClaimsPerTopic", "currentState:\n  maxClaimsPerTopic: 20\n", 28},
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
			// Only generations BEFORE the removal can still carry the key: a tree
			// already at removedAt has had it removed, so forward-porting from
			// there correctly skips the branch and is not a reachable state.
			for from := 1; from < tc.removedAt; from++ {
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
