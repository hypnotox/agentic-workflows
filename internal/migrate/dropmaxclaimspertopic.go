package migrate

import (
	"bytes"
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// applyDropMaxClaimsPerTopic ports schema 27 -> 28: currentState.maxClaimsPerTopic
// is removed (ADR-0194), so awf check emits no topic claim-count note. config.yaml
// is strict-parsed, so a surviving key would hard-fail on the new binary rather
// than warn. The removal is announced because deleting a value an adopter
// deliberately set must be readable from command output rather than recovered by
// git archaeology. The edit routes through RemoveMappingKey because the key is
// nested under currentState, which RemoveKey cannot reach.
//
// Unlike applyDropSeveritySettings, this migration seeds nothing when the removal
// empties the block. ADR-0192 made topic coverage and fan-out evaluate
// independently of currentState block presence, so a dropped block changes no
// behaviour, and seeding would write back a key the adopter never set.
func applyDropMaxClaimsPerTopic(root string, w *Changes) error {
	return editConfig(root, func(src []byte) ([]byte, error) {
		out, err := config.RemoveMappingKey(src, "currentState", "maxClaimsPerTopic")
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(out, src) {
			fmt.Fprint(w, "drop-max-claims-per-topic: removed currentState.maxClaimsPerTopic\n")
		}
		return out, nil
	})
}
