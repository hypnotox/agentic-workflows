package telemetry

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/effort"
)

type telemetryFailWriter struct{ err error }

func (w telemetryFailWriter) Write([]byte) (int, error) { return 0, w.err }

func TestExportSortsFiltersAndWrapsReadOnlyRecords(t *testing.T) {
	reads := ReadSet{
		Assignments: map[string]string{"session-b": "effort-a", "session-a": "effort-b"},
		Sessions: []SessionRead{
			{SessionID: "session-b", Records: []json.RawMessage{json.RawMessage(`{"record":"header","sessionId":"session-b"}`)}},
			{SessionID: "session-a", Records: []json.RawMessage{json.RawMessage(`{"record":"header","sessionId":"session-a"}`), json.RawMessage(`{"record":"observation"}`)}},
		},
		Legacy: []LegacyEffortRead{
			{EffortID: "effort-b", Records: []json.RawMessage{json.RawMessage(`{"kind":"usage_observed"}`)}},
			{EffortID: "effort-a", Records: []json.RawMessage{json.RawMessage(`not-json`)}},
		},
	}
	all, err := Export(reads, Selector{})
	if err != nil {
		t.Fatal(err)
	}
	var sources []string
	var firstIDs []string
	for _, raw := range all {
		var envelope struct {
			Source string         `json:"source"`
			Record map[string]any `json:"record"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			t.Fatal(err)
		}
		sources = append(sources, envelope.Source)
		if envelope.Source == "session-v1" && envelope.Record["sessionId"] != nil {
			firstIDs = append(firstIDs, envelope.Record["sessionId"].(string))
		}
	}
	if !reflect.DeepEqual(sources, []string{"session-v1", "session-v1", "session-v1", "legacy-protocol-2", "legacy-protocol-2"}) || !reflect.DeepEqual(firstIDs, []string{"session-a", "session-b"}) {
		t.Fatalf("export order sources=%v ids=%v", sources, firstIDs)
	}
	effortA, err := Export(reads, Selector{EffortID: selectorString("effort-a")})
	if err != nil || len(effortA) != 2 {
		t.Fatalf("effort filter = %q, %v", effortA, err)
	}
	sessionB, err := Export(reads, Selector{SessionID: selectorString("session-b")})
	if err != nil || len(sessionB) != 3 {
		t.Fatalf("session filter keeps legacy records = %q, %v", sessionB, err)
	}
	if _, err := Export(reads, Selector{SessionID: selectorString("../bad")}); err == nil {
		t.Fatal("Export accepted invalid selector")
	}
	var invalid struct {
		Source string `json:"source"`
		Record any    `json:"record"`
	}
	if err := json.Unmarshal(wrap("source", json.RawMessage(`broken`)), &invalid); err != nil || invalid.Source != "source" || invalid.Record != nil {
		t.Fatalf("invalid wrapped record = %#v, %v", invalid, err)
	}
	identified := ReadSet{Legacy: []LegacyEffortRead{{EffortID: "effort-a", Entries: []LegacyRecord{
		{Source: "legacy-protocol-1", SessionID: "session-a", Raw: json.RawMessage(`{"version":{"major":1},"timestamp":"2026-07-27T00:00:00Z"}`)},
		{SessionID: "session-b", Raw: json.RawMessage(`{"version":{"major":2},"timestamp":"bad"}`)},
	}}}}
	filtered, err := Export(identified, Selector{SessionID: selectorString("session-a"), Since: selectorTime("2026-07-27T00:00:00Z"), Until: selectorTime("2026-07-27T00:00:01Z")})
	if err != nil || len(filtered) != 1 || !bytes.Contains(filtered[0], []byte(`legacy-protocol-1`)) {
		t.Fatalf("identified legacy export=%q err=%v", filtered, err)
	}
	if string(mustJSON(map[string]string{"a": "b"})) != `{"a":"b"}` {
		t.Fatal("mustJSON changed a serializable value")
	}
}

func TestExportSelectorRejectsMalformedAndUnselectedEntries(t *testing.T) {
	session, other := "session-a", "session-b"
	reads := ReadSet{Sessions: []SessionRead{{SessionID: session, Records: []json.RawMessage{json.RawMessage(`{"record":"header"}`), json.RawMessage(`not-json`)}}}, Legacy: []LegacyEffortRead{{EffortID: "effort-a", Entries: []LegacyRecord{{SessionID: session, Raw: json.RawMessage(`{"timestamp":"bad"}`)}, {SessionID: other, Raw: json.RawMessage(`{"timestamp":"2026-07-27T00:00:00Z"}`)}, {SessionID: session, Raw: json.RawMessage(`{"version":{"major":2},"timestamp":"2026-07-27T00:00:00Z"}`)}}}}}
	if values, err := Export(reads, Selector{SessionID: &session, Since: selectorTime("2026-07-27T00:00:00Z")}); err != nil || len(values) != 2 {
		t.Fatalf("filtered export=%q err=%v", values, err)
	}
	if legacySource(json.RawMessage(`{"version":{"major":1}}`)) != "legacy-protocol-1" || legacySource(json.RawMessage(`{}`)) != "legacy-protocol-2" {
		t.Fatal("legacy source classification")
	}
}

func TestRenderHumanAndDoctorOutputErrors(t *testing.T) {
	id := "effort-a"
	report := Report{
		Efforts:  []EffortReport{{ID: id, Title: "A", State: effort.StateActive, Current: Counters{InputTokens: 1, OutputTokens: 2, CostUSD: 3, ToolFailures: 4, GatesFailed: 5, Subagents: 6, Handoffs: 7}, Legacy: Counters{InputTokens: 8, OutputTokens: 9, CostUSD: 10}}},
		Sessions: []SessionReport{{SessionID: "assigned", EffortID: &id, Counters: Counters{InputTokens: 11, OutputTokens: 12, CostUSD: 13}}, {SessionID: "free", Counters: Counters{InputTokens: 14, OutputTokens: 15, CostUSD: 16}}},
	}
	var out bytes.Buffer
	if err := RenderHuman(&out, report); err != nil {
		t.Fatal(err)
	}
	want := "effort effort-a title=\"A\" state=active\n  current input=1 output=2 cost=3 tool-failures=4 gates-failed=5 subagents=6 handoffs=7\n  legacy input=8 output=9 cost=10\nsession assigned effort=effort-a input=11 output=12 cost=13\nsession free effort=unassigned input=14 output=15 cost=16\n"
	if out.String() != want {
		t.Fatalf("human report = %q, want %q", out.String(), want)
	}
	failure := errors.New("write failed")
	if err := RenderHuman(telemetryFailWriter{failure}, report); !errors.Is(err, failure) {
		t.Fatalf("RenderHuman effort error = %v", err)
	}
	if err := RenderHuman(telemetryFailWriter{failure}, Report{Sessions: report.Sessions}); !errors.Is(err, failure) {
		t.Fatalf("RenderHuman session error = %v", err)
	}
	out.Reset()
	doctor := DoctorReport{Findings: []IntegrityFinding{{Source: "legacy", SessionID: "b", Code: "bad"}, {Source: "session-v1", SessionID: "a", Code: "broken"}}}
	if err := RenderDoctorHuman(&out, doctor); err != nil {
		t.Fatal(err)
	}
	if out.String() != "legacy session=b code=bad\nsession-v1 session=a code=broken\n" {
		t.Fatalf("doctor human = %q", out.String())
	}
	if err := RenderDoctorHuman(telemetryFailWriter{failure}, doctor); !errors.Is(err, failure) {
		t.Fatalf("RenderDoctorHuman error = %v", err)
	}
}

func TestSelectorsHonorBoundarySemantics(t *testing.T) {
	parsed, err := ParseSelectorTime("2026-07-27T00:00:00.123456789Z")
	if err != nil || parsed.Nanosecond() != 123456789 {
		t.Fatalf("ParseSelectorTime = %v, %v", parsed, err)
	}
	if _, err := ParseSelectorTime("yesterday"); err == nil {
		t.Fatal("accepted non-RFC3339 time")
	}
	start := selectorTime("2026-07-27T00:00:00Z")
	end := selectorTime("2026-07-27T00:00:01Z")
	for _, selector := range []Selector{
		{},
		{EffortID: selectorString("effort")},
		{SessionID: selectorString("session")},
		{Since: start, Until: end},
	} {
		if err := ValidateSelector(selector); err != nil {
			t.Errorf("ValidateSelector(%#v): %v", selector, err)
		}
	}
	for _, selector := range []Selector{
		{EffortID: selectorString("../effort")},
		{SessionID: selectorString("../session")},
		{Since: start, Until: start},
		{Since: end, Until: start},
	} {
		if err := ValidateSelector(selector); err == nil {
			t.Errorf("ValidateSelector accepted %#v", selector)
		}
	}
	observation := Observation{Timestamp: *start}
	if !selectObservation(observation, Selector{Since: start, Until: end}) {
		t.Fatal("since must be inclusive")
	}
	observation.Timestamp = *end
	if selectObservation(observation, Selector{Since: start, Until: end}) {
		t.Fatal("until must be exclusive")
	}
	before := start.Add(-time.Nanosecond)
	observation.Timestamp = before
	if selectObservation(observation, Selector{Since: start}) {
		t.Fatal("selected observation before since")
	}
	observation.Timestamp = end.Add(time.Nanosecond)
	if selectObservation(observation, Selector{Until: end}) {
		t.Fatal("selected observation at or after until")
	}
}
