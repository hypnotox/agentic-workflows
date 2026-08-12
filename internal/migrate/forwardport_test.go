package migrate

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// retiredConfigKeyCensus is the independent test oracle for every config key a
// historical migration once wrote (or an adopter once legitimately set) that
// the current schema no longer declares. The production ledger owns removal;
// this census makes omission from that ledger observable.
var retiredConfigKeyCensus = []retiredConfigKey{
	{"", "invariants"},
	{"audit", "baseBranch"},
	{"", "workflowTelemetry"},
	{"currentState", "topicCoverage"},
	{"currentState", "topicFanout"},
	{"currentState", "maxClaimsPerTopic"},
	{"", "hooks"}, {"", "runner"},
	{"proseGate", "enabled"}, {"memoryCite", "enabled"},
	{"audit", "allowedTypes"}, {"audit", "subjectMaxLength"}, {"audit", "diffThreshold"},
	{"audit", "dependencyManifests"}, {"audit", "domainDocStaleness"}, {"audit", "domainCodeStaleness"},
	{"audit", "undocumentedDomain"}, {"audit", "plainPunctuation"}, {"audit", "uncommittedChanges"},
	{"currentState", "maxTopicsPerPath"},
	{"", "skills"}, {"", "agents"}, {"", "docs"}, {"", "targets"}, {"", "docsDir"},
}

func retiredConfigKeyPath(key retiredConfigKey) string {
	return strings.TrimPrefix(key.parent+".", ".") + key.key
}

func retiredKeyCensusError(ledger, census []retiredConfigKey) error {
	ledgerSet := map[string]bool{}
	for _, key := range ledger {
		path := retiredConfigKeyPath(key)
		if ledgerSet[path] {
			return fmt.Errorf("duplicate production ledger entry %s", path)
		}
		ledgerSet[path] = true
	}
	censusSet := map[string]bool{}
	for _, key := range census {
		path := retiredConfigKeyPath(key)
		if censusSet[path] {
			return fmt.Errorf("duplicate census entry %s", path)
		}
		censusSet[path] = true
	}
	for path := range censusSet {
		if !ledgerSet[path] {
			return fmt.Errorf("retired key %s is missing from the production ledger", path)
		}
	}
	for path := range ledgerSet {
		if !censusSet[path] {
			return fmt.Errorf("production ledger entry %s has no retired-key authority", path)
		}
	}
	return nil
}

func TestRetiredKeyCensusRejectsLedgerOmission(t *testing.T) {
	err := retiredKeyCensusError(retiredKeyRemovals[1:], retiredConfigKeyCensus)
	if err == nil || !strings.Contains(err.Error(), "invariants is missing") {
		t.Fatalf("omitted ledger entry error = %v", err)
	}
}

// TestConfigForCurrentSchemaParsesEveryRetiredKey is the deterministic backstop
// for a failure the per-migration tests structurally cannot catch: they each
// assert their own key, so nothing goes red when a key-removing migration ships
// without its ConfigForCurrentSchema ledger entry.
//
// The consequence of missing that entry is remote from its cause. The staged
// check loads the BEFORE side from HEAD, which still carries the key, and
// forward-ports it through ConfigForCurrentSchema for the current strict parser.
// Without an entry the key survives and parsing fails, so the phase-closing
// `awf check staged` breaks on the very commit that removes the key, while the
// migration's own test passes because it operates on fixture bytes.
//
// docs/pitfalls.md has recorded this since ADR-0183 and it recurred at ADR-0194,
// which is why the rule lives in an independently enumerated test oracle.
// invariant: config/migrations-and-locks:retired-keys-forward-ported (TestConfigForCurrentSchemaParsesEveryRetiredKey)
func TestConfigForCurrentSchemaParsesEveryRetiredKey(t *testing.T) {
	if err := retiredKeyCensusError(retiredKeyRemovals, retiredConfigKeyCensus); err != nil {
		t.Fatal(err)
	}
	const base = "prefix: example\nintegrationBranch: main\n"
	for _, retired := range retiredKeyRemovals {
		t.Run(retiredConfigKeyPath(retired), func(t *testing.T) {
			value := "retired"
			if retired.parent == "" {
				value = "[]"
			}
			fragment := retired.key + ": " + value + "\n"
			if retired.parent != "" {
				fragment = retired.parent + ":\n  " + fragment
			}
			src := []byte(base + fragment)
			// The current strict parser must reject it, or the ledger entry is stale
			// and this subtest would pass without proving anything.
			if _, err := config.Parse("staged/.awf", src); err == nil {
				t.Fatalf("%q is still accepted by the current schema; it does not belong in retiredKeyRemovals", fragment)
			}
			// Every generation, including the generation-zero legacy boundary. A
			// later stamp is not proof the removal ever ran: concurrent branches can
			// stamp past a migration introduced elsewhere.
			for from := 0; from <= Current(); from++ {
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
