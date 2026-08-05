package migrate

import (
	"bytes"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

// applyWorkflowTelemetry is retained exclusively as the schema-17 historical
// upgrade. Generation 20 subsequently removes the block it materializes.
func applyWorkflowTelemetry(root string, out *Changes) error {
	return editConfig(root, func(src []byte) ([]byte, error) {
		var doc yaml.Node
		if err := yaml.Unmarshal(src, &doc); err != nil {
			return nil, fmt.Errorf("config: parse: %w", err)
		}
		if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode { // coverage-ignore: historical migration only receives config.yaml skeleton mappings
			return nil, errors.New("config: not a YAML mapping")
		}
		root := doc.Content[0]
		for i := 0; i < len(root.Content); i += 2 {
			if root.Content[i].Value == "workflowTelemetry" {
				return src, nil
			}
		}
		var value yaml.Node
		if err := value.Encode(map[string]any{"retention": map[string]int{"maxCompletedEffortAgeDays": 90, "maxCompletedEffortCount": 100}, "widget": map[string]bool{"enabled": true, "showCost": true}, "diagnostics": map[string]any{"heuristicsEnabled": true, "minimumBaselineSamples": 10, "baselinePercentile": 95, "thresholds": map[string]int{"phaseReentryCount": 2, "phaseDurationSeconds": 14400, "phaseTokens": 200000, "compactionCount": 3, "handoffCount": 3, "toolFailureCount": 3, "gateFailureCount": 2, "cacheReadPercentBelow": 10, "subagentQueueWaitSeconds": 60, "implementationReworkCount": 2}}}); err != nil { // coverage-ignore: encoding this fixed in-memory Go value cannot fail
			return nil, err
		}
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "workflowTelemetry"}, &value)
		var b bytes.Buffer
		enc := yaml.NewEncoder(&b)
		enc.SetIndent(2)
		if err := enc.Encode(&doc); err != nil { // coverage-ignore: bytes.Buffer writes cannot fail
			return nil, err
		}
		_ = enc.Close()
		fmt.Fprintln(out, "workflow-telemetry: added workflowTelemetry defaults")
		return b.Bytes(), nil
	})
}
