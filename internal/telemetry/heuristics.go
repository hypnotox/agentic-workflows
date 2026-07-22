package telemetry

import (
	"encoding/json"
	"math"
	"sort"
	"time"
)

const heuristicRuleVersion = 1

// HeuristicThresholds are the absolute version-one diagnostic thresholds.
type HeuristicThresholds struct {
	PhaseReentryCount         int
	PhaseDurationSeconds      int
	PhaseTokens               int
	CompactionCount           int
	HandoffCount              int
	ToolFailureCount          int
	GateFailureCount          int
	CacheReadPercentBelow     int
	SubagentQueueWaitSeconds  int
	ImplementationReworkCount int
}

// HeuristicOptions controls optional heuristic diagnosis. Exact diagnosis is
// independent of Enabled.
type HeuristicOptions struct {
	Enabled                bool
	MinimumBaselineSamples int
	BaselinePercentile     int
	Thresholds             HeuristicThresholds
}

type heuristicMetric struct {
	code       string
	scope      string
	key        string
	route      string
	value      float64
	unit       string
	lower      bool
	threshold  float64
	eventIDs   []string
	counterIDs []string
}

// Diagnose evaluates exact diagnostics and, when enabled, version-one
// heuristics against comparable completed-effort baselines.
func Diagnose(reads []EffortRead, selector Selector, options HeuristicOptions, generatedAt time.Time) (DoctorResult, error) {
	result, err := DiagnoseExact(reads, selector, generatedAt)
	if err != nil || !options.Enabled {
		return result, err
	}
	findings, err := DiagnoseHeuristics(reads, selector, options)
	if err != nil { // coverage-ignore: exact diagnosis validated the same selector
		return DoctorResult{}, err
	}
	result.Findings = stableFindings(append(result.Findings, findings...))
	return result, nil
}

// DiagnoseHeuristics evaluates only the version-one heuristic rule set.
func DiagnoseHeuristics(reads []EffortRead, selector Selector, options HeuristicOptions) ([]Finding, error) {
	if err := ValidateSelector(selector); err != nil {
		return nil, err
	}
	all := make(map[string][]heuristicMetric, len(reads))
	completed := make(map[string]bool, len(reads))
	compatible := make(map[string]bool, len(reads))
	for _, read := range reads {
		all[read.Metadata.EffortID] = effortHeuristicMetrics(read, options.Thresholds)
		lifecycle := projectLifecycleFromRead(read)
		completed[read.Metadata.EffortID] = lifecycle.State == EffortCompleted
		compatible[read.Metadata.EffortID] = heuristicCohortCompatible(read)
	}
	findings := []Finding{}
	for _, read := range reads {
		if selector.EffortID != nil && read.Metadata.EffortID != *selector.EffortID {
			continue
		}
		_, selected, err := selectEffortEvents(read, selector)
		if err != nil { // coverage-ignore: selector validation above is authoritative
			return nil, err
		}
		for _, metric := range all[read.Metadata.EffortID] {
			if !heuristicMetricSelected(metric, selected, selector) {
				continue
			}
			values := comparableMetricValues(read.Metadata.EffortID, metric, reads, all, completed, compatible)
			finding, triggered := heuristicFinding(metric, values, options)
			if triggered {
				findings = append(findings, finding)
			}
		}
	}
	return stableFindings(findings), nil
}

func effortHeuristicMetrics(read EffortRead, thresholds HeuristicThresholds) []heuristicMetric {
	// Work across every trajectory while excluding lifecycle evidence whose
	// effect was rejected or superseded. Passive observations remain evidence.
	read.Events = append([]EventEnvelope(nil), read.Events...)
	lifecycle := projectLifecycleFromRead(read)
	events := make([]EventEnvelope, 0, len(read.Events))
	for _, event := range read.Events {
		if descriptor.Payloads[string(event.Kind)].Class == "lifecycle" && !lifecycle.EffectApplied[event.EventID] {
			continue
		}
		events = append(events, event)
	}
	order, _ := BuildCausalOrder(events)
	phases := projectEventPhases(events)
	route := string(lifecycle.Route)
	effortID := read.Metadata.EffortID
	metrics := []heuristicMetric{}

	startsByPhase := map[Phase][]EventEnvelope{}
	finishesByStart := map[string]EventEnvelope{}
	for _, event := range events {
		switch event.Kind {
		case "phase_started":
			var payload PhaseStartedPayload
			if json.Unmarshal(event.Payload, &payload) == nil {
				startsByPhase[payload.Phase] = append(startsByPhase[payload.Phase], event)
			}
		case "phase_finished":
			var payload PhaseFinishedPayload
			if json.Unmarshal(event.Payload, &payload) == nil {
				finishesByStart[payload.StartEventID] = event
			}
		}
	}
	for phase, starts := range startsByPhase {
		sort.Slice(starts, func(i, j int) bool { return starts[i].EventID < starts[j].EventID })
		reentries := len(starts) - 1
		ids := make([]string, len(starts))
		for index := range starts {
			ids[index] = starts[index].EventID
		}
		metrics = append(metrics, newHeuristicMetric("WFH1-PHASE-REENTRY", effortScope(effortID)+":phase:"+string(phase), "phase-reentry:"+string(phase), route, float64(reentries), "count", false, thresholds.PhaseReentryCount, ids))
		for _, start := range starts {
			finish, exists := finishesByStart[start.EventID]
			if !exists || !order.HappensBefore(start.EventID, finish.EventID) {
				continue
			}
			startTime, startErr := time.Parse(time.RFC3339Nano, start.Timestamp)
			finishTime, finishErr := time.Parse(time.RFC3339Nano, finish.Timestamp)
			if startErr == nil && finishErr == nil && !finishTime.Before(startTime) {
				duration := finishTime.Sub(startTime)
				metrics = append(metrics, newHeuristicMetric("WFH1-PHASE-DURATION", intervalScope(effortID, start.EventID, finish.EventID), "phase-duration:"+string(phase), route, duration.Seconds(), "seconds", false, thresholds.PhaseDurationSeconds, []string{start.EventID, finish.EventID}))
			}
			tokens, tokenIDs := intervalTokens(events, phases, phase, start, finish, order)
			metrics = append(metrics, newHeuristicMetric("WFH1-PHASE-TOKENS", intervalScope(effortID, start.EventID, finish.EventID), "phase-tokens:"+string(phase), route, float64(tokens), "tokens", false, thresholds.PhaseTokens, append([]string{start.EventID, finish.EventID}, tokenIDs...)))
		}
	}

	all := aggregateScope("all-work", events)
	reworkIDs := implementationReworkIDs(events, order)
	metrics = append(metrics,
		newHeuristicMetric("WFH1-COMPACTIONS", effortScope(effortID), "compactions", route, float64(all.Counters.Compactions), "count", false, thresholds.CompactionCount, counterEventIDs(events, "compaction_observed")),
		newHeuristicMetric("WFH1-HANDOFFS", effortScope(effortID), "handoffs", route, float64(all.Counters.Handoffs), "count", false, thresholds.HandoffCount, counterEventIDs(events, "handoff_observed")),
		newHeuristicMetric("WFH1-IMPLEMENTATION-REWORK", effortScope(effortID), "implementation-rework", route, float64(len(reworkIDs)), "count", false, thresholds.ImplementationReworkCount, reworkIDs),
	)
	denominator := all.Usage.InputTokens + all.Usage.CacheReadTokens
	if denominator > 0 {
		percentage := float64(all.Usage.CacheReadTokens) / float64(denominator) * 100
		metrics = append(metrics, newHeuristicMetric("WFH1-CACHE-READ", effortScope(effortID), "cache-read", route, percentage, "percent", true, thresholds.CacheReadPercentBelow, usageEventIDs(events)))
	}
	for _, event := range events {
		if event.Kind != "subagent_observed" {
			continue
		}
		var payload SubagentObservedPayload
		if json.Unmarshal(event.Payload, &payload) == nil {
			metrics = append(metrics, newHeuristicMetric("WFH1-SUBAGENT-QUEUE-WAIT", boundedDiagnosticScope(effortScope(effortID)+":subagent:"+event.EventID), "subagent-queue-wait", route, float64(payload.QueueDurationMS)/1000, "seconds", false, thresholds.SubagentQueueWaitSeconds, []string{event.EventID}))
		}
	}
	metrics = append(metrics, implementationSegmentMetrics(effortID, route, events, startsByPhase["implementation"], finishesByStart, order, thresholds)...)
	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].code+"\x00"+metrics[i].scope < metrics[j].code+"\x00"+metrics[j].scope
	})
	return metrics
}

func newHeuristicMetric(code, scope, key, route string, value float64, unit string, lower bool, threshold int, eventIDs []string) heuristicMetric {
	return heuristicMetric{code: code, scope: boundedDiagnosticScope(scope), key: key, route: route, value: value, unit: unit, lower: lower, threshold: float64(threshold), eventIDs: sortedUnique(eventIDs), counterIDs: []string{}}
}

func intervalTokens(events []EventEnvelope, phases map[string]map[string]bool, phase Phase, start, finish EventEnvelope, order *CausalOrder) (uint64, []string) {
	var total uint64
	ids := []string{}
	for _, event := range events {
		if !phases[event.EventID][string(phase)] || (event.EventID != start.EventID && !order.HappensBefore(start.EventID, event.EventID)) || (event.EventID != finish.EventID && !order.HappensBefore(event.EventID, finish.EventID)) {
			continue
		}
		switch event.Kind {
		case "usage_observed":
			var payload UsageObservedPayload
			if json.Unmarshal(event.Payload, &payload) == nil {
				total = saturatingAdd(total, saturatingAdd(payload.InputTokens, payload.OutputTokens))
				ids = append(ids, event.EventID)
			}
		case "subagent_observed":
			var payload SubagentObservedPayload
			if json.Unmarshal(event.Payload, &payload) == nil {
				total = saturatingAdd(total, saturatingAdd(payload.InputTokens, payload.OutputTokens))
				ids = append(ids, event.EventID)
			}
		}
	}
	return total, sortedUnique(ids)
}

func implementationSegmentMetrics(effortID, route string, events, starts []EventEnvelope, finishes map[string]EventEnvelope, order *CausalOrder, thresholds HeuristicThresholds) []heuristicMetric {
	result := []heuristicMetric{}
	for _, start := range starts {
		finish, exists := finishes[start.EventID]
		if !exists || !order.HappensBefore(start.EventID, finish.EventID) {
			continue
		}
		var toolFailures, gateFailures uint64
		toolIDs, gateIDs := []string{}, []string{}
		for _, event := range events {
			if !order.HappensBefore(start.EventID, event.EventID) || !order.HappensBefore(event.EventID, finish.EventID) {
				continue
			}
			switch event.Kind {
			case "tool_observed":
				var payload ToolObservedPayload
				if json.Unmarshal(event.Payload, &payload) == nil && payload.Outcome != "success" {
					toolFailures = saturatingAdd(toolFailures, 1)
					toolIDs = append(toolIDs, event.EventID)
				}
			case "subagent_observed":
				var payload SubagentObservedPayload
				if json.Unmarshal(event.Payload, &payload) == nil && payload.ToolFailureCount > 0 {
					toolFailures = saturatingAdd(toolFailures, payload.ToolFailureCount)
					toolIDs = append(toolIDs, event.EventID)
				}
			case "shell_observed":
				var payload ShellObservedPayload
				if json.Unmarshal(event.Payload, &payload) == nil && payload.Classification == "gate" && payload.Outcome != "success" {
					gateFailures = saturatingAdd(gateFailures, 1)
					gateIDs = append(gateIDs, event.EventID)
				}
			}
		}
		scope := intervalScope(effortID, start.EventID, finish.EventID)
		result = append(result,
			newHeuristicMetric("WFH1-TOOL-FAILURES", scope, "implementation-tool-failures", route, float64(toolFailures), "count", false, thresholds.ToolFailureCount, append([]string{start.EventID, finish.EventID}, toolIDs...)),
			newHeuristicMetric("WFH1-GATE-FAILURES", scope, "implementation-gate-failures", route, float64(gateFailures), "count", false, thresholds.GateFailureCount, append([]string{start.EventID, finish.EventID}, gateIDs...)),
		)
	}
	return result
}

func comparableMetricValues(targetID string, target heuristicMetric, reads []EffortRead, all map[string][]heuristicMetric, completed, compatible map[string]bool) []float64 {
	values := []float64{}
	for _, read := range reads {
		id := read.Metadata.EffortID
		if id == targetID || !completed[id] || !compatible[id] {
			continue
		}
		best, found := 0.0, false
		for _, candidate := range all[id] {
			if candidate.key != target.key || candidate.route != target.route {
				continue
			}
			if !found || !target.lower && candidate.value > best || target.lower && candidate.value < best {
				best, found = candidate.value, true
			}
		}
		if found {
			values = append(values, best)
		}
	}
	sort.Float64s(values)
	return values
}

func heuristicFinding(metric heuristicMetric, cohort []float64, options HeuristicOptions) (Finding, bool) {
	absolute := metric.value >= metric.threshold
	comparator := "gte"
	if metric.lower {
		absolute = metric.value <= metric.threshold
		comparator = "lte"
	}
	percentile := options.BaselinePercentile
	if metric.lower {
		percentile = 101 - percentile
	}
	baseline := 0.0
	historical := false
	available := options.MinimumBaselineSamples > 0 && len(cohort) >= options.MinimumBaselineSamples
	if available {
		baseline = nearestRank(cohort, percentile)
		historical = metric.value >= baseline
		if metric.lower {
			historical = metric.value <= baseline
		}
	}
	if !absolute && !historical {
		return Finding{}, false
	}
	observed := metric.value
	counterIDs := append([]string(nil), metric.counterIDs...)
	counterIDs = append(counterIDs,
		"rule-version:1", "route:"+metric.route,
		"absolute-trigger:"+boolWord(absolute), "historical-trigger:"+boolWord(historical),
		"baseline-samples:"+uintString(uint64(len(cohort))),
	)
	if !available {
		counterIDs = append(counterIDs, "baseline-required:"+uintString(uint64(options.MinimumBaselineSamples)))
	}
	confidence := "medium"
	if absolute && historical {
		confidence = "high"
	}
	finding := Finding{
		Code: metric.code, Type: "heuristic", Severity: "warning", Scope: metric.scope,
		Evidence:   FindingEvidence{EventIDs: metric.eventIDs, CounterIDs: sortedUnique(counterIDs), ObservedValue: &observed, Unit: metric.unit},
		Threshold:  &FindingThreshold{Kind: "absolute", Comparator: comparator, Value: metric.threshold, Unit: metric.unit},
		Baseline:   &FindingBaseline{Route: metric.route, RuleVersion: heuristicRuleVersion, SampleCount: len(cohort), Percentile: percentile, Value: baseline, Unit: metric.unit},
		Confidence: confidence, Explanation: heuristicExplanation(metric.code), NextAction: heuristicNextAction(metric.code), Waived: false,
	}
	return finding, true
}

func nearestRank(sortedValues []float64, percentile int) float64 {
	rank := int(math.Ceil(float64(percentile) / 100 * float64(len(sortedValues))))
	if rank < 1 {
		rank = 1
	}
	if rank > len(sortedValues) {
		rank = len(sortedValues)
	}
	return sortedValues[rank-1]
}

func heuristicCohortCompatible(read EffortRead) bool {
	for _, issue := range read.Integrity {
		if issue.Code == "unsupported-protocol" {
			return false
		}
	}
	for _, event := range read.Events {
		if event.Version.Major != descriptor.Version.Major {
			return false
		}
	}
	return true
}

func heuristicMetricSelected(metric heuristicMetric, selected map[string]bool, selector Selector) bool {
	if selector.SessionID == nil && selector.Phase == nil && selector.Since == nil && selector.Until == nil {
		return true
	}
	for _, eventID := range metric.eventIDs {
		if selected[eventID] {
			return true
		}
	}
	return false
}

func counterEventIDs(events []EventEnvelope, kind EventKind) []string {
	ids := []string{}
	for _, event := range events {
		if event.Kind == kind {
			ids = append(ids, event.EventID)
		}
	}
	return sortedUnique(ids)
}

func usageEventIDs(events []EventEnvelope) []string {
	ids := append(counterEventIDs(events, "usage_observed"), counterEventIDs(events, "subagent_observed")...)
	return sortedUnique(ids)
}

func implementationReworkIDs(events []EventEnvelope, order *CausalOrder) []string {
	ids := []string{}
	for _, event := range events {
		if event.Kind == "phase_started" {
			var payload PhaseStartedPayload
			if json.Unmarshal(event.Payload, &payload) == nil && payload.Phase == "implementation" && hasPriorImplementationReview(event, events, order) {
				ids = append(ids, event.EventID)
			}
		}
	}
	return sortedUnique(ids)
}

func boolWord(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func heuristicExplanation(code string) string {
	switch code {
	case "WFH1-PHASE-REENTRY":
		return "a named phase was reentered unusually often"
	case "WFH1-PHASE-DURATION":
		return "a completed phase interval took unusually long"
	case "WFH1-PHASE-TOKENS":
		return "a completed phase interval used unusually many tokens"
	case "WFH1-COMPACTIONS":
		return "the effort required unusually many context compactions"
	case "WFH1-HANDOFFS":
		return "the effort required unusually many session handoffs"
	case "WFH1-TOOL-FAILURES":
		return "an implementation segment recorded unusually many tool failures"
	case "WFH1-GATE-FAILURES":
		return "an implementation segment recorded unusually many failed gates"
	case "WFH1-CACHE-READ":
		return "cache-read reuse was unusually low"
	case "WFH1-SUBAGENT-QUEUE-WAIT":
		return "a subagent invocation waited unusually long in the queue"
	default:
		return "implementation returned after review unusually often"
	}
}

func heuristicNextAction(code string) string {
	if code == "WFH1-CACHE-READ" {
		return "inspect model and session continuity without treating the signal as a workflow violation"
	}
	return "inspect the contributing bounded evidence and comparable baseline before changing the workflow"
}
