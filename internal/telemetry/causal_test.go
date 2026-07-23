package telemetry

import (
	"encoding/json"
	"testing"
)

func causalEvent(id, session string, kind EventKind, predecessors []string, payload any) EventEnvelope {
	raw, _ := json.Marshal(payload)
	return EventEnvelope{Version: ProtocolVersion{Major: 2}, EventID: id, IdempotencyKey: "key-" + id, EffortID: "effort", SessionID: session, Timestamp: "2026-07-22T00:00:00Z", Kind: kind, Predecessors: predecessors, Payload: raw}
}

func trajectoryAncestryForTest(nodes map[string]TrajectoryNode, trajectoryID string) ([]string, bool) {
	ancestry := []string{}
	seen := map[string]bool{}
	for trajectoryID != "" {
		if seen[trajectoryID] {
			return nil, false
		}
		seen[trajectoryID] = true
		node, ok := nodes[trajectoryID]
		if !ok {
			return nil, false
		}
		ancestry = append([]string{trajectoryID}, ancestry...)
		trajectoryID = node.ParentTrajectoryID
	}
	return ancestry, true
}

func TestCausalPartialOrderUsesProtocolEdgesOnly(t *testing.T) {
	created := causalEvent("created", "parent", "effort_created", nil, EffortCreatedPayload{CreationMode: "independent"})
	first := causalEvent("first", "parent", "route_selected", []string{"created"}, RoutePayload{Route: "direct"})
	left := causalEvent("z-left", "left", "session_detached", []string{"first"}, SessionDetachedPayload{Reason: "manual"})
	right := causalEvent("a-right", "right", "session_detached", []string{"first"}, SessionDetachedPayload{Reason: "manual"})
	// Deliberately reverse wall-clock order. It must not create an edge.
	left.Timestamp = "2030-01-01T00:00:00Z"
	right.Timestamp = "2020-01-01T00:00:00Z"
	order, issues := BuildCausalOrder([]EventEnvelope{created, first, left, right})
	if len(issues) != 0 || !order.HappensBefore(created.EventID, left.EventID) || !order.Concurrent(left.EventID, right.EventID) || !equalStrings(order.frontiers[left.EventID], order.frontiers[right.EventID]) {
		t.Fatalf("unexpected partial order: issues=%v concurrent=%v", issues, order.Concurrent(left.EventID, right.EventID))
	}
	if order.HappensBefore(right.EventID, left.EventID) || order.HappensBefore(left.EventID, right.EventID) {
		t.Fatal("timestamp or identifier invented a total order")
	}
	if got := order.frontiers[left.EventID]; len(got) != 1 || got[0] != first.EventID {
		t.Fatalf("frontier = %v", got)
	}
}

func TestCausalPerSessionHandoffAndTrajectoryAncestry(t *testing.T) {
	created := causalEvent("create", "one", "effort_created", nil, EffortCreatedPayload{})
	handoff := causalEvent("handoff", "one", "handoff_observed", nil, HandoffObservedPayload{})
	handoff.IdempotencyKey, handoff.ObservationID = "", "obs-handoff"
	associated := causalEvent("associated", "two", "session_associated", nil, SessionAssociatedPayload{AssociationOrigin: "handoff", TrajectoryID: "child", HandoffEventID: "handoff"})
	parent := causalEvent("parent", "one", "trajectory_started", []string{"create"}, TrajectoryPayload{TrajectoryID: "parent", AnchorID: "anchor"})
	parent.PiAnchorID = "fork-anchor"
	fork := causalEvent("fork", "two", "trajectory_forked", []string{"associated"}, TrajectoryForkedPayload{TrajectoryID: "child", ParentTrajectoryID: "parent", ForkAnchorID: "fork-anchor"})
	order, issues := BuildCausalOrder([]EventEnvelope{created, handoff, associated, parent, fork})
	if len(issues) != 0 || !order.HappensBefore(created.EventID, handoff.EventID) || !order.HappensBefore(handoff.EventID, associated.EventID) || !order.HappensBefore(parent.EventID, fork.EventID) {
		t.Fatalf("session/handoff order missing: %#v", issues)
	}
	ancestry, ok := trajectoryAncestryForTest(order.trajectories, "child")
	if !ok || len(ancestry) != 2 || ancestry[0] != "parent" || ancestry[1] != "child" {
		t.Fatalf("ancestry = %v, valid=%v", ancestry, ok)
	}
	if _, ok := trajectoryAncestryForTest(order.trajectories, "missing"); ok {
		t.Fatal("unknown trajectory ancestry accepted")
	}
}

func TestCausalTrajectoryAnchorLinksAndAmbiguity(t *testing.T) {
	anchor := causalEvent("anchor", "one", "phase_started", nil, PhaseStartedPayload{Phase: "planning"})
	anchor.PiAnchorID = "anchor-id"
	started := causalEvent("started", "one", "trajectory_started", nil, TrajectoryPayload{TrajectoryID: "trajectory", AnchorID: "anchor-id"})
	resumed := causalEvent("resumed", "two", "trajectory_resumed", nil, TrajectoryPayload{TrajectoryID: "trajectory", AnchorID: "anchor-id"})
	reopened := causalEvent("reopened", "three", "effort_reopened", nil, EffortReopenedPayload{TerminalEpoch: 2, TrajectoryID: "reopened-trajectory", AnchorID: "anchor-id"})
	order, issues := BuildCausalOrder([]EventEnvelope{anchor, started, resumed, reopened})
	if len(issues) != 0 || !order.HappensBefore(anchor.EventID, resumed.EventID) || !order.HappensBefore(anchor.EventID, reopened.EventID) {
		t.Fatalf("resume anchor link missing: %#v", issues)
	}
	duplicate := anchor
	duplicate.EventID, duplicate.SessionID = "duplicate-anchor", "three"
	_, issues = BuildCausalOrder([]EventEnvelope{anchor, duplicate})
	if !hasIssue(issues, "ambiguous-anchor") {
		t.Fatalf("ambiguous anchor not reported: %#v", issues)
	}
}

func TestTrajectoryContainment(t *testing.T) {
	nodes := map[string]TrajectoryNode{"parent": {ID: "parent"}, "child": {ID: "child", ParentTrajectoryID: "parent"}}
	if !trajectoryContains(nodes, "child", "parent") || trajectoryContains(nodes, "parent", "child") {
		t.Fatal("trajectory containment is incorrect")
	}
}

func TestCausalReportsBrokenReferencesAndCycles(t *testing.T) {
	broken := causalEvent("broken", "one", "route_selected", []string{"missing"}, RoutePayload{Route: "direct"})
	left := causalEvent("left", "two", "route_selected", []string{"right"}, RoutePayload{Route: "direct"})
	right := causalEvent("right", "three", "route_selected", []string{"left"}, RoutePayload{Route: "direct"})
	order, issues := BuildCausalOrder([]EventEnvelope{broken, left, right})
	ordered := order.ordered([]EventEnvelope{broken, left, right})
	if len(ordered) != 3 {
		t.Fatalf("cyclic evidence lost: %v", ordered)
	}
	codes := map[string]bool{}
	for _, issue := range issues {
		codes[issue.Code] = true
	}
	if !codes["broken-predecessor"] || !codes["causal-cycle"] {
		t.Fatalf("issues = %#v", issues)
	}
	order.trajectories["cycle-a"] = TrajectoryNode{ID: "cycle-a", ParentTrajectoryID: "cycle-b"}
	order.trajectories["cycle-b"] = TrajectoryNode{ID: "cycle-b", ParentTrajectoryID: "cycle-a"}
	if _, ok := trajectoryAncestryForTest(order.trajectories, "cycle-a"); ok {
		t.Fatal("trajectory cycle accepted")
	}
	if equalStrings([]string{"a"}, []string{"a", "b"}) || equalStrings([]string{"a"}, []string{"b"}) || !equalStrings(nil, nil) {
		t.Fatal("string equality helper is incorrect")
	}
}
