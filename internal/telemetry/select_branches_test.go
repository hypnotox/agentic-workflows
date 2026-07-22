package telemetry

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSelectorFilteringBranchesAndOpenPhaseProjection(t *testing.T) {
	read := EffortRead{Metadata: EffortMetadata{EffortID: "effort"}, Events: lifecycleBaseEvents()}
	other := "other"
	selected, ids, err := selectEffortEvents(read, Selector{EffortID: &other})
	if err != nil || len(selected) != 0 || len(ids) != 0 {
		t.Fatalf("effort mismatch = %#v %#v err=%v", selected, ids, err)
	}
	bad := "bad"
	if _, _, err := selectEffortEvents(read, Selector{Phase: &bad}); err == nil {
		t.Fatal("invalid selector accepted")
	}

	events := lifecycleBaseEvents()
	events[0].Timestamp = "2026-07-22T00:00:00Z"
	events = appendEvent(events, "open", "phase_started", PhaseStartedPayload{Phase: "planning"})
	events[len(events)-1].Timestamp = "2026-07-22T00:00:01Z"
	payload, _ := json.Marshal(ToolObservedPayload{Tool: "read", Outcome: "success", DurationMS: 1})
	events = append(events, EventEnvelope{Version: ProtocolVersion{Major: 1}, EventID: "inside", ObservationID: "inside", EffortID: "effort", SessionID: "other-session", Timestamp: "2026-07-22T00:00:02Z", Kind: "tool_observed", Predecessors: []string{"open"}, Payload: payload})
	phases := projectEventPhases(events)
	if !phases["inside"]["planning"] {
		t.Fatalf("open phase projection = %#v", phases)
	}

	phase, session := "planning", "session"
	since := time.Date(2026, 7, 22, 0, 0, 1, 0, time.UTC)
	until := since.Add(time.Second)
	selected, _, err = selectEffortEvents(EffortRead{Metadata: EffortMetadata{EffortID: "effort"}, Events: events}, Selector{SessionID: &session, Phase: &phase, Since: &since, Until: &until})
	if err != nil || len(selected) != 1 || selected[0].EventID != "open" {
		t.Fatalf("session/time/phase branches = %#v err=%v", selected, err)
	}
}

func TestProjectEventPhasesMalformedAndMissingReferences(t *testing.T) {
	malformed := json.RawMessage("{")
	events := []EventEnvelope{
		{EventID: "bad-start", Kind: "phase_started", Payload: malformed},
		{EventID: "bad-finish", Kind: "phase_finished", Payload: malformed},
		{EventID: "bad-usage", Kind: "usage_observed", Payload: malformed},
		causalEvent("missing-finish", "session", "phase_finished", nil, PhaseFinishedPayload{Phase: "planning", StartEventID: "missing"}),
	}
	phases := projectEventPhases(events)
	if len(phases) != len(events) || !phases["missing-finish"]["planning"] {
		t.Fatalf("malformed phase evidence projection = %#v", phases)
	}
}
