package main

import (
	"github.com/hypnotox/agentic-workflows/internal/telemetry"
)

func runDoctor(c *cmdCtx) error {
	deps, err := normalMetricsReadDeps(c.root)
	if err != nil {
		return boundedTelemetryError(c.root, err)
	}
	return runDoctorWith(c, deps)
}

func runDoctorWith(c *cmdCtx, deps metricsReadDeps) error {
	selector, err := parseTelemetrySelector(c.inv)
	if err != nil {
		return err
	}
	reads, err := readTelemetryQueryInputs(deps.Root)
	if err != nil {
		return boundedTelemetryError(deps.Root, err)
	}
	diagnostics := deps.Policy.Diagnostics
	thresholds := diagnostics.Thresholds
	result, err := telemetry.Diagnose(reads, selector, telemetry.HeuristicOptions{
		Enabled:                diagnostics.HeuristicsEnabled,
		MinimumBaselineSamples: diagnostics.MinimumBaselineSamples,
		BaselinePercentile:     diagnostics.BaselinePercentile,
		Thresholds: telemetry.HeuristicThresholds{
			PhaseReentryCount:         thresholds.PhaseReentryCount,
			PhaseDurationSeconds:      thresholds.PhaseDurationSeconds,
			PhaseTokens:               thresholds.PhaseTokens,
			CompactionCount:           thresholds.CompactionCount,
			HandoffCount:              thresholds.HandoffCount,
			ToolFailureCount:          thresholds.ToolFailureCount,
			GateFailureCount:          thresholds.GateFailureCount,
			CacheReadPercentBelow:     thresholds.CacheReadPercentBelow,
			SubagentQueueWaitSeconds:  thresholds.SubagentQueueWaitSeconds,
			ImplementationReworkCount: thresholds.ImplementationReworkCount,
		},
	}, telemetryNow())
	if err != nil { // coverage-ignore: parsing validated the selector used by diagnosis
		return boundedTelemetryError(c.root, err)
	}
	if c.inv.bools["--json"] {
		return writeMetricsJSON(c.stdout, result)
	}
	return telemetry.RenderDoctorHuman(c.stdout, result)
}
