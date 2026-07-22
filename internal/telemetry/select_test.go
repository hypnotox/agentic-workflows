package telemetry

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSelectorCombinationsAndTimeEdges(t *testing.T) {
	since, err := ParseSelectorTime("2026-07-22T00:00:01Z")
	if err != nil {
		t.Fatal(err)
	}
	until, err := ParseSelectorTime("2026-07-22T00:00:03.000000001Z")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseSelectorTime("yesterday"); err == nil {
		t.Fatal("non-RFC3339 selector time accepted")
	}
	phase := "implementation"
	effortID, sessionID := "effort", "session"
	selector := Selector{EffortID: &effortID, SessionID: &sessionID, Phase: &phase, Since: &since, Until: &until}
	if err := ValidateSelector(selector); err != nil {
		t.Fatal(err)
	}

	events := lifecycleBaseEvents()
	events[0].Timestamp = "2026-07-22T00:00:00Z"
	events = appendEvent(events, "start", "phase_started", PhaseStartedPayload{Phase: "implementation"})
	events[len(events)-1].Timestamp = "2026-07-22T00:00:01Z"
	payload, _ := json.Marshal(ToolObservedPayload{Tool: "read", Outcome: "success", DurationMS: 1})
	inside := EventEnvelope{Version: ProtocolVersion{Major: 1}, EventID: "inside", ObservationID: "inside-observation", EffortID: effortID, SessionID: sessionID, Timestamp: "2026-07-22T00:00:03Z", Kind: "tool_observed", Predecessors: []string{"start"}, Payload: payload}
	events = append(events, inside)
	events = appendEvent(events, "finish", "phase_finished", PhaseFinishedPayload{Phase: "implementation", StartEventID: "start"})
	events[len(events)-1].Timestamp = "2026-07-22T00:00:03.000000001Z"
	read := EffortRead{Metadata: EffortMetadata{EffortID: effortID}, Events: events}
	selected, ids, err := selectEffortEvents(read, selector)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 2 || !ids["start"] || !ids["inside"] || ids["finish"] {
		t.Fatalf("inclusive since/exclusive until AND selection = %#v", ids)
	}

	equal := since
	badPhase, badID := "not-a-phase", "../unsafe"
	for name, invalid := range map[string]Selector{
		"equal window":   {Since: &since, Until: &equal},
		"unknown phase":  {Phase: &badPhase},
		"unsafe effort":  {EffortID: &badID},
		"unsafe session": {SessionID: &badID},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateSelector(invalid); err == nil {
				t.Fatal("invalid selector accepted")
			}
		})
	}

	later := since.Add(time.Nanosecond)
	if err := ValidateSelector(Selector{Since: &since, Until: &later}); err != nil {
		t.Fatal(err)
	}
}
