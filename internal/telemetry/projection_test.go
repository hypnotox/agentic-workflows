package telemetry

import (
	"encoding/json"
	"testing"
)

// invariant: tooling/workflow-telemetry:trajectory-and-derived-effort-model
func TestTrajectoryAndDerivedEffortModel(t *testing.T) {
	events := lifecycleBaseEvents()
	events = appendEvent(events, "parent", "trajectory_started", TrajectoryPayload{TrajectoryID: "parent", AnchorID: "anchor-parent"})
	events[len(events)-1].TrajectoryID = "parent"
	parentWork := passiveProjectionEvent("parent-work", "parent")
	events = append(events, parentWork)
	events = appendEvent(events, "fork", "trajectory_forked", TrajectoryForkedPayload{TrajectoryID: "discarded", ParentTrajectoryID: "parent", ForkAnchorID: "fork-anchor"})
	events[len(events)-1].TrajectoryID = "discarded"
	discardedWork := passiveProjectionEvent("discarded-work", "discarded")
	events = append(events, discardedWork)
	events = appendEvent(events, "resume", "trajectory_resumed", TrajectoryPayload{TrajectoryID: "parent", AnchorID: "anchor-parent"})
	events[len(events)-1].TrajectoryID = "parent"
	activeWork := passiveProjectionEvent("active-work", "parent")
	events = append(events, activeWork)

	metadata := EffortMetadata{EffortID: "parent-effort", CreatedAt: "2026-07-22T00:00:00Z", CreationMode: "independent"}
	projection := ProjectWorkflow(EffortRead{Metadata: metadata, Events: events, Integrity: []IntegrityIssue{}})
	if projection.Lifecycle.ActiveTrajectoryID != "parent" || !containsString(projection.CurrentPathEventIDs, "parent-work") || !containsString(projection.CurrentPathEventIDs, "active-work") {
		t.Fatalf("active path missing ancestry work: %#v", projection)
	}
	if containsString(projection.CurrentPathEventIDs, "discarded-work") || !containsString(projection.AllWorkEventIDs, "discarded-work") || !containsString(projection.DiscardedEventIDs, "discarded-work") {
		t.Fatalf("discarded work accounting = %#v", projection)
	}
	if len(projection.Trajectories) != 2 || projection.Trajectories[0].ForkAnchorID != "fork-anchor" && projection.Trajectories[1].ForkAnchorID != "fork-anchor" {
		t.Fatalf("fork metadata = %#v", projection.Trajectories)
	}

	origin := &OriginMetadata{EffortID: "parent-effort", TrajectoryID: "parent", AnchorID: "anchor-parent"}
	child := ProjectWorkflow(EffortRead{Metadata: EffortMetadata{EffortID: "derived-effort", CreatedAt: "2026-07-22T01:00:00Z", CreationMode: "derived", Origin: origin}, Events: lifecycleBaseEvents()})
	grandchildOrigin := &OriginMetadata{EffortID: "derived-effort", TrajectoryID: "child-trajectory", AnchorID: "child-anchor"}
	grandchild := ProjectWorkflow(EffortRead{Metadata: EffortMetadata{EffortID: "grandchild-effort", CreationMode: "derived", Origin: grandchildOrigin}, Events: lifecycleBaseEvents()})
	family := GroupDerivedFamilies([]WorkflowProjection{projection, child, grandchild})
	if len(family[0].DerivedEffortIDs) != 2 || family[0].DerivedEffortIDs[0] != "derived-effort" || family[0].DerivedEffortIDs[1] != "grandchild-effort" || len(family[1].DerivedEffortIDs) != 1 || family[1].Origin != origin {
		t.Fatalf("derived family = %#v", family)
	}
	if containsString(family[1].AllWorkEventIDs, "parent-work") {
		t.Fatal("derived effort duplicated parent work")
	}
	if projection.Origin != nil || projection.Metadata.CreationMode != "independent" || child.Origin == nil || child.Origin.EffortID != "parent-effort" {
		t.Fatalf("independent/derived origin contract = parent %#v child %#v", projection.Origin, child.Origin)
	}

	reopenedEvents := completedRoute("direct")
	reopenedEvents = appendEvent(reopenedEvents, "reopen", "effort_reopened", EffortReopenedPayload{TerminalEpoch: 2, TrajectoryID: "reopened", AnchorID: "reopen-anchor"})
	reopenedEvents[len(reopenedEvents)-1].TrajectoryID = "reopened"
	reopened := ProjectWorkflow(EffortRead{Metadata: metadata, Events: reopenedEvents})
	if reopened.Lifecycle.State != EffortActive || reopened.Lifecycle.TerminalEpoch != 2 || reopened.Lifecycle.ActiveTrajectoryID != "reopened" {
		t.Fatalf("reopened effort projection = %#v", reopened.Lifecycle)
	}

	segments := lifecycleBaseEvents()
	segments = appendEvent(segments, "segment-start", "trajectory_started", TrajectoryPayload{TrajectoryID: "segment", AnchorID: "anchor-a"})
	segments = appendEvent(segments, "segment-close-a", "trajectory_closed", TrajectoryPayload{TrajectoryID: "segment", AnchorID: "anchor-a"})
	segments = appendEvent(segments, "segment-resume", "trajectory_resumed", TrajectoryPayload{TrajectoryID: "segment", AnchorID: "anchor-b"})
	segments = appendEvent(segments, "segment-close-b", "trajectory_closed", TrajectoryPayload{TrajectoryID: "segment", AnchorID: "anchor-b"})
	segmentProjection := ProjectLifecycle(segments)
	if len(segmentProjection.Invalid) != 0 || segmentProjection.ActiveTrajectoryID != "" || !segmentProjection.closedTrajectories["segment"] || !segmentProjection.EffectApplied["segment-close-b"] {
		t.Fatalf("close-resume-close segment lifecycle = %#v", segmentProjection)
	}
}

func TestProtocol2TransitionProjectsBothPhaseEffectsOnce(t *testing.T) {
	events := lifecycleBaseEvents()
	events = appendEvent(events, "brainstorm-start", "phase_started", PhaseStartedPayload{Phase: "brainstorming"})
	transition := protocol2TransitionEnvelope(t, "transition", []string{"brainstorm-start"}, "planning", "plan")
	projection := ProjectWorkflow(EffortRead{Metadata: EffortMetadata{EffortID: "effort-id", CreationMode: "independent"}, Events: append(events, transition)})
	if projection.Lifecycle.Route != "plan" || len(projection.Lifecycle.PhaseIntervals) != 1 || projection.Lifecycle.OpenPhases["transition"].Phase != "planning" {
		t.Fatalf("transition projection did not atomically close and open phases: %#v", projection.Lifecycle)
	}
	if countProjectionString(projection.EvidenceEventIDs, "transition") != 1 || countProjectionString(projection.AllWorkEventIDs, "transition") != 1 {
		t.Fatalf("transition was not projected exactly once: evidence=%v all=%v", projection.EvidenceEventIDs, projection.AllWorkEventIDs)
	}
}

func countProjectionString(values []string, wanted string) int {
	count := 0
	for _, value := range values {
		if value == wanted {
			count++
		}
	}
	return count
}

func TestWorkflowProjectionStopsAtUnsupportedProtocolBoundary(t *testing.T) {
	read := EffortRead{
		Metadata: EffortMetadata{EffortID: "future-effort"},
		Events:   lifecycleBaseEvents(),
		Integrity: []IntegrityIssue{
			{Code: "unsupported-protocol", Scope: "session", Line: 2},
			{Code: "unsupported-protocol", Scope: "session", Line: 2},
		},
	}
	projection := ProjectWorkflow(read)
	if projection.Lifecycle.State != "" || len(projection.EvidenceEventIDs) != 0 || len(projection.Integrity) != 1 {
		t.Fatalf("unsupported workflow projection = %#v", projection)
	}
}

func TestProtocol21WorkflowProjectionRetainsAdoptionBoundaryAndDetourLineage(t *testing.T) {
	adopted := protocol21Envelope(t, "adopt", "effort_adopted", nil, map[string]any{
		"creationMode": "adopted", "phase": "planning", "workflow": "writing-plans",
		"trajectoryId": "adopted-trajectory", "anchorId": "adopted-anchor", "associationOrigin": "manual",
	})
	adoptedProjection := ProjectWorkflow(EffortRead{
		Metadata: protocol21Metadata(t, map[string]any{"effortId": "effort-id", "createdAt": adopted.Timestamp, "creationMode": "adopted"}),
		Events:   []EventEnvelope{adopted},
	})
	if adoptedProjection.Lifecycle.State != EffortDiscovery || adoptedProjection.Lifecycle.OpenPhases["adopt"].StartEventID != "adopt" || adoptedProjection.Lifecycle.ActiveTrajectoryID != "adopted-trajectory" {
		t.Fatalf("canonical adoption boundary projection = %#v", adoptedProjection)
	}
	if adoptedProjection.AdoptionBoundary == nil || adoptedProjection.AdoptionBoundary.EventID != "adopt" || adoptedProjection.AdoptionBoundary.Phase != "planning" || adoptedProjection.AdoptionBoundary.Workflow != "writing-plans" || adoptedProjection.Lifecycle.CurrentWorkflow != "writing-plans" {
		t.Fatalf("bounded adoption projection and attribution = %#v", adoptedProjection)
	}
	if !containsString(adoptedProjection.EvidenceEventIDs, "adopt") || !containsString(adoptedProjection.AllWorkEventIDs, "adopt") {
		t.Fatalf("adoption boundary is absent from canonical evidence: %#v", adoptedProjection)
	}

	origin := map[string]any{"effortId": "parent-effort", "trajectoryId": "parent-trajectory", "anchorId": "parent-anchor"}
	started := protocol21Envelope(t, "detour-start", "detour_started", nil, map[string]any{
		"creationMode": "derived", "origin": origin, "returnPhase": "implementation", "returnPhaseStartEventId": "parent-phase-start",
		"trajectoryId": "child-trajectory", "anchorId": "child-anchor", "workflow": "brainstorming", "associationOrigin": "detour",
	})
	detourProjection := ProjectWorkflow(EffortRead{
		Metadata: protocol21Metadata(t, map[string]any{
			"effortId": "effort-id", "createdAt": started.Timestamp, "creationMode": "derived", "origin": origin,
			"detourReturn": map[string]any{"sessionId": "session-id", "phase": "implementation", "phaseStartEventId": "parent-phase-start"},
		}),
		Events: []EventEnvelope{started},
	})
	if detourProjection.Origin == nil || detourProjection.Origin.EffortID != "parent-effort" || detourProjection.Lifecycle.State != EffortDiscovery || detourProjection.Lifecycle.OpenPhases["detour-start"].Phase != "brainstorming" {
		t.Fatalf("canonical detour lineage projection = %#v", detourProjection)
	}
	if detourProjection.DetourReturn == nil || detourProjection.DetourReturn.Pending || detourProjection.DetourReturn.Settled || detourProjection.DetourReturn.SessionID != "session-id" || detourProjection.DetourReturn.PhaseStartEventID != "parent-phase-start" {
		t.Fatalf("active detour return target projection = %#v", detourProjection.DetourReturn)
	}

	abandoned := protocol21Envelope(t, "abandon", "effort_abandoned", []string{"detour-start"}, map[string]any{"terminalEpoch": 1})
	pending := ProjectWorkflow(EffortRead{Metadata: detourProjection.Metadata, Events: []EventEnvelope{started, abandoned}})
	if pending.DetourReturn == nil || !pending.DetourReturn.Pending || pending.DetourReturn.Settled || pending.DetourReturn.TerminalOutcome != "abandoned" || pending.DetourReturn.ParentAssociationEventID != "" || pending.DetourReturn.ReturnEventID != "" {
		t.Fatalf("pending detour return projection = %#v", pending.DetourReturn)
	}
	returned := protocol21Envelope(t, "returned", "detour_returned", []string{"abandon"}, map[string]any{"terminalOutcome": "abandoned", "parentAssociationEventId": "parent-association"})
	settled := ProjectWorkflow(EffortRead{Metadata: detourProjection.Metadata, Events: []EventEnvelope{started, abandoned, returned}})
	if settled.DetourReturn == nil || settled.DetourReturn.Pending || !settled.DetourReturn.Settled || settled.DetourReturn.TerminalOutcome != "abandoned" || settled.DetourReturn.ParentAssociationEventID != "parent-association" || settled.DetourReturn.ReturnEventID != "returned" {
		t.Fatalf("settled detour return projection = %#v", settled.DetourReturn)
	}
}

func TestProjectionUsesAssociationAndActiveAncestry(t *testing.T) {
	events := lifecycleBaseEvents()
	events = appendEvent(events, "parent", "trajectory_started", TrajectoryPayload{TrajectoryID: "parent", AnchorID: "parent-anchor"})
	events[len(events)-1].TrajectoryID = "parent"
	events = appendEvent(events, "child", "trajectory_forked", TrajectoryForkedPayload{TrajectoryID: "child", ParentTrajectoryID: "parent", ForkAnchorID: "fork-anchor"})
	events[len(events)-1].TrajectoryID = "child"
	beforeAssociation := passiveProjectionEvent("before-association", "")
	beforeAssociation.Predecessors = []string{"child"}
	events = append(events, beforeAssociation)
	events = appendEvent(events, "associate", "session_associated", SessionAssociatedPayload{AssociationOrigin: "manual", TrajectoryID: "child"})
	associatedWork := passiveProjectionEvent("associated-work", "")
	associatedWork.Predecessors = []string{"associate"}
	events = append(events, associatedWork)
	events = appendEvent(events, "detach", "session_detached", SessionDetachedPayload{Reason: "manual"})
	afterDetach := passiveProjectionEvent("after-detach", "")
	afterDetach.Predecessors = []string{"detach"}
	events = append(events, afterDetach)
	projection := ProjectWorkflow(EffortRead{Metadata: EffortMetadata{EffortID: "effort"}, Events: events})
	if len(projection.ActiveAncestry) != 2 || projection.ActiveAncestry[0] != "parent" || projection.ActiveAncestry[1] != "child" || !containsString(projection.CurrentPathEventIDs, "associated-work") {
		t.Fatalf("association/ancestry projection = %#v", projection)
	}
	for _, trajectory := range projection.Trajectories {
		if trajectory.TrajectoryID == "child" && (containsString(trajectory.EventIDs, "before-association") || containsString(trajectory.EventIDs, "after-detach")) {
			t.Fatal("event outside the association interval was assigned using future or stale state")
		}
	}
}

func TestProjectionPreservesInvalidEvidenceWithoutEffects(t *testing.T) {
	events := lifecycleBaseEvents()
	events = appendEvent(events, "illegal-complete", "effort_completed", EffortTerminalPayload{TerminalEpoch: 1})
	read := EffortRead{Metadata: EffortMetadata{EffortID: "effort", CreationMode: "independent"}, Events: events}
	projection := ProjectWorkflow(read)
	if projection.Lifecycle.State == EffortCompleted || !hasInvalidEvent(projection.Integrity, "illegal-complete") {
		t.Fatalf("invalid effect projected: %#v", projection)
	}
}

func TestDerivedFamilyCycleIsBounded(t *testing.T) {
	leftOrigin := &OriginMetadata{EffortID: "right", TrajectoryID: "t", AnchorID: "a"}
	rightOrigin := &OriginMetadata{EffortID: "left", TrajectoryID: "t", AnchorID: "a"}
	family := GroupDerivedFamilies([]WorkflowProjection{{Metadata: EffortMetadata{EffortID: "left"}, Origin: leftOrigin}, {Metadata: EffortMetadata{EffortID: "right"}, Origin: rightOrigin}})
	if len(family) != 2 {
		t.Fatal("cyclic opaque origins changed family cardinality")
	}
}

func TestProjectionIndependentOriginAndUnknownDerivedParent(t *testing.T) {
	independent := ProjectWorkflow(EffortRead{Metadata: EffortMetadata{EffortID: "independent", CreationMode: "independent"}, Events: lifecycleBaseEvents()})
	orphanOrigin := &OriginMetadata{EffortID: "pruned-parent", TrajectoryID: "trajectory", AnchorID: "anchor"}
	derived := ProjectWorkflow(EffortRead{Metadata: EffortMetadata{EffortID: "derived", CreationMode: "derived", Origin: orphanOrigin}, Events: lifecycleBaseEvents()})
	family := GroupDerivedFamilies([]WorkflowProjection{independent, derived})
	if family[0].Origin != nil || len(family[0].DerivedEffortIDs) != 0 || family[1].Origin == nil {
		t.Fatalf("origin semantics = %#v", family)
	}
}

func passiveProjectionEvent(id, trajectory string) EventEnvelope {
	payload, _ := json.Marshal(UsageObservedPayload{Model: "model", InputTokens: 1, OutputTokens: 1, DurationMS: 1})
	return EventEnvelope{Version: ProtocolVersion{Major: 2}, EventID: id, ObservationID: "observation-" + id, EffortID: "effort", SessionID: "session", TrajectoryID: trajectory, Timestamp: "2026-07-22T00:00:00Z", Kind: "usage_observed", Predecessors: []string{}, Payload: payload}
}
