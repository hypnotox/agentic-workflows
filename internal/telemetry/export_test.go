package telemetry

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadAllEffortsAndNormalizedExport(t *testing.T) {
	ledger, firstMetadata, first := createTestEffort(t)
	secondMetadata := firstMetadata
	secondMetadata.EffortID = "a-effort"
	var secondEvent map[string]any
	if err := json.Unmarshal(first, &secondEvent); err != nil {
		t.Fatal(err)
	}
	secondEvent["effortId"] = secondMetadata.EffortID
	secondEvent["eventId"] = "a-event"
	secondEvent["idempotencyKey"] = "a-key"
	second := mustJSON(t, secondEvent)
	if _, err := ledger.CreateEffort(secondMetadata, second); err != nil {
		t.Fatal(err)
	}
	reads, err := ledger.ReadAllEfforts()
	if err != nil {
		t.Fatal(err)
	}
	if len(reads) != 2 || reads[0].Metadata.EffortID != "a-effort" || reads[1].Metadata.EffortID != firstMetadata.EffortID {
		t.Fatalf("stable effort reads = %#v", reads)
	}
	selected := firstMetadata.EffortID
	lines, err := SelectNormalizedEvents(reads, Selector{EffortID: &selected})
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || !json.Valid(lines[0]) || strings.Contains(string(lines[0]), "Raw") {
		t.Fatalf("normalized export = %q", lines)
	}
	var event EventEnvelope
	if err := json.Unmarshal(lines[0], &event); err != nil || event.EffortID != selected {
		t.Fatalf("normalized event = %#v, %v", event, err)
	}
}

func TestNormalizedExportOrdersSessionsAndLines(t *testing.T) {
	events := []EventEnvelope{
		causalEvent("z-event", "z-session", "effort_created", []string{}, EffortCreatedPayload{CreationMode: "independent"}),
		{Version: ProtocolVersion{Major: 2}, EventID: "a-later", ObservationID: "observation-later", EffortID: "effort", SessionID: "a-session", Timestamp: "2026-07-22T00:00:02Z", Kind: "tool_observed", Predecessors: []string{}, Payload: json.RawMessage(`{"tool":"read","outcome":"success","durationMs":2}`)},
		{Version: ProtocolVersion{Major: 2}, EventID: "a-first", ObservationID: "observation-first", EffortID: "effort", SessionID: "a-session", Timestamp: "2026-07-22T00:00:01Z", Kind: "tool_observed", Predecessors: []string{}, Payload: json.RawMessage(`{"tool":"read","outcome":"success","durationMs":1}`)},
	}
	read := EffortRead{Metadata: EffortMetadata{EffortID: "effort"}, Events: events, Records: []LedgerRecord{
		{SessionID: "z-session", Line: 1, Event: &events[0]},
		{SessionID: "a-session", Line: 2, Event: &events[1]},
		{SessionID: "a-session", Line: 1, Event: &events[2]},
	}}
	lines, err := SelectNormalizedEvents([]EffortRead{read}, Selector{})
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(lines))
	for _, line := range lines {
		var event EventEnvelope
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatal(err)
		}
		got = append(got, event.EventID)
	}
	if strings.Join(got, ",") != "a-first,a-later,z-event" {
		t.Fatalf("normalized order = %v", got)
	}
}

func TestRenderDoctorHumanDeterministicAndWriteFailures(t *testing.T) {
	observed := 3.0
	result := DoctorResult{
		SchemaVersion: 1, ProtocolMajor: 2, GeneratedAt: time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC),
		Findings: []Finding{{
			Code: "WFH1-TEST", Type: "heuristic", Severity: "warning", Scope: "effort", Confidence: "high", Waived: false,
			Evidence:       FindingEvidence{EventIDs: []string{"event"}, CounterIDs: []string{}, ObservedValue: &observed, Unit: "count"},
			Threshold:      &FindingThreshold{Kind: "absolute", Comparator: "gte", Value: 2, Unit: "count"},
			Baseline:       &FindingBaseline{Route: "direct", RuleVersion: 1, SampleCount: 10, Percentile: 95, Value: 2.5, Unit: "count"},
			Reconciliation: &ReconciliationProposal{Kind: "correct-phase", SourceEventIDs: []string{"event"}, Replacement: RepairReplacement{EventKind: "phase_started", Payload: json.RawMessage(`{"phase":"implementation"}`)}},
			Explanation:    "explanation", NextAction: "next",
		}},
		Integrity: []IntegrityNotice{{Code: "clock", Severity: "warning", Scope: "session", EventIDs: []string{"event"}, Explanation: "clock issue"}},
	}
	var output bytes.Buffer
	if err := RenderDoctorHuman(&output, result); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"workflow doctor schema 1", "observed=3count", "threshold kind=absolute", "baseline route=direct", "reconciliation kind=correct-phase", "integrity clock"} {
		if !strings.Contains(output.String(), text) {
			t.Errorf("doctor human missing %q: %s", text, output.String())
		}
	}
	for failAt := range 9 {
		writer := &failAtWriter{failAt: failAt}
		if err := RenderDoctorHuman(writer, result); err == nil {
			t.Fatalf("write failure %d was ignored", failAt)
		}
	}
}

func TestNormalizedExportRejectsFatalIntegrity(t *testing.T) {
	read := EffortRead{Metadata: EffortMetadata{EffortID: "effort"}, Integrity: []IntegrityIssue{{Code: "unsupported-protocol", Scope: "session"}}}
	if _, err := SelectNormalizedEvents([]EffortRead{read}, Selector{}); err == nil || !strings.Contains(err.Error(), "unsupported-protocol") {
		t.Fatalf("fatal integrity export error = %v", err)
	}
	if issue, fatal := fatalExportIntegrity([]IntegrityIssue{{Code: "partial-final-line"}}); fatal || issue.Code != "" {
		t.Fatalf("reportable final partial line became fatal: %#v, %v", issue, fatal)
	}
}

func TestReadAllEffortsRejectsUnsafeEntryAndExportSelector(t *testing.T) {
	ledger := newRetentionLedger(t)
	if err := os.WriteFile(filepath.Join(ledger.paths.efforts, "unsafe"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.ReadAllEfforts(); err == nil {
		t.Fatal("unsafe effort entry accepted")
	}
	bad := "unknown"
	if _, err := SelectNormalizedEvents(nil, Selector{Phase: &bad}); err == nil {
		t.Fatal("invalid export selector accepted")
	}
}
