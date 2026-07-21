package migrate

import (
	"bytes"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

const defaultMaxClaimsPerTopic = 20

// applyTopicClaimBudget ports schema 15 to 16 by making the backward-compatible
// topic claim-count advisory default explicit in adopted config.
func applyTopicClaimBudget(root string, out io.Writer) error {
	return editConfig(root, func(src []byte) ([]byte, error) {
		edited, err := config.SetMappingInteger(src, "currentState", "maxClaimsPerTopic", defaultMaxClaimsPerTopic)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(edited, src) {
			fmt.Fprintln(out, "topic-claim-budget: set currentState.maxClaimsPerTopic to 20")
		}
		return edited, nil
	})
}
