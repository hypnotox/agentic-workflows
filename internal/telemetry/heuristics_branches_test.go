package telemetry

import (
	"encoding/json"
	"testing"
	"time"
)

func TestHeuristicEntryPointAndDefensiveBranches(t *testing.T) {
	badPhase := "not-a-phase"
	if _, err := DiagnoseHeuristics(nil, Selector{Phase: &badPhase}, HeuristicOptions{}); err == nil {
		t.Fatal("invalid heuristic selector accepted")
	}
	if _, err := Diagnose(nil, Selector{Phase: &badPhase}, HeuristicOptions{}, time.Time{}); err == nil {
		t.Fatal("invalid combined selector accepted")
	}

	read := heuristicSignalRead("effort")
	options := HeuristicOptions{Enabled: true, MinimumBaselineSamples: 1, BaselinePercentile: 95, Thresholds: HeuristicThresholds{CompactionCount: 1}}
	combined, err := Diagnose([]EffortRead{read}, Selector{}, options, time.Date(2026, 7, 22, 2, 0, 0, 0, time.UTC))
	if err != nil || !hasFindingCode(combined.Findings, "WFH1-COMPACTIONS") {
		t.Fatalf("combined heuristic entrypoint = %#v err=%v", combined.Findings, err)
	}
	other := "other"
	if findings, err := DiagnoseHeuristics([]EffortRead{read}, Selector{EffortID: &other}, options); err != nil || len(findings) != 0 {
		t.Fatalf("other effort selector = %#v err=%v", findings, err)
	}

	unsupportedIssue := read
	unsupportedIssue.Integrity = []IntegrityIssue{{Code: "unsupported-protocol"}}
	if heuristicCohortCompatible(unsupportedIssue) {
		t.Fatal("unsupported protocol issue entered cohort")
	}
	unsupportedMajor := read
	unsupportedMajor.Events = append([]EventEnvelope(nil), read.Events...)
	unsupportedMajor.Events[0].Version.Major = 2
	if heuristicCohortCompatible(unsupportedMajor) {
		t.Fatal("unsupported protocol major entered cohort")
	}
	if !heuristicCohortCompatible(read) {
		t.Fatal("compatible effort excluded from cohort")
	}

	if got := nearestRank([]float64{2, 4}, 0); got != 2 {
		t.Fatalf("bounded low nearest rank = %v", got)
	}
	if got := nearestRank([]float64{2, 4}, 200); got != 4 {
		t.Fatalf("bounded high nearest rank = %v", got)
	}
}

func TestHeuristicMalformedAndInvalidIntervalBranches(t *testing.T) {
	events := lifecycleBaseEvents()
	events = appendEvent(events, "route", "route_selected", RoutePayload{Route: "direct"})
	events = appendEvent(events, "start", "phase_started", PhaseStartedPayload{Phase: "implementation"})
	events[len(events)-1].Timestamp = "invalid"

	malformedUsage := heuristicPassive("bad-usage", "start", "usage_observed", UsageObservedPayload{Model: "model", Phase: "implementation"})
	malformedUsage.Payload = json.RawMessage(`{"bad":`)
	events = append(events, malformedUsage)
	malformedSubagent := heuristicPassive("bad-subagent", "bad-usage", "subagent_observed", SubagentObservedPayload{})
	malformedSubagent.Payload = json.RawMessage(`{"bad":`)
	events = append(events, malformedSubagent)
	malformedTool := heuristicPassive("bad-tool", "bad-subagent", "tool_observed", ToolObservedPayload{})
	malformedTool.Payload = json.RawMessage(`{"bad":`)
	events = append(events, malformedTool)
	malformedGate := heuristicPassive("bad-gate", "bad-tool", "shell_observed", ShellObservedPayload{})
	malformedGate.Payload = json.RawMessage(`{"bad":`)
	events = append(events, malformedGate)

	finish := causalEvent("finish", "session", "phase_finished", []string{"bad-gate"}, PhaseFinishedPayload{Phase: "implementation", StartEventID: "start"})
	finish.Timestamp = "2026-07-22T00:00:00Z"
	events = append(events, finish)
	orphanFinish := causalEvent("orphan", "session", "phase_finished", []string{"finish"}, PhaseFinishedPayload{Phase: "planning", StartEventID: "missing"})
	events = append(events, orphanFinish)
	malformedStart := causalEvent("malformed-start", "session", "phase_started", []string{"orphan"}, PhaseStartedPayload{})
	malformedStart.Payload = json.RawMessage(`{"bad":`)
	events = append(events, malformedStart)
	malformedFinish := causalEvent("malformed-finish", "session", "phase_finished", []string{"malformed-start"}, PhaseFinishedPayload{})
	malformedFinish.Payload = json.RawMessage(`{"bad":`)
	events = append(events, malformedFinish)
	events = appendEvent(events, "open-implementation", "phase_started", PhaseStartedPayload{Phase: "implementation"})

	read := EffortRead{Metadata: EffortMetadata{EffortID: "effort"}, Events: events, EffectApplied: allEffects(events)}
	openEvents := appendEvent(appendEvent(lifecycleBaseEvents(), "open-route", "route_selected", RoutePayload{Route: "direct"}), "open", "phase_started", PhaseStartedPayload{Phase: "implementation"})
	_ = effortHeuristicMetrics(EffortRead{Metadata: EffortMetadata{EffortID: "open"}, Events: openEvents}, HeuristicThresholds{})
	order, _ := BuildCausalOrder(events)
	if got := implementationSegmentMetrics("effort", "direct", events, []EventEnvelope{causalEvent("open", "session", "phase_started", nil, PhaseStartedPayload{Phase: "implementation"})}, map[string]EventEnvelope{}, order, HeuristicThresholds{}); len(got) != 0 {
		t.Fatalf("open implementation segment produced metrics: %#v", got)
	}
	metrics := effortHeuristicMetrics(read, HeuristicThresholds{})
	for _, metric := range metrics {
		if metric.code == "WFH1-PHASE-DURATION" && metric.scope == intervalScope("effort", "start", "finish") {
			t.Fatal("clock-invalid interval entered duration metrics")
		}
	}
}
