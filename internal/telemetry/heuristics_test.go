package telemetry

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestHeuristicAbsoluteThresholdEqualityAndRuleCoverage(t *testing.T) {
	cases := []struct {
		code  string
		lower bool
	}{
		{"WFH1-PHASE-REENTRY", false}, {"WFH1-PHASE-DURATION", false}, {"WFH1-PHASE-TOKENS", false},
		{"WFH1-COMPACTIONS", false}, {"WFH1-HANDOFFS", false}, {"WFH1-TOOL-FAILURES", false},
		{"WFH1-GATE-FAILURES", false}, {"WFH1-CACHE-READ", true}, {"WFH1-SUBAGENT-QUEUE-WAIT", false},
		{"WFH1-IMPLEMENTATION-REWORK", false},
	}
	options := HeuristicOptions{Enabled: true, MinimumBaselineSamples: 3, BaselinePercentile: 95}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			metric := heuristicMetric{code: tc.code, scope: "scope", key: tc.code, route: "direct", value: 10, unit: "count", lower: tc.lower, threshold: 10, eventIDs: []string{"event"}}
			finding, triggered := heuristicFinding(metric, []float64{1}, options)
			if !triggered || finding.Type != "heuristic" || finding.Severity != "warning" || finding.Confidence != "medium" || finding.Threshold == nil || finding.Baseline == nil || finding.Baseline.SampleCount != 1 || finding.Baseline.RuleVersion != 1 || finding.Evidence.ObservedValue == nil || *finding.Evidence.ObservedValue != 10 || finding.Explanation == "" || finding.NextAction == "" {
				t.Fatalf("inclusive absolute contract = %#v triggered=%v", finding, triggered)
			}
			if !contains(finding.Evidence.CounterIDs, "absolute-trigger:true") || !contains(finding.Evidence.CounterIDs, "historical-trigger:false") || !contains(finding.Evidence.CounterIDs, "baseline-required:3") {
				t.Fatalf("independent/insufficient evidence = %#v", finding.Evidence)
			}
			metric.value = 9
			if tc.lower {
				metric.value = 11
			}
			if _, got := heuristicFinding(metric, nil, options); got {
				t.Fatal("non-triggering side of inclusive threshold produced a finding")
			}
		})
	}
}

func TestHeuristicHistoricalNearestRankComparableCohortAndCacheComplement(t *testing.T) {
	options := HeuristicOptions{Enabled: true, MinimumBaselineSamples: 4, BaselinePercentile: 75}
	upper := heuristicMetric{code: "WFH1-HANDOFFS", scope: "scope", route: "direct", value: 8, threshold: 100, unit: "count", eventIDs: []string{"event"}}
	finding, triggered := heuristicFinding(upper, []float64{1, 3, 8, 20}, options)
	if !triggered || finding.Confidence != "medium" || finding.Baseline.Percentile != 75 || finding.Baseline.Value != 8 || !contains(finding.Evidence.CounterIDs, "absolute-trigger:false") || !contains(finding.Evidence.CounterIDs, "historical-trigger:true") {
		t.Fatalf("upper historical trigger = %#v", finding)
	}
	upper.threshold = 8
	finding, triggered = heuristicFinding(upper, []float64{1, 3, 8, 20}, options)
	if !triggered || finding.Confidence != "high" {
		t.Fatalf("combined trigger confidence = %#v", finding)
	}

	cache := heuristicMetric{code: "WFH1-CACHE-READ", scope: "scope", route: "direct", value: 3, threshold: 0, lower: true, unit: "percent", eventIDs: []string{"cache"}}
	finding, triggered = heuristicFinding(cache, []float64{1, 3, 8, 20}, options)
	if !triggered || finding.Baseline.Percentile != 26 || finding.Baseline.Value != 3 || finding.Threshold.Comparator != "lte" {
		t.Fatalf("complementary lower percentile = %#v", finding)
	}
	if got := nearestRank([]float64{1, 3, 8, 20}, 100); got != 20 {
		t.Fatalf("nearest rank 100 = %v", got)
	}
}

func TestDiagnoseHeuristicsIntegrationSelectorsDisabledAndStableEvidence(t *testing.T) {
	read := heuristicSignalRead("effort")
	thresholds := HeuristicThresholds{
		PhaseReentryCount: 1, PhaseDurationSeconds: 1, PhaseTokens: 1,
		CompactionCount: 1, HandoffCount: 1, ToolFailureCount: 1, GateFailureCount: 1,
		CacheReadPercentBelow: 50, SubagentQueueWaitSeconds: 1, ImplementationReworkCount: 1,
	}
	options := HeuristicOptions{Enabled: true, MinimumBaselineSamples: 2, BaselinePercentile: 95, Thresholds: thresholds}
	findings, err := DiagnoseHeuristics([]EffortRead{read}, Selector{}, options)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, finding := range findings {
		got[finding.Code] = true
		if !sort.StringsAreSorted(finding.Evidence.EventIDs) || !sort.StringsAreSorted(finding.Evidence.CounterIDs) || len(finding.Evidence.EventIDs) == 0 || finding.Baseline == nil || finding.Baseline.SampleCount != 0 {
			t.Fatalf("stable contributing evidence = %#v", finding)
		}
	}
	want := []string{"WFH1-PHASE-REENTRY", "WFH1-PHASE-DURATION", "WFH1-PHASE-TOKENS", "WFH1-COMPACTIONS", "WFH1-HANDOFFS", "WFH1-TOOL-FAILURES", "WFH1-GATE-FAILURES", "WFH1-CACHE-READ", "WFH1-SUBAGENT-QUEUE-WAIT", "WFH1-IMPLEMENTATION-REWORK"}
	for _, code := range want {
		if !got[code] {
			t.Errorf("missing integrated heuristic %s: %#v", code, findings)
		}
	}

	phase := "implementation-review"
	filtered, err := DiagnoseHeuristics([]EffortRead{read}, Selector{Phase: &phase}, options)
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range filtered {
		for _, id := range finding.Evidence.EventIDs {
			if id == "compact" || id == "handoff" || id == "usage" {
				t.Fatalf("phase selector retained unrelated metric: %#v", finding)
			}
		}
	}

	exactInput := diagnosticRead(appendEvent(lifecycleBaseEvents(), "bad-change", "route_changed", RoutePayload{Route: "direct"}))
	disabled, err := Diagnose([]EffortRead{exactInput}, Selector{}, HeuristicOptions{Enabled: false}, time.Time{})
	if err != nil || !hasFindingCode(disabled.Findings, "WFV1-LIFECYCLE-TRANSITION") {
		t.Fatalf("disabled heuristics affected exact rules: %#v err=%v", disabled.Findings, err)
	}
}

func TestComparableBaselinesUseDistinctCompletedCompatibleSameRouteEfforts(t *testing.T) {
	targetMetric := heuristicMetric{key: "handoffs", route: "direct", lower: false}
	reads := []EffortRead{}
	all := map[string][]heuristicMetric{}
	completed, compatible := map[string]bool{}, map[string]bool{}
	add := func(id, route string, value float64, terminal, supported bool) {
		read := EffortRead{Metadata: EffortMetadata{EffortID: id}}
		reads = append(reads, read)
		all[id] = []heuristicMetric{{key: "handoffs", route: route, value: value}, {key: "handoffs", route: route, value: value + 1}}
		completed[id], compatible[id] = terminal, supported
	}
	add("target", "direct", 0, false, true)
	add("valid", "direct", 3, true, true)
	add("wrong-route", "plan", 9, true, true)
	add("active", "direct", 8, false, true)
	add("unsupported", "direct", 7, true, false)
	if got := comparableMetricValues("target", targetMetric, reads, all, completed, compatible); !reflect.DeepEqual(got, []float64{4}) {
		t.Fatalf("comparable cohort = %#v", got)
	}

	zeroCache := heuristicSignalRead("zero-cache")
	for index := range zeroCache.Events {
		if zeroCache.Events[index].Kind == "usage_observed" {
			zeroCache.Events[index].Payload, _ = json.Marshal(UsageObservedPayload{Model: "model"})
		}
	}
	metrics := effortHeuristicMetrics(zeroCache, HeuristicThresholds{})
	for _, metric := range metrics {
		if metric.code == "WFH1-CACHE-READ" {
			t.Fatal("zero cache denominator produced a cache metric")
		}
	}
}

func heuristicSignalRead(effortID string) EffortRead {
	events := lifecycleBaseEvents()
	events = appendEvent(events, "route", "route_selected", RoutePayload{Route: "direct"})
	events = appendFinishedPhase(events, "brainstorm", "brainstorming", "")
	events = appendEvent(events, "impl-start", "phase_started", PhaseStartedPayload{Phase: "implementation"})
	events[len(events)-1].Timestamp = "2026-07-22T00:00:00Z"
	events = append(events, heuristicPassive("usage", events[len(events)-1].EventID, "usage_observed", UsageObservedPayload{Model: "model", InputTokens: 10, OutputTokens: 2, CacheReadTokens: 1, DurationMS: 10, Phase: "implementation"}))
	events = append(events, heuristicPassive("compact", events[len(events)-1].EventID, "compaction_observed", CompactionObservedPayload{Count: 1}))
	events = append(events, heuristicPassive("handoff", events[len(events)-1].EventID, "handoff_observed", HandoffObservedPayload{Outcome: "failure", TargetSessionID: "child"}))
	events = append(events, heuristicPassive("tool", events[len(events)-1].EventID, "tool_observed", ToolObservedPayload{Tool: "read", Outcome: "failure", DurationMS: 1, ErrorCategory: "tool"}))
	events = append(events, heuristicPassive("gate", events[len(events)-1].EventID, "shell_observed", ShellObservedPayload{Classification: "gate", Outcome: "failure", GateMode: "standard"}))
	events = append(events, heuristicPassive("subagent", events[len(events)-1].EventID, "subagent_observed", SubagentObservedPayload{Role: "reviewer", RequestedModel: "model", ResolvedModel: "model", ThinkingLevel: "medium", QueueDurationMS: 1000, RunDurationMS: 1, Outcome: "success", StopReason: "complete", ToolCount: 1, ToolFailureCount: 1}))
	events = appendEvent(events, "impl-finish", "phase_finished", PhaseFinishedPayload{Phase: "implementation", StartEventID: "impl-start"})
	events[len(events)-1].Timestamp = "2026-07-22T00:00:02Z"
	events = appendFinishedPhase(events, "review-one", "implementation-review", "")
	events = appendFinishedPhase(events, "impl-two", "implementation", "")
	events = appendFinishedPhase(events, "review-two", "implementation-review", "")
	events = appendFinishedPhase(events, "retro", "retrospective", "")
	events = appendEvent(events, "complete", "effort_completed", EffortTerminalPayload{TerminalEpoch: 1})
	for index := range events {
		events[index].EffortID = effortID
	}
	return EffortRead{Metadata: EffortMetadata{EffortID: effortID, CreatedAt: "2026-07-22T00:00:00Z", CheckpointID: "checkpoint.md", CreationMode: "independent"}, Events: events}
}

func heuristicPassive(id, predecessor string, kind EventKind, payload any) EventEnvelope {
	raw, _ := json.Marshal(payload)
	return EventEnvelope{Version: ProtocolVersion{Major: 1}, EventID: id, ObservationID: "observation-" + id, EffortID: "effort", SessionID: "session", Timestamp: "2026-07-22T00:00:01Z", Kind: kind, Predecessors: []string{predecessor}, Payload: raw}
}
