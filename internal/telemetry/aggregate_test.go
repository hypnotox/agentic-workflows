package telemetry

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/effort"
)

func selectorString(value string) *string { return &value }

func selectorTime(value string) *time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		panic(err)
	}
	return &parsed
}

// invariant: tooling/workflow-telemetry:canonical-projections-and-diagnostics
func TestAggregateEmpty(t *testing.T) {
	report, err := Aggregate(ReadSet{Records: map[string]effort.Record{}}, Selector{})
	if err != nil || len(report.Efforts) != 0 {
		t.Fatalf("%+v %v", report, err)
	}
	if report.SchemaVersion != SchemaVersion || report.Efforts == nil {
		t.Fatalf("empty report = %#v", report)
	}
}

func TestAggregateAssignedUnassignedSelectorsAndLegacy(t *testing.T) {
	assigned := "effort-a"
	unassigned := "session-free"
	reads := ReadSet{
		Records: map[string]effort.Record{
			"effort-b": {ID: "effort-b", Title: "B", State: effort.StateCompleted},
			"effort-a": {ID: "effort-a", Title: "A", State: effort.StateActive},
		},
		Assignments: map[string]string{"session-a": assigned},
		Sessions: []SessionRead{
			{SessionID: "session-a", Observations: []Observation{
				mustObservation(t, "123e4567-e89b-42d3-a456-426614174001", "2026-07-27T00:00:00Z", "usage", `{"inputTokens":2,"outputTokens":3,"cacheReadTokens":4,"cacheWriteTokens":5,"costUsd":1.25}`),
				mustObservation(t, "123e4567-e89b-42d3-a456-426614174002", "2026-07-27T00:00:01Z", "tool", `{"tool":"go","outcome":"failure","durationMs":6}`),
			}},
			{SessionID: unassigned, Observations: []Observation{
				mustObservation(t, "123e4567-e89b-42d3-a456-426614174003", "2026-07-27T00:00:02Z", "usage", `{"inputTokens":10,"outputTokens":10,"cacheReadTokens":10,"cacheWriteTokens":10,"costUsd":2}`),
			}},
		},
		Legacy: []LegacyEffortRead{{EffortID: assigned, Records: []json.RawMessage{
			json.RawMessage(`{"kind":"usage_observed","payload":{"inputTokens":7,"outputTokens":8,"cacheReadTokens":9,"cacheWriteTokens":10,"costUsd":3.5}}`),
			json.RawMessage(`not json`),
			json.RawMessage(`{"kind":"other","payload":{}}`),
		}}},
	}
	report, err := Aggregate(reads, Selector{})
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{report.Efforts[0].ID, report.Efforts[1].ID}; !reflect.DeepEqual(got, []string{"effort-a", "effort-b"}) {
		t.Fatalf("effort order = %v", got)
	}
	got := report.Efforts[0]
	if got.Current != (Counters{InputTokens: 2, OutputTokens: 3, CacheReadTokens: 4, CacheWriteTokens: 5, CostUSD: 1.25, ToolFailures: 1, DurationMS: 6}) {
		t.Fatalf("assigned counters = %#v", got.Current)
	}
	if got.Legacy != (Counters{InputTokens: 7, OutputTokens: 8, CacheReadTokens: 9, CacheWriteTokens: 10, CostUSD: 3.5}) {
		t.Fatalf("legacy counters = %#v", got.Legacy)
	}
	if report.Efforts[1].Current != (Counters{}) || len(report.Sessions) != 0 {
		t.Fatalf("unselected report should not expose unassigned sessions: %#v", report)
	}

	sessionReport, err := Aggregate(reads, Selector{SessionID: &unassigned})
	if err != nil {
		t.Fatal(err)
	}
	if len(sessionReport.Efforts) != 0 || len(sessionReport.Sessions) != 1 || sessionReport.Sessions[0].EffortID != nil || sessionReport.Sessions[0].Counters.InputTokens != 10 {
		t.Fatalf("unassigned session report = %#v", sessionReport)
	}
	assignedReport, err := Aggregate(reads, Selector{SessionID: selectorString("session-a"), EffortID: &assigned})
	if err != nil {
		t.Fatal(err)
	}
	if len(assignedReport.Efforts) != 1 || assignedReport.Efforts[0].ID != assigned || len(assignedReport.Sessions) != 1 || assignedReport.Sessions[0].EffortID == nil || *assignedReport.Sessions[0].EffortID != assigned {
		t.Fatalf("assigned session report = %#v", assignedReport)
	}
	missing, err := Aggregate(reads, Selector{SessionID: selectorString("session-missing")})
	if err != nil || len(missing.Sessions) != 1 || missing.Sessions[0].EffortID != nil || missing.Sessions[0].Counters != (Counters{}) {
		t.Fatalf("missing session report = %#v, %v", missing, err)
	}
	filtered, err := Aggregate(reads, Selector{EffortID: &assigned, Since: selectorTime("2026-07-27T00:00:01Z"), Until: selectorTime("2026-07-27T00:00:02Z")})
	if err != nil || len(filtered.Efforts) != 1 || filtered.Efforts[0].Current != (Counters{ToolFailures: 1, DurationMS: 6}) {
		t.Fatalf("time-filtered report = %#v, %v", filtered, err)
	}
	if _, err := Aggregate(reads, Selector{EffortID: selectorString("../bad")}); err == nil {
		t.Fatal("Aggregate accepted invalid selector")
	}
}

func TestAggregateLegacySelectorsUseProtocolEntriesAndTime(t *testing.T) {
	id, session := "effort-a", "session-a"
	reads := ReadSet{Records: map[string]effort.Record{id: {ID: id}}, Assignments: map[string]string{session: id}, Legacy: []LegacyEffortRead{{EffortID: id, Entries: []LegacyRecord{
		{Source: "legacy-protocol-1", SessionID: session, Raw: json.RawMessage(`{"kind":"usage_observed","timestamp":"2026-07-27T00:00:00Z","payload":{"inputTokens":2}}`)},
		{Source: "legacy-protocol-2", SessionID: "other", Raw: json.RawMessage(`{"kind":"usage_observed","timestamp":"not-a-time","payload":{"inputTokens":9}}`)},
	}}}}
	report, err := Aggregate(reads, Selector{SessionID: &session, Since: selectorTime("2026-07-27T00:00:00Z"), Until: selectorTime("2026-07-27T00:00:01Z")})
	if err != nil || len(report.Efforts) != 1 || report.Efforts[0].Legacy.InputTokens != 2 {
		t.Fatalf("legacy selector report=%#v err=%v", report, err)
	}
	other := "other-effort"
	if report, err := Aggregate(reads, Selector{EffortID: &other}); err != nil || len(report.Efforts) != 0 {
		t.Fatalf("mismatched effort selector=%#v err=%v", report, err)
	}
}

func TestCountersSaturateAndObservationKinds(t *testing.T) {
	all := Counters{
		InputTokens: math.MaxUint64, OutputTokens: math.MaxUint64, CacheReadTokens: math.MaxUint64, CacheWriteTokens: math.MaxUint64,
		ToolSuccesses: math.MaxUint64, ToolFailures: math.MaxUint64, ToolCancelled: math.MaxUint64,
		GatesPassed: math.MaxUint64, GatesFailed: math.MaxUint64, GatesCancelled: math.MaxUint64,
		Subagents: math.MaxUint64, Compactions: math.MaxUint64, Handoffs: math.MaxUint64, DurationMS: math.MaxUint64,
		CostUSD: 1,
	}
	if got := addCounters(all, Counters{InputTokens: 1, OutputTokens: 1, CacheReadTokens: 1, CacheWriteTokens: 1, ToolSuccesses: 1, ToolFailures: 1, ToolCancelled: 1, GatesPassed: 1, GatesFailed: 1, GatesCancelled: 1, Subagents: 1, Compactions: 1, Handoffs: 1, DurationMS: 1, CostUSD: 2}); got != (Counters{InputTokens: math.MaxUint64, OutputTokens: math.MaxUint64, CacheReadTokens: math.MaxUint64, CacheWriteTokens: math.MaxUint64, ToolSuccesses: math.MaxUint64, ToolFailures: math.MaxUint64, ToolCancelled: math.MaxUint64, GatesPassed: math.MaxUint64, GatesFailed: math.MaxUint64, GatesCancelled: math.MaxUint64, Subagents: math.MaxUint64, Compactions: math.MaxUint64, Handoffs: math.MaxUint64, DurationMS: math.MaxUint64, CostUSD: 3}) {
		t.Fatalf("saturated counters = %#v", got)
	}
	if sat(2, 3) != 5 || sat(math.MaxUint64, 1) != math.MaxUint64 {
		t.Fatal("unexpected saturation arithmetic")
	}
	var counters Counters
	for _, observation := range []Observation{
		{Kind: "usage", Payload: json.RawMessage(`{"inputTokens":1,"outputTokens":2,"cacheReadTokens":3,"cacheWriteTokens":4,"costUsd":5.5}`)},
		{Kind: "tool", Payload: json.RawMessage(`{"outcome":"success","durationMs":6}`)},
		{Kind: "tool", Payload: json.RawMessage(`{"outcome":"failure","durationMs":7}`)},
		{Kind: "tool", Payload: json.RawMessage(`{"outcome":"cancelled","durationMs":8}`)},
		{Kind: "gate", Payload: json.RawMessage(`{"outcome":"success","durationMs":9}`)},
		{Kind: "gate", Payload: json.RawMessage(`{"outcome":"failure","durationMs":10}`)},
		{Kind: "gate", Payload: json.RawMessage(`{"outcome":"cancelled","durationMs":11}`)},
		{Kind: "subagent", Payload: json.RawMessage(`{"durationMs":12}`)},
		{Kind: "compaction", Payload: json.RawMessage(`{}`)},
		{Kind: "handoff", Payload: json.RawMessage(`{"durationMs":13}`)},
		{Kind: "unknown", Payload: json.RawMessage(`{}`)},
		{Kind: "usage", Payload: json.RawMessage(`broken`)},
	} {
		addObservation(&counters, observation)
	}
	want := Counters{InputTokens: 1, OutputTokens: 2, CacheReadTokens: 3, CacheWriteTokens: 4, CostUSD: 5.5, ToolSuccesses: 1, ToolFailures: 1, ToolCancelled: 1, GatesPassed: 1, GatesFailed: 1, GatesCancelled: 1, Subagents: 1, Compactions: 1, Handoffs: 1, DurationMS: 76}
	if counters != want {
		t.Fatalf("kind counters = %#v, want %#v", counters, want)
	}
	addLegacy(&counters, json.RawMessage(`{"kind":"usage_observed","payload":{"inputTokens":2,"outputTokens":3,"cacheReadTokens":4,"cacheWriteTokens":5,"costUsd":6.5}}`))
	addLegacy(&counters, json.RawMessage(`{"kind":"usage_observed","payload":null}`))
	addLegacy(&counters, json.RawMessage(`broken`))
	addLegacy(&counters, json.RawMessage(`{"kind":"other","payload":{}}`))
	if counters.InputTokens != 3 || counters.OutputTokens != 5 || counters.CacheReadTokens != 7 || counters.CacheWriteTokens != 9 || counters.CostUSD != 12 {
		t.Fatalf("legacy addition = %#v", counters)
	}
}
