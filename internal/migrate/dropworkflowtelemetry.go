package migrate

import (
	"bytes"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// applyDropWorkflowTelemetry removes the retired, now-consumerless root block
// without reserializing unrelated YAML nodes.
func applyDropWorkflowTelemetry(root string, out io.Writer) error {
	return editConfig(root, func(src []byte) ([]byte, error) {
		next, err := config.RemoveKey(src, "workflowTelemetry")
		if err != nil { // coverage-ignore: migration tests exercise malformed input through the same RemoveKey editor; this callback only receives its error unchanged
			return nil, err
		}
		if !bytes.Equal(src, next) {
			fmt.Fprintln(out, "drop-workflow-telemetry: removed retired workflowTelemetry block")
		}
		return next, nil
	})
}
