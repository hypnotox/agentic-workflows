package telemetry

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func lifecycleBaseEvents() []EventEnvelope {
	return []EventEnvelope{causalEvent("create", "session", "effort_created", nil, EffortCreatedPayload{CreationMode: "independent"})}
}

func appendEvent(events []EventEnvelope, id string, kind EventKind, payload any) []EventEnvelope {
	predecessors := []string{}
	if len(events) != 0 {
		predecessors = []string{events[len(events)-1].EventID}
	}
	return append(events, causalEvent(id, "session", kind, predecessors, payload))
}

func appendFinishedPhase(events []EventEnvelope, prefix string, phase Phase, trajectory string) []EventEnvelope {
	start := causalEvent(prefix+"-start", "session", "phase_started", []string{events[len(events)-1].EventID}, PhaseStartedPayload{Phase: phase})
	start.TrajectoryID = trajectory
	finish := causalEvent(prefix+"-finish", "session", "phase_finished", []string{start.EventID}, PhaseFinishedPayload{Phase: phase, StartEventID: start.EventID})
	finish.TrajectoryID = trajectory
	return append(events, start, finish)
}

func completedRoute(route Route) []EventEnvelope {
	events := lifecycleBaseEvents()
	events = appendEvent(events, "route", "route_selected", RoutePayload{Route: route})
	for index, phase := range routeRequirements[route] {
		events = appendFinishedPhase(events, string(rune('a'+index)), phase, "")
	}
	events = appendEvent(events, "complete", "effort_completed", EffortTerminalPayload{TerminalEpoch: 1})
	return events
}

func TestEffortLifecycleAndRoutes(t *testing.T) {
	for route := range routeRequirements {
		projection := ProjectLifecycle(completedRoute(route))
		if projection.State != EffortCompleted || len(projection.Invalid) != 0 {
			t.Errorf("route %s projection = state %s invalid %#v", route, projection.State, projection.Invalid)
		}
	}

	missing := lifecycleBaseEvents()
	missing = appendEvent(missing, "route", "route_selected", RoutePayload{Route: "direct"})
	missing = appendEvent(missing, "complete", "effort_completed", EffortTerminalPayload{TerminalEpoch: 1})
	if projection := ProjectLifecycle(missing); projection.State == EffortCompleted || !hasIssue(projection.Invalid, "invalid-transition") {
		t.Fatalf("completion without route evidence applied: %#v", projection)
	}

	earlyReentry := lifecycleBaseEvents()
	earlyReentry = appendEvent(earlyReentry, "early-route", "route_selected", RoutePayload{Route: "direct"})
	earlyReentry = appendFinishedPhase(earlyReentry, "early-implementation", "implementation", "")
	for index, phase := range routeRequirements["direct"] {
		earlyReentry = appendFinishedPhase(earlyReentry, "proper-"+string(rune('a'+index)), phase, "")
	}
	earlyReentry = appendEvent(earlyReentry, "early-complete", "effort_completed", EffortTerminalPayload{TerminalEpoch: 1})
	if projection := ProjectLifecycle(earlyReentry); projection.State != EffortCompleted {
		t.Fatalf("legal early phase reentry blocked: %#v", projection.Invalid)
	}

	investigation := lifecycleBaseEvents()
	investigation = appendFinishedPhase(investigation, "impl", "implementation", "")
	investigation = appendEvent(investigation, "route", "route_selected", RoutePayload{Route: "investigation-only"})
	if projection := ProjectLifecycle(investigation); projection.Route != "" {
		t.Fatal("investigation-only accepted after implementation")
	}

	concurrent := lifecycleBaseEvents()
	concurrent = appendEvent(concurrent, "route-a", "route_selected", RoutePayload{Route: "direct"})
	concurrent[len(concurrent)-1].Predecessors = []string{"create"}
	concurrent = appendEvent(concurrent, "route-b", "route_selected", RoutePayload{Route: "plan"})
	concurrent[len(concurrent)-1].Predecessors = []string{"create"}
	concurrent[len(concurrent)-1].SessionID = "concurrent-session"
	if projection := ProjectLifecycle(concurrent); !hasIssue(projection.Invalid, "concurrent-state") || projection.Route != "" {
		t.Fatalf("concurrent route mutation gained an invented order: %#v", projection)
	}

	terminalCorrections := completedRoute("direct")
	replacement, _ := json.Marshal(RoutePayload{Route: "direct"})
	terminalCorrections = appendEvent(terminalCorrections, "repair", "repair_applied", RepairAppliedPayload{ProposalKind: "supersede-event", SourceEventIDs: []string{"route"}, Replacement: RepairReplacement{EventKind: "route_selected", Payload: replacement}})
	terminalCorrections = appendEvent(terminalCorrections, "waiver", "finding_waived", FindingWaivedPayload{RuleCode: "WFV1-PHASE-OVERLAP", Scope: "trajectory", EvidenceIDs: []string{"evidence"}, ReasonCode: "approved-phase-overlap"})
	if projection := ProjectLifecycle(terminalCorrections); projection.State != EffortCompleted || len(projection.Repairs) != 1 || len(projection.Waivers) != 1 {
		t.Fatalf("terminal repair or waiver changed effort state: %#v", projection)
	}

	illegal := lifecycleBaseEvents()
	illegal = appendEvent(illegal, "illegal-route-change", "route_changed", RoutePayload{Route: "plan"})
	illegalReplacement, _ := json.Marshal(RoutePayload{Route: "direct"})
	illegal = appendEvent(illegal, "illegal-repair", "repair_applied", RepairAppliedPayload{ProposalKind: "supersede-event", SourceEventIDs: []string{"illegal-route-change"}, Replacement: RepairReplacement{EventKind: "route_selected", Payload: illegalReplacement}})
	if projection := ProjectLifecycle(illegal); projection.Route != "direct" || projection.State != EffortActive || projection.EffectApplied["illegal-route-change"] || !projection.EffectApplied["illegal-repair"] || len(projection.Repairs) != 1 {
		t.Fatalf("illegal retained evidence was not repairable: %#v", projection)
	}

	concurrentRepair := lifecycleBaseEvents()
	concurrentRepair = appendEvent(concurrentRepair, "concurrent-a", "route_selected", RoutePayload{Route: "direct"})
	concurrentRepair[len(concurrentRepair)-1].Predecessors = []string{"create"}
	concurrentRepair = appendEvent(concurrentRepair, "concurrent-b", "route_selected", RoutePayload{Route: "plan"})
	concurrentRepair[len(concurrentRepair)-1].Predecessors = []string{"create"}
	concurrentRepair[len(concurrentRepair)-1].SessionID = "concurrent-session"
	concurrentRepair = appendEvent(concurrentRepair, "concurrent-repair", "repair_applied", RepairAppliedPayload{ProposalKind: "supersede-event", SourceEventIDs: []string{"concurrent-a", "concurrent-b"}, Replacement: RepairReplacement{EventKind: "route_selected", Payload: illegalReplacement}})
	concurrentRepair[len(concurrentRepair)-1].Predecessors = []string{"concurrent-a", "concurrent-b"}
	if projection := ProjectLifecycle(concurrentRepair); projection.Route != "direct" || projection.State != EffortActive || projection.EffectApplied["concurrent-a"] || projection.EffectApplied["concurrent-b"] || !projection.EffectApplied["concurrent-repair"] {
		t.Fatalf("concurrent retained evidence was not repairable: %#v", projection)
	}

	reopened := completedRoute("direct")
	reopened = appendEvent(reopened, "reopen", "effort_reopened", EffortReopenedPayload{TerminalEpoch: 2, TrajectoryID: "reopened-trajectory", AnchorID: "reopen-anchor"})
	reopened[len(reopened)-1].TrajectoryID = "reopened-trajectory"
	if projection := ProjectLifecycle(reopened); projection.State != EffortActive || projection.TerminalEpoch != 2 || projection.ActiveTrajectoryID != "reopened-trajectory" {
		t.Fatalf("reopen did not establish a new terminal epoch and trajectory: %#v", projection)
	}
}

// invariant: tooling/workflow-telemetry:effort-lifecycle-and-routes
func TestProtocol2SingleEventPhaseTransitionAndRouteEffect(t *testing.T) {
	events := lifecycleBaseEvents()
	events = appendEvent(events, "brainstorm-start", "phase_started", PhaseStartedPayload{Phase: "brainstorming"})
	transition := protocol2TransitionEnvelope(t, "transition", []string{"brainstorm-start"}, "implementation", "direct")
	projection := ProjectLifecycle(append(events, transition))
	if projection.State != EffortActive || projection.Route != "direct" {
		t.Fatalf("transition route effect = state %q route %q invalid %#v", projection.State, projection.Route, projection.Invalid)
	}
	if len(projection.PhaseIntervals) != 1 || projection.PhaseIntervals[0].Phase != "brainstorming" || projection.PhaseIntervals[0].FinishEventID != "transition" {
		t.Fatalf("transition did not close predecessor phase: %#v", projection.PhaseIntervals)
	}
	interval, ok := projection.OpenPhases["transition"]
	if !ok || interval.Phase != "implementation" || interval.StartEventID != "transition" {
		t.Fatalf("transition did not open successor phase: %#v", projection.OpenPhases)
	}
}

func TestProtocol2CompetingTransitionsRemainConcurrentEvidence(t *testing.T) {
	events := lifecycleBaseEvents()
	events = appendEvent(events, "brainstorm-start", "phase_started", PhaseStartedPayload{Phase: "brainstorming"})
	left := protocol2TransitionEnvelope(t, "left-transition", []string{"brainstorm-start"}, "implementation", "direct")
	right := protocol2TransitionEnvelope(t, "right-transition", []string{"brainstorm-start"}, "planning", "plan")
	right.SessionID = "other-session"
	projection := ProjectLifecycle(append(events, left, right))
	if !hasInvalidEvent(projection.Invalid, left.EventID) || !hasInvalidEvent(projection.Invalid, right.EventID) || !hasIssue(projection.Invalid, "concurrent-state") {
		t.Fatalf("competing transitions did not remain concurrent evidence: %#v", projection)
	}
	if projection.Route != "" || len(projection.PhaseIntervals) != 0 || len(projection.OpenPhases) != 1 {
		t.Fatalf("competing transition effects were invented: %#v", projection)
	}
}

func protocol2TransitionEnvelope(t *testing.T, eventID string, predecessors []string, nextPhase, route string) EventEnvelope {
	t.Helper()
	event := protocol2TransitionEvent(eventID, "brainstorm-start", predecessors, "brainstorming", nextPhase, "select", route)
	raw := mustJSON(t, event)
	var envelope EventEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope
}

// invariant: tooling/workflow-telemetry:effort-lifecycle-and-routes
// invariant: tooling/workflow-telemetry:trajectory-and-derived-effort-model
func TestProtocol21AdoptionAndContinuationProjection(t *testing.T) {
	adopted := protocol21Envelope(t, "adopt", "effort_adopted", nil, map[string]any{
		"creationMode": "adopted", "route": "direct", "phase": "implementation",
		"workflow": "executing-direct", "trajectoryId": "adopted-trajectory", "anchorId": "adopted-anchor",
		"associationOrigin": "manual",
	})
	continued := protocol21Envelope(t, "continue", "phase_continued", []string{"adopt"}, map[string]any{
		"phase": "implementation", "startEventId": "adopt", "workflow": "tdd", "activity": "tdd",
	})
	cleared := protocol21Envelope(t, "clear", "phase_continued", []string{"continue"}, map[string]any{
		"phase": "implementation", "startEventId": "adopt", "workflow": "executing-direct",
	})
	projection := ProjectLifecycle([]EventEnvelope{adopted, continued, cleared})
	if projection.State != EffortActive || projection.Route != "direct" || projection.ActiveTrajectoryID != "adopted-trajectory" {
		t.Fatalf("adoption state was not projected: %#v", projection)
	}
	interval, ok := projection.OpenPhases["adopt"]
	if !ok || interval.Phase != "implementation" || interval.StartEventID != "adopt" || len(projection.PhaseIntervals) != 0 {
		t.Fatalf("continuation did not preserve the adopted phase start: %#v", projection)
	}
	association, ok := projection.Associations["session-id"]
	if !ok || association.TrajectoryID != "adopted-trajectory" || association.AssociationOrigin != "manual" {
		t.Fatalf("adoption association was not projected: %#v", projection.Associations)
	}
	if !projection.EffectApplied["adopt"] || !projection.EffectApplied["continue"] || !projection.EffectApplied["clear"] {
		t.Fatalf("new lifecycle effects were not applied: %#v", projection.EffectApplied)
	}
	if projection.CurrentWorkflow != "executing-direct" || projection.CurrentActivity != "" || projection.CurrentImplementationMode != "" {
		t.Fatalf("continuation omission did not replace and clear current attribution: %#v", projection)
	}
	interval = projection.OpenPhases["adopt"]
	if interval.Workflow != "executing-direct" || interval.Activity != "" || interval.ImplementationMode != "" {
		t.Fatalf("continuation omission did not clear interval attribution: %#v", interval)
	}
}

// invariant: tooling/workflow-telemetry:trajectory-and-derived-effort-model
func TestPhaseLifecycleUpdatesAndClearsCurrentAttribution(t *testing.T) {
	events := lifecycleBaseEvents()
	started := appendEvent(events, "start", "phase_started", PhaseStartedPayload{Phase: "implementation", Activity: "tdd", ImplementationMode: "inline-execution"})
	projection := ProjectLifecycle(started)
	if projection.CurrentWorkflow != "" || projection.CurrentActivity != "tdd" || projection.CurrentImplementationMode != "inline-execution" {
		t.Fatalf("phase start attribution = %#v", projection)
	}
	finished := appendEvent(started, "finish", "phase_finished", PhaseFinishedPayload{Phase: "implementation", StartEventID: "start", Outcome: "success"})
	projection = ProjectLifecycle(finished)
	if projection.CurrentWorkflow != "" || projection.CurrentActivity != "" || projection.CurrentImplementationMode != "" {
		t.Fatalf("phase finish retained current attribution: %#v", projection)
	}
}

// invariant: tooling/workflow-telemetry:trajectory-and-derived-effort-model
func TestProtocol21DetourLineageAndPostTerminalReturnProjection(t *testing.T) {
	origin := map[string]any{"effortId": "parent-effort", "trajectoryId": "parent-trajectory", "anchorId": "parent-anchor"}
	started := protocol21Envelope(t, "detour-start", "detour_started", nil, map[string]any{
		"creationMode": "derived", "origin": origin, "returnPhase": "implementation",
		"returnPhaseStartEventId": "parent-phase-start", "trajectoryId": "child-trajectory",
		"anchorId": "child-anchor", "workflow": "brainstorming", "associationOrigin": "detour",
	})
	abandoned := protocol21Envelope(t, "detour-abandon", "effort_abandoned", []string{"detour-start"}, map[string]any{"terminalEpoch": 1})
	returned := protocol21Envelope(t, "detour-return", "detour_returned", []string{"detour-abandon"}, map[string]any{
		"terminalOutcome": "abandoned", "parentAssociationEventId": "parent-return-association",
	})
	metadata := protocol21Metadata(t, map[string]any{
		"effortId": "effort-id", "createdAt": started.Timestamp, "creationMode": "derived", "origin": origin,
		"detourReturn": map[string]any{"sessionId": "session-id", "phase": "implementation", "phaseStartEventId": "parent-phase-start"},
	})
	workflow := ProjectWorkflow(EffortRead{Metadata: metadata, Events: []EventEnvelope{started, abandoned, returned}})
	if workflow.Origin == nil || workflow.Origin.EffortID != "parent-effort" || workflow.Origin.TrajectoryID != "parent-trajectory" || workflow.Origin.AnchorID != "parent-anchor" {
		t.Fatalf("detour lineage was not projected: %#v", workflow)
	}
	if workflow.Lifecycle.State != EffortAbandoned || !workflow.Lifecycle.EffectApplied["detour-return"] {
		t.Fatalf("post-terminal return marker was not projected without changing outcome: %#v", workflow.Lifecycle)
	}
	metadataJSON, err := json.Marshal(workflow.Metadata)
	if err != nil {
		t.Fatal(err)
	}
	var metadataFields map[string]any
	if err := json.Unmarshal(metadataJSON, &metadataFields); err != nil {
		t.Fatal(err)
	}
	if metadataFields["detourReturn"] == nil {
		t.Fatalf("canonical detour return metadata was not retained: %s", metadataJSON)
	}
}

func TestProtocol21LifecycleRejectsInvalidCreationContinuationAndReturnState(t *testing.T) {
	adopted := protocol21Envelope(t, "adopt", "effort_adopted", nil, map[string]any{
		"creationMode": "adopted", "phase": "implementation", "workflow": "executing-direct",
		"trajectoryId": "trajectory", "anchorId": "anchor", "associationOrigin": "manual",
	})
	detour := protocol21Envelope(t, "detour", "detour_started", nil, map[string]any{
		"creationMode": "derived", "origin": map[string]any{"effortId": "parent", "trajectoryId": "parent-trajectory", "anchorId": "parent-anchor"},
		"returnPhase": "implementation", "returnPhaseStartEventId": "parent-start", "trajectoryId": "trajectory",
		"anchorId": "anchor", "workflow": "brainstorming", "associationOrigin": "detour",
	})
	for _, creation := range []EventEnvelope{adopted, detour} {
		projection := newLifecycleProjection()
		projection.State = EffortDiscovery
		projection.AppliedEventIDs = []string{"existing"}
		order, _ := BuildCausalOrder([]EventEnvelope{creation})
		if err := projection.apply(creation, order); err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("duplicate %s accepted: %v", creation.Kind, err)
		}
	}

	continued := protocol21Envelope(t, "continue", "phase_continued", nil, map[string]any{
		"phase": "implementation", "startEventId": "start", "workflow": "tdd",
	})
	projection := newLifecycleProjection()
	projection.State = EffortActive
	order, _ := BuildCausalOrder([]EventEnvelope{continued})
	if err := projection.apply(continued, order); err == nil || !strings.Contains(err.Error(), "unmatched matching start") {
		t.Fatalf("continuation without open phase accepted: %v", err)
	}
	start := protocol21Envelope(t, "start", "phase_started", nil, map[string]any{"phase": "implementation"})
	continued.SessionID = "other-session"
	projection.OpenPhases[start.EventID] = PhaseInterval{Phase: "implementation", StartEventID: start.EventID}
	order, _ = BuildCausalOrder([]EventEnvelope{start, continued})
	if err := projection.apply(continued, order); err == nil || !strings.Contains(err.Error(), "causally visible") {
		t.Fatalf("concurrent continuation accepted: %v", err)
	}

	returned := protocol21Envelope(t, "returned", "detour_returned", nil, map[string]any{"terminalOutcome": "completed", "parentAssociationEventId": "parent-association"})
	projection = newLifecycleProjection()
	projection.State = EffortAbandoned
	order, _ = BuildCausalOrder([]EventEnvelope{returned})
	if err := projection.apply(returned, order); err == nil || !strings.Contains(err.Error(), "outcome") {
		t.Fatalf("mismatched terminal outcome accepted: %v", err)
	}
	returned.Payload = mustJSON(t, map[string]any{"terminalOutcome": "abandoned", "parentAssociationEventId": "parent-association"})
	if err := projection.apply(returned, order); err == nil || !strings.Contains(err.Error(), "pending terminal detour") {
		t.Fatalf("return without detour start accepted: %v", err)
	}
	priorReturn := returned
	priorReturn.EventID = "prior-return"
	projection.AppliedEventIDs = []string{detour.EventID, priorReturn.EventID}
	order, _ = BuildCausalOrder([]EventEnvelope{detour, priorReturn, returned})
	if err := projection.apply(returned, order); err == nil || !strings.Contains(err.Error(), "pending terminal detour") {
		t.Fatalf("second detour return accepted: %v", err)
	}
}

func TestAdoptionBoundaryAllowsCompletionFromAdoptedPhase(t *testing.T) {
	events := []EventEnvelope{protocol21Envelope(t, "adopt", "effort_adopted", nil, map[string]any{
		"creationMode": "adopted", "route": "direct", "phase": "implementation", "workflow": "executing-direct",
		"trajectoryId": "trajectory", "anchorId": "anchor", "associationOrigin": "manual",
	})}
	events = appendEvent(events, "implementation-finish", "phase_finished", PhaseFinishedPayload{Phase: "implementation", StartEventID: "adopt"})
	events = appendFinishedPhase(events, "review", "implementation-review", "trajectory")
	events = appendFinishedPhase(events, "retro", "retrospective", "trajectory")
	events = appendEvent(events, "complete", "effort_completed", EffortTerminalPayload{TerminalEpoch: 1})
	projection := ProjectLifecycle(events)
	if projection.State != EffortCompleted || len(projection.Invalid) != 0 {
		t.Fatalf("adopted route completion = state %q invalid %#v", projection.State, projection.Invalid)
	}
}

func protocol21Envelope(t *testing.T, eventID string, kind EventKind, predecessors []string, payload any) EventEnvelope {
	t.Helper()
	if predecessors == nil {
		predecessors = []string{}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return EventEnvelope{
		Version: ProtocolVersion{Major: 2, Minor: 1}, EventID: eventID, IdempotencyKey: "key-" + eventID,
		EffortID: "effort-id", SessionID: "session-id", Timestamp: "2026-07-22T00:00:00Z",
		Kind: kind, Predecessors: predecessors, Payload: raw,
	}
}

func protocol21Metadata(t *testing.T, fields map[string]any) EffortMetadata {
	t.Helper()
	raw, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	var metadata EffortMetadata
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatal(err)
	}
	return metadata
}

func TestTerminalRepairsAndWaiversDoNotReopen(t *testing.T) {
	events := completedRoute("direct")
	routeReplacement, _ := json.Marshal(RoutePayload{Route: "direct"})
	events = appendEvent(events, "terminal-repair", "repair_applied", RepairAppliedPayload{ProposalKind: "supersede-event", SourceEventIDs: []string{"route"}, Replacement: RepairReplacement{EventKind: "route_selected", Payload: routeReplacement}})
	events = appendEvent(events, "terminal-waiver", "finding_waived", FindingWaivedPayload{RuleCode: "WFV1-PHASE-OVERLAP", Scope: "trajectory", EvidenceIDs: []string{"evidence"}, ReasonCode: "approved-phase-overlap"})
	projection := ProjectLifecycle(events)
	if projection.State != EffortCompleted || len(projection.Repairs) != 1 || len(projection.Waivers) != 1 || len(projection.Invalid) != 0 {
		t.Fatalf("terminal correction changed lifecycle state: %#v", projection)
	}
}

func TestLifecycleClosedMutationMatrixAndTerminalEpochs(t *testing.T) {
	abandoned := lifecycleBaseEvents()
	abandoned = appendEvent(abandoned, "abandon", "effort_abandoned", EffortAbandonedPayload{TerminalEpoch: 1, Reason: "provisional-overflow-resume"})
	abandoned = appendEvent(abandoned, "detach-after-abandon", "session_detached", SessionDetachedPayload{Reason: "manual"})
	abandoned = appendEvent(abandoned, "route-after", "route_selected", RoutePayload{Route: "direct"})
	abandoned = appendEvent(abandoned, "reopen", "effort_reopened", EffortReopenedPayload{TerminalEpoch: 2, TrajectoryID: "new", AnchorID: "anchor"})
	projection := ProjectLifecycle(abandoned)
	if projection.State != EffortAbandoned || !projection.EffectApplied["detach-after-abandon"] || len(projection.Associations) != 0 || !hasInvalidEvent(projection.Invalid, "route-after") || !hasInvalidEvent(projection.Invalid, "reopen") {
		t.Fatalf("abandoned mutation matrix = %#v", projection)
	}

	completed := completedRoute("investigation-only")
	completed = appendEvent(completed, "reopen", "effort_reopened", EffortReopenedPayload{TerminalEpoch: 2, TrajectoryID: "epoch-two", AnchorID: "anchor"})
	projection = ProjectLifecycle(completed)
	if projection.State != EffortActive || projection.TerminalEpoch != 2 || projection.ActiveTrajectoryID != "epoch-two" {
		t.Fatalf("reopen = %#v", projection)
	}
	completed = appendEvent(completed, "bad-reopen", "effort_reopened", EffortReopenedPayload{TerminalEpoch: 3, TrajectoryID: "again", AnchorID: "anchor"})
	if projection = ProjectLifecycle(completed); !hasInvalidEvent(projection.Invalid, "bad-reopen") {
		t.Fatal("active effort reopened")
	}
}

func TestLifecyclePhaseOverlapFreshnessAndRouteChanges(t *testing.T) {
	events := lifecycleBaseEvents()
	events = appendEvent(events, "route", "route_selected", RoutePayload{Route: "adr-plan"})
	for index, phase := range routeRequirements["adr-plan"] {
		events = appendFinishedPhase(events, string(rune('a'+index)), phase, "")
	}
	// Reenter authoring after the prior review/resync. Completion must require a
	// fresh review and resync, rather than accepting stale downstream evidence.
	events = appendFinishedPhase(events, "reentry", "adr-authoring", "")
	events = appendEvent(events, "complete", "effort_completed", EffortTerminalPayload{TerminalEpoch: 1})
	if projection := ProjectLifecycle(events); !hasInvalidDetail(projection.Invalid, "complete", "invalidates stale") {
		t.Fatalf("stale review/resync evidence accepted or misclassified: %#v", projection.Invalid)
	}
	for _, stalePhase := range []Phase{"planning", "implementation"} {
		route := Route("plan")
		if stalePhase == "implementation" {
			route = "direct"
		}
		stale := completedRoute(route)
		stale = stale[:len(stale)-1]
		stale = appendFinishedPhase(stale, "stale-"+string(stalePhase), stalePhase, "")
		stale = appendEvent(stale, "stale-complete-"+string(stalePhase), "effort_completed", EffortTerminalPayload{TerminalEpoch: 1})
		if projection := ProjectLifecycle(stale); !hasInvalidEvent(projection.Invalid, "stale-complete-"+string(stalePhase)) {
			t.Fatalf("stale %s downstream evidence accepted", stalePhase)
		}
	}
	for _, review := range []Phase{"adr-review", "plan-review"} {
		staleResync := completedRoute("adr-plan")
		staleResync = staleResync[:len(staleResync)-1]
		staleResync = appendFinishedPhase(staleResync, "late-"+string(review), review, "")
		staleResync = appendEvent(staleResync, "resync-complete-"+string(review), "effort_completed", EffortTerminalPayload{TerminalEpoch: 1})
		if projection := ProjectLifecycle(staleResync); !hasInvalidDetail(projection.Invalid, "resync-complete-"+string(review), "invalidates stale adr-plan-resync") {
			t.Fatalf("stale resync after %s accepted: %#v", review, projection.Invalid)
		}
	}

	overlap := lifecycleBaseEvents()
	overlap = appendEvent(overlap, "one", "phase_started", PhaseStartedPayload{Phase: "brainstorming"})
	overlap = appendEvent(overlap, "two", "phase_started", PhaseStartedPayload{Phase: "implementation"})
	if projection := ProjectLifecycle(overlap); !hasInvalidEvent(projection.Invalid, "two") {
		t.Fatal("phase overlap accepted")
	}

	changed := completedRoute("direct")
	changed = changed[:len(changed)-1]
	changed = appendEvent(changed, "change", "route_changed", RoutePayload{Route: "adr"})
	changed = appendEvent(changed, "complete", "effort_completed", EffortTerminalPayload{TerminalEpoch: 1})
	if projection := ProjectLifecycle(changed); !hasInvalidEvent(projection.Invalid, "complete") {
		t.Fatal("route change did not apply final route requirements")
	}
}

func TestLifecycleConcurrencyDoesNotInventOrder(t *testing.T) {
	events := lifecycleBaseEvents()
	left := causalEvent("left-route", "left", "route_selected", []string{"create"}, RoutePayload{Route: "direct"})
	right := causalEvent("right-route", "right", "route_selected", []string{"create"}, RoutePayload{Route: "plan"})
	events = append(events, left, right)
	projection := ProjectLifecycle(events)
	if projection.Route != "" || !hasIssue(projection.Invalid, "concurrent-state") || !hasInvalidEvent(projection.Invalid, left.EventID) || !hasInvalidEvent(projection.Invalid, right.EventID) {
		t.Fatalf("shared-frontier conflict projected an order: %#v", projection)
	}

	base := lifecycleBaseEvents()
	start := causalEvent("phase-start", "parent", "phase_started", []string{"create"}, PhaseStartedPayload{Phase: "planning"})
	finishLeft := causalEvent("finish-left", "left", "phase_finished", []string{"phase-start"}, PhaseFinishedPayload{Phase: "planning", StartEventID: "phase-start"})
	finishRight := causalEvent("finish-right", "right", "phase_finished", []string{"phase-start"}, PhaseFinishedPayload{Phase: "planning", StartEventID: "phase-start"})
	projection = ProjectLifecycle(append(base, start, finishLeft, finishRight))
	if !hasInvalidEvent(projection.Invalid, "finish-left") || !hasInvalidEvent(projection.Invalid, "finish-right") {
		t.Fatalf("competing finishes projected an order: %#v", projection)
	}

	terminal := causalEvent("terminal", "terminal-session", "effort_abandoned", []string{"create"}, EffortTerminalPayload{TerminalEpoch: 1})
	trajectory := causalEvent("trajectory", "trajectory-session", "trajectory_started", []string{"create"}, TrajectoryPayload{TrajectoryID: "trajectory", AnchorID: "anchor"})
	projection = ProjectLifecycle(append(base, terminal, trajectory))
	if !hasInvalidEvent(projection.Invalid, "terminal") || !hasInvalidEvent(projection.Invalid, "trajectory") {
		t.Fatalf("terminal concurrency projected an order: %#v", projection)
	}

	route := causalEvent("repair-source", "parent", "route_selected", []string{"create"}, RoutePayload{Route: "direct"})
	replacement, _ := json.Marshal(RoutePayload{Route: "plan"})
	repairPayload := RepairAppliedPayload{ProposalKind: "supersede-event", SourceEventIDs: []string{"repair-source"}, Replacement: RepairReplacement{EventKind: "route_selected", Payload: replacement}}
	repairLeft := causalEvent("repair-left", "left", "repair_applied", []string{"repair-source"}, repairPayload)
	repairRight := causalEvent("repair-right", "right", "repair_applied", []string{"repair-source"}, repairPayload)
	projection = ProjectLifecycle(append(base, route, repairLeft, repairRight))
	if !hasInvalidEvent(projection.Invalid, "repair-left") || !hasInvalidEvent(projection.Invalid, "repair-right") {
		t.Fatalf("competing repairs projected an order: %#v", projection)
	}
}

// invariant: tooling/workflow-telemetry:anchor-claims-and-location-metadata
func TestTrajectoryResumeKeepsAssociationAcrossPriorAnchors(t *testing.T) {
	// Normal tree navigation resumes at a tip whose close predates the freshly
	// re-asserted association; anchor causal position is never a detach signal,
	// so only a trajectory-family mismatch detaches.
	events := lifecycleBaseEvents()
	events = appendEvent(events, "trajectory", "trajectory_started", TrajectoryPayload{TrajectoryID: "trajectory", AnchorID: "start"})
	events[len(events)-1].TrajectoryID = "trajectory"
	events = appendEvent(events, "closed", "trajectory_closed", TrajectoryPayload{TrajectoryID: "trajectory", AnchorID: "before-anchor"})
	events[len(events)-1].TrajectoryID = "trajectory"
	events = appendEvent(events, "associate", "session_associated", SessionAssociatedPayload{AssociationOrigin: "manual", TrajectoryID: "trajectory"})
	events = appendEvent(events, "resume-before", "trajectory_resumed", TrajectoryPayload{TrajectoryID: "trajectory", AnchorID: "before-anchor"})
	events[len(events)-1].TrajectoryID = "trajectory"
	projection := ProjectLifecycle(events)
	if len(projection.Invalid) != 0 || len(projection.Associations) != 1 {
		t.Fatalf("resume across a prior anchor claim detached the association: %#v", projection)
	}
}

func TestLifecycleTrajectoriesAssociationsRepairsAndWaivers(t *testing.T) {
	events := lifecycleBaseEvents()
	events = appendEvent(events, "parent", "trajectory_started", TrajectoryPayload{TrajectoryID: "parent", AnchorID: "a"})
	events[len(events)-1].TrajectoryID = "parent"
	events = appendEvent(events, "associate", "session_associated", SessionAssociatedPayload{AssociationOrigin: "manual", TrajectoryID: "parent"})
	events = appendEvent(events, "fork", "trajectory_forked", TrajectoryForkedPayload{TrajectoryID: "child", ParentTrajectoryID: "parent", ForkAnchorID: "fork-a"})
	events[len(events)-1].TrajectoryID = "child"
	events = appendEvent(events, "associate-child", "session_associated", SessionAssociatedPayload{AssociationOrigin: "manual", TrajectoryID: "child"})
	events = appendEvent(events, "resume-parent", "trajectory_resumed", TrajectoryPayload{TrajectoryID: "parent", AnchorID: "before-association"})
	events[len(events)-1].TrajectoryID = "parent"
	events = appendEvent(events, "waive", "finding_waived", FindingWaivedPayload{RuleCode: "WFV1-PHASE-OVERLAP", Scope: "trajectory", EvidenceIDs: []string{"evidence"}, ReasonCode: "approved-phase-overlap"})
	events = appendEvent(events, "bad-waive", "finding_waived", FindingWaivedPayload{RuleCode: "WFV1-EVENT-INTEGRITY", Scope: "stream", EvidenceIDs: []string{"evidence"}, ReasonCode: "approved-phase-overlap"})
	projection := ProjectLifecycle(events)
	if projection.ActiveTrajectoryID != "parent" || len(projection.Associations) != 0 || len(projection.Waivers) != 1 || !hasInvalidEvent(projection.Invalid, "bad-waive") {
		t.Fatalf("trajectory/waiver projection = %#v", projection)
	}

	bad := causalEvent("bad-phase", "session", "phase_started", []string{events[len(events)-1].EventID}, PhaseStartedPayload{Phase: "implementation"})
	events = append(events, bad)
	replacement, _ := json.Marshal(PhaseStartedPayload{Phase: "brainstorming"})
	repair := causalEvent("repair", "session", "repair_applied", []string{"bad-phase"}, RepairAppliedPayload{ProposalKind: "correct-phase", SourceEventIDs: []string{"bad-phase"}, Replacement: RepairReplacement{EventKind: "phase_started", Payload: replacement}})
	events = append(events, repair)
	projection = ProjectLifecycle(events)
	if projection.SupersededEventIDs["bad-phase"] != "repair" || len(projection.Repairs) != 1 || len(projection.OpenPhases) != 1 {
		t.Fatalf("typed repair did not supersede evidence: %#v", projection)
	}
}

func TestLifecycleAppendIsDurableValidatedAndIdempotent(t *testing.T) {
	ledger, err := NewLedger(newTestProject(t))
	if err != nil {
		t.Fatal(err)
	}
	base := LifecycleRequestBase{Action: "create", IdempotencyKey: "create-key", EventID: "create-event", EffortID: "effort-id", SessionID: "session-id", Timestamp: "2026-07-22T00:00:00Z", Predecessors: []string{}}
	create := CreateLifecycleRequest{LifecycleRequestBase: base, CreationMode: "independent"}
	if result, err := ledger.ApplyLifecycle(context.Background(), create); err != nil || result.Idempotent {
		t.Fatalf("create = %#v, %v", result, err)
	}
	if result, err := ledger.ApplyLifecycle(context.Background(), create); err != nil || !result.Idempotent {
		t.Fatalf("create retry = %#v, %v", result, err)
	}
	invalidBase := LifecycleRequestBase{Action: "complete", IdempotencyKey: "complete-key", EventID: "complete-event", EffortID: "effort-id", SessionID: "session-id", Timestamp: "2026-07-22T00:00:01Z", Predecessors: []string{"create-event"}}
	if _, err := ledger.ApplyLifecycle(context.Background(), TerminalLifecycleRequest{LifecycleRequestBase: invalidBase}); err == nil {
		t.Fatal("invalid completion was written")
	}
	read, err := ledger.ReadEffort("effort-id")
	if err != nil || len(read.Records) != 1 {
		t.Fatalf("invalid append changed durable stream: records=%d err=%v", len(read.Records), err)
	}
}

func TestProtocol21ContinuationRequiresWholeCurrentFrontier(t *testing.T) {
	ledger, err := NewLedger(newTestProject(t))
	if err != nil {
		t.Fatal(err)
	}
	base := LifecycleRequestBase{EffortID: "adopted-effort", SessionID: "session-id", Timestamp: "2026-07-22T00:00:00Z"}
	adopt := AdoptLifecycleRequest{LifecycleRequestBase: withAction(base, "adopt"), Route: "direct", Phase: "implementation", Workflow: "executing-direct", TrajectoryID: "trajectory", AnchorID: "anchor"}
	adopt.IdempotencyKey, adopt.EventID, adopt.Predecessors = "adopt-key", "adopt", []string{}
	if _, err := ledger.ApplyLifecycle(context.Background(), adopt); err != nil {
		t.Fatal(err)
	}
	observation := passiveEvent(t, "observation", "observation", base.EffortID, []string{"adopt"})
	if _, err := ledger.Append(context.Background(), observation); err != nil {
		t.Fatal(err)
	}
	continued := ContinuePhaseLifecycleRequest{LifecycleRequestBase: withAction(base, "continue-phase"), Phase: "implementation", StartEventID: "adopt", Workflow: "tdd", Activity: "tdd"}
	continued.IdempotencyKey, continued.EventID, continued.Predecessors = "continue-key", "continue", []string{"adopt"}
	if _, err := ledger.ApplyLifecycle(context.Background(), continued); err == nil || !strings.Contains(err.Error(), "frontier") {
		t.Fatalf("partial-frontier continuation error = %v", err)
	}
	continued.Predecessors = []string{"observation"}
	if _, err := ledger.ApplyLifecycle(context.Background(), continued); err != nil {
		t.Fatal(err)
	}
	projection := ProjectLifecycle(ledgerReadEvents(t, ledger, base.EffortID))
	interval := projection.OpenPhases["adopt"]
	if interval.StartEventID != "adopt" || interval.Workflow != "tdd" || interval.Activity != "tdd" {
		t.Fatalf("continuation did not preserve and replace phase attribution: %#v", interval)
	}
}

// invariant: tooling/workflow-telemetry:trajectory-and-derived-effort-model
func TestStartingDerivedDetourPreservesActiveParentProjection(t *testing.T) {
	ledger, err := NewLedger(newTestProject(t))
	if err != nil {
		t.Fatal(err)
	}
	at := "2026-07-22T00:00:00Z"
	parentBase := LifecycleRequestBase{EffortID: "active-parent", SessionID: "parent-session", Timestamp: at}
	create := CreateLifecycleRequest{LifecycleRequestBase: withAction(parentBase, "create"), CreationMode: "independent"}
	create.IdempotencyKey, create.EventID, create.Predecessors = "parent-create-key", "parent-create", []string{}
	if _, err := ledger.ApplyLifecycle(context.Background(), create); err != nil {
		t.Fatal(err)
	}
	route := RouteLifecycleRequest{LifecycleRequestBase: withAction(parentBase, "select-route"), Route: "direct"}
	route.IdempotencyKey, route.EventID, route.Predecessors = "parent-route-key", "parent-route", []string{"parent-create"}
	if _, err := ledger.ApplyLifecycle(context.Background(), route); err != nil {
		t.Fatal(err)
	}
	trajectory := TrajectoryLifecycleRequest{LifecycleRequestBase: withAction(parentBase, "start-trajectory"), TrajectoryID: "parent-trajectory", AnchorID: "parent-anchor"}
	trajectory.IdempotencyKey, trajectory.EventID, trajectory.Predecessors = "parent-trajectory-key", "parent-trajectory-start", []string{"parent-route"}
	if _, err := ledger.ApplyLifecycle(context.Background(), trajectory); err != nil {
		t.Fatal(err)
	}
	association := AssociateLifecycleRequest{LifecycleRequestBase: withAction(parentBase, "associate"), TrajectoryID: "parent-trajectory", AssociationOrigin: "manual"}
	association.IdempotencyKey, association.EventID, association.Predecessors = "parent-association-key", "parent-association", []string{"parent-trajectory-start"}
	if _, err := ledger.ApplyLifecycle(context.Background(), association); err != nil {
		t.Fatal(err)
	}
	phase := StartPhaseLifecycleRequest{LifecycleRequestBase: withAction(parentBase, "start-phase"), Phase: "implementation", Activity: "tdd", ImplementationMode: "inline-execution"}
	phase.IdempotencyKey, phase.EventID, phase.Predecessors = "parent-phase-key", "parent-phase-start", []string{"parent-association"}
	if _, err := ledger.ApplyLifecycle(context.Background(), phase); err != nil {
		t.Fatal(err)
	}
	parentEventsBefore := ledgerReadEvents(t, ledger, parentBase.EffortID)
	before := ProjectLifecycle(parentEventsBefore)
	frontierBefore := currentCausalFrontier(parentEventsBefore)

	childBase := LifecycleRequestBase{EffortID: "derived-child", SessionID: parentBase.SessionID, Timestamp: at}
	origin := OriginMetadata{EffortID: parentBase.EffortID, TrajectoryID: "parent-trajectory", AnchorID: "parent-anchor"}
	detour := StartDetourLifecycleRequest{LifecycleRequestBase: withAction(childBase, "start-detour"), CreationMode: "derived", Origin: origin, ReturnPhase: "implementation", ReturnPhaseStartEventID: "parent-phase-start", TrajectoryID: "child-trajectory", AnchorID: "child-anchor", Workflow: "brainstorming"}
	detour.IdempotencyKey, detour.EventID, detour.Predecessors = "child-detour-key", "child-detour-start", []string{}
	if _, err := ledger.ApplyLifecycle(context.Background(), detour); err != nil {
		t.Fatal(err)
	}
	parentEventsAfter := ledgerReadEvents(t, ledger, parentBase.EffortID)
	after := ProjectLifecycle(parentEventsAfter)
	frontierAfter := currentCausalFrontier(parentEventsAfter)
	beforePhase, beforeOpen := before.OpenPhases["parent-phase-start"]
	afterPhase, afterOpen := after.OpenPhases["parent-phase-start"]
	if before.State != EffortActive || after.State != before.State || !beforeOpen || !afterOpen || beforePhase.Phase != "implementation" || beforePhase.StartEventID != "parent-phase-start" || afterPhase.Phase != beforePhase.Phase || afterPhase.StartEventID != beforePhase.StartEventID {
		t.Fatalf("derived detour changed parent open phase or start: before=%#v after=%#v", before, after)
	}
	if before.ActiveTrajectoryID != "parent-trajectory" || strings.Join(frontierBefore, ",") != "parent-phase-start" || after.ActiveTrajectoryID != before.ActiveTrajectoryID || strings.Join(frontierAfter, ",") != strings.Join(frontierBefore, ",") {
		t.Fatalf("derived detour changed parent trajectory or frontier: before=%#v after=%#v", before, after)
	}
	if before.CurrentWorkflow != "" || before.CurrentActivity != "tdd" || before.CurrentImplementationMode != "inline-execution" || after.CurrentWorkflow != before.CurrentWorkflow || after.CurrentActivity != before.CurrentActivity || after.CurrentImplementationMode != before.CurrentImplementationMode {
		t.Fatalf("derived detour changed parent attribution: before=%#v after=%#v", before, after)
	}
	childRead, err := ledger.ReadEffort(childBase.EffortID)
	if err != nil {
		t.Fatal(err)
	}
	child := ProjectWorkflow(childRead)
	if child.Origin == nil || *child.Origin != origin || child.Metadata.DetourReturn == nil || child.Metadata.DetourReturn.SessionID != childBase.SessionID || child.Metadata.DetourReturn.Phase != "implementation" || child.Metadata.DetourReturn.PhaseStartEventID != "parent-phase-start" {
		t.Fatalf("derived child lost lineage or return metadata: %#v", child)
	}
	if child.Lifecycle.ActiveTrajectoryID != "child-trajectory" || child.Lifecycle.State != EffortDiscovery || child.Lifecycle.CurrentWorkflow != "brainstorming" {
		t.Fatalf("derived child detour projection = %#v", child.Lifecycle)
	}
}

func createActiveDetourParent(t *testing.T, ledger *Ledger, effortID, sessionID, phaseStartID string) OriginMetadata {
	t.Helper()
	base := LifecycleRequestBase{EffortID: effortID, SessionID: sessionID, Timestamp: "2026-07-22T00:00:00Z"}
	requests := []LifecycleRequest{
		CreateLifecycleRequest{LifecycleRequestBase: LifecycleRequestBase{Action: "create", IdempotencyKey: "parent-create-key", EventID: "parent-create", EffortID: effortID, SessionID: sessionID, Timestamp: base.Timestamp, Predecessors: []string{}}, CreationMode: "independent"},
		RouteLifecycleRequest{LifecycleRequestBase: LifecycleRequestBase{Action: "select-route", IdempotencyKey: "parent-route-key", EventID: "parent-route", EffortID: effortID, SessionID: sessionID, Timestamp: base.Timestamp, Predecessors: []string{"parent-create"}}, Route: "direct"},
		TrajectoryLifecycleRequest{LifecycleRequestBase: LifecycleRequestBase{Action: "start-trajectory", IdempotencyKey: "parent-trajectory-key", EventID: "parent-trajectory-start", EffortID: effortID, SessionID: sessionID, Timestamp: base.Timestamp, Predecessors: []string{"parent-route"}}, TrajectoryID: "parent-trajectory", AnchorID: "parent-anchor"},
		AssociateLifecycleRequest{LifecycleRequestBase: LifecycleRequestBase{Action: "associate", IdempotencyKey: "parent-association-key", EventID: "parent-association", EffortID: effortID, SessionID: sessionID, Timestamp: base.Timestamp, Predecessors: []string{"parent-trajectory-start"}}, TrajectoryID: "parent-trajectory", AssociationOrigin: "detour"},
		StartPhaseLifecycleRequest{LifecycleRequestBase: LifecycleRequestBase{Action: "start-phase", IdempotencyKey: "parent-phase-key", EventID: phaseStartID, EffortID: effortID, SessionID: sessionID, Timestamp: base.Timestamp, Predecessors: []string{"parent-association"}}, Phase: "implementation"},
	}
	for _, request := range requests {
		if _, err := ledger.ApplyLifecycle(context.Background(), request); err != nil {
			t.Fatalf("create active detour parent: %v", err)
		}
	}
	return OriginMetadata{EffortID: effortID, TrajectoryID: "parent-trajectory", AnchorID: "parent-anchor"}
}

func TestStartDetourRequestAndActiveTrajectoryAnchorVariants(t *testing.T) {
	request := &StartDetourLifecycleRequest{}
	if got, ok := asStartDetourRequest(request); !ok || got.Action != request.Action {
		t.Fatalf("pointer start-detour request = %#v, %v", got, ok)
	}
	var nilRequest *StartDetourLifecycleRequest
	if _, ok := asStartDetourRequest(nilRequest); ok {
		t.Fatal("nil start-detour request was accepted")
	}
	ledger, err := NewLedger(newTestProject(t))
	if err != nil {
		t.Fatal(err)
	}
	invalid := StartDetourLifecycleRequest{LifecycleRequestBase: LifecycleRequestBase{Action: "start-detour"}, CreationMode: "derived", Workflow: "brainstorming"}
	if _, err := ledger.ApplyLifecycle(context.Background(), invalid); err == nil {
		t.Fatal("invalid start-detour request was accepted")
	}
	for _, test := range []struct {
		name    string
		kind    EventKind
		payload any
		anchor  string
	}{
		{"adopted", "effort_adopted", EffortAdoptedPayload{TrajectoryID: "trajectory", AnchorID: "anchor"}, "anchor"},
		{"detour", "detour_started", DetourStartedPayload{TrajectoryID: "trajectory", AnchorID: "anchor"}, "anchor"},
		{"trajectory", "trajectory_resumed", TrajectoryPayload{TrajectoryID: "trajectory", AnchorID: "anchor"}, "anchor"},
		{"fork", "trajectory_forked", TrajectoryForkedPayload{TrajectoryID: "trajectory", ForkAnchorID: "anchor"}, "anchor"},
		{"reopened", "effort_reopened", EffortReopenedPayload{TrajectoryID: "trajectory", AnchorID: "anchor"}, "anchor"},
	} {
		t.Run(test.name, func(t *testing.T) {
			payload, err := json.Marshal(test.payload)
			if err != nil {
				t.Fatal(err)
			}
			event := EventEnvelope{EventID: "event", Kind: test.kind, Payload: payload}
			anchor, ok := activeTrajectoryAnchor([]EventEnvelope{event}, LifecycleProjection{AppliedEventIDs: []string{"event"}}, "trajectory")
			if !ok || anchor != test.anchor {
				t.Fatalf("active trajectory anchor = %q, %v", anchor, ok)
			}
		})
	}
	if _, ok := activeTrajectoryAnchor([]EventEnvelope{{EventID: "excluded"}}, LifecycleProjection{}, "trajectory"); ok {
		t.Fatal("excluded trajectory event supplied an anchor")
	}
	if err := ledger.validateDetourReturnAssociation(EffortRead{}, EventEnvelope{}); err == nil {
		t.Fatal("non-derived detour return was accepted")
	}
	child := EffortRead{Metadata: EffortMetadata{CreationMode: "derived", Origin: &OriginMetadata{EffortID: "parent"}, DetourReturn: &DetourReturnMetadata{}}}
	if err := ledger.validateDetourReturnAssociation(child, EventEnvelope{Payload: json.RawMessage("invalid")}); err == nil {
		t.Fatal("malformed detour return payload was accepted")
	}
}

func TestApplyLifecycleRejectsForgedDetourParentBoundary(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*StartDetourLifecycleRequest)
	}{
		{"origin effort", func(request *StartDetourLifecycleRequest) { request.Origin.EffortID = "forged-parent" }},
		{"origin trajectory", func(request *StartDetourLifecycleRequest) { request.Origin.TrajectoryID = "forged-trajectory" }},
		{"origin anchor", func(request *StartDetourLifecycleRequest) { request.Origin.AnchorID = "forged-anchor" }},
		{"return phase start", func(request *StartDetourLifecycleRequest) { request.ReturnPhaseStartEventID = "forged-phase-start" }},
		{"parent association", func(request *StartDetourLifecycleRequest) { request.SessionID = "forged-session" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			ledger, err := NewLedger(newTestProject(t))
			if err != nil {
				t.Fatal(err)
			}
			origin := createActiveDetourParent(t, ledger, "parent", "session", "parent-phase-start")
			request := StartDetourLifecycleRequest{LifecycleRequestBase: LifecycleRequestBase{Action: "start-detour", IdempotencyKey: "detour-key", EventID: "detour-start", EffortID: "child", SessionID: "session", Timestamp: "2026-07-22T00:00:00Z", Predecessors: []string{}}, CreationMode: "derived", Origin: origin, ReturnPhase: "implementation", ReturnPhaseStartEventID: "parent-phase-start", TrajectoryID: "child-trajectory", AnchorID: "child-anchor", Workflow: "brainstorming"}
			test.mutate(&request)
			if _, err := ledger.ApplyLifecycle(context.Background(), request); err == nil {
				t.Fatal("forged detour parent boundary was accepted")
			}
			if _, err := ledger.ReadEffort(request.EffortID); err == nil {
				t.Fatal("forged detour child was durably created")
			}
		})
	}
}

func TestApplyStartDetourReportsParentLeaseHeartbeatAndReleaseFailures(t *testing.T) {
	newRequest := func(t *testing.T) (*Ledger, StartDetourLifecycleRequest) {
		t.Helper()
		ledger, err := NewLedger(newTestProject(t))
		if err != nil {
			t.Fatal(err)
		}
		origin := createActiveDetourParent(t, ledger, "parent", "session", "parent-phase-start")
		return ledger, StartDetourLifecycleRequest{LifecycleRequestBase: LifecycleRequestBase{Action: "start-detour", IdempotencyKey: "detour-key", EventID: "detour-start", EffortID: "child", SessionID: "session", Timestamp: "2026-07-22T00:00:00Z", Predecessors: []string{}}, CreationMode: "derived", Origin: origin, ReturnPhase: "implementation", ReturnPhaseStartEventID: "parent-phase-start", TrajectoryID: "child-trajectory", AnchorID: "child-anchor", Workflow: "brainstorming"}
	}
	ledger, request := newRequest(t)
	ledger.ops.sleep = func(context.Context, time.Duration) error { return errInjected }
	if _, err := ledger.ApplyLifecycle(context.Background(), request); err == nil {
		t.Fatal("detour parent heartbeat failure was accepted")
	}

	ledger, request = newRequest(t)
	originalRemove := ledger.ops.remove
	ledger.ops.remove = func(path string) error {
		if path == ledger.paths.appendLease(request.Origin.EffortID) {
			return errInjected
		}
		return originalRemove(path)
	}
	if _, err := ledger.ApplyLifecycle(context.Background(), request); err == nil {
		t.Fatal("detour parent lease release failure was accepted")
	}
}

func TestDetourReturnIdentityAndTerminalBoundary(t *testing.T) {
	if got, want := detourReturnIdentity("child", 1), "7ec845cf3ebadce6a7bd3acae661b7d2f5d092684016e4fc90cf9423b582e4fe"; got != want {
		t.Fatalf("detour return identity = %q, want %q", got, want)
	}
	completed := EventEnvelope{EventID: "completed", Kind: "effort_completed", Payload: mustJSON(t, EffortTerminalPayload{TerminalEpoch: 2})}
	terminal, ok := detourTerminalEvent(EffortRead{Records: []LedgerRecord{{Event: &completed, Applied: true}}}, LifecycleProjection{State: EffortCompleted, TerminalEpoch: 2})
	if !ok || terminal.EventID != completed.EventID {
		t.Fatalf("completed terminal boundary = %#v, %v", terminal, ok)
	}
	if _, ok := detourTerminalEvent(EffortRead{}, LifecycleProjection{State: EffortAbandoned, TerminalEpoch: 1}); ok {
		t.Fatal("missing terminal boundary was accepted")
	}
}

func TestApplyLifecycleRejectsOtherChildDetourReturnAssociation(t *testing.T) {
	ledger, err := NewLedger(newTestProject(t))
	if err != nil {
		t.Fatal(err)
	}
	origin := createActiveDetourParent(t, ledger, "parent", "session", "parent-phase-start")
	start := StartDetourLifecycleRequest{LifecycleRequestBase: LifecycleRequestBase{Action: "start-detour", IdempotencyKey: "detour-key", EventID: "detour-start", EffortID: "child", SessionID: "session", Timestamp: "2026-07-22T00:00:00Z", Predecessors: []string{}}, CreationMode: "derived", Origin: origin, ReturnPhase: "implementation", ReturnPhaseStartEventID: "parent-phase-start", TrajectoryID: "child-trajectory", AnchorID: "child-anchor", Workflow: "brainstorming"}
	if _, err := ledger.ApplyLifecycle(context.Background(), start); err != nil {
		t.Fatal(err)
	}
	abandon := AbandonLifecycleRequest{LifecycleRequestBase: LifecycleRequestBase{Action: "abandon", IdempotencyKey: "abandon-key", EventID: "terminal", EffortID: "child", SessionID: "session", Timestamp: "2026-07-22T00:00:00Z", Predecessors: []string{"detour-start"}}, Reason: "blocked"}
	if _, err := ledger.ApplyLifecycle(context.Background(), abandon); err != nil {
		t.Fatal(err)
	}
	associate := func(childID, predecessor string) AssociateLifecycleRequest {
		identity := detourReturnIdentity(childID, 1)
		return AssociateLifecycleRequest{LifecycleRequestBase: LifecycleRequestBase{Action: "associate", IdempotencyKey: "detour-return-" + identity, EventID: "detour-return-event-" + identity + "-parent", EffortID: "parent", SessionID: "session", Timestamp: abandon.Timestamp, Predecessors: []string{predecessor}}, TrajectoryID: origin.TrajectoryID, AssociationOrigin: "detour"}
	}
	otherChild := associate("other-child", "parent-phase-start")
	if _, err := ledger.ApplyLifecycle(context.Background(), otherChild); err != nil {
		t.Fatal(err)
	}
	returned := MarkDetourReturnedLifecycleRequest{LifecycleRequestBase: LifecycleRequestBase{Action: "mark-detour-returned", IdempotencyKey: "return-key", EventID: "return", EffortID: "child", SessionID: "session", Timestamp: abandon.Timestamp, Predecessors: []string{"terminal"}}, TerminalOutcome: "abandoned", ParentAssociationEventID: otherChild.EventID}
	if _, err := ledger.ApplyLifecycle(context.Background(), returned); err == nil {
		t.Fatal("another child's deterministic parent association was accepted")
	}
	matchingChild := associate(start.EffortID, otherChild.EventID)
	if _, err := ledger.ApplyLifecycle(context.Background(), matchingChild); err != nil {
		t.Fatal(err)
	}
	returned.ParentAssociationEventID = matchingChild.EventID
	if _, err := ledger.ApplyLifecycle(context.Background(), returned); err != nil {
		t.Fatalf("matching deterministic parent association was rejected: %v", err)
	}
}

func TestProtocol21OnlyReturnIsLegalAfterDetourTerminal(t *testing.T) {
	ledger, err := NewLedger(newTestProject(t))
	if err != nil {
		t.Fatal(err)
	}
	base := LifecycleRequestBase{EffortID: "detour-effort", SessionID: "session-id", Timestamp: "2026-07-22T00:00:00Z"}
	origin := createActiveDetourParent(t, ledger, "parent", base.SessionID, "parent-start")
	start := StartDetourLifecycleRequest{LifecycleRequestBase: withAction(base, "start-detour"), CreationMode: "derived", Origin: origin, ReturnPhase: "implementation", ReturnPhaseStartEventID: "parent-start", TrajectoryID: "child-trajectory", AnchorID: "child-anchor", Workflow: "brainstorming"}
	start.IdempotencyKey, start.EventID, start.Predecessors = "start-key", "detour-start", []string{}
	if _, err := ledger.ApplyLifecycle(context.Background(), start); err != nil {
		t.Fatal(err)
	}
	abandon := AbandonLifecycleRequest{LifecycleRequestBase: withAction(base, "abandon"), Reason: "blocked"}
	abandon.IdempotencyKey, abandon.EventID, abandon.Predecessors = "abandon-key", "terminal", []string{"detour-start"}
	if _, err := ledger.ApplyLifecycle(context.Background(), abandon); err != nil {
		t.Fatal(err)
	}
	continued := ContinuePhaseLifecycleRequest{LifecycleRequestBase: withAction(base, "continue-phase"), Phase: "brainstorming", StartEventID: "detour-start", Workflow: "brainstorming"}
	continued.IdempotencyKey, continued.EventID, continued.Predecessors = "continue-key", "illegal-continue", []string{"terminal"}
	if _, err := ledger.ApplyLifecycle(context.Background(), continued); err == nil {
		t.Fatal("phase continuation was legal after terminal detour state")
	}
	observation := passiveEvent(t, "terminal-observation", "terminal-observation", base.EffortID, []string{"terminal"})
	if _, err := ledger.Append(context.Background(), observation); err != nil {
		t.Fatal(err)
	}
	identity := detourReturnIdentity(base.EffortID, 1)
	association := AssociateLifecycleRequest{LifecycleRequestBase: LifecycleRequestBase{Action: "associate", IdempotencyKey: "detour-return-" + identity, EventID: "detour-return-event-" + identity + "-parent", EffortID: origin.EffortID, SessionID: base.SessionID, Timestamp: base.Timestamp, Predecessors: []string{"parent-start"}}, TrajectoryID: origin.TrajectoryID, AssociationOrigin: "detour"}
	if _, err := ledger.ApplyLifecycle(context.Background(), association); err != nil {
		t.Fatal(err)
	}
	returned := MarkDetourReturnedLifecycleRequest{LifecycleRequestBase: withAction(base, "mark-detour-returned"), TerminalOutcome: "abandoned", ParentAssociationEventID: association.EventID}
	returned.IdempotencyKey, returned.EventID, returned.Predecessors = "return-key", "return", []string{"terminal"}
	if _, err := ledger.ApplyLifecycle(context.Background(), returned); err == nil || !strings.Contains(err.Error(), "frontier") {
		t.Fatalf("partial-frontier return error = %v", err)
	}
	returned.Predecessors, returned.TerminalOutcome = []string{"terminal-observation"}, "completed"
	if _, err := ledger.ApplyLifecycle(context.Background(), returned); err == nil || !strings.Contains(err.Error(), "outcome") {
		t.Fatalf("mismatched return outcome error = %v", err)
	}
	returned.TerminalOutcome = "abandoned"
	if _, err := ledger.ApplyLifecycle(context.Background(), returned); err != nil {
		t.Fatal(err)
	}
}

func ledgerReadEvents(t *testing.T, ledger *Ledger, effortID string) []EventEnvelope {
	t.Helper()
	read, err := ledger.ReadEffort(effortID)
	if err != nil {
		t.Fatal(err)
	}
	return read.Events
}

func TestReaderRetainsExternalIllegalLifecycleEvidence(t *testing.T) {
	ledger, metadata, _ := createTestEffort(t)
	invalid := lifecycleRaw(t, "illegal", "illegal-key", metadata.EffortID, "effort_completed", []string{"event-id"}, EffortTerminalPayload{TerminalEpoch: 1})
	stream := ledger.paths.stream(metadata.EffortID, "session-id")
	file, err := openAppend(stream)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(append(invalid, '\n')); err != nil || file.Close() != nil {
		t.Fatal(err)
	}
	read, err := ledger.ReadEffort(metadata.EffortID)
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Records) != 2 || read.Records[1].Event == nil || read.Records[1].Applied || !hasIssue(read.Integrity, "invalid-transition") {
		t.Fatalf("illegal external evidence was not retained unapplied: %#v", read)
	}
	if _, err := ledger.Append(context.Background(), invalid); err == nil {
		t.Fatal("identical illegal external evidence was treated as an applied retry")
	}
	request := TerminalLifecycleRequest{LifecycleRequestBase: LifecycleRequestBase{Action: "complete", IdempotencyKey: "illegal-key", EventID: "illegal", EffortID: metadata.EffortID, SessionID: "session-id", Timestamp: "2026-07-22T00:00:01Z", Predecessors: []string{"event-id"}}}
	if _, err := ledger.ApplyLifecycle(context.Background(), request); err == nil {
		t.Fatal("illegal external lifecycle evidence was treated as an applied retry")
	}
}

func lifecycleRaw(t *testing.T, eventID, key, effortID string, kind EventKind, predecessors []string, payload any) []byte {
	t.Helper()
	rawPayload, _ := json.Marshal(payload)
	raw, err := json.Marshal(EventEnvelope{Version: ProtocolVersion{Major: 2}, EventID: eventID, IdempotencyKey: key, EffortID: effortID, SessionID: "session-id", Timestamp: "2026-07-22T00:00:01Z", Kind: kind, Predecessors: predecessors, Payload: rawPayload})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func openAppend(path string) (interface {
	Write([]byte) (int, error)
	Close() error
}, error) {
	return defaultLedgerOps().openFile(path, 1|1024, 0o600)
}

func hasIssue(issues []IntegrityIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func hasInvalidEvent(issues []IntegrityIssue, eventID string) bool {
	return hasInvalidDetail(issues, eventID, "")
}

func hasInvalidDetail(issues []IntegrityIssue, eventID, detail string) bool {
	for _, issue := range issues {
		if containsString(issue.EventIDs, eventID) && strings.Contains(issue.Detail, detail) {
			return true
		}
	}
	return false
}

func TestLifecycleRejectsUnknownRequestTypeAndMismatchedAction(t *testing.T) {
	base := LifecycleRequestBase{Action: "change-route"}
	_, _, _, _, _, err := lifecycleRequestParts(RouteLifecycleRequest{LifecycleRequestBase: base, Route: "direct"})
	if err != nil {
		t.Fatal(err)
	}
	base.Action = "nonsense"
	_, _, _, _, _, err = lifecycleRequestParts(RouteLifecycleRequest{LifecycleRequestBase: base, Route: "direct"})
	if err == nil || !strings.Contains(err.Error(), "invalid lifecycle action") {
		t.Fatalf("mismatched action error = %v", err)
	}
}
