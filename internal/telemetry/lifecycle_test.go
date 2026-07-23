package telemetry

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
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

func TestTrajectoryResumeBeforeAssociationDetaches(t *testing.T) {
	events := lifecycleBaseEvents()
	events = appendEvent(events, "trajectory", "trajectory_started", TrajectoryPayload{TrajectoryID: "trajectory", AnchorID: "before-anchor"})
	events[len(events)-1].TrajectoryID = "trajectory"
	events[len(events)-1].PiAnchorID = "before-anchor"
	events = appendEvent(events, "associate", "session_associated", SessionAssociatedPayload{AssociationOrigin: "manual", TrajectoryID: "trajectory"})
	events[len(events)-1].PiAnchorID = "association-anchor"
	events = appendEvent(events, "resume-before", "trajectory_resumed", TrajectoryPayload{TrajectoryID: "trajectory", AnchorID: "before-anchor"})
	events[len(events)-1].TrajectoryID = "trajectory"
	projection := ProjectLifecycle(events)
	if len(projection.Invalid) != 0 || len(projection.Associations) != 0 {
		t.Fatalf("pre-association anchor did not detach: %#v", projection)
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
