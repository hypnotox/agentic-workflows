package telemetry

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestAggregateMetricsBranchContracts(t *testing.T) {
	badPhase := "bad"
	if _, err := AggregateMetrics(nil, Selector{Phase: &badPhase}, MetricsOptions{}); err == nil {
		t.Fatal("invalid selector reached aggregation")
	}
	result, err := AggregateMetrics(nil, Selector{}, MetricsOptions{})
	if err != nil || result.GeneratedAt.IsZero() || result.Efforts == nil || result.Integrity == nil {
		t.Fatalf("default aggregate result = %#v err=%v", result, err)
	}

	other := "other"
	read := EffortRead{Metadata: EffortMetadata{EffortID: "effort"}, Events: lifecycleBaseEvents()}
	result, err = AggregateMetrics([]EffortRead{read}, Selector{EffortID: &other}, MetricsOptions{GeneratedAt: time.Now()})
	if err != nil || len(result.Efforts) != 0 {
		t.Fatalf("nonmatching effort projection = %#v err=%v", result, err)
	}
}

func TestAggregateScopeEveryCounterAndMalformedPayloadBranch(t *testing.T) {
	events := []EventEnvelope{}
	appendPassive := func(id string, kind EventKind, payload any) {
		raw, _ := json.Marshal(payload)
		events = append(events, EventEnvelope{Version: ProtocolVersion{Major: 2}, EventID: id, ObservationID: "observation-" + id, EffortID: "effort", SessionID: "session", Timestamp: "2026-07-22T00:00:00Z", Kind: kind, Predecessors: []string{}, Payload: raw})
	}
	appendPassive("handoff", "handoff_observed", HandoffObservedPayload{Outcome: "failure", TargetSessionID: "target"})
	appendPassive("tool", "tool_observed", ToolObservedPayload{Tool: "read", Outcome: "failure", DurationMS: 1})
	appendPassive("tool-success", "tool_observed", ToolObservedPayload{Tool: "read", Outcome: "success", DurationMS: 1})
	appendPassive("gate", "shell_observed", ShellObservedPayload{Classification: "gate", Outcome: "failure", GateMode: "standard"})
	appendPassive("unclassified", "shell_observed", ShellObservedPayload{Classification: "unclassified", Outcome: "failure"})
	reviewStart := causalEvent("review-start", "session", "phase_started", nil, PhaseStartedPayload{Phase: "implementation-review"})
	reviewFinish := causalEvent("review-finish", "session", "phase_finished", []string{"review-start"}, PhaseFinishedPayload{Phase: "implementation-review", StartEventID: "review-start"})
	implementation := causalEvent("implementation-again", "session", "phase_started", []string{"review-finish"}, PhaseStartedPayload{Phase: "implementation"})
	transitionReviewStart := causalEvent("transition-review-start", "session", "phase_started", []string{"implementation-again"}, PhaseStartedPayload{Phase: "implementation-review"})
	transitionFromReview := causalEvent("transition-from-review", "session", "phase_transitioned", []string{"transition-review-start"}, PhaseTransitionedPayload{Phase: "implementation-review", StartEventID: "transition-review-start", NextPhase: "planning"})
	transitionToImplementation := causalEvent("transition-to-implementation", "session", "phase_transitioned", []string{"transition-from-review"}, PhaseTransitionedPayload{Phase: "planning", StartEventID: "transition-from-review", NextPhase: "implementation"})
	events = append(events, reviewStart, reviewFinish, implementation, transitionReviewStart, transitionFromReview, transitionToImplementation)
	events = append(events, EventEnvelope{EventID: "duplicate-id"}, EventEnvelope{EventID: "duplicate-id"})
	for _, kind := range []EventKind{"usage_observed", "subagent_observed", "compaction_observed", "tool_observed", "shell_observed", "phase_started", "phase_transitioned"} {
		events = append(events, EventEnvelope{EventID: "malformed-" + string(kind), Kind: kind, Payload: json.RawMessage("{")})
	}
	scope := aggregateScope("scope", events)
	if scope.Counters.Handoffs != 1 || scope.Counters.ToolFailures != 1 || scope.Counters.GateFailures != 1 || scope.Counters.ImplementationRework != 2 {
		t.Fatalf("counter branches = %#v", scope.Counters)
	}
	if countString(scope.EventIDs, "duplicate-id") != 1 {
		t.Fatalf("duplicate event IDs = %#v", scope.EventIDs)
	}
	groups := aggregateGroupedScopes(events[:1], func(EventEnvelope) []string { return []string{"", "kept"} })
	if len(groups) != 1 || groups[0].ScopeID != "kept" {
		t.Fatalf("empty scope key was retained: %#v", groups)
	}
}

func TestIntegrityProjectionStableCasesAndFiltering(t *testing.T) {
	selected := map[string]bool{"selected": true}
	issues := []IntegrityIssue{
		{Code: "unsupported-protocol", Scope: "scope", EventIDs: []string{"selected"}},
		{Code: "malformed-complete-line", Scope: "scope", EventIDs: []string{"selected"}},
		{Code: "invalid-transition", Scope: "scope", EventIDs: []string{"selected"}},
		{Code: "concurrent-state", Scope: "scope", EventIDs: []string{"selected"}},
		{Code: "broken-predecessor", Scope: "scope", EventIDs: []string{"selected"}},
		{Code: "clock-invalid", Scope: "scope", EventIDs: []string{"selected"}},
		{Code: "other", Scope: "scope", EventIDs: []string{"selected"}},
		{Code: "filtered", Scope: "scope", EventIDs: []string{"other"}},
	}
	notices := integrityNotices(issues, selected)
	if len(notices) != 7 {
		t.Fatalf("integrity filtering = %#v", notices)
	}
	duplicate := append(append([]IntegrityNotice{}, notices...), notices[0])
	stable := stableIntegrityNotices(duplicate)
	if len(stable) != len(notices) || stable[0].Code > stable[len(stable)-1].Code {
		t.Fatalf("stable integrity notices = %#v", stable)
	}
}

func TestRetentionProjectionCountLastRunAndInvalidCandidate(t *testing.T) {
	terminal := completedRoute("direct")
	for index := range terminal {
		terminal[index].EffortID = "terminal"
		terminal[index].Timestamp = "2026-07-20T00:00:00Z"
	}
	valid := EffortRead{Metadata: EffortMetadata{EffortID: "terminal", CreatedAt: "2026-07-01T00:00:00Z"}, Events: terminal}
	invalid := valid
	invalid.Events = append([]EventEnvelope(nil), valid.Events...)
	invalid.Metadata.EffortID = "invalid"
	invalid.Metadata.CreatedAt = "not-a-time"
	for index := range invalid.Events {
		invalid.Events[index].EffortID = "invalid"
	}
	lastRun := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	state := projectRetentionState([]EffortRead{valid, invalid}, RetentionPolicy{MaxCompletedEffortCount: 0}, time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC), &lastRun)
	if state.TerminalEffortCount != 1 || state.LastRunAt == nil || !state.LastRunAt.Equal(lastRun) {
		t.Fatalf("retention projection = %#v", state)
	}

	second := valid
	second.Events = append([]EventEnvelope(nil), valid.Events...)
	second.Metadata.EffortID = "second"
	second.Metadata.CreatedAt = "2026-07-02T00:00:00Z"
	for index := range second.Events {
		second.Events[index].EffortID = "second"
		second.Events[index].Timestamp = "2026-07-21T00:00:00Z"
	}
	third := valid
	third.Events = append([]EventEnvelope(nil), valid.Events...)
	third.Metadata.EffortID = "third"
	third.Metadata.CreatedAt = "2026-07-03T00:00:00Z"
	for index := range third.Events {
		third.Events[index].EffortID = "third"
		third.Events[index].Timestamp = "2026-07-19T00:00:00Z"
	}
	state = projectRetentionState([]EffortRead{valid, second, third}, RetentionPolicy{MaxCompletedEffortCount: 1}, time.Now(), nil)
	if state.TerminalEffortCount != 3 || len(state.Candidates) != 2 || state.Candidates[0] != "third" || state.Candidates[1] != "terminal" {
		t.Fatalf("count retention projection = %#v", state)
	}
}

func TestRenderMetricsHumanWriteFailures(t *testing.T) {
	result := MetricsResult{SchemaVersion: 1, ProtocolMajor: 2, GeneratedAt: time.Now(), Efforts: []EffortProjection{{EffortID: "effort", CurrentPath: ScopeProjection{ScopeID: "current"}, AllWork: ScopeProjection{ScopeID: "all"}}}, Retention: RetentionState{Candidates: []string{}}, Integrity: []IntegrityNotice{{Code: "code", Severity: "warning", Scope: "scope", EventIDs: []string{}, Explanation: "explanation"}}}
	for failAt := range 6 {
		writer := &failAtWriter{failAt: failAt}
		if err := RenderMetricsHuman(writer, result); err == nil {
			t.Fatalf("write failure %d was ignored", failAt)
		}
	}
}

type failAtWriter struct {
	calls  int
	failAt int
}

func (w *failAtWriter) Write(value []byte) (int, error) {
	if w.calls == w.failAt {
		return 0, errors.New("write failed")
	}
	w.calls++
	return len(value), nil
}

func countString(values []string, target string) int {
	count := 0
	for _, value := range values {
		if value == target {
			count++
		}
	}
	return count
}
