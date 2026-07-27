package telemetry

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

func TestSelectedEffortRejectsBroadUnknownIncompatibleAndEmpty(t *testing.T) {
	events := lifecycleBaseEvents()
	read := EffortRead{Metadata: EffortMetadata{EffortID: "effort", CreatedAt: "2026-07-22T00:00:00Z"}, Events: events}
	if _, err := SelectEffort([]EffortRead{read}, Selector{}); !errors.Is(err, ErrSelectedEffortRequired) {
		t.Fatalf("broad selection error = %v", err)
	}
	invalidPhase := "invalid"
	if _, err := SelectEffort([]EffortRead{read}, Selector{Phase: &invalidPhase}); err == nil {
		t.Fatal("invalid selector accepted")
	}
	unknown := "unknown"
	if _, err := SelectEffort([]EffortRead{read}, Selector{EffortID: &unknown}); !errors.Is(err, ErrSelectedEffortUnknown) {
		t.Fatalf("unknown selection error = %v", err)
	}
	effort := "effort"
	incompatible := read
	incompatible.Integrity = []IntegrityIssue{{Code: "unsupported-protocol"}}
	if _, err := SelectEffort([]EffortRead{incompatible}, Selector{EffortID: &effort}); !errors.Is(err, ErrSelectedEffortIncompatible) {
		t.Fatalf("incompatible selection error = %v", err)
	}
	session := "other"
	if _, err := SelectEffort([]EffortRead{read}, Selector{EffortID: &effort, SessionID: &session}); !errors.Is(err, ErrSelectedEffortEmpty) {
		t.Fatalf("empty selection error = %v", err)
	}
}

func TestSelectedEffortAdaptersPreserveOneEffortProjection(t *testing.T) {
	events := lifecycleBaseEvents()
	read := EffortRead{Metadata: EffortMetadata{EffortID: "effort", CreatedAt: "2026-07-22T00:00:00Z"}, Events: events}
	effort := "effort"
	metrics, err := AggregateSelectedMetrics([]EffortRead{read}, Selector{EffortID: &effort}, MetricsOptions{})
	if err != nil || len(metrics.Efforts) != 1 || metrics.Efforts[0].EffortID != effort {
		t.Fatalf("metrics = %#v, %v", metrics, err)
	}
	if _, err := AggregateSelectedMetrics([]EffortRead{read}, Selector{}, MetricsOptions{}); !errors.Is(err, ErrSelectedEffortRequired) {
		t.Fatalf("broad aggregate error = %v", err)
	}
	if _, err := DiagnoseSelected([]EffortRead{read}, Selector{}, HeuristicOptions{}, time.Time{}); !errors.Is(err, ErrSelectedEffortRequired) {
		t.Fatalf("broad diagnosis error = %v", err)
	}
	doctor, err := DiagnoseSelected([]EffortRead{read}, Selector{EffortID: &effort}, HeuristicOptions{}, time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC))
	if err != nil || doctor.Selector.EffortID == nil || *doctor.Selector.EffortID != effort {
		t.Fatalf("doctor = %#v, %v", doctor, err)
	}
}

func TestListEffortsOrdersAndValidatesCursorBoundary(t *testing.T) {
	olderEvents := lifecycleBaseEvents()
	older := EffortRead{Metadata: EffortMetadata{EffortID: "z", CreatedAt: "2026-07-21T00:00:00Z"}, Events: olderEvents, Records: []LedgerRecord{{Applied: true, Event: &olderEvents[0]}}}
	newAEvents := lifecycleBaseEvents()
	newA := EffortRead{Metadata: EffortMetadata{EffortID: "a", CreatedAt: "2026-07-22T00:00:00Z"}, Events: newAEvents, Records: []LedgerRecord{{Applied: true, Event: &newAEvents[0]}}}
	newB := EffortRead{Metadata: EffortMetadata{EffortID: "b", CreatedAt: "2026-07-22T00:00:00Z"}, Integrity: []IntegrityIssue{{Code: "unsupported-protocol"}}}
	if empty, err := ListEfforts([]EffortRead{}, DefaultEffortPageLimit, ""); err != nil || len(empty.Efforts) != 0 {
		t.Fatalf("default empty page = %#v, %v", empty, err)
	}
	page, err := ListEfforts([]EffortRead{older, newB, newA}, 2, "")
	if err != nil || len(page.Efforts) != 2 || page.Efforts[0].EffortID != "a" || page.Efforts[1].EffortID != "b" || !page.Efforts[1].Incompatible || page.NextCursor == "" {
		t.Fatalf("first page = %#v, %v", page, err)
	}
	next, err := ListEfforts([]EffortRead{older, newB, newA}, 2, page.NextCursor)
	if err != nil || len(next.Efforts) != 1 || next.Efforts[0].EffortID != "z" || next.NextCursor != "" {
		t.Fatalf("next page = %#v, %v", next, err)
	}
	if _, err := ListEfforts([]EffortRead{older, newA}, 2, page.NextCursor); err == nil {
		t.Fatal("deleted cursor boundary accepted")
	}
	terminalEvents := appendEvent(lifecycleBaseEvents(), "abandon", "effort_abandoned", EffortTerminalPayload{TerminalEpoch: 1})
	terminal, err := ListEfforts([]EffortRead{{Metadata: EffortMetadata{EffortID: "terminal", CreatedAt: "2026-07-20T00:00:00Z"}, Events: terminalEvents}}, 1, "")
	if err != nil || len(terminal.Efforts) != 1 || terminal.Efforts[0].Outcome != "abandoned" || terminal.Efforts[0].Discovery {
		t.Fatalf("terminal list row = %#v, %v", terminal, err)
	}
	for _, limit := range []int{0, -1, 101} {
		if _, err := ListEfforts([]EffortRead{older}, limit, ""); err == nil {
			t.Fatalf("invalid limit %d accepted", limit)
		}
	}
	if _, err := ListEfforts([]EffortRead{older}, 1, "not-a-cursor"); err == nil {
		t.Fatal("malformed cursor accepted")
	}
	if _, err := ListEfforts([]EffortRead{{Metadata: EffortMetadata{EffortID: "bad", CreatedAt: "not-time"}}}, 1, ""); err == nil {
		t.Fatal("invalid creation time accepted")
	}
	if _, err := decodeEffortCursor("eyJ2IjoyLCJjcmVhdGVkQXQiOiIyMDI2LTA3LTIyVDAwOjAwOjAwWiIsImVmZm9ydElkIjoiYSJ9"); err == nil {
		t.Fatal("unsupported cursor accepted")
	}
	if _, err := decodeEffortCursor("eyJ2IjoxLCJjcmVhdGVkQXQiOiIyMDI2LTA3LTIyVDAwOjAwOjAwWiIsImVmZm9ydElkIjoiYSJ9IA"); err == nil {
		t.Fatal("noncanonical cursor accepted")
	}
	if _, err := decodeEffortCursor("!"); err == nil {
		t.Fatal("undecodable cursor accepted")
	}
	if _, err := decodeEffortCursor("eyJ2IjoxLCJjcmVhdGVkQXQiOiJiYWQiLCJlZmZvcnRJZCI6ImEifQ"); err == nil {
		t.Fatal("invalid cursor timestamp accepted")
	}
	if currentListPhase(LifecycleProjection{}) != "" || currentListPhase(LifecycleProjection{OpenPhases: map[string]PhaseInterval{"b": {Phase: "implementation"}}}) != "implementation" {
		t.Fatal("current phase projection is not deterministic")
	}
	invalid := EventEnvelope{Timestamp: "invalid"}
	if lastAppliedAt(EffortRead{Records: []LedgerRecord{{Applied: false}, {Applied: true, Event: &invalid}}}) != nil {
		t.Fatal("invalid or unapplied events contributed a timestamp")
	}
}

func TestRenderEffortListHuman(t *testing.T) {
	created := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	applied := created.Add(time.Second)
	page := EffortListPage{Efforts: []EffortListRow{
		{EffortID: "compatible", CreatedAt: created, LastAppliedAt: &applied, State: "open", Route: "direct", Phase: "implementation"},
		{EffortID: "incompatible", CreatedAt: created, Incompatible: true},
	}, NextCursor: "cursor"}
	var output bytes.Buffer
	if err := RenderEffortListHuman(&output, page); err != nil || output.String() == "" {
		t.Fatalf("list render = %q, %v", output.String(), err)
	}
	for failAt := range 3 {
		if err := RenderEffortListHuman(&failAtWriter{failAt: failAt}, page); err == nil {
			t.Fatalf("write failure %d was ignored", failAt)
		}
	}
}
