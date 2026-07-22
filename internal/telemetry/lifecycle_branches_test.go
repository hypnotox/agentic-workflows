package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"testing"
)

type unknownLifecycleRequest struct{ LifecycleRequestBase }

func (r unknownLifecycleRequest) lifecycleAction() string { return r.Action }

func TestLifecycleRequestRejectsNilPointer(t *testing.T) {
	var request *CreateLifecycleRequest
	if _, _, _, _, _, err := lifecycleRequestParts(request); err == nil {
		t.Fatal("expected nil lifecycle request error")
	}
}

func TestLifecycleRequestUnionBuildsEveryMutation(t *testing.T) {
	base := LifecycleRequestBase{IdempotencyKey: "key", EventID: "event", EffortID: "effort", SessionID: "session", Timestamp: "2026-07-22T00:00:00Z", Predecessors: []string{}}
	payload, _ := json.Marshal(RoutePayload{Route: "direct"})
	origin := &OriginMetadata{EffortID: "origin", TrajectoryID: "origin-trajectory", AnchorID: "origin-anchor"}
	requests := []LifecycleRequest{
		CreateLifecycleRequest{LifecycleRequestBase: withAction(base, "create"), CheckpointID: "checkpoint.md", CreationMode: "derived", Origin: origin},
		AssociateLifecycleRequest{LifecycleRequestBase: withAction(base, "associate"), TrajectoryID: "trajectory", AssociationOrigin: "handoff", HandoffEventID: "handoff"},
		DetachLifecycleRequest{LifecycleRequestBase: withAction(base, "detach"), Reason: "manual"},
		RouteLifecycleRequest{LifecycleRequestBase: withAction(base, "select-route"), Route: "direct"},
		RouteLifecycleRequest{LifecycleRequestBase: withAction(base, "change-route"), Route: "plan"},
		StartPhaseLifecycleRequest{LifecycleRequestBase: withAction(base, "start-phase"), Phase: "implementation", Activity: "tdd", ImplementationMode: "inline"},
		FinishPhaseLifecycleRequest{LifecycleRequestBase: withAction(base, "finish-phase"), Phase: "implementation", StartEventID: "start", Outcome: "success"},
		TrajectoryLifecycleRequest{LifecycleRequestBase: withAction(base, "start-trajectory"), TrajectoryID: "trajectory", AnchorID: "anchor", Reason: "manual"},
		TrajectoryLifecycleRequest{LifecycleRequestBase: withAction(base, "resume-trajectory"), TrajectoryID: "trajectory", AnchorID: "anchor"},
		TrajectoryLifecycleRequest{LifecycleRequestBase: withAction(base, "close-trajectory"), TrajectoryID: "trajectory", AnchorID: "anchor"},
		ForkTrajectoryLifecycleRequest{LifecycleRequestBase: withAction(base, "fork-trajectory"), TrajectoryID: "child", ParentTrajectoryID: "parent", ForkAnchorID: "anchor"},
		TerminalLifecycleRequest{LifecycleRequestBase: withAction(base, "complete")},
		TerminalLifecycleRequest{LifecycleRequestBase: withAction(base, "abandon")},
		ReopenLifecycleRequest{LifecycleRequestBase: withAction(base, "reopen"), TrajectoryID: "new", AnchorID: "anchor"},
		WaiveLifecycleRequest{LifecycleRequestBase: withAction(base, "waive"), RuleCode: "WFV1-PHASE-ORDER", Scope: "epoch", EvidenceIDs: []string{"source"}, ReasonCode: "approved-route-deviation"},
		RepairLifecycleRequest{LifecycleRequestBase: withAction(base, "repair"), Proposal: RepairProposal{Kind: "supersede-event", SourceEventIDs: []string{"source"}, Replacement: RepairReplacement{EventKind: "route_selected", Payload: payload}}},
	}
	for index, request := range requests {
		base.EventID = "event-" + string(rune('a'+index))
		raw, _, creating, err := lifecycleRequestEvent(request)
		if err != nil || len(raw) == 0 {
			t.Errorf("request %T = %s, creating=%v, err=%v", request, raw, creating, err)
		}
	}
	if _, _, _, _, _, err := lifecycleRequestParts(unknownLifecycleRequest{}); err == nil {
		t.Fatal("unknown lifecycle request type accepted")
	}
}

func withAction(base LifecycleRequestBase, action string) LifecycleRequestBase {
	base.Action = action
	return base
}

func TestLifecycleLegalMutationRows(t *testing.T) {
	events := lifecycleBaseEvents()
	events = appendEvent(events, "trajectory", "trajectory_started", TrajectoryPayload{TrajectoryID: "parent", AnchorID: "anchor"})
	events[len(events)-1].TrajectoryID = "parent"
	events = appendEvent(events, "associate", "session_associated", SessionAssociatedPayload{AssociationOrigin: "manual", TrajectoryID: "parent"})
	events = appendEvent(events, "detach", "session_detached", SessionDetachedPayload{Reason: "manual"})
	events = appendEvent(events, "route", "route_selected", RoutePayload{Route: "direct"})
	events = appendEvent(events, "change", "route_changed", RoutePayload{Route: "bugfix"})
	events = appendEvent(events, "fork", "trajectory_forked", TrajectoryForkedPayload{TrajectoryID: "child", ParentTrajectoryID: "parent", ForkAnchorID: "fork"})
	events = appendEvent(events, "resume", "trajectory_resumed", TrajectoryPayload{TrajectoryID: "parent", AnchorID: "anchor"})
	events = appendEvent(events, "close", "trajectory_closed", TrajectoryPayload{TrajectoryID: "parent", AnchorID: "anchor"})
	projection := ProjectLifecycle(events)
	if len(projection.Invalid) != 0 || projection.Route != "bugfix" || projection.ActiveTrajectoryID != "" || len(projection.Associations) != 0 {
		t.Fatalf("legal rows = %#v", projection)
	}
}

func TestLifecycleRequiresCreation(t *testing.T) {
	route := causalEvent("route", "session", "route_selected", nil, RoutePayload{Route: "direct"})
	projection := ProjectLifecycle([]EventEnvelope{route})
	if projection.State != "" || !hasInvalidEvent(projection.Invalid, "route") {
		t.Fatalf("mutation without creation applied: %#v", projection)
	}
}

func TestLifecycleAdditionalInvalidBranches(t *testing.T) {
	active := lifecycleBaseEvents()
	active = appendEvent(active, "route", "route_selected", RoutePayload{Route: "direct"})
	active = appendEvent(active, "second-route", "route_selected", RoutePayload{Route: "plan"})
	if projection := ProjectLifecycle(active); !hasInvalidEvent(projection.Invalid, "second-route") {
		t.Fatal("second route selection accepted")
	}

	implementation := lifecycleBaseEvents()
	implementation = appendFinishedPhase(implementation, "implementation", "implementation", "")
	implementation = appendEvent(implementation, "route", "route_selected", RoutePayload{Route: "direct"})
	implementation = appendEvent(implementation, "bad-change", "route_changed", RoutePayload{Route: "investigation-only"})
	if projection := ProjectLifecycle(implementation); !hasInvalidEvent(projection.Invalid, "bad-change") {
		t.Fatal("investigation-only route change accepted after implementation")
	}
	openImplementation := lifecycleBaseEvents()
	openImplementation = appendEvent(openImplementation, "implementation", "phase_started", PhaseStartedPayload{Phase: "implementation"})
	openImplementation = appendEvent(openImplementation, "bad-route", "route_selected", RoutePayload{Route: "investigation-only"})
	if projection := ProjectLifecycle(openImplementation); !hasInvalidEvent(projection.Invalid, "bad-route") {
		t.Fatal("investigation-only route accepted with open implementation")
	}

	created := lifecycleBaseEvents()
	start := causalEvent("start", "left", "phase_started", []string{"create"}, PhaseStartedPayload{Phase: "planning"})
	finish := causalEvent("z-finish", "right", "phase_finished", []string{"create"}, PhaseFinishedPayload{Phase: "planning", StartEventID: "start"})
	if projection := ProjectLifecycle(append(created, start, finish)); !hasInvalidEvent(projection.Invalid, "z-finish") {
		t.Fatal("causally invisible phase start accepted")
	}
	mismatch := lifecycleBaseEvents()
	mismatch = appendEvent(mismatch, "mismatch-start", "phase_started", PhaseStartedPayload{Phase: "planning"})
	mismatch = appendEvent(mismatch, "mismatch-finish", "phase_finished", PhaseFinishedPayload{Phase: "implementation", StartEventID: "mismatch-start"})
	if projection := ProjectLifecycle(mismatch); !hasInvalidEvent(projection.Invalid, "mismatch-finish") {
		t.Fatal("mismatched phase finish accepted")
	}

	handoff := causalEvent("associate", "child", "session_associated", []string{"create"}, SessionAssociatedPayload{AssociationOrigin: "handoff", TrajectoryID: "trajectory", HandoffEventID: "missing"})
	if projection := ProjectLifecycle(append(created, handoff)); !hasInvalidEvent(projection.Invalid, "associate") {
		t.Fatal("invisible handoff accepted")
	}

	completed := completedRoute("investigation-only")
	wrongEpoch := appendEvent(completed, "wrong-epoch", "effort_reopened", EffortReopenedPayload{TerminalEpoch: 3, TrajectoryID: "new", AnchorID: "anchor"})
	if projection := ProjectLifecycle(wrongEpoch); !hasInvalidEvent(projection.Invalid, "wrong-epoch") {
		t.Fatal("wrong reopen epoch accepted")
	}
	withTrajectory := lifecycleBaseEvents()
	withTrajectory = appendEvent(withTrajectory, "existing", "trajectory_started", TrajectoryPayload{TrajectoryID: "same", AnchorID: "anchor"})
	withTrajectory = appendEvent(withTrajectory, "route", "route_selected", RoutePayload{Route: "investigation-only"})
	withTrajectory = appendFinishedPhase(withTrajectory, "investigation", "investigation", "same")
	withTrajectory = appendFinishedPhase(withTrajectory, "retrospective", "retrospective", "same")
	withTrajectory = appendEvent(withTrajectory, "complete", "effort_completed", EffortTerminalPayload{TerminalEpoch: 1})
	withTrajectory = appendEvent(withTrajectory, "duplicate-reopen", "effort_reopened", EffortReopenedPayload{TerminalEpoch: 2, TrajectoryID: "same", AnchorID: "anchor"})
	if projection := ProjectLifecycle(withTrajectory); !hasInvalidEvent(projection.Invalid, "duplicate-reopen") {
		t.Fatal("reopen reused a trajectory")
	}

	rawRoute, _ := json.Marshal(RoutePayload{Route: "direct"})
	sourceRoute := causalEvent("source-route", "session", "route_selected", []string{"create"}, RoutePayload{Route: "direct"})
	badRepair := causalEvent("bad-repair", "session", "repair_applied", []string{"source-route"}, RepairAppliedPayload{ProposalKind: "supersede-event", SourceEventIDs: []string{"source-route"}, Replacement: RepairReplacement{EventKind: "route_changed", Payload: rawRoute}})
	if projection := ProjectLifecycle(append(created, sourceRoute, badRepair)); !hasInvalidEvent(projection.Invalid, "bad-repair") {
		t.Fatal("illegal repair replacement effect accepted")
	}
}

func TestLifecycleIllegalEffectRows(t *testing.T) {
	active := func() []EventEnvelope {
		events := lifecycleBaseEvents()
		return appendEvent(events, "route", "route_selected", RoutePayload{Route: "direct"})
	}
	started := active()
	started = appendEvent(started, "open", "phase_started", PhaseStartedPayload{Phase: "brainstorming"})
	duplicateTrajectory := lifecycleBaseEvents()
	duplicateTrajectory = appendEvent(duplicateTrajectory, "trajectory", "trajectory_started", TrajectoryPayload{TrajectoryID: "same", AnchorID: "a"})
	duplicateTrajectory = appendEvent(duplicateTrajectory, "bad", "trajectory_started", TrajectoryPayload{TrajectoryID: "same", AnchorID: "b"})
	duplicateFork := lifecycleBaseEvents()
	duplicateFork = appendEvent(duplicateFork, "parent", "trajectory_started", TrajectoryPayload{TrajectoryID: "parent", AnchorID: "a"})
	duplicateFork = appendEvent(duplicateFork, "child", "trajectory_forked", TrajectoryForkedPayload{TrajectoryID: "child", ParentTrajectoryID: "parent", ForkAnchorID: "a"})
	duplicateFork = appendEvent(duplicateFork, "bad", "trajectory_forked", TrajectoryForkedPayload{TrajectoryID: "child", ParentTrajectoryID: "parent", ForkAnchorID: "b"})
	tests := []struct {
		name   string
		events []EventEnvelope
		badID  string
	}{
		{"duplicate-create", append(lifecycleBaseEvents(), causalEvent("duplicate", "session", "effort_created", []string{"create"}, EffortCreatedPayload{})), "duplicate"},
		{"change-before-route", appendEvent(lifecycleBaseEvents(), "bad", "route_changed", RoutePayload{Route: "direct"}), "bad"},
		{"associate-unknown-trajectory", appendEvent(lifecycleBaseEvents(), "bad", "session_associated", SessionAssociatedPayload{AssociationOrigin: "manual", TrajectoryID: "missing"}), "bad"},
		{"finish-missing", appendEvent(lifecycleBaseEvents(), "bad", "phase_finished", PhaseFinishedPayload{Phase: "planning", StartEventID: "missing"}), "bad"},
		{"resume-missing", appendEvent(lifecycleBaseEvents(), "bad", "trajectory_resumed", TrajectoryPayload{TrajectoryID: "missing", AnchorID: "anchor"}), "bad"},
		{"fork-missing-parent", appendEvent(lifecycleBaseEvents(), "bad", "trajectory_forked", TrajectoryForkedPayload{TrajectoryID: "child", ParentTrajectoryID: "missing", ForkAnchorID: "anchor"}), "bad"},
		{"complete-discovery", appendEvent(lifecycleBaseEvents(), "bad", "effort_completed", EffortTerminalPayload{TerminalEpoch: 1}), "bad"},
		{"complete-wrong-epoch", appendEvent(active(), "bad", "effort_completed", EffortTerminalPayload{TerminalEpoch: 2}), "bad"},
		{"complete-open-phase", appendEvent(started, "bad", "effort_completed", EffortTerminalPayload{TerminalEpoch: 1}), "bad"},
		{"abandon-wrong-epoch", appendEvent(lifecycleBaseEvents(), "bad", "effort_abandoned", EffortTerminalPayload{TerminalEpoch: 2}), "bad"},
		{"duplicate-trajectory", duplicateTrajectory, "bad"},
		{"duplicate-fork", duplicateFork, "bad"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if projection := ProjectLifecycle(test.events); !hasInvalidEvent(projection.Invalid, test.badID) {
				t.Fatalf("illegal row applied: %#v", projection)
			}
		})
	}
	broken := causalEvent("broken", "other", "route_selected", []string{"missing"}, RoutePayload{Route: "direct"})
	if projection := ProjectLifecycle(append(lifecycleBaseEvents(), broken)); !hasInvalidEvent(projection.Invalid, "broken") {
		t.Fatal("causally broken lifecycle event applied")
	}
	malformedRepair := causalEvent("malformed-repair", "session", "repair_applied", []string{"create"}, json.RawMessage(`{`))
	if projection := ProjectLifecycle(append(lifecycleBaseEvents(), malformedRepair)); containsString(projection.AppliedEventIDs, "malformed-repair") {
		t.Fatal("malformed repair applied")
	}
	invalidRepair := causalEvent("invalid-repair", "session", "repair_applied", []string{"create"}, RepairAppliedPayload{ProposalKind: "supersede-event", SourceEventIDs: []string{"missing"}, Replacement: RepairReplacement{EventKind: "route_selected", Payload: json.RawMessage(`{"route":"direct"}`)}})
	if projection := ProjectLifecycle(append(lifecycleBaseEvents(), invalidRepair)); !hasInvalidEvent(projection.Invalid, "invalid-repair") {
		t.Fatal("invalid repair proposal applied")
	}
}

func TestRepairMultipleSourcesAndInvalidFallback(t *testing.T) {
	events := lifecycleBaseEvents()
	events = appendEvent(events, "route", "route_selected", RoutePayload{Route: "direct"})
	events = appendEvent(events, "change", "route_changed", RoutePayload{Route: "plan"})
	replacement, _ := json.Marshal(RoutePayload{Route: "direct"})
	events = appendEvent(events, "multi-repair", "repair_applied", RepairAppliedPayload{ProposalKind: "supersede-event", SourceEventIDs: []string{"route", "change"}, Replacement: RepairReplacement{EventKind: "route_selected", Payload: replacement}})
	projection := ProjectLifecycle(events)
	if len(projection.Repairs) != 1 || projection.Route != "direct" {
		t.Fatalf("multi-source repair = %#v", projection)
	}

	invalidSource := lifecycleBaseEvents()
	invalidSource = appendEvent(invalidSource, "invalid-source", "phase_finished", PhaseFinishedPayload{Phase: "planning", StartEventID: "missing"})
	invalidSource = appendEvent(invalidSource, "invalid-fallback-repair", "repair_applied", RepairAppliedPayload{ProposalKind: "supersede-event", SourceEventIDs: []string{"invalid-source"}, Replacement: RepairReplacement{EventKind: "route_changed", Payload: replacement}})
	projection = ProjectLifecycle(invalidSource)
	if !hasInvalidEvent(projection.Invalid, "invalid-source") || !hasInvalidEvent(projection.Invalid, "invalid-fallback-repair") {
		t.Fatalf("invalid repair fallback lost violations: %#v", projection)
	}
}

func TestRepairValidationRows(t *testing.T) {
	created := causalEvent("create", "session", "effort_created", nil, EffortCreatedPayload{})
	route := causalEvent("route", "session", "route_selected", []string{"create"}, RoutePayload{Route: "direct"})
	passive := passiveProjectionEvent("passive", "")
	passive.Predecessors = []string{"route"}
	byID := map[string]EventEnvelope{"create": created, "route": route, "passive": passive}
	rawRoute, _ := json.Marshal(RoutePayload{Route: "plan"})
	valid := RepairAppliedPayload{ProposalKind: "supersede-event", SourceEventIDs: []string{"route"}, Replacement: RepairReplacement{EventKind: "route_selected", Payload: rawRoute}}
	repair := causalEvent("repair", "other", "repair_applied", []string{"route"}, valid)
	order, _ := BuildCausalOrder([]EventEnvelope{created, route, repair, passive})
	byID["repair"] = repair
	if err := validateRepair(repair, valid, byID, order); err != nil {
		t.Fatal(err)
	}
	passiveRepair := causalEvent("passive-repair", "other", "repair_applied", []string{"passive"}, valid)
	passiveOrder, _ := BuildCausalOrder([]EventEnvelope{created, route, passive, passiveRepair})
	if err := validateRepair(passiveRepair, RepairAppliedPayload{ProposalKind: "supersede-event", SourceEventIDs: []string{"passive"}, Replacement: valid.Replacement}, byID, passiveOrder); err == nil {
		t.Fatal("passive source accepted as repair evidence")
	}
	associationPayload, _ := json.Marshal(SessionDetachedPayload{Reason: "repair"})
	trajectoryPayload, _ := json.Marshal(TrajectoryPayload{TrajectoryID: "trajectory", AnchorID: "anchor"})
	phasePayload, _ := json.Marshal(PhaseStartedPayload{Phase: "planning"})
	for _, replacement := range []struct {
		kind    ProposalKind
		event   EventKind
		payload json.RawMessage
	}{
		{"correct-phase", "phase_started", phasePayload},
		{"correct-association", "session_detached", associationPayload},
		{"correct-trajectory", "trajectory_started", trajectoryPayload},
	} {
		proposal := RepairAppliedPayload{ProposalKind: replacement.kind, SourceEventIDs: []string{"route"}, Replacement: RepairReplacement{EventKind: replacement.event, Payload: replacement.payload}}
		if err := validateRepair(repair, proposal, byID, order); err != nil {
			t.Errorf("valid %s repair rejected: %v", replacement.kind, err)
		}
	}
	concurrentSource := causalEvent("concurrent", "third", "route_selected", []string{"create"}, RoutePayload{Route: "plan"})
	concurrentByID := map[string]EventEnvelope{"create": created, "concurrent": concurrentSource, "repair": repair}
	concurrentOrder, _ := BuildCausalOrder([]EventEnvelope{created, concurrentSource, repair})
	if err := validateRepair(repair, RepairAppliedPayload{ProposalKind: "supersede-event", SourceEventIDs: []string{"concurrent"}, Replacement: valid.Replacement}, concurrentByID, concurrentOrder); err == nil {
		t.Fatal("causally invisible repair source accepted")
	}
	cases := []RepairAppliedPayload{
		{ProposalKind: "supersede-event", Replacement: valid.Replacement},
		{ProposalKind: "supersede-event", SourceEventIDs: []string{"missing"}, Replacement: valid.Replacement},
		{ProposalKind: "supersede-event", SourceEventIDs: []string{"passive"}, Replacement: valid.Replacement},
		{ProposalKind: "supersede-event", SourceEventIDs: []string{"route"}, Replacement: RepairReplacement{EventKind: "repair_applied", Payload: rawRoute}},
		{ProposalKind: "correct-phase", SourceEventIDs: []string{"route"}, Replacement: valid.Replacement},
		{ProposalKind: "correct-association", SourceEventIDs: []string{"route"}, Replacement: valid.Replacement},
		{ProposalKind: "correct-trajectory", SourceEventIDs: []string{"route"}, Replacement: valid.Replacement},
	}
	for index, payload := range cases {
		if err := validateRepair(repair, payload, byID, order); err == nil {
			t.Errorf("invalid repair case %d accepted", index)
		}
	}
}

func TestApplyLifecycleDerivesAssociatedTrajectory(t *testing.T) {
	ledger, err := NewLedger(newTestProject(t))
	if err != nil {
		t.Fatal(err)
	}
	base := LifecycleRequestBase{IdempotencyKey: "create-key", EventID: "create", EffortID: "associated-effort", SessionID: "session", Timestamp: "2026-07-22T00:00:00Z", Predecessors: []string{}}
	if _, err := ledger.ApplyLifecycle(context.Background(), CreateLifecycleRequest{LifecycleRequestBase: withAction(base, "create"), CheckpointID: "checkpoint.md", CreationMode: "independent"}); err != nil {
		t.Fatal(err)
	}
	base.IdempotencyKey, base.EventID, base.Predecessors = "trajectory-key", "trajectory", []string{"create"}
	if _, err := ledger.ApplyLifecycle(context.Background(), TrajectoryLifecycleRequest{LifecycleRequestBase: withAction(base, "start-trajectory"), TrajectoryID: "trajectory", AnchorID: "anchor"}); err != nil {
		t.Fatal(err)
	}
	base.IdempotencyKey, base.EventID, base.Predecessors = "associate-key", "associate", []string{"trajectory"}
	if _, err := ledger.ApplyLifecycle(context.Background(), AssociateLifecycleRequest{LifecycleRequestBase: withAction(base, "associate"), TrajectoryID: "trajectory", AssociationOrigin: "manual"}); err != nil {
		t.Fatal(err)
	}
	base.IdempotencyKey, base.EventID, base.Predecessors = "phase-key", "phase", []string{"associate"}
	phase := StartPhaseLifecycleRequest{LifecycleRequestBase: withAction(base, "start-phase"), Phase: "brainstorming"}
	if _, err := ledger.ApplyLifecycle(context.Background(), phase); err != nil {
		t.Fatal(err)
	}
	if result, err := ledger.ApplyLifecycle(context.Background(), phase); err != nil || !result.Idempotent {
		t.Fatalf("associated phase retry = %#v, %v", result, err)
	}
	read, err := ledger.ReadEffort("associated-effort")
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range read.Events {
		if event.EventID == "phase" && event.TrajectoryID != "trajectory" {
			t.Fatalf("phase trajectory = %q", event.TrajectoryID)
		}
	}
}

func TestApplyLifecycleReopenAdjustsEpoch(t *testing.T) {
	ledger, err := NewLedger(newTestProject(t))
	if err != nil {
		t.Fatal(err)
	}
	create := CreateLifecycleRequest{LifecycleRequestBase: LifecycleRequestBase{Action: "create", IdempotencyKey: "key-create", EventID: "create", EffortID: "effort", SessionID: "session", Timestamp: "2026-07-22T00:00:00Z", Predecessors: []string{}}, CheckpointID: "x", CreationMode: "independent"}
	if _, err := ledger.ApplyLifecycle(context.Background(), create); err != nil {
		t.Fatal(err)
	}
	for _, event := range completedRoute("investigation-only")[1:] {
		raw, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ledger.Append(context.Background(), raw); err != nil {
			t.Fatalf("append %s: %v", event.EventID, err)
		}
	}
	reopen := ReopenLifecycleRequest{LifecycleRequestBase: LifecycleRequestBase{Action: "reopen", IdempotencyKey: "key-reopen", EventID: "reopen", EffortID: "effort", SessionID: "session", Timestamp: "2026-07-22T00:01:00Z", Predecessors: []string{"complete"}}, TrajectoryID: "new-trajectory", AnchorID: "anchor"}
	if _, err := ledger.ApplyLifecycle(context.Background(), reopen); err != nil {
		t.Fatal(err)
	}
	if result, err := ledger.ApplyLifecycle(context.Background(), reopen); err != nil || !result.Idempotent {
		t.Fatalf("reopen retry = %#v, %v", result, err)
	}
	conflict := reopen
	conflict.AnchorID = "different-anchor"
	if _, err := ledger.ApplyLifecycle(context.Background(), conflict); err == nil {
		t.Fatal("conflicting reopen retry accepted")
	}
	read, err := ledger.ReadEffort("effort")
	projection := ProjectLifecycle(read.Events)
	if err != nil || projection.TerminalEpoch != 2 || projection.State != EffortActive {
		t.Fatalf("reopen adjustment = %#v, %v", projection, err)
	}
}

func TestApplyLifecycleTerminalEpochAdjustment(t *testing.T) {
	ledger, err := NewLedger(newTestProject(t))
	if err != nil {
		t.Fatal(err)
	}
	create := CreateLifecycleRequest{LifecycleRequestBase: LifecycleRequestBase{Action: "create", IdempotencyKey: "create-key", EventID: "create", EffortID: "terminal-effort", SessionID: "session", Timestamp: "2026-07-22T00:00:00Z", Predecessors: []string{}}, CheckpointID: "checkpoint.md", CreationMode: "independent"}
	if _, err := ledger.ApplyLifecycle(context.Background(), create); err != nil {
		t.Fatal(err)
	}
	abandon := TerminalLifecycleRequest{LifecycleRequestBase: LifecycleRequestBase{Action: "abandon", IdempotencyKey: "abandon-key", EventID: "abandon", EffortID: "terminal-effort", SessionID: "session", Timestamp: "2026-07-22T00:00:01Z", Predecessors: []string{"create"}}}
	if _, err := ledger.ApplyLifecycle(context.Background(), abandon); err != nil {
		t.Fatal(err)
	}
	if result, err := ledger.ApplyLifecycle(context.Background(), abandon); err != nil || !result.Idempotent {
		t.Fatalf("terminal retry = %#v, %v", result, err)
	}
	read, err := ledger.ReadEffort("terminal-effort")
	if err != nil || ProjectLifecycle(read.Events).State != EffortAbandoned {
		t.Fatalf("adjusted terminal append = %#v, %v", read, err)
	}
}

func TestAppendSkipsMalformedEvidenceDuringIdentityScan(t *testing.T) {
	ledger, metadata, _ := createTestEffort(t)
	stream := ledger.paths.stream(metadata.EffortID, "session-id")
	file, err := openAppend(stream)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("{malformed}\n")); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.Append(context.Background(), passiveEvent(t, "passive-after-malformed", "observation-after-malformed", metadata.EffortID, []string{"event-id"})); err != nil {
		t.Fatal(err)
	}
}

func TestRawLedgerRejectsIllegalLifecycleAppend(t *testing.T) {
	ledger, metadata, _ := createTestEffort(t)
	invalid := lifecycleRaw(t, "invalid-complete", "invalid-complete-key", metadata.EffortID, "effort_completed", []string{"event-id"}, EffortTerminalPayload{TerminalEpoch: 1})
	if _, err := ledger.Append(context.Background(), invalid); err == nil {
		t.Fatal("raw invalid lifecycle append accepted")
	}
}

func TestLifecycleRequestIdentityComparison(t *testing.T) {
	payload, _ := json.Marshal(RoutePayload{Route: "direct"})
	requested := EventEnvelope{EventID: "requested", EffortID: "effort", SessionID: "session", Timestamp: "2026-07-22T00:00:00Z", Kind: "route_selected", Predecessors: []string{"one"}, Payload: payload}
	prior := requested
	prior.EventID = "different"
	equal, err := lifecycleRequestMatchesEvent(requested, eventToRaw(requested), prior)
	if err != nil || equal {
		t.Fatalf("different request identity matched: %v, %v", equal, err)
	}
}

func TestApplyLifecycleCreationAndVerificationFailures(t *testing.T) {
	root := newTestProject(t)
	ledger, err := NewLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	base := LifecycleRequestBase{Action: "create", IdempotencyKey: "create-key", EventID: "create", EffortID: "effort", SessionID: "session", Timestamp: "2026-07-22T00:00:00Z", Predecessors: []string{}}
	request := CreateLifecycleRequest{LifecycleRequestBase: base, CheckpointID: "checkpoint.md", CreationMode: "independent"}
	originalReadDir := ledger.ops.readDir
	ledger.ops.readDir = func(string) ([]os.DirEntry, error) { return nil, errors.New("verify read failed") }
	if _, err := ledger.ApplyLifecycle(context.Background(), request); err == nil {
		t.Fatal("lifecycle creation verification read failure was hidden")
	}
	ledger.ops.readDir = originalReadDir
	request.CheckpointID = "different.md"
	if _, err := ledger.ApplyLifecycle(context.Background(), request); err == nil {
		t.Fatal("conflicting lifecycle creation retry accepted")
	}
}

func TestApplyLifecycleSuccessfulAppendRetryAndConflict(t *testing.T) {
	ledger, err := NewLedger(newTestProject(t))
	if err != nil {
		t.Fatal(err)
	}
	createBase := LifecycleRequestBase{Action: "create", IdempotencyKey: "create-key", EventID: "create", EffortID: "effort", SessionID: "session", Timestamp: "2026-07-22T00:00:00Z", Predecessors: []string{}}
	if _, err := ledger.ApplyLifecycle(context.Background(), CreateLifecycleRequest{LifecycleRequestBase: createBase, CheckpointID: "checkpoint.md", CreationMode: "independent"}); err != nil {
		t.Fatal(err)
	}
	routeBase := LifecycleRequestBase{Action: "select-route", IdempotencyKey: "route-key", EventID: "route", EffortID: "effort", SessionID: "session", Timestamp: "2026-07-22T00:00:01Z", Predecessors: []string{"create"}}
	request := RouteLifecycleRequest{LifecycleRequestBase: routeBase, Route: "direct"}
	if result, err := ledger.ApplyLifecycle(context.Background(), request); err != nil || result.Idempotent {
		t.Fatalf("route append = %#v, %v", result, err)
	}
	if result, err := ledger.ApplyLifecycle(context.Background(), request); err != nil || !result.Idempotent {
		t.Fatalf("route retry = %#v, %v", result, err)
	}
	request.Route = "plan"
	if _, err := ledger.ApplyLifecycle(context.Background(), request); err == nil {
		t.Fatal("conflicting lifecycle retry accepted")
	}

	invalid := request
	invalid.Action = "nonsense"
	if _, err := ledger.ApplyLifecycle(context.Background(), invalid); err == nil {
		t.Fatal("invalid request action accepted")
	}
	unsafe := request
	unsafe.IdempotencyKey, unsafe.EventID, unsafe.EffortID = "other-key", "other-event", "bad/id"
	if _, err := ledger.ApplyLifecycle(context.Background(), unsafe); err == nil {
		t.Fatal("unsafe lifecycle event accepted")
	}
	missing := request
	missing.IdempotencyKey, missing.EventID, missing.EffortID = "missing-key", "missing-event", "missing"
	if _, err := ledger.ApplyLifecycle(context.Background(), missing); err == nil {
		t.Fatal("missing effort mutation accepted")
	}

	failing, err := NewLedger(newTestProject(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := failing.ApplyLifecycle(context.Background(), CreateLifecycleRequest{LifecycleRequestBase: createBase, CheckpointID: "checkpoint.md", CreationMode: "independent"}); err != nil {
		t.Fatal(err)
	}
	originalOpen := failing.ops.openFile
	failing.ops.openFile = func(string, int, fs.FileMode) (syncedFile, error) { return nil, errors.New("injected append failure") }
	_ = originalOpen
	if _, err := failing.ApplyLifecycle(context.Background(), request); err == nil {
		t.Fatal("durable append failure hidden")
	}
}
