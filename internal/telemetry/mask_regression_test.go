package telemetry

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

func TestExcludedTrajectoryOriginCannotAuthorizeConsumers(t *testing.T) {
	for name, consumer := range map[string]func([]EventEnvelope) []EventEnvelope{
		"association": func(events []EventEnvelope) []EventEnvelope {
			return appendEvent(events, "consumer", "session_associated", SessionAssociatedPayload{AssociationOrigin: "manual", TrajectoryID: "excluded-trajectory"})
		},
		"resume": func(events []EventEnvelope) []EventEnvelope {
			return appendEvent(events, "consumer", "trajectory_resumed", TrajectoryPayload{TrajectoryID: "excluded-trajectory", AnchorID: "anchor"})
		},
		"fork": func(events []EventEnvelope) []EventEnvelope {
			return appendEvent(events, "consumer", "trajectory_forked", TrajectoryForkedPayload{TrajectoryID: "child", ParentTrajectoryID: "excluded-trajectory", ForkAnchorID: "anchor"})
		},
	} {
		t.Run(name, func(t *testing.T) {
			events := lifecycleBaseEvents()
			events = appendEvent(events, "excluded-origin", "trajectory_started", TrajectoryPayload{TrajectoryID: "excluded-trajectory", AnchorID: "anchor"})
			events = consumer(events)
			projection := projectLifecycle(events, map[string]bool{"excluded-origin": true})
			if projection.EffectApplied["consumer"] || len(projection.Trajectories) != 0 || !hasIssue(projection.Invalid, "invalid-transition") {
				t.Fatalf("excluded trajectory origin authorized %s: %#v", name, projection)
			}
		})
	}
}

func TestRejectedDuplicateEvidenceNeverChangesStateProjectionRetentionOrMutation(t *testing.T) {
	ledger, metadata, firstRaw := createTestEffort(t)
	first, err := ValidateEvent(firstRaw)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(EffortTerminalPayload{TerminalEpoch: 1})
	if err != nil {
		t.Fatal(err)
	}
	conflicting := EventEnvelope{
		Version: descriptor.Version, EventID: "conflicting-terminal", IdempotencyKey: first.IdempotencyKey,
		EffortID: metadata.EffortID, SessionID: first.SessionID, Timestamp: "2026-07-22T00:00:01Z",
		Kind: "effort_abandoned", Predecessors: []string{first.EventID}, Payload: payload,
	}
	raw, err := json.Marshal(conflicting)
	if err != nil {
		t.Fatal(err)
	}
	stream := ledger.paths.stream(metadata.EffortID, first.SessionID)
	file, err := os.OpenFile(stream, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(append(raw, '\n')); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	read, err := ledger.ReadEffort(metadata.EffortID)
	if err != nil {
		t.Fatal(err)
	}
	if read.EffectApplied[conflicting.EventID] || !read.EffectApplied[first.EventID] {
		t.Fatalf("reader mask = %#v", read.EffectApplied)
	}
	workflow := ProjectWorkflow(read)
	if workflow.Lifecycle.State != EffortDiscovery || workflow.EffectApplied[conflicting.EventID] || workflow.Lifecycle.EffectApplied[conflicting.EventID] {
		t.Fatalf("rejected evidence changed workflow projection: %#v", workflow)
	}
	retained, err := ledger.Retain(context.Background(), RetentionPolicy{MaxCompletedEffortCount: 1}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(retained.Candidates) != 0 {
		t.Fatalf("rejected terminal evidence became retention candidate: %#v", retained)
	}
	route := RouteLifecycleRequest{LifecycleRequestBase: LifecycleRequestBase{
		Action: "select-route", IdempotencyKey: "route-key", EventID: "route", EffortID: metadata.EffortID,
		SessionID: first.SessionID, Timestamp: "2026-07-22T00:00:02Z", Predecessors: []string{first.EventID},
	}, Route: "direct"}
	routeRaw, _, _, err := lifecycleRequestEvent(route)
	if err != nil {
		t.Fatal(err)
	}
	routeEvent, err := ValidateEvent(routeRaw)
	if err != nil {
		t.Fatal(err)
	}
	candidate := projectLifecycle(append(append([]EventEnvelope(nil), read.Events...), routeEvent), lifecycleExclusions(read))
	if candidate.State != EffortActive || len(candidate.Invalid) != 0 {
		t.Fatalf("masked candidate projection = %#v, exclusions=%#v", candidate, lifecycleExclusions(read))
	}
	if _, err := ledger.ApplyLifecycle(context.Background(), route); err != nil {
		t.Fatalf("rejected terminal evidence blocked valid discovery mutation: %v", err)
	}
}
