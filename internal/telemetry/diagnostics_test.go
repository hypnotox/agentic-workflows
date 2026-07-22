package telemetry

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// invariant: tooling/workflow-telemetry:canonical-projections-and-diagnostics
func TestCanonicalProjectionsAndDiagnostics(t *testing.T) {
	badTransition := lifecycleBaseEvents()
	badTransition = appendEvent(badTransition, "bad-change", "route_changed", RoutePayload{Route: "direct"})

	missingRouteEvidence := lifecycleBaseEvents()
	missingRouteEvidence = appendEvent(missingRouteEvidence, "route", "route_selected", RoutePayload{Route: "adr-plan"})
	missingRouteEvidence = appendEvent(missingRouteEvidence, "incomplete", "effort_completed", EffortTerminalPayload{TerminalEpoch: 1})

	overlap := lifecycleBaseEvents()
	overlap = appendEvent(overlap, "outer-start", "phase_started", PhaseStartedPayload{Phase: "brainstorming"})
	overlap = appendEvent(overlap, "inner-start", "phase_started", PhaseStartedPayload{Phase: "implementation"})
	overlap = appendEvent(overlap, "inner-finish", "phase_finished", PhaseFinishedPayload{Phase: "implementation", StartEventID: "inner-start"})
	overlap = appendEvent(overlap, "outer-finish", "phase_finished", PhaseFinishedPayload{Phase: "brainstorming", StartEventID: "outer-start"})

	concurrent := lifecycleBaseEvents()
	concurrent = appendEvent(concurrent, "route-a", "route_selected", RoutePayload{Route: "direct"})
	concurrent[len(concurrent)-1].Predecessors = []string{"create"}
	concurrent = appendEvent(concurrent, "route-b", "route_selected", RoutePayload{Route: "plan"})
	concurrent[len(concurrent)-1].Predecessors = []string{"create"}
	concurrent[len(concurrent)-1].SessionID = "other-session"

	clock := lifecycleBaseEvents()
	clock = appendFinishedPhase(clock, "clock", "brainstorming", "")
	clock[len(clock)-2].Timestamp = "2026-07-22T00:00:02Z"
	clock[len(clock)-1].Timestamp = "2026-07-22T00:00:01Z"

	handoff := lifecycleBaseEvents()
	handoffPayload, _ := json.Marshal(HandoffObservedPayload{Outcome: "success", TargetSessionID: "child-session"})
	handoff = append(handoff, EventEnvelope{Version: ProtocolVersion{Major: 1}, EventID: "handoff", ObservationID: "handoff-observation", EffortID: "effort", SessionID: "session", Timestamp: "2026-07-22T00:00:01Z", Kind: "handoff_observed", Predecessors: []string{"create"}, Payload: handoffPayload})

	reads := []EffortRead{
		diagnosticRead(badTransition),
		diagnosticRead(missingRouteEvidence),
		diagnosticRead(overlap),
		diagnosticRead(concurrent),
		diagnosticRead(clock),
		diagnosticRead(handoff),
		{Metadata: EffortMetadata{EffortID: "effort", CheckpointID: "checkpoint.md"}, Events: lifecycleBaseEvents(), Integrity: []IntegrityIssue{
			{Code: "malformed-complete-line", Scope: "session", Line: 2, EventIDs: []string{"available"}},
			{Code: "unsupported-protocol", Scope: "session", Line: 3, EventIDs: []string{}},
		}},
	}
	result, err := DiagnoseExact(reads, Selector{}, time.Date(2026, 7, 22, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	severity := map[string]string{
		"WFV1-LIFECYCLE-TRANSITION": "violation", "WFV1-PHASE-ORDER": "violation", "WFV1-ADR-REVIEW": "violation",
		"WFV1-PLAN-REVIEW": "violation", "WFV1-ADR-PLAN-RESYNC": "violation", "WFV1-IMPLEMENTATION-REVIEW": "violation",
		"WFV1-RETROSPECTIVE": "violation", "WFV1-PHASE-OVERLAP": "violation", "WFV1-CONCURRENT-STATE": "violation",
		"WFV1-HANDOFF-ASSOCIATION": "warning", "WFV1-EVENT-INTEGRITY": "violation", "WFV1-SCHEMA-COMPATIBILITY": "violation",
		"WFV1-CLOCK-INTEGRITY": "warning",
	}
	scopeFragment := map[string]string{
		"WFV1-LIFECYCLE-TRANSITION": "effort:", "WFV1-PHASE-ORDER": ":epoch:", "WFV1-ADR-REVIEW": ":epoch:",
		"WFV1-PLAN-REVIEW": ":epoch:", "WFV1-ADR-PLAN-RESYNC": ":epoch:", "WFV1-IMPLEMENTATION-REVIEW": ":epoch:",
		"WFV1-RETROSPECTIVE": ":epoch:", "WFV1-PHASE-OVERLAP": ":trajectory:", "WFV1-CONCURRENT-STATE": "effort:",
		"WFV1-HANDOFF-ASSOCIATION": ":session-link:", "WFV1-EVENT-INTEGRITY": "session", "WFV1-SCHEMA-COMPATIBILITY": "session",
		"WFV1-CLOCK-INTEGRITY": ":interval:",
	}
	got := map[string]bool{}
	for _, finding := range result.Findings {
		got[finding.Code] = true
		if finding.Type != "exact" || finding.Confidence != "certain" || finding.Severity != severity[finding.Code] || finding.Waived || finding.Evidence.EventIDs == nil || finding.Evidence.CounterIDs == nil || !sort.StringsAreSorted(finding.Evidence.EventIDs) || !strings.Contains(finding.Scope, scopeFragment[finding.Code]) || finding.Explanation == "" || finding.NextAction == "" {
			t.Fatalf("invalid production exact finding contract: %#v", finding)
		}
		if len([]byte(finding.Scope)) > descriptor.Limits.CategoryBytes {
			t.Fatalf("unbounded finding scope: %#v", finding)
		}
		for _, counterID := range finding.Evidence.CounterIDs {
			if len([]byte(counterID)) > descriptor.Limits.CategoryBytes {
				t.Fatalf("unbounded structured evidence: %#v", finding)
			}
		}
		if finding.Code != "WFV1-SCHEMA-COMPATIBILITY" && len(finding.Evidence.EventIDs) == 0 {
			t.Fatalf("finding omitted event evidence: %#v", finding)
		}
	}
	for _, code := range descriptor.Vocabularies["diagnosticRuleCodes"] {
		if !got[code] {
			t.Errorf("positive case did not produce %s: %#v", code, result.Findings)
		}
	}
	if finding := findingByCode(t, result.Findings, "WFV1-EVENT-INTEGRITY"); !equalStringSets(finding.Evidence.EventIDs, []string{"available"}) || !equalStringSets(finding.Evidence.CounterIDs, []string{"line:2"}) {
		t.Fatalf("event-integrity evidence contract = %#v", finding.Evidence)
	}
	if finding := findingByCode(t, result.Findings, "WFV1-SCHEMA-COMPATIBILITY"); !equalStringSets(finding.Evidence.CounterIDs, []string{"line:3"}) {
		t.Fatalf("schema evidence contract = %#v", finding.Evidence)
	}
	for rule, reasons := range descriptor.WaiverRules {
		if len(reasons) != 1 || severity[rule] == "" {
			t.Fatalf("exact waiver eligibility is not closed for %s: %#v", rule, reasons)
		}
	}

	for route := range routeRequirements {
		negative, diagnoseErr := DiagnoseExact([]EffortRead{diagnosticRead(completedRoute(route))}, Selector{}, time.Time{})
		if diagnoseErr != nil || len(negative.Findings) != 0 {
			t.Errorf("valid %s route produced findings %#v, err %v", route, negative.Findings, diagnoseErr)
		}
	}
	validHandoff := validHandoffRead()
	negative, err := DiagnoseExact([]EffortRead{validHandoff}, Selector{}, time.Time{})
	if err != nil || hasFindingCode(negative.Findings, "WFV1-HANDOFF-ASSOCIATION") {
		t.Fatalf("validated handoff produced finding %#v, err %v", negative.Findings, err)
	}
}

func TestDiagnosticsWaiversRequireExactEligibleMatch(t *testing.T) {
	events := lifecycleBaseEvents()
	events = appendEvent(events, "route", "route_selected", RoutePayload{Route: "direct"})
	for index, phase := range []Phase{"brainstorming", "implementation", "implementation-review"} {
		events = appendFinishedPhase(events, "phase-"+uintString(uint64(index)), phase, "")
	}
	events = appendEvent(events, "complete", "effort_completed", EffortTerminalPayload{TerminalEpoch: 1})
	read := diagnosticRead(events)
	initial, err := DiagnoseExact([]EffortRead{read}, Selector{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	retrospective := findingByCode(t, initial.Findings, "WFV1-RETROSPECTIVE")

	waivedEvents := append([]EventEnvelope(nil), events...)
	waivedEvents = appendEvent(waivedEvents, "wrong-scope", "finding_waived", FindingWaivedPayload{RuleCode: "WFV1-RETROSPECTIVE", Scope: "wrong", EvidenceIDs: retrospective.Evidence.EventIDs, ReasonCode: "approved-route-deviation"})
	waivedEvents = appendEvent(waivedEvents, "wrong-evidence", "finding_waived", FindingWaivedPayload{RuleCode: "WFV1-RETROSPECTIVE", Scope: BoundedCategory(retrospective.Scope), EvidenceIDs: []string{"route"}, ReasonCode: "approved-route-deviation"})
	waivedEvents = appendEvent(waivedEvents, "exact-waiver", "finding_waived", FindingWaivedPayload{RuleCode: "WFV1-RETROSPECTIVE", Scope: BoundedCategory(retrospective.Scope), EvidenceIDs: retrospective.Evidence.EventIDs, ReasonCode: "approved-route-deviation"})
	waived, err := DiagnoseExact([]EffortRead{diagnosticRead(waivedEvents)}, Selector{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	finding := findingByCode(t, waived.Findings, "WFV1-RETROSPECTIVE")
	if !finding.Waived || finding.Severity != "informational" {
		t.Fatalf("exact eligible waiver did not change presentation: %#v", finding)
	}
	for _, candidate := range waived.Findings {
		if candidate.Code != "WFV1-RETROSPECTIVE" && candidate.Waived {
			t.Fatalf("waiver changed another finding: %#v", candidate)
		}
	}

	unwaivable := EffortRead{Metadata: EffortMetadata{EffortID: "effort"}, Events: appendEvent(lifecycleBaseEvents(), "ineligible-waiver", "finding_waived", FindingWaivedPayload{RuleCode: "WFV1-EVENT-INTEGRITY", Scope: "session", EvidenceIDs: []string{"bad"}, ReasonCode: "approved-route-deviation"}), Integrity: []IntegrityIssue{{Code: "malformed-complete-line", Scope: "session", EventIDs: []string{"bad"}}}}
	unwaived, err := DiagnoseExact([]EffortRead{unwaivable}, Selector{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if finding := findingByCode(t, unwaived.Findings, "WFV1-EVENT-INTEGRITY"); finding.Waived || finding.Severity != "violation" {
		t.Fatalf("ineligible waiver changed event integrity: %#v", finding)
	}
}

func TestDiagnosticsTypedRepairsCompatibilitySortingAndReadOnly(t *testing.T) {
	events := lifecycleBaseEvents()
	events = appendEvent(events, "bad-change", "route_changed", RoutePayload{Route: "direct"})
	events[0].Version.Minor = 1 // A compatible same-major event remains interpretable.
	read := diagnosticRead(events)
	before, _ := json.Marshal(read)
	result, err := DiagnoseExact([]EffortRead{read}, Selector{}, time.Date(2026, 7, 22, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(read)
	if string(before) != string(after) {
		t.Fatal("read-only diagnosis mutated its ledger input")
	}
	transition := findingByCode(t, result.Findings, "WFV1-LIFECYCLE-TRANSITION")
	if transition.Reconciliation == nil || transition.Reconciliation.Kind != "supersede-event" || transition.Reconciliation.Replacement.EventKind != "route_selected" || !reflect.DeepEqual(transition.Reconciliation.SourceEventIDs, []string{"bad-change"}) {
		t.Fatalf("typed route remediation = %#v", transition.Reconciliation)
	}
	if hasFindingCode(result.Findings, "WFV1-SCHEMA-COMPATIBILITY") {
		t.Fatal("compatible minor produced a schema finding")
	}

	incompatible := read
	incompatible.Integrity = []IntegrityIssue{{Code: "unsupported-protocol", Scope: "session"}}
	first, err := DiagnoseExact([]EffortRead{incompatible}, Selector{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := DiagnoseExact([]EffortRead{incompatible}, Selector{}, first.GeneratedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !hasFindingCode(first.Findings, "WFV1-SCHEMA-COMPATIBILITY") || !reflect.DeepEqual(first, second) {
		t.Fatalf("incompatible/stable diagnosis = first %#v second %#v", first, second)
	}
	for index := 1; index < len(first.Findings); index++ {
		left := first.Findings[index-1].Code + first.Findings[index-1].Scope
		right := first.Findings[index].Code + first.Findings[index].Scope
		if left > right {
			t.Fatalf("findings are not stable-sorted: %#v", first.Findings)
		}
	}
}

func TestDiagnosticsSelectorsAndCausalOrderingOnly(t *testing.T) {
	concurrent := lifecycleBaseEvents()
	concurrent = appendEvent(concurrent, "route", "route_selected", RoutePayload{Route: "direct"})
	concurrent = appendEvent(concurrent, "phase-a", "phase_started", PhaseStartedPayload{Phase: "brainstorming"})
	concurrent[len(concurrent)-1].SessionID = "a"
	concurrent[len(concurrent)-1].Predecessors = []string{"route"}
	concurrent = appendEvent(concurrent, "phase-b", "phase_started", PhaseStartedPayload{Phase: "implementation"})
	concurrent[len(concurrent)-1].SessionID = "b"
	concurrent[len(concurrent)-1].Predecessors = []string{"route"}
	concurrent[len(concurrent)-2].Timestamp = "2026-07-22T02:00:00Z"
	concurrent[len(concurrent)-1].Timestamp = "2026-07-22T01:00:00Z"
	read := diagnosticRead(concurrent)
	result, err := DiagnoseExact([]EffortRead{read}, Selector{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if hasFindingCode(result.Findings, "WFV1-PHASE-OVERLAP") {
		t.Fatal("wall-clock order invented a causal phase overlap")
	}
	if !hasFindingCode(result.Findings, "WFV1-CONCURRENT-STATE") {
		t.Fatal("concurrent state was not diagnosed separately")
	}

	session := "a"
	filtered, err := DiagnoseExact([]EffortRead{read}, Selector{SessionID: &session}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range filtered.Findings {
		matched := false
		for _, eventID := range finding.Evidence.EventIDs {
			matched = matched || eventID == "phase-a"
		}
		if !matched {
			t.Fatalf("selector retained unrelated finding %#v", finding)
		}
	}
}

func diagnosticRead(events []EventEnvelope) EffortRead {
	return EffortRead{Metadata: EffortMetadata{EffortID: "effort", CreatedAt: "2026-07-22T00:00:00Z", CheckpointID: "checkpoint.md", CreationMode: "independent"}, Events: events}
}

func validHandoffRead() EffortRead {
	events := lifecycleBaseEvents()
	events = appendEvent(events, "trajectory", "trajectory_started", TrajectoryPayload{TrajectoryID: "trajectory", AnchorID: "anchor"})
	events[len(events)-1].TrajectoryID = "trajectory"
	handoffPayload, _ := json.Marshal(HandoffObservedPayload{Outcome: "success", TargetSessionID: "child-session"})
	events = append(events, EventEnvelope{Version: ProtocolVersion{Major: 1}, EventID: "handoff", ObservationID: "handoff-observation", EffortID: "effort", SessionID: "session", TrajectoryID: "trajectory", Timestamp: "2026-07-22T00:00:01Z", Kind: "handoff_observed", Predecessors: []string{"trajectory"}, Payload: handoffPayload})
	association := causalEvent("association", "child-session", "session_associated", []string{"handoff"}, SessionAssociatedPayload{AssociationOrigin: "handoff", TrajectoryID: "trajectory", HandoffEventID: "handoff"})
	association.TrajectoryID = "trajectory"
	events = append(events, association)
	return diagnosticRead(events)
}

func hasFindingCode(findings []Finding, code string) bool {
	for _, finding := range findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}

func findingByCode(t *testing.T, findings []Finding, code string) Finding {
	t.Helper()
	for _, finding := range findings {
		if finding.Code == code {
			return finding
		}
	}
	t.Fatalf("finding %s absent from %#v", code, findings)
	return Finding{}
}
