package migrate

import (
	"bytes"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// applyDropSeveritySettings ports schema 23 -> 24: currentState.topicCoverage
// and currentState.topicFanout are removed (ADR-0179), so topic coverage and
// fan-out always evaluate at ranks fixed in code. config.yaml is strict-parsed,
// so a surviving key would hard-fail on the new binary rather than warn. Each
// removal is announced for the applyDropAuditBase reason: deleting a value an
// adopter deliberately set must be readable from command output rather than
// recovered by git archaeology. The edit routes through RemoveMappingKey because
// both keys are nested under currentState, which RemoveKey cannot reach.
func applyDropSeveritySettings(root string, w io.Writer) error {
	return editConfig(root, func(src []byte) ([]byte, error) {
		out := src
		for _, key := range []string{"topicCoverage", "topicFanout"} {
			next, err := config.RemoveMappingKey(out, "currentState", key)
			if err != nil {
				return nil, err
			}
			if !bytes.Equal(next, out) {
				fmt.Fprintf(w, "drop-severity-settings: removed currentState.%s\n", key)
			}
			out = next
		}
		return out, nil
	})
}
