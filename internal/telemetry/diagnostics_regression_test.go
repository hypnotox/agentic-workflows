package telemetry

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDiagnosticsRouteHistoryRejectedEffectsAndConcreteRemediation(t *testing.T) {
	phasesBeforeRoute := lifecycleBaseEvents()
	phasesBeforeRoute = appendFinishedPhase(phasesBeforeRoute, "brainstorm", "brainstorming", "")
	phasesBeforeRoute = appendEvent(phasesBeforeRoute, "late-route", "route_selected", RoutePayload{Route: "direct"})
	for _, phase := range []Phase{"implementation", "implementation-review", "retrospective"} {
		phasesBeforeRoute = appendFinishedPhase(phasesBeforeRoute, "later-"+string(phase), phase, "")
	}
	phasesBeforeRoute = appendEvent(phasesBeforeRoute, "complete", "effort_completed", EffortTerminalPayload{TerminalEpoch: 1})
	result, err := DiagnoseExact([]EffortRead{diagnosticRead(phasesBeforeRoute)}, Selector{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if hasFindingCode(result.Findings, "WFV1-PHASE-ORDER") {
		t.Fatalf("phase before route selection was rejected: %#v", result.Findings)
	}

	beforeRouteChange := completedRoute("direct")
	beforeRouteChange = beforeRouteChange[:len(beforeRouteChange)-1]
	beforeRouteChange = appendEvent(beforeRouteChange, "late-change", "route_changed", RoutePayload{Route: "bugfix"})
	beforeRouteChange = appendEvent(beforeRouteChange, "changed-complete", "effort_completed", EffortTerminalPayload{TerminalEpoch: 1})
	result, err = DiagnoseExact([]EffortRead{diagnosticRead(beforeRouteChange)}, Selector{}, time.Time{})
	if err != nil || hasFindingCode(result.Findings, "WFV1-PHASE-ORDER") {
		t.Fatalf("full history before route change was rejected: %#v, %v", result.Findings, err)
	}

	rejected := completedRoute("direct")
	read := diagnosticRead(rejected)
	read.RejectedEffects = map[string]bool{"c-start": true, "c-finish": true}
	result, err = DiagnoseExact([]EffortRead{read}, Selector{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if !hasFindingCode(result.Findings, "WFV1-PHASE-ORDER") || !hasFindingCode(result.Findings, "WFV1-IMPLEMENTATION-REVIEW") {
		t.Fatalf("rejected phase effects satisfied completion diagnostics: %#v", result.Findings)
	}
	if finding := findingByCode(t, result.Findings, "WFV1-PHASE-ORDER"); finding.Reconciliation != nil {
		t.Fatalf("non-corrective identical phase remediation was advertised: %#v", finding.Reconciliation)
	}

	superseded := completedRoute("direct")
	replacement, _ := json.Marshal(PhaseStartedPayload{Phase: "planning"})
	superseded = appendEvent(superseded, "repair-review-start", "repair_applied", RepairAppliedPayload{
		ProposalKind: "correct-phase", SourceEventIDs: []string{"c-start"},
		Replacement: RepairReplacement{EventKind: "phase_started", Payload: replacement},
	})
	result, err = DiagnoseExact([]EffortRead{diagnosticRead(superseded)}, Selector{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if !hasFindingCode(result.Findings, "WFV1-PHASE-ORDER") || !hasFindingCode(result.Findings, "WFV1-IMPLEMENTATION-REVIEW") {
		t.Fatalf("repair-superseded phase evidence satisfied completion: %#v", result.Findings)
	}
}

func TestDiagnosticsEffortSelectorAndHandoffTrajectoryCopy(t *testing.T) {
	selected := renameDiagnosticEffort(diagnosticRead(lifecycleBaseEvents()), "selected")
	brokenEvents := lifecycleBaseEvents()
	handoffPayload, _ := json.Marshal(HandoffObservedPayload{Outcome: "success", TargetSessionID: "child"})
	brokenEvents = append(brokenEvents, EventEnvelope{Version: ProtocolVersion{Major: 2}, EventID: "broken-handoff", ObservationID: "broken-observation", EffortID: "other", SessionID: "session", TrajectoryID: "source", Timestamp: "2026-07-22T00:00:01Z", Kind: "handoff_observed", Predecessors: []string{"create"}, Payload: handoffPayload})
	other := renameDiagnosticEffort(diagnosticRead(brokenEvents), "other")
	effortID := "selected"
	result, err := DiagnoseExact([]EffortRead{selected, other}, Selector{EffortID: &effortID}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if hasFindingCode(result.Findings, "WFV1-HANDOFF-ASSOCIATION") {
		t.Fatalf("effort selector retained another effort's handoff: %#v", result.Findings)
	}

	events := lifecycleBaseEvents()
	events = appendEvent(events, "source-trajectory", "trajectory_started", TrajectoryPayload{TrajectoryID: "source", AnchorID: "source-anchor"})
	events[len(events)-1].TrajectoryID = "source"
	events = appendEvent(events, "wrong-trajectory", "trajectory_started", TrajectoryPayload{TrajectoryID: "wrong", AnchorID: "wrong-anchor"})
	events[len(events)-1].TrajectoryID = "wrong"
	events = append(events, EventEnvelope{Version: ProtocolVersion{Major: 2}, EventID: "handoff", ObservationID: "handoff-observation", EffortID: "effort", SessionID: "session", TrajectoryID: "source", Timestamp: "2026-07-22T00:00:01Z", Kind: "handoff_observed", Predecessors: []string{"wrong-trajectory"}, Payload: handoffPayload})
	association := causalEvent("association", "child", "session_associated", []string{"handoff"}, SessionAssociatedPayload{AssociationOrigin: "handoff", TrajectoryID: "wrong", HandoffEventID: "handoff"})
	association.TrajectoryID = "wrong"
	events = append(events, association)
	result, err = DiagnoseExact([]EffortRead{diagnosticRead(events)}, Selector{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if !hasFindingCode(result.Findings, "WFV1-HANDOFF-ASSOCIATION") {
		t.Fatalf("wrong-trajectory association accepted as a copy: %#v", result.Findings)
	}
}

func TestDiagnosticsCompatibilityEvidenceBranches(t *testing.T) {
	incompatible := EffortRead{Metadata: EffortMetadata{EffortID: "incompatible"}, Integrity: []IntegrityIssue{{Code: "unsupported-protocol"}}}
	if findings := diagnoseEffort(incompatible, ProjectWorkflow(incompatible)); len(findings) != 0 {
		t.Fatalf("incompatible effort produced findings: %#v", findings)
	}
	if _, ok := findingFromIntegrity(IntegrityIssue{Code: "informational"}, nil, "effort"); ok {
		t.Fatal("unknown integrity issue produced an exact finding")
	}

	finding, ok := findingFromIntegrity(IntegrityIssue{Code: "unsupported-protocol", Scope: "stream", Line: 7}, nil, "effort")
	if !ok {
		t.Fatal("unsupported protocol did not map to a finding")
	}
	read := EffortRead{Records: []LedgerRecord{
		{SessionID: "other", Line: 7, Raw: json.RawMessage(`{"version":{"major":8,"minor":1}}`)},
		{SessionID: "stream", Line: 6, Raw: json.RawMessage(`{"version":{"major":8,"minor":2}}`)},
		{SessionID: "stream", Line: 7, Raw: json.RawMessage(`not-json`)},
		{SessionID: "stream", Line: 7, Raw: json.RawMessage(`{"version":{"major":9,"minor":3}}`)},
	}}
	enriched := enrichIntegrityEvidence(finding, IntegrityIssue{Code: "unsupported-protocol", Scope: "stream", Line: 7}, read, nil)
	if !equalStringSets(enriched.Evidence.CounterIDs, []string{"line:7", "protocol:9.3"}) {
		t.Fatalf("compatibility evidence = %#v", enriched.Evidence)
	}
}

func TestDiagnosticsExactEvidenceContract(t *testing.T) {
	bad := lifecycleBaseEvents()
	bad = appendEvent(bad, "bad-change", "route_changed", RoutePayload{Route: "direct"})
	read := diagnosticRead(bad)
	unsupportedRaw := json.RawMessage(`{"version":{"major":2,"minor":3}}`)
	incompatible := EffortRead{
		Metadata:  EffortMetadata{EffortID: "incompatible"},
		Records:   []LedgerRecord{{SessionID: "other-stream", Line: 1, Raw: unsupportedRaw}, {SessionID: "stream", Line: 6, Raw: unsupportedRaw}, {SessionID: "stream", Line: 7, Raw: unsupportedRaw}},
		Integrity: []IntegrityIssue{{Code: "unsupported-protocol", Scope: "stream", Line: 7}},
	}
	result, err := DiagnoseExact([]EffortRead{read, incompatible}, Selector{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	transition := findingByCode(t, result.Findings, "WFV1-LIFECYCLE-TRANSITION")
	if !equalStringSets(transition.Evidence.EventIDs, []string{"bad-change", "create"}) {
		t.Fatalf("transition omits frontier evidence: %#v", transition.Evidence)
	}
	if hasFindingCode(result.Findings, "WFV1-SCHEMA-COMPATIBILITY") {
		t.Fatalf("incompatible effort emitted a diagnostic finding: %#v", result.Findings)
	}
	compatibilityNotices := 0
	for _, notice := range result.Integrity {
		if notice.Code == "unsupported-protocol" {
			compatibilityNotices++
			if notice.Scope != "incompatible" || len(notice.EventIDs) != 0 {
				t.Fatalf("incompatible effort emitted an unbounded integrity notice: %#v", notice)
			}
		}
	}
	if compatibilityNotices != 1 {
		t.Fatalf("incompatible effort did not emit exactly one compatibility notice: %#v", result.Integrity)
	}

	clock := lifecycleBaseEvents()
	clock = appendFinishedPhase(clock, "clock", "planning", "")
	clock[len(clock)-2].Timestamp = "2026-07-22T00:00:02Z"
	clock[len(clock)-1].Timestamp = "2026-07-22T00:00:01Z"
	clockResult, err := DiagnoseExact([]EffortRead{diagnosticRead(clock)}, Selector{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	clockFinding := findingByCode(t, clockResult.Findings, "WFV1-CLOCK-INTEGRITY")
	if len(clockFinding.Evidence.EventIDs) != 2 || len(clockFinding.Evidence.CounterIDs) != 2 {
		t.Fatalf("clock omits endpoint/timestamp evidence: %#v", clockFinding.Evidence)
	}

	severity := map[string]string{
		"WFV1-LIFECYCLE-TRANSITION": "violation", "WFV1-PHASE-ORDER": "violation", "WFV1-ADR-REVIEW": "violation",
		"WFV1-PLAN-REVIEW": "violation", "WFV1-ADR-PLAN-RESYNC": "violation", "WFV1-IMPLEMENTATION-REVIEW": "violation",
		"WFV1-RETROSPECTIVE": "violation", "WFV1-PHASE-OVERLAP": "violation", "WFV1-CONCURRENT-STATE": "violation",
		"WFV1-HANDOFF-ASSOCIATION": "warning", "WFV1-EVENT-INTEGRITY": "violation", "WFV1-SCHEMA-COMPATIBILITY": "violation",
		"WFV1-CLOCK-INTEGRITY": "warning",
	}
	for code, want := range severity {
		finding := exactFinding(code, want, "scope", []string{"event"}, "explanation", "next")
		if finding.Severity != want || finding.Type != "exact" || finding.Confidence != "certain" || finding.Scope != "scope" || len(finding.Evidence.EventIDs) != 1 || finding.Explanation == "" || finding.NextAction == "" {
			t.Fatalf("%s exact field contract = %#v", code, finding)
		}
	}
}

func renameDiagnosticEffort(read EffortRead, effortID string) EffortRead {
	read.Metadata.EffortID = effortID
	for index := range read.Events {
		read.Events[index].EffortID = effortID
	}
	return read
}
