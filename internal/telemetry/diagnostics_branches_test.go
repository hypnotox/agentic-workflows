package telemetry

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func otherEffortWithHandoff() EffortRead {
	payload, _ := json.Marshal(HandoffObservedPayload{Outcome: "success", TargetSessionID: "child"})
	event := EventEnvelope{Version: ProtocolVersion{Major: 2}, EventID: "broken-handoff", ObservationID: "observation", EffortID: "other", SessionID: "session", TrajectoryID: "trajectory", Timestamp: "2026-07-22T00:00:00Z", Kind: "handoff_observed", Predecessors: []string{}, Payload: payload}
	return EffortRead{Metadata: EffortMetadata{EffortID: "other"}, Events: []EventEnvelope{event}}
}

func allEffects(events []EventEnvelope) map[string]bool {
	result := map[string]bool{}
	for _, event := range events {
		result[event.EventID] = true
	}
	return result
}

func TestDiagnosticsDefensiveAndHelperBranches(t *testing.T) {
	since := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	until := since.Add(-time.Hour)
	if _, err := DiagnoseExact(nil, Selector{Since: &since, Until: &until}, time.Time{}); err == nil {
		t.Fatal("invalid selector accepted")
	}
	other := "other"
	if result, err := DiagnoseExact([]EffortRead{diagnosticRead(lifecycleBaseEvents())}, Selector{EffortID: &other}, time.Time{}); err != nil || len(result.Findings) != 0 {
		t.Fatalf("effort selector mismatch = %#v, %v", result, err)
	}

	byID := eventsByID(lifecycleBaseEvents())
	for _, issue := range []IntegrityIssue{
		{Code: "partial-final-line"},
		{Code: "clock-invalid"},
		{Code: "unknown-integrity"},
	} {
		finding, ok := findingFromIntegrity(issue, byID, "effort")
		if issue.Code == "partial-final-line" || issue.Code == "unknown-integrity" {
			if ok {
				t.Fatalf("non-diagnostic issue became finding: %#v", finding)
			}
		} else if !ok || finding.Scope != "effort" {
			t.Fatalf("clock issue = %#v, %v", finding, ok)
		}
	}
	for _, code := range []string{"broken-predecessor", "not-event-integrity"} {
		if got := isEventIntegrityCode(code); got != (code == "broken-predecessor") {
			t.Fatalf("isEventIntegrityCode(%q) = %v", code, got)
		}
	}

	malformedTerminal := causalEvent("bad-terminal", "session", "effort_completed", []string{"create"}, EffortTerminalPayload{TerminalEpoch: 1})
	malformedTerminal.Payload = json.RawMessage(`{`)
	noRouteTerminal := causalEvent("no-route-terminal", "session", "effort_completed", []string{"create"}, EffortTerminalPayload{TerminalEpoch: 1})
	defensiveEvents := append(lifecycleBaseEvents(), malformedTerminal, noRouteTerminal)
	workflow := ProjectWorkflow(diagnosticRead(defensiveEvents))
	order, _ := BuildCausalOrder(defensiveEvents)
	if findings := diagnoseTerminalRoutes(diagnosticRead(defensiveEvents), workflow, order); len(findings) != 0 {
		t.Fatalf("malformed/no-route terminal diagnosed as route failure: %#v", findings)
	}

	routes := lifecycleBaseEvents()
	routes = appendEvent(routes, "route-one", "route_selected", RoutePayload{Route: "direct"})
	routes = appendEvent(routes, "route-two", "route_changed", RoutePayload{Route: "plan"})
	routes = appendEvent(routes, "terminal", "effort_completed", EffortTerminalPayload{TerminalEpoch: 1})
	routeOrder, _ := BuildCausalOrder(routes)
	route, eventID := effectiveRouteBefore(routes, map[string]bool{"route-one": true, "route-two": true}, routes[len(routes)-1], routeOrder)
	if route != "plan" || eventID != "route-two" {
		t.Fatalf("effective changed route = %s %s", route, eventID)
	}
	if route, eventID = effectiveRouteBefore(routes, nil, routes[len(routes)-1], routeOrder); route != "" || eventID != "" {
		t.Fatalf("absent applied route = %s %s", route, eventID)
	}
	reversedRouteOne := causalEvent("ordered-one", "one", "route_selected", []string{"create"}, RoutePayload{Route: "direct"})
	reversedRouteTwo := causalEvent("ordered-two", "two", "route_changed", []string{"ordered-one"}, RoutePayload{Route: "plan"})
	reversedTerminal := causalEvent("ordered-terminal", "terminal", "effort_completed", []string{"ordered-two"}, EffortTerminalPayload{TerminalEpoch: 1})
	reversedRoutes := []EventEnvelope{lifecycleBaseEvents()[0], reversedRouteTwo, reversedRouteOne, reversedTerminal}
	reversedOrder, _ := BuildCausalOrder(reversedRoutes)
	if _, selectedID := effectiveRouteBefore(reversedRoutes, map[string]bool{"ordered-one": true, "ordered-two": true}, reversedTerminal, reversedOrder); selectedID != "ordered-two" {
		t.Fatalf("causal route evidence was not sorted: %s", selectedID)
	}
	concurrentRoutes := lifecycleBaseEvents()
	leftRoute := causalEvent("z-route", "left", "route_selected", []string{"create"}, RoutePayload{Route: "direct"})
	rightRoute := causalEvent("a-route", "right", "route_changed", []string{"create"}, RoutePayload{Route: "plan"})
	concurrentTerminal := causalEvent("both-terminal", "terminal", "effort_completed", []string{"z-route", "a-route"}, EffortTerminalPayload{TerminalEpoch: 1})
	concurrentRoutes = append(concurrentRoutes, leftRoute, rightRoute, concurrentTerminal)
	concurrentRouteOrder, _ := BuildCausalOrder(concurrentRoutes)
	if _, selectedID := effectiveRouteBefore(concurrentRoutes, map[string]bool{"z-route": true, "a-route": true}, concurrentTerminal, concurrentRouteOrder); selectedID != "z-route" {
		t.Fatalf("concurrent route evidence was not stable-sorted: %s", selectedID)
	}

	malformedFinish := causalEvent("malformed-finish", "session", "phase_finished", []string{"create"}, PhaseFinishedPayload{Phase: "brainstorming", StartEventID: "missing"})
	malformedFinish.Payload = json.RawMessage(`{`)
	missingFinish := causalEvent("missing-finish", "session", "phase_finished", []string{"create"}, PhaseFinishedPayload{Phase: "brainstorming", StartEventID: "missing"})
	start := causalEvent("start", "session-two", "phase_started", []string{"create"}, PhaseStartedPayload{Phase: "planning"})
	malformedStart := causalEvent("malformed-start", "session-three", "phase_started", []string{"create"}, PhaseStartedPayload{Phase: "planning"})
	malformedStart.Payload = json.RawMessage(`{`)
	malformedStartFinish := causalEvent("malformed-start-finish", "session-three", "phase_finished", []string{"malformed-start"}, PhaseFinishedPayload{Phase: "planning", StartEventID: "malformed-start"})
	mismatch := causalEvent("mismatch", "session-two", "phase_finished", []string{"start"}, PhaseFinishedPayload{Phase: "plan-review", StartEventID: "start"})
	reopen := causalEvent("reopen", "session", "effort_reopened", []string{"create"}, EffortReopenedPayload{TerminalEpoch: 2, TrajectoryID: "new", AnchorID: "anchor"})
	reopenedStart := causalEvent("reopened-start", "session", "phase_started", []string{"reopen"}, PhaseStartedPayload{Phase: "implementation"})
	reopenedFinish := causalEvent("reopened-finish", "session", "phase_finished", []string{"reopened-start"}, PhaseFinishedPayload{Phase: "implementation", StartEventID: "reopened-start"})
	terminal := causalEvent("epoch-two-terminal", "session", "effort_completed", []string{"reopened-finish", "malformed-start-finish", "mismatch"}, EffortTerminalPayload{TerminalEpoch: 2})
	intervalEvents := []EventEnvelope{lifecycleBaseEvents()[0], malformedFinish, missingFinish, start, mismatch, malformedStart, malformedStartFinish, reopen, reopenedStart, reopenedFinish, terminal}
	intervalOrder, _ := BuildCausalOrder(intervalEvents)
	intervals := completedIntervalsBefore(intervalEvents, allEffects(intervalEvents), 2, terminal.EventID, intervalOrder)
	if len(intervals) != 1 || intervals[0].StartEventID != "reopened-start" {
		t.Fatalf("defensive/epoch interval projection = %#v", intervals)
	}
	if got := completedIntervalsBefore(intervalEvents, allEffects(intervalEvents), 1, terminal.EventID, intervalOrder); len(got) != 0 {
		t.Fatalf("wrong epoch retained intervals: %#v", got)
	}

	reentry := completedRoute("adr")
	reentry = reentry[:len(reentry)-1]
	reentry = appendFinishedPhase(reentry, "second-authoring", "adr-authoring", "")
	reentry = appendEvent(reentry, "reentry-terminal", "effort_completed", EffortTerminalPayload{TerminalEpoch: 1})
	reentryOrder, _ := BuildCausalOrder(reentry)
	reentryIntervals := completedIntervalsBefore(reentry, allEffects(reentry), 1, "reentry-terminal", reentryOrder)
	maximal := maximalPhaseFinishes("adr-authoring", reentryIntervals, "reentry-terminal", reentryOrder)
	if len(maximal) != 1 || maximal[0] != "second-authoring-finish" {
		t.Fatalf("maximal phase finish = %#v", maximal)
	}
	if got := phaseIntervals("adr-authoring", append([]PhaseInterval{{Phase: "implementation", StartEventID: "ignored"}}, reentryIntervals...)); len(got) != 2 || got[0].StartEventID > got[1].StartEventID {
		t.Fatalf("phase filtering/sorting = %#v", got)
	}

	badClockFinish := causalEvent("bad-clock-finish", "session", "phase_finished", []string{"missing-clock-start"}, PhaseFinishedPayload{Phase: "planning", StartEventID: "missing-clock-start"})
	badClockFinish.Payload = json.RawMessage(`{`)
	missingClockStart := causalEvent("missing-clock-finish", "session", "phase_finished", []string{"create"}, PhaseFinishedPayload{Phase: "planning", StartEventID: "missing-clock-start"})
	clockEvents := append(lifecycleBaseEvents(), badClockFinish, missingClockStart)
	clockOrder, _ := BuildCausalOrder(clockEvents)
	if got := diagnoseClockIntegrity(diagnosticRead(clockEvents), clockOrder); len(got) != 0 {
		t.Fatalf("malformed/unlinked clock endpoint diagnosed: %#v", got)
	}
	longClock := lifecycleBaseEvents()
	longClock = appendFinishedPhase(longClock, "long", "planning", "")
	longClock[len(longClock)-2].Timestamp = "0001-01-01T00:00:00Z"
	longClock[len(longClock)-1].Timestamp = "9999-12-31T23:59:59Z"
	longOrder, _ := BuildCausalOrder(longClock)
	if got := diagnoseClockIntegrity(diagnosticRead(longClock), longOrder); len(got) != 1 {
		t.Fatalf("protocol-bound clock interval = %#v", got)
	}

	handoffFailure, _ := json.Marshal(HandoffObservedPayload{Outcome: "failure", TargetSessionID: "child"})
	malformedHandoff := causalEvent("malformed-handoff", "session", "handoff_observed", []string{"create"}, HandoffObservedPayload{Outcome: "success", TargetSessionID: "child"})
	malformedHandoff.Payload = json.RawMessage(`{`)
	failedHandoff := causalEvent("failed-handoff", "session", "handoff_observed", []string{"create"}, HandoffObservedPayload{})
	failedHandoff.Payload, failedHandoff.ObservationID, failedHandoff.IdempotencyKey = handoffFailure, "observation", ""
	if got := diagnoseHandoffs([]EffortRead{diagnosticRead(append(lifecycleBaseEvents(), malformedHandoff, failedHandoff))}); len(got) != 0 {
		t.Fatalf("failed/malformed handoff diagnosed: %#v", got)
	}

	if transitionRepair(nil, byID) != nil || transitionRepair([]string{"missing"}, byID) != nil {
		t.Fatal("under-specified transition remediation produced a proposal")
	}
	if concurrentRepair([]string{"one"}, byID) != nil || concurrentRepair([]string{"missing", "other"}, byID) != nil {
		t.Fatal("under-specified concurrent remediation produced a proposal")
	}
	passivePayload, _ := json.Marshal(CompactionObservedPayload{Count: 1})
	passive := EventEnvelope{Kind: "compaction_observed", Payload: passivePayload}
	if concurrentRepair([]string{"passive", "other"}, map[string]EventEnvelope{"passive": passive}) != nil {
		t.Fatal("non-repairable concurrent evidence produced a proposal")
	}

	session := "absent"
	if findingMatchesSelection(exactFinding("code", "warning", "scope", []string{"event"}, "why", "next"), map[string]bool{}, Selector{SessionID: &session}) {
		t.Fatal("unselected finding matched selector")
	}
	if equalStringSets([]string{"a"}, []string{"b"}) || equalStringSets([]string{"a"}, nil) {
		t.Fatal("unequal evidence sets compared equal")
	}
	if len(boundedDiagnosticScope(strings.Repeat("x", descriptor.Limits.CategoryBytes+1))) > descriptor.Limits.CategoryBytes {
		t.Fatal("diagnostic scope exceeds protocol category bound")
	}
	if got := uintString(0); got != "0" {
		t.Fatalf("uintString zero = %q", got)
	}

	handoffFinding := exactFinding("WFV1-HANDOFF-ASSOCIATION", "warning", "scope", []string{"broken-handoff"}, "why", "next")
	handoffFinding.EffortID = "other"
	if !findingBelongsToEffort(handoffFinding, []EffortRead{otherEffortWithHandoff()}, "other") {
		t.Fatal("handoff finding was not attributed to its canonical owner")
	}
	finding := exactFinding("code", "warning", "scope", []string{"missing"}, "why", "next")
	finding.EffortID = "effort"
	selected := selectedForFinding(finding, map[string]map[string]bool{"effort": {"missing": true}})
	if !selected["missing"] {
		t.Fatalf("selected evidence union = %#v", selected)
	}
	if projection := lifecycleForFinding(nil, finding); projection.State != "" {
		t.Fatalf("missing finding lifecycle = %#v", projection)
	}
}

func TestProtocol2DiagnosticTransitionBranches(t *testing.T) {
	events := lifecycleBaseEvents()
	events = appendEvent(events, "brainstorm", "phase_started", PhaseStartedPayload{Phase: "brainstorming"})
	events = appendEvent(events, "to-implementation", "phase_transitioned", PhaseTransitionedPayload{Phase: "brainstorming", StartEventID: "brainstorm", NextPhase: "implementation", RouteAction: "select", Route: "direct"})
	events = appendEvent(events, "to-review", "phase_transitioned", PhaseTransitionedPayload{Phase: "implementation", StartEventID: "to-implementation", NextPhase: "implementation-review"})
	events = appendEvent(events, "to-retrospective", "phase_transitioned", PhaseTransitionedPayload{Phase: "implementation-review", StartEventID: "to-review", NextPhase: "retrospective"})
	terminal := causalEvent("terminal", "session", "effort_completed", []string{"to-retrospective"}, EffortTerminalPayload{TerminalEpoch: 1})
	events = append(events, terminal)
	order, _ := BuildCausalOrder(events)
	applied := allEffects(events)
	if route, eventID := effectiveRouteBefore(events, applied, terminal, order); route != "direct" || eventID != "to-implementation" {
		t.Fatalf("transition route = %q %q", route, eventID)
	}
	intervals := completedIntervalsBefore(events, applied, 1, terminal.EventID, order)
	if len(intervals) != 3 {
		t.Fatalf("transition intervals = %#v", intervals)
	}

	malformed := causalEvent("malformed-transition", "session", "phase_transitioned", []string{"brainstorm"}, PhaseTransitionedPayload{})
	malformed.Payload = json.RawMessage("{")
	missing := causalEvent("missing-transition", "session", "phase_transitioned", []string{"brainstorm"}, PhaseTransitionedPayload{Phase: "brainstorming", StartEventID: "missing", NextPhase: "planning"})
	badStart := causalEvent("bad-start-transition", "session", "phase_transitioned", []string{"create"}, PhaseTransitionedPayload{Phase: "brainstorming", StartEventID: "create", NextPhase: "planning"})
	badStart.Payload = json.RawMessage("{")
	finishBadStart := causalEvent("finish-bad-start", "session", "phase_finished", []string{"bad-start-transition"}, PhaseFinishedPayload{Phase: "planning", StartEventID: "bad-start-transition"})
	defensiveTerminal := causalEvent("defensive-terminal", "session", "effort_completed", []string{"finish-bad-start"}, EffortTerminalPayload{TerminalEpoch: 1})
	defensive := append(lifecycleBaseEvents(), malformed, missing, badStart, finishBadStart, defensiveTerminal)
	defensiveOrder, _ := BuildCausalOrder(defensive)
	_, _ = effectiveRouteBefore(defensive, allEffects(defensive), defensiveTerminal, defensiveOrder)
	_ = completedIntervalsBefore(defensive, allEffects(defensive), 1, defensiveTerminal.EventID, defensiveOrder)
}
