package migrate

import (
	"bytes"
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// defaultMaxTopicsPerPath mirrors the implicit fan-out budget
// config.CurrentStateConfig.EffectiveMaxTopicsPerPath applies. It is written
// explicitly only to keep an otherwise-emptied block alive; see
// applyDropSeveritySettings.
const defaultMaxTopicsPerPath = 8

// applyDropSeveritySettings ports schema 24 -> 25: currentState.topicCoverage
// and currentState.topicFanout are removed (ADR-0183), so topic coverage and
// fan-out always evaluate at ranks fixed in code. config.yaml is strict-parsed,
// so a surviving key would hard-fail on the new binary rather than warn. Each
// removal is announced for the applyDropAuditBase reason: deleting a value an
// adopter deliberately set must be readable from command output rather than
// recovered by git archaeology. The edit routes through RemoveMappingKey because
// both keys are nested under currentState, which RemoveKey cannot reach.
//
// When the two keys were the block's only children, RemoveMappingKey drops the
// emptied currentState key with them. At generation 25 an absent block suppressed
// coverage and fan-out outright, because internal/project/currentstate.go gated
// both on CurrentState != nil, which was the exact inverse of ADR-0183 item 1.
// This migration therefore seeds the explicit default budget to keep the block
// alive and the checks evaluating. ADR-0192 removed that gate, so the seed no
// longer protects anything: it is inert but harmless, and it stays because
// historical migrations are never edited.
//
// The announcement below still says the seed keeps coverage and fan-out
// evaluating. That wording is stale for the same reason and is retained
// deliberately: it is command output and therefore behaviour, frozen by ADR-0192
// item 7. Nothing would go red if it were reworded, because the migration test
// pins only the announcement prefix by substring and never the trailing clause,
// which is precisely why retaining it has to be a recorded choice rather than
// something the suite backstops.
func applyDropSeveritySettings(root string, w *Changes) error {
	return editConfig(root, func(src []byte) ([]byte, error) {
		// The removals run first so a malformed config surfaces its parse error
		// here, on the path every tree takes, rather than from one of the
		// presence probes below.
		out, err := dropSeverityKeys(src, w)
		if err != nil {
			return nil, err
		}
		stillHasBlock, err := config.HasMapping(out, "currentState")
		if err != nil { // coverage-ignore: out was produced by the removals above, so src already parsed
			return nil, err
		}
		if stillHasBlock {
			return out, nil
		}
		hadBlock, err := config.HasMapping(src, "currentState")
		if err != nil { // coverage-ignore: src already parsed in the removals above
			return nil, err
		}
		if !hadBlock {
			return out, nil
		}
		// Seed into the ORIGINAL bytes and redo the removals, rather than adding
		// the key to the collapsed result: SetMappingInteger appends an absent
		// parent at the end of the document, which would silently relocate the
		// adopter's currentState block. Seeding first keeps it where it was.
		seeded, err := config.SetMappingInteger(src, "currentState", defaultMaxTopicsPerPathKey, defaultMaxTopicsPerPath)
		if err != nil { // coverage-ignore: src parsed above and its currentState is a mapping, so neither error path is reachable
			return nil, err
		}
		out, err = dropSeverityKeys(seeded, &Changes{}) // already announced on the first pass
		if err != nil {                                 // coverage-ignore: seeded is a re-encode of bytes whose removals already succeeded
			return nil, err
		}
		w.Add(fmt.Sprintf("drop-severity-settings: set currentState.%s to %d, keeping coverage and fan-out evaluating\n", defaultMaxTopicsPerPathKey, defaultMaxTopicsPerPath))
		return out, nil
	})
}

// defaultMaxTopicsPerPathKey is the surviving currentState child the seed writes.
const defaultMaxTopicsPerPathKey = "maxTopicsPerPath"

// dropSeverityKeys removes both retired keys, announcing each removal it makes.
// It is shared by the announcing first pass and the silent re-run the seed needs.
func dropSeverityKeys(src []byte, w *Changes) ([]byte, error) {
	out := src
	for _, key := range []string{"topicCoverage", "topicFanout"} {
		next, err := config.RemoveMappingKey(out, "currentState", key)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(next, out) {
			w.Add(fmt.Sprintf("drop-severity-settings: removed currentState.%s\n", key))
		}
		out = next
	}
	return out, nil
}
