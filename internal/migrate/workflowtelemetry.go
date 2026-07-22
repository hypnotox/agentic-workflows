package migrate

import (
	"bytes"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"gopkg.in/yaml.v3"
)

// applyWorkflowTelemetry ports schema 16 to 17 by materializing the complete
// tracked workflow telemetry configuration without replacing explicit leaves.
func applyWorkflowTelemetry(root string, out io.Writer) error {
	return editConfig(root, func(src []byte) ([]byte, error) {
		var doc yaml.Node
		if err := yaml.Unmarshal(src, &doc); err != nil {
			return nil, fmt.Errorf("config: parse: %w", err)
		}
		changed, err := config.EnsureWorkflowTelemetryDefaults(&doc)
		if err != nil {
			return nil, err
		}
		if !changed {
			return src, nil
		}
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(&doc); err != nil { // coverage-ignore: doc was decoded from valid YAML
			return nil, err
		}
		_ = enc.Close()
		fmt.Fprintln(out, "workflow-telemetry: added workflowTelemetry defaults")
		return buf.Bytes(), nil
	})
}
