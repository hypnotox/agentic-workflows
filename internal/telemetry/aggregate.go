package telemetry

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
	"time"
)

// MetricsOptions supplies repository-wide state which is not encoded in an
// effort stream.
type MetricsOptions struct {
	GeneratedAt time.Time
	Retention   RetentionPolicy
	LastRunAt   *time.Time
}

// AggregateMetrics builds the canonical, deterministically ordered metrics
// projection from validated effort reads.
func AggregateMetrics(reads []EffortRead, selector Selector, options MetricsOptions) (MetricsResult, error) {
	if err := ValidateSelector(selector); err != nil {
		return MetricsResult{}, err
	}
	generatedAt := options.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	workflows := make([]WorkflowProjection, 0, len(reads))
	readsByID := make(map[string]EffortRead, len(reads))
	for _, read := range reads {
		workflows = append(workflows, ProjectWorkflow(read))
		readsByID[read.Metadata.EffortID] = read
	}
	workflows = GroupDerivedFamilies(workflows)
	result := MetricsResult{
		SchemaVersion: 1,
		ProtocolMajor: int(descriptor.Version.Major),
		GeneratedAt:   generatedAt,
		Selector:      selector,
		Efforts:       []EffortProjection{},
		Retention:     projectRetentionState(reads, options.Retention, generatedAt, options.LastRunAt),
		Integrity:     []IntegrityNotice{},
	}
	for _, workflow := range workflows {
		read := readsByID[workflow.Metadata.EffortID]
		events, selectedIDs, err := selectEffortEvents(read, selector)
		if err != nil { // coverage-ignore: selector was validated before projection
			return MetricsResult{}, err
		}
		if !effortMatchesSelector(workflow.Metadata.EffortID, selector, events) {
			continue
		}
		projection := aggregateEffort(read, workflow, events, selectedIDs)
		result.Efforts = append(result.Efforts, projection)
		result.Integrity = append(result.Integrity, projection.Integrity...)
	}
	sort.Slice(result.Efforts, func(i, j int) bool { return result.Efforts[i].EffortID < result.Efforts[j].EffortID })
	result.Integrity = stableIntegrityNotices(result.Integrity)
	return result, nil
}

func effortMatchesSelector(effortID string, selector Selector, events []EventEnvelope) bool {
	if selector.EffortID != nil && effortID != *selector.EffortID {
		return false
	}
	return len(events) != 0 || selector.SessionID == nil && selector.Phase == nil && selector.Since == nil && selector.Until == nil
}

func aggregateEffort(read EffortRead, workflow WorkflowProjection, events []EventEnvelope, selectedIDs map[string]bool) EffortProjection {
	byID := make(map[string]EventEnvelope, len(events))
	for _, event := range events {
		byID[event.EventID] = event
	}
	currentEvents := eventsForIDs(byID, workflow.CurrentPathEventIDs)
	allEvents := eventsForIDs(byID, workflow.AllWorkEventIDs)
	result := EffortProjection{
		EffortID:           workflow.Metadata.EffortID,
		CheckpointID:       workflow.Metadata.CheckpointID,
		State:              string(workflow.Lifecycle.State),
		Route:              string(workflow.Lifecycle.Route),
		ActiveTrajectoryID: workflow.Lifecycle.ActiveTrajectoryID,
		CurrentPath:        aggregateScope("current-path", currentEvents),
		AllWork:            aggregateScope("all-work", allEvents),
		Sessions:           []ScopeProjection{},
		Phases:             []ScopeProjection{},
		Trajectories:       []ScopeProjection{},
		DerivedEffortIDs:   append([]string(nil), workflow.DerivedEffortIDs...),
		Origin:             workflow.Origin,
		Integrity:          integrityNotices(workflow.Integrity, selectedIDs),
	}
	result.Sessions = aggregateGroupedScopes(allEvents, func(event EventEnvelope) []string { return []string{event.SessionID} })
	phases := projectEventPhases(read.Events)
	result.Phases = aggregateGroupedScopes(allEvents, func(event EventEnvelope) []string {
		values := make([]string, 0, len(phases[event.EventID]))
		for phase := range phases[event.EventID] {
			values = append(values, phase)
		}
		return values
	})
	trajectoryByEvent := make(map[string]string)
	for _, trajectory := range workflow.Trajectories {
		for _, eventID := range trajectory.EventIDs {
			trajectoryByEvent[eventID] = trajectory.TrajectoryID
		}
	}
	result.Trajectories = aggregateGroupedScopes(allEvents, func(event EventEnvelope) []string {
		if trajectory := trajectoryByEvent[event.EventID]; trajectory != "" {
			return []string{trajectory}
		}
		return nil
	})
	return result
}

func eventsForIDs(byID map[string]EventEnvelope, ids []string) []EventEnvelope {
	result := make([]EventEnvelope, 0, len(ids))
	for _, id := range ids {
		if event, exists := byID[id]; exists {
			result = append(result, event)
		}
	}
	return result
}

func aggregateGroupedScopes(events []EventEnvelope, keys func(EventEnvelope) []string) []ScopeProjection {
	groups := map[string][]EventEnvelope{}
	for _, event := range events {
		for _, key := range keys(event) {
			if key != "" {
				groups[key] = append(groups[key], event)
			}
		}
	}
	result := make([]ScopeProjection, 0, len(groups))
	for key, group := range groups {
		result = append(result, aggregateScope(key, group))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ScopeID < result[j].ScopeID })
	return result
}

func aggregateScope(scopeID string, events []EventEnvelope) ScopeProjection {
	result := ScopeProjection{ScopeID: scopeID, EventIDs: []string{}}
	seenEvents := map[string]bool{}
	seenObservations := map[string]bool{}
	order, _ := BuildCausalOrder(events)
	for _, event := range events {
		if seenEvents[event.EventID] {
			continue
		}
		seenEvents[event.EventID] = true
		if event.ObservationID != "" {
			if seenObservations[event.ObservationID] {
				continue
			}
			seenObservations[event.ObservationID] = true
		}
		result.EventIDs = append(result.EventIDs, event.EventID)
		switch event.Kind {
		case "usage_observed":
			var payload UsageObservedPayload
			if json.Unmarshal(event.Payload, &payload) == nil {
				addUsage(&result.Usage, payload.InputTokens, payload.OutputTokens, payload.CacheReadTokens, payload.CacheWriteTokens, payload.CostUSD, payload.DurationMS)
			}
		case "subagent_observed":
			var payload SubagentObservedPayload
			if json.Unmarshal(event.Payload, &payload) == nil {
				addUsage(&result.Usage, payload.InputTokens, payload.OutputTokens, payload.CacheReadTokens, payload.CacheWriteTokens, payload.CostUSD, payload.RunDurationMS)
				result.Counters.SubagentInvocations++
				result.Counters.ToolFailures = saturatingAdd(result.Counters.ToolFailures, payload.ToolFailureCount)
			}
		case "compaction_observed":
			var payload CompactionObservedPayload
			if json.Unmarshal(event.Payload, &payload) == nil {
				result.Counters.Compactions = saturatingAdd(result.Counters.Compactions, payload.Count)
			}
		case "handoff_observed":
			result.Counters.Handoffs++
		case "tool_observed":
			var payload ToolObservedPayload
			if json.Unmarshal(event.Payload, &payload) == nil && payload.Outcome != "success" {
				result.Counters.ToolFailures++
			}
		case "shell_observed":
			var payload ShellObservedPayload
			if json.Unmarshal(event.Payload, &payload) == nil && payload.Classification == "gate" && payload.Outcome != "success" {
				result.Counters.GateFailures++
			}
		case "phase_started":
			var payload PhaseStartedPayload
			if json.Unmarshal(event.Payload, &payload) == nil && payload.Phase == "implementation" && hasPriorImplementationReview(event, events, order) {
				result.Counters.ImplementationRework++
			}
		}
	}
	sort.Strings(result.EventIDs)
	return result
}

func hasPriorImplementationReview(start EventEnvelope, events []EventEnvelope, order *CausalOrder) bool {
	for _, candidate := range events {
		if candidate.Kind != "phase_finished" || !order.HappensBefore(candidate.EventID, start.EventID) {
			continue
		}
		var payload PhaseFinishedPayload
		if json.Unmarshal(candidate.Payload, &payload) == nil && payload.Phase == "implementation-review" {
			return true
		}
	}
	return false
}

func addUsage(total *UsageTotals, input, output, cacheRead, cacheWrite uint64, cost float64, duration uint64) {
	total.InputTokens = saturatingAdd(total.InputTokens, input)
	total.OutputTokens = saturatingAdd(total.OutputTokens, output)
	total.CacheReadTokens = saturatingAdd(total.CacheReadTokens, cacheRead)
	total.CacheWriteTokens = saturatingAdd(total.CacheWriteTokens, cacheWrite)
	total.DurationMS = saturatingAdd(total.DurationMS, duration)
	total.CostUSD += cost
}

func saturatingAdd(left, right uint64) uint64 {
	if math.MaxUint64-left < right {
		return math.MaxUint64
	}
	return left + right
}

func integrityNotices(issues []IntegrityIssue, selectedIDs map[string]bool) []IntegrityNotice {
	result := []IntegrityNotice{}
	for _, issue := range issues {
		if len(selectedIDs) != 0 && len(issue.EventIDs) != 0 {
			matched := false
			for _, eventID := range issue.EventIDs {
				matched = matched || selectedIDs[eventID]
			}
			if !matched {
				continue
			}
		}
		severity := "violation"
		if issue.Code == "partial-final-line" || strings.Contains(issue.Code, "clock") {
			severity = "warning"
		}
		result = append(result, IntegrityNotice{
			Code: issue.Code, Severity: severity, Scope: issue.Scope,
			EventIDs:    append([]string(nil), issue.EventIDs...),
			Explanation: integrityExplanation(issue.Code),
		})
	}
	return stableIntegrityNotices(result)
}

func integrityExplanation(code string) string {
	switch code {
	case "partial-final-line":
		return "the incomplete final ledger line was ignored"
	case "unsupported-protocol":
		return "the ledger contains an unsupported protocol event"
	case "malformed-complete-line":
		return "the ledger contains a malformed complete event"
	case "invalid-transition":
		return "a recorded lifecycle event has no applied state effect"
	case "concurrent-state":
		return "concurrent lifecycle mutations have no invented order"
	case "broken-predecessor":
		return "an event names a predecessor absent from the ledger"
	default:
		return "the ledger contains an integrity issue: " + code
	}
}

func stableIntegrityNotices(notices []IntegrityNotice) []IntegrityNotice {
	seen := map[string]bool{}
	result := make([]IntegrityNotice, 0, len(notices))
	for _, notice := range notices {
		sort.Strings(notice.EventIDs)
		key := notice.Code + "\x00" + notice.Scope + "\x00" + strings.Join(notice.EventIDs, "\x00")
		if !seen[key] {
			seen[key] = true
			result = append(result, notice)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		left := result[i].Code + "\x00" + result[i].Scope + "\x00" + strings.Join(result[i].EventIDs, "\x00")
		right := result[j].Code + "\x00" + result[j].Scope + "\x00" + strings.Join(result[j].EventIDs, "\x00")
		return left < right
	})
	return result
}

func projectRetentionState(reads []EffortRead, policy RetentionPolicy, now time.Time, lastRunAt *time.Time) RetentionState {
	state := RetentionState{MaxAgeDays: policy.MaxCompletedEffortAgeDays, MaxCount: policy.MaxCompletedEffortCount, Candidates: []string{}, LastRunAt: lastRunAt}
	newest := []retentionCandidate{}
	for _, read := range reads {
		if len(read.EffectApplied) == 0 {
			read.EffectApplied = projectLifecycleFromRead(read).EffectApplied
		}
		candidate, terminal, err := retentionCandidateFromRead(read, read.Metadata.EffortID)
		if err == nil && terminal {
			newest = append(newest, candidate)
		}
	}
	state.TerminalEffortCount = len(newest)
	sort.Slice(newest, func(i, j int) bool { return candidateNewer(newest[i], newest[j]) })
	selected := map[string]retentionCandidate{}
	if policy.MaxCompletedEffortAgeDays > 0 {
		cutoff := now.Add(-time.Duration(policy.MaxCompletedEffortAgeDays) * 24 * time.Hour)
		for _, candidate := range newest {
			if candidate.TerminalTimestamp.Before(cutoff) {
				selected[candidate.EffortID] = candidate
			}
		}
	}
	if policy.MaxCompletedEffortCount > 0 && len(newest) > policy.MaxCompletedEffortCount {
		for _, candidate := range newest[policy.MaxCompletedEffortCount:] {
			selected[candidate.EffortID] = candidate
		}
	}
	oldest := make([]retentionCandidate, 0, len(selected))
	for _, candidate := range selected {
		oldest = append(oldest, candidate)
	}
	sort.Slice(oldest, func(i, j int) bool { return candidateNewer(oldest[j], oldest[i]) })
	state.Candidates = candidateIDs(oldest)
	return state
}
