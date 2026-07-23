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
