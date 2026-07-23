package dashboardruntime

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

type policySnapshot struct {
	SchemaVersion     int                             `json:"schemaVersion"`
	ProtocolMajor     int                             `json:"protocolMajor"`
	WorkflowTelemetry workflowTelemetryPolicySnapshot `json:"workflowTelemetry"`
}

type workflowTelemetryPolicySnapshot struct {
	Retention   retentionPolicySnapshot   `json:"retention"`
	Widget      widgetPolicySnapshot      `json:"widget"`
	Diagnostics diagnosticsPolicySnapshot `json:"diagnostics"`
}

type retentionPolicySnapshot struct {
	MaxCompletedEffortAgeDays int `json:"maxCompletedEffortAgeDays"`
	MaxCompletedEffortCount   int `json:"maxCompletedEffortCount"`
}

type widgetPolicySnapshot struct {
	Enabled  bool `json:"enabled"`
	ShowCost bool `json:"showCost"`
}

type diagnosticsPolicySnapshot struct {
	HeuristicsEnabled      bool                     `json:"heuristicsEnabled"`
	MinimumBaselineSamples int                      `json:"minimumBaselineSamples"`
	BaselinePercentile     int                      `json:"baselinePercentile"`
	Thresholds             thresholdsPolicySnapshot `json:"thresholds"`
}

type thresholdsPolicySnapshot struct {
	PhaseReentryCount         int `json:"phaseReentryCount"`
	PhaseDurationSeconds      int `json:"phaseDurationSeconds"`
	PhaseTokens               int `json:"phaseTokens"`
	CompactionCount           int `json:"compactionCount"`
	HandoffCount              int `json:"handoffCount"`
	ToolFailureCount          int `json:"toolFailureCount"`
	GateFailureCount          int `json:"gateFailureCount"`
	CacheReadPercentBelow     int `json:"cacheReadPercentBelow"`
	SubagentQueueWaitSeconds  int `json:"subagentQueueWaitSeconds"`
	ImplementationReworkCount int `json:"implementationReworkCount"`
}

func readPolicySnapshot(materializedRoot string) ([]byte, error) {
	content, err := os.ReadFile(materializedRoot + string(os.PathSeparator) + ".awf" + string(os.PathSeparator) + "config.yaml")
	if err != nil {
		return nil, fmt.Errorf("%w: read tracked workflow telemetry: %w", ErrSnapshot, err)
	}
	cfg, err := config.Parse(materializedRoot+string(os.PathSeparator)+".awf", content)
	if err != nil {
		return nil, fmt.Errorf("%w: parse tracked workflow telemetry: %w", ErrSnapshot, err)
	}
	telemetry := cfg.WorkflowTelemetry
	if err := validateTelemetry(telemetry); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSnapshot, err)
	}
	thresholds := telemetry.Diagnostics.Thresholds
	policy := policySnapshot{
		SchemaVersion: policySchema,
		ProtocolMajor: protocolMajor,
		WorkflowTelemetry: workflowTelemetryPolicySnapshot{
			Retention: retentionPolicySnapshot{
				MaxCompletedEffortAgeDays: telemetry.Retention.MaxCompletedEffortAgeDays,
				MaxCompletedEffortCount:   telemetry.Retention.MaxCompletedEffortCount,
			},
			Widget: widgetPolicySnapshot{Enabled: telemetry.Widget.Enabled, ShowCost: telemetry.Widget.ShowCost},
			Diagnostics: diagnosticsPolicySnapshot{
				HeuristicsEnabled:      telemetry.Diagnostics.HeuristicsEnabled,
				MinimumBaselineSamples: telemetry.Diagnostics.MinimumBaselineSamples,
				BaselinePercentile:     telemetry.Diagnostics.BaselinePercentile,
				Thresholds: thresholdsPolicySnapshot{
					PhaseReentryCount: thresholds.PhaseReentryCount, PhaseDurationSeconds: thresholds.PhaseDurationSeconds,
					PhaseTokens: thresholds.PhaseTokens, CompactionCount: thresholds.CompactionCount,
					HandoffCount: thresholds.HandoffCount, ToolFailureCount: thresholds.ToolFailureCount,
					GateFailureCount: thresholds.GateFailureCount, CacheReadPercentBelow: thresholds.CacheReadPercentBelow,
					SubagentQueueWaitSeconds: thresholds.SubagentQueueWaitSeconds, ImplementationReworkCount: thresholds.ImplementationReworkCount,
				},
			},
		},
	}
	encoded, err := compactJSON(policy)
	if err != nil { // coverage-ignore: policySnapshot contains only JSON-supported scalar fields
		return nil, fmt.Errorf("%w: encode policy: %w", ErrSnapshot, err)
	}
	return encoded, nil
}

func validateTelemetry(t config.WorkflowTelemetryConfig) error {
	values := []struct {
		name      string
		value     int
		allowZero bool
	}{
		{"retention.maxCompletedEffortAgeDays", t.Retention.MaxCompletedEffortAgeDays, true},
		{"retention.maxCompletedEffortCount", t.Retention.MaxCompletedEffortCount, true},
		{"diagnostics.minimumBaselineSamples", t.Diagnostics.MinimumBaselineSamples, false},
		{"diagnostics.thresholds.phaseReentryCount", t.Diagnostics.Thresholds.PhaseReentryCount, false},
		{"diagnostics.thresholds.phaseDurationSeconds", t.Diagnostics.Thresholds.PhaseDurationSeconds, false},
		{"diagnostics.thresholds.phaseTokens", t.Diagnostics.Thresholds.PhaseTokens, false},
		{"diagnostics.thresholds.compactionCount", t.Diagnostics.Thresholds.CompactionCount, false},
		{"diagnostics.thresholds.handoffCount", t.Diagnostics.Thresholds.HandoffCount, false},
		{"diagnostics.thresholds.toolFailureCount", t.Diagnostics.Thresholds.ToolFailureCount, false},
		{"diagnostics.thresholds.gateFailureCount", t.Diagnostics.Thresholds.GateFailureCount, false},
		{"diagnostics.thresholds.subagentQueueWaitSeconds", t.Diagnostics.Thresholds.SubagentQueueWaitSeconds, false},
		{"diagnostics.thresholds.implementationReworkCount", t.Diagnostics.Thresholds.ImplementationReworkCount, false},
	}
	for _, value := range values {
		if value.value < 0 || (!value.allowZero && value.value == 0) {
			return fmt.Errorf("workflowTelemetry.%s has invalid value %d", value.name, value.value)
		}
	}
	if t.Diagnostics.BaselinePercentile < 1 || t.Diagnostics.BaselinePercentile > 100 {
		return errors.New("workflowTelemetry.diagnostics.baselinePercentile is outside 1..100")
	}
	if value := t.Diagnostics.Thresholds.CacheReadPercentBelow; value < 0 || value > 100 {
		return errors.New("workflowTelemetry.diagnostics.thresholds.cacheReadPercentBelow is outside 0..100")
	}
	return nil
}

func canonicalJSON(content []byte) bool {
	var value policySnapshot
	if err := json.Unmarshal(content, &value); err != nil {
		return false
	}
	encoded, err := compactJSON(value)
	return err == nil && bytes.Equal(content, encoded)
}
