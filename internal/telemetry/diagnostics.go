package telemetry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

const maximumLinkedInterval = time.Duration(1<<63 - 1)

// DiagnoseExact evaluates the closed WFV1 exact rule set without mutating the
// ledger. Callers may combine its stable result with separately evaluated
// heuristic findings.
func DiagnoseExact(reads []EffortRead, selector Selector, generatedAt time.Time) (DoctorResult, error) {
	if err := ValidateSelector(selector); err != nil {
		return DoctorResult{}, err
	}
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	result := DoctorResult{
		SchemaVersion: 1,
		ProtocolMajor: int(descriptor.Version.Major),
		GeneratedAt:   generatedAt,
		Selector:      selector,
		Findings:      []Finding{},
		Integrity:     []IntegrityNotice{},
	}
	selectedByEffort := make(map[string]map[string]bool, len(reads))
	for _, read := range reads {
		_, selected, err := selectEffortEvents(read, selector)
		if err != nil { // coverage-ignore: selector validation above is authoritative
			return DoctorResult{}, err
		}
		if selector.EffortID != nil && read.Metadata.EffortID != *selector.EffortID {
			continue
		}
		selectedByEffort[read.Metadata.EffortID] = selected
		workflow := ProjectWorkflow(read)
		result.Integrity = append(result.Integrity, integrityNotices(workflow.Integrity, selected)...)
		findings := diagnoseEffort(read, workflow)
		for _, finding := range findings {
			if findingMatchesSelection(finding, selected, selector) {
				result.Findings = append(result.Findings, applyExactWaiver(finding, workflow.Lifecycle.Waivers))
			}
		}
	}
	for _, finding := range diagnoseHandoffs(reads) {
		if selector.EffortID != nil && !findingBelongsToEffort(finding, reads, *selector.EffortID) {
			continue
		}
		selected := selectedForFinding(finding, selectedByEffort)
		if findingMatchesSelection(finding, selected, selector) {
			workflow := lifecycleForFinding(reads, finding)
			result.Findings = append(result.Findings, applyExactWaiver(finding, workflow.Waivers))
		}
	}
	result.Findings = stableFindings(result.Findings)
	result.Integrity = stableIntegrityNotices(result.Integrity)
	return result, nil
}

func diagnoseEffort(read EffortRead, workflow WorkflowProjection) []Finding {
	findings := []Finding{}
	byID := eventsByID(read.Events)
	order, _ := BuildCausalOrder(read.Events)
	for _, issue := range workflow.Integrity {
		finding, ok := findingFromIntegrity(issue, byID, read.Metadata.EffortID)
		if ok {
			finding = enrichIntegrityEvidence(finding, issue, read, byID)
			findings = append(findings, finding)
		}
	}
	findings = append(findings, diagnosePhaseOverlap(read, order)...)
	findings = append(findings, diagnoseClockIntegrity(read, order)...)
	findings = append(findings, diagnoseTerminalRoutes(read, workflow, order)...)
	return stableFindings(findings)
}

func findingFromIntegrity(issue IntegrityIssue, byID map[string]EventEnvelope, effortID string) (Finding, bool) {
	evidence := append([]string(nil), issue.EventIDs...)
	sort.Strings(evidence)
	scope := issue.Scope
	if scope == "" {
		scope = effortID
	}
	switch issue.Code {
	case "invalid-transition":
		finding := exactFinding("WFV1-LIFECYCLE-TRANSITION", "violation", effortScope(effortID), evidence,
			"a lifecycle event has a source state or effect outside the closed transition table",
			"append a typed repair that corrects or supersedes the rejected lifecycle evidence")
		finding.Reconciliation = transitionRepair(evidence, byID)
		return finding, true
	case "concurrent-state":
		finding := exactFinding("WFV1-CONCURRENT-STATE", "violation", effortScope(effortID), evidence,
			"competing lifecycle mutations share a frontier and have no causal order",
			"append a typed repair selecting or superseding the competing evidence")
		finding.Reconciliation = concurrentRepair(evidence, byID)
		return finding, true
	case "unsupported-protocol":
		return exactFinding("WFV1-SCHEMA-COMPATIBILITY", "violation", scope, evidence,
			"the stream requires an unsupported protocol interpretation",
			"use a compatible awf binary or perform an explicit protocol migration"), true
	case "partial-final-line":
		return Finding{}, false
	}
	if strings.Contains(issue.Code, "clock") {
		return exactFinding("WFV1-CLOCK-INTEGRITY", "warning", scope, evidence,
			"a causally linked interval has an invalid clock duration",
			"verify the endpoint clocks; the interval is excluded from duration baselines"), true
	}
	if isEventIntegrityCode(issue.Code) {
		return exactFinding("WFV1-EVENT-INTEGRITY", "violation", scope, evidence,
			"the ledger contains malformed, conflicting, unsafe, invalid, or causally broken evidence",
			"append a typed repair when possible, otherwise use explicit confirmed cleanup"), true
	}
	return Finding{}, false
}

func enrichIntegrityEvidence(finding Finding, issue IntegrityIssue, read EffortRead, byID map[string]EventEnvelope) Finding {
	counterIDs := append([]string(nil), finding.Evidence.CounterIDs...)
	if issue.Line > 0 {
		counterIDs = append(counterIDs, "line:"+uintString(uint64(issue.Line)))
	}
	if finding.Code == "WFV1-LIFECYCLE-TRANSITION" {
		for _, eventID := range issue.EventIDs {
			if event, exists := byID[eventID]; exists {
				finding.Evidence.EventIDs = append(finding.Evidence.EventIDs, event.Predecessors...)
			}
		}
	}
	if finding.Code == "WFV1-SCHEMA-COMPATIBILITY" {
		for _, record := range read.Records {
			if record.SessionID != issue.Scope || issue.Line > 0 && record.Line != issue.Line {
				continue
			}
			var header struct {
				Version ProtocolVersion `json:"version"`
			}
			if json.Unmarshal(record.Raw, &header) == nil {
				counterIDs = append(counterIDs, "protocol:"+uintString(uint64(header.Version.Major))+"."+uintString(uint64(header.Version.Minor)))
			}
		}
	}
	finding.Evidence.EventIDs = sortedUnique(finding.Evidence.EventIDs)
	finding.Evidence.CounterIDs = sortedUnique(counterIDs)
	return finding
}

func isEventIntegrityCode(code string) bool {
	switch code {
	case "malformed-complete-line", "conflicting-duplicate", "unsafe-stream-entry", "unsafe-stream-identifier", "unsafe-stream-path", "stream-identity-mismatch", "broken-predecessor", "causal-cycle", "ambiguous-anchor", "misplaced-creation-event", "creation-metadata-mismatch", "duplicate-creation-event", "missing-or-duplicate-creation-event":
		return true
	default:
		return false
	}
}

func diagnoseTerminalRoutes(read EffortRead, workflow WorkflowProjection, order *CausalOrder) []Finding {
	result := []Finding{}
	for _, terminal := range read.Events {
		if terminal.Kind != "effort_completed" {
			continue
		}
		var payload EffortTerminalPayload
		if json.Unmarshal(terminal.Payload, &payload) != nil {
			continue
		}
		route, routeEventID := effectiveRouteBefore(read.Events, workflow.Lifecycle.EffectApplied, terminal, order)
		if route == "" {
			continue
		}
		scope := terminalEpochScope(read.Metadata.EffortID, payload.TerminalEpoch)
		intervals := completedIntervalsBefore(read.Events, workflow.Lifecycle.EffectApplied, payload.TerminalEpoch, terminal.EventID, order)
		required := routeRequirements[route]
		chainOK, chainEvidence := requiredPhaseChain(required, intervals, routeEventID, terminal.EventID, order)
		if !chainOK {
			finding := exactFinding("WFV1-PHASE-ORDER", "violation", scope, chainEvidence,
				"the causally ordered phase sequence does not satisfy the effective route",
				"append the missing ordered phase evidence or apply a typed phase repair")
			result = append(result, finding)
		}
		baseEvidence := []string{routeEventID, terminal.EventID}
		if route == "adr" || route == "adr-plan" {
			if evidence, fresh := freshPhaseEvidence("adr-authoring", "adr-review", intervals, terminal.EventID, order); !fresh {
				result = append(result, exactFinding("WFV1-ADR-REVIEW", "violation", scope, append(baseEvidence, evidence...),
					"ADR authoring required by the route has no later fresh ADR review",
					"record a fresh ADR review or append a typed correction to the review evidence"))
			}
		}
		if route == "plan" || route == "adr-plan" {
			if evidence, fresh := freshPhaseEvidence("planning", "plan-review", intervals, terminal.EventID, order); !fresh {
				result = append(result, exactFinding("WFV1-PLAN-REVIEW", "violation", scope, append(baseEvidence, evidence...),
					"planning required by the route has no later fresh plan review",
					"record a fresh plan review or append a typed correction to the review evidence"))
			}
		}
		if route == "adr-plan" {
			if evidence, fresh := freshResyncEvidence(intervals, terminal.EventID, order); !fresh {
				result = append(result, exactFinding("WFV1-ADR-PLAN-RESYNC", "violation", scope, append(baseEvidence, evidence...),
					"the adr-plan route has no resync after the final ADR and plan reviews",
					"record a fresh ADR-plan resync or append a typed correction to its evidence"))
			}
		}
		if route != "investigation-only" {
			if evidence, fresh := freshPhaseEvidence("implementation", "implementation-review", intervals, terminal.EventID, order); !fresh {
				result = append(result, exactFinding("WFV1-IMPLEMENTATION-REVIEW", "violation", scope, append(baseEvidence, evidence...),
					"implementation has no later fresh implementation review before completion",
					"record a fresh implementation review or append a typed correction to the review evidence"))
			}
		}
		if evidence, present := phaseEvidenceBefore("retrospective", intervals, terminal.EventID, order); !present {
			result = append(result, exactFinding("WFV1-RETROSPECTIVE", "violation", scope, append(baseEvidence, evidence...),
				"the effective route lacks its required retrospective before completion",
				"record the retrospective or approve the narrowly scoped route deviation"))
		}
	}
	return result
}

func effectiveRouteBefore(events []EventEnvelope, applied map[string]bool, terminal EventEnvelope, order *CausalOrder) (Route, string) {
	var candidates []EventEnvelope
	for _, event := range events {
		if (event.Kind == "route_selected" || event.Kind == "route_changed") && applied[event.EventID] && order.HappensBefore(event.EventID, terminal.EventID) {
			candidates = append(candidates, event)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if order.HappensBefore(candidates[i].EventID, candidates[j].EventID) {
			return true
		}
		if order.HappensBefore(candidates[j].EventID, candidates[i].EventID) {
			return false
		}
		return candidates[i].EventID < candidates[j].EventID
	})
	if len(candidates) == 0 {
		return "", ""
	}
	selected := candidates[len(candidates)-1]
	var payload RoutePayload
	_ = json.Unmarshal(selected.Payload, &payload)
	return payload.Route, selected.EventID
}

func completedIntervalsBefore(events []EventEnvelope, applied map[string]bool, epoch uint64, terminalID string, order *CausalOrder) []PhaseInterval {
	starts := map[string]EventEnvelope{}
	for _, event := range events {
		if event.Kind == "phase_started" {
			starts[event.EventID] = event
		}
	}
	result := []PhaseInterval{}
	for _, finish := range events {
		if finish.Kind != "phase_finished" || !applied[finish.EventID] || !order.HappensBefore(finish.EventID, terminalID) {
			continue
		}
		var payload PhaseFinishedPayload
		if json.Unmarshal(finish.Payload, &payload) != nil {
			continue
		}
		start, ok := starts[payload.StartEventID]
		if !ok || !applied[start.EventID] || !order.HappensBefore(start.EventID, finish.EventID) {
			continue
		}
		var startPayload PhaseStartedPayload
		if json.Unmarshal(start.Payload, &startPayload) != nil || startPayload.Phase != payload.Phase {
			continue
		}
		// Epoch one is the initial history. Each reopen starts the next epoch;
		// count causally prior reopen events to assign interval membership.
		intervalEpoch := uint64(1)
		for _, event := range events {
			if event.Kind == "effort_reopened" && order.HappensBefore(event.EventID, start.EventID) {
				var reopen EffortReopenedPayload
				if json.Unmarshal(event.Payload, &reopen) == nil && reopen.TerminalEpoch > intervalEpoch {
					intervalEpoch = reopen.TerminalEpoch
				}
			}
		}
		if intervalEpoch == epoch {
			result = append(result, PhaseInterval{Phase: payload.Phase, StartEventID: start.EventID, FinishEventID: finish.EventID, TrajectoryID: start.TrajectoryID, TerminalEpoch: epoch})
		}
	}
	return result
}

func requiredPhaseChain(required []Phase, intervals []PhaseInterval, routeEventID, terminalID string, order *CausalOrder) (bool, []string) {
	evidence := []string{routeEventID, terminalID}
	last := ""
	for _, phase := range required {
		candidates := phaseIntervals(phase, intervals)
		chosen := PhaseInterval{}
		for _, candidate := range candidates {
			evidence = append(evidence, candidate.StartEventID, candidate.FinishEventID)
			if (last == "" || order.HappensBefore(last, candidate.StartEventID)) && order.HappensBefore(candidate.FinishEventID, terminalID) && chosen.FinishEventID == "" {
				chosen = candidate
			}
		}
		if chosen.FinishEventID == "" {
			return false, sortedUnique(evidence)
		}
		last = chosen.FinishEventID
	}
	return true, sortedUnique(evidence)
}

func freshPhaseEvidence(upstream, downstream Phase, intervals []PhaseInterval, terminalID string, order *CausalOrder) ([]string, bool) {
	upstreamIDs := maximalPhaseFinishes(upstream, intervals, terminalID, order)
	downstreamIntervals := phaseIntervals(downstream, intervals)
	evidence := append([]string(nil), upstreamIDs...)
	for _, interval := range downstreamIntervals {
		evidence = append(evidence, interval.StartEventID, interval.FinishEventID)
		fresh := len(upstreamIDs) != 0
		for _, upstreamID := range upstreamIDs {
			fresh = fresh && order.HappensBefore(upstreamID, interval.StartEventID)
		}
		if fresh && order.HappensBefore(interval.FinishEventID, terminalID) {
			return sortedUnique(evidence), true
		}
	}
	return sortedUnique(evidence), false
}

func freshResyncEvidence(intervals []PhaseInterval, terminalID string, order *CausalOrder) ([]string, bool) {
	upstream := append(maximalPhaseFinishes("adr-review", intervals, terminalID, order), maximalPhaseFinishes("plan-review", intervals, terminalID, order)...)
	evidence := append([]string(nil), upstream...)
	for _, interval := range phaseIntervals("adr-plan-resync", intervals) {
		evidence = append(evidence, interval.StartEventID, interval.FinishEventID)
		fresh := len(upstream) == 2
		for _, eventID := range upstream {
			fresh = fresh && order.HappensBefore(eventID, interval.StartEventID)
		}
		if fresh && order.HappensBefore(interval.FinishEventID, terminalID) {
			return sortedUnique(evidence), true
		}
	}
	return sortedUnique(evidence), false
}

func phaseEvidenceBefore(phase Phase, intervals []PhaseInterval, terminalID string, order *CausalOrder) ([]string, bool) {
	evidence := []string{}
	for _, interval := range phaseIntervals(phase, intervals) {
		evidence = append(evidence, interval.StartEventID, interval.FinishEventID)
		if order.HappensBefore(interval.FinishEventID, terminalID) {
			return sortedUnique(evidence), true
		}
	}
	return sortedUnique(evidence), false
}

func maximalPhaseFinishes(phase Phase, intervals []PhaseInterval, terminalID string, order *CausalOrder) []string {
	ids := []string{}
	for _, interval := range phaseIntervals(phase, intervals) {
		if order.HappensBefore(interval.FinishEventID, terminalID) {
			ids = append(ids, interval.FinishEventID)
		}
	}
	result := []string{}
	for _, candidate := range ids {
		maximal := true
		for _, other := range ids {
			if order.HappensBefore(candidate, other) {
				maximal = false
			}
		}
		if maximal {
			result = append(result, candidate)
		}
	}
	return sortedUnique(result)
}

func phaseIntervals(phase Phase, intervals []PhaseInterval) []PhaseInterval {
	result := []PhaseInterval{}
	for _, interval := range intervals {
		if interval.Phase == phase {
			result = append(result, interval)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].StartEventID < result[j].StartEventID })
	return result
}

func diagnosePhaseOverlap(read EffortRead, order *CausalOrder) []Finding {
	type rawInterval struct {
		start  EventEnvelope
		finish *EventEnvelope
	}
	intervals := []rawInterval{}
	finishes := map[string]EventEnvelope{}
	for _, event := range read.Events {
		if event.Kind == "phase_finished" {
			var payload PhaseFinishedPayload
			if json.Unmarshal(event.Payload, &payload) == nil {
				finishes[payload.StartEventID] = event
			}
		}
	}
	for _, event := range read.Events {
		if event.Kind != "phase_started" {
			continue
		}
		interval := rawInterval{start: event}
		if finish, ok := finishes[event.EventID]; ok && order.HappensBefore(event.EventID, finish.EventID) {
			interval.finish = &finish
		}
		intervals = append(intervals, interval)
	}
	result := []Finding{}
	for i := range intervals {
		for j := range intervals {
			if i == j || !order.HappensBefore(intervals[i].start.EventID, intervals[j].start.EventID) {
				continue
			}
			if intervals[i].finish != nil && !order.HappensBefore(intervals[j].start.EventID, intervals[i].finish.EventID) {
				continue
			}
			evidence := []string{intervals[i].start.EventID, intervals[j].start.EventID}
			if intervals[i].finish != nil {
				evidence = append(evidence, intervals[i].finish.EventID)
			}
			if intervals[j].finish != nil {
				evidence = append(evidence, intervals[j].finish.EventID)
			}
			evidence = sortedUnique(evidence)
			trajectory := intervals[j].start.TrajectoryID
			if trajectory == "" {
				trajectory = "unassigned"
			}
			result = append(result, exactFinding("WFV1-PHASE-OVERLAP", "violation", trajectoryScope(read.Metadata.EffortID, trajectory), evidence,
				"causally ordered top-level phase intervals overlap",
				"append a typed phase repair or approve this exact phase overlap"))
		}
	}
	return result
}

func diagnoseClockIntegrity(read EffortRead, order *CausalOrder) []Finding {
	byID := eventsByID(read.Events)
	result := []Finding{}
	for _, finish := range read.Events {
		if finish.Kind != "phase_finished" {
			continue
		}
		var payload PhaseFinishedPayload
		if json.Unmarshal(finish.Payload, &payload) != nil {
			continue
		}
		start, ok := byID[payload.StartEventID]
		if !ok || !order.HappensBefore(start.EventID, finish.EventID) {
			continue
		}
		startTime, startErr := time.Parse(time.RFC3339Nano, start.Timestamp)
		finishTime, finishErr := time.Parse(time.RFC3339Nano, finish.Timestamp)
		invalid := startErr != nil || finishErr != nil || finishTime.Before(startTime)
		if !invalid {
			seconds := finishTime.Unix() - startTime.Unix()
			nanos := int64(finishTime.Nanosecond()) - int64(startTime.Nanosecond())
			invalid = seconds > int64(maximumLinkedInterval/time.Second) || seconds == int64(maximumLinkedInterval/time.Second) && nanos > int64(maximumLinkedInterval%time.Second)
		}
		if invalid {
			evidence := []string{start.EventID, finish.EventID}
			finding := exactFinding("WFV1-CLOCK-INTEGRITY", "warning", intervalScope(read.Metadata.EffortID, start.EventID, finish.EventID), evidence,
				"a causally linked interval has a negative or protocol-bound-exceeding duration",
				"verify the endpoint clocks; the interval is excluded from duration baselines")
			finding.Evidence.CounterIDs = sortedUnique([]string{
				boundedDiagnosticScope("timestamp:" + start.EventID + ":" + start.Timestamp),
				boundedDiagnosticScope("timestamp:" + finish.EventID + ":" + finish.Timestamp),
			})
			result = append(result, finding)
		}
	}
	return result
}

func diagnoseHandoffs(reads []EffortRead) []Finding {
	type associationRecord struct {
		effortID string
		event    EventEnvelope
		payload  SessionAssociatedPayload
		applied  bool
	}
	associations := []associationRecord{}
	for _, read := range reads {
		workflow := ProjectWorkflow(read)
		for _, event := range read.Events {
			if event.Kind != "session_associated" {
				continue
			}
			var payload SessionAssociatedPayload
			if json.Unmarshal(event.Payload, &payload) == nil {
				associations = append(associations, associationRecord{effortID: read.Metadata.EffortID, event: event, payload: payload, applied: workflow.Lifecycle.EffectApplied[event.EventID]})
			}
		}
	}
	result := []Finding{}
	for _, read := range reads {
		sourceWorkflow := ProjectWorkflow(read)
		sourceOrder, _ := BuildCausalOrder(read.Events)
		for _, handoff := range read.Events {
			if handoff.Kind != "handoff_observed" {
				continue
			}
			var payload HandoffObservedPayload
			if json.Unmarshal(handoff.Payload, &payload) != nil || payload.Outcome != "success" {
				continue
			}
			sourceTrajectory := handoff.TrajectoryID
			if sourceTrajectory == "" {
				sourceTrajectory = causallyVisibleAssociationTrajectory(handoff, read.Events, sourceOrder, sourceWorkflow.Lifecycle.EffectApplied)
			}
			matching := []associationRecord{}
			for _, association := range associations {
				if association.event.SessionID == payload.TargetSessionID && association.payload.HandoffEventID == handoff.EventID {
					matching = append(matching, association)
				}
			}
			valid := false
			evidence := []string{handoff.EventID}
			for _, association := range matching {
				evidence = append(evidence, association.event.EventID)
				valid = valid || sourceTrajectory != "" && association.applied && association.effortID == read.Metadata.EffortID && association.payload.AssociationOrigin == "handoff" && association.payload.TrajectoryID == sourceTrajectory
			}
			if !valid {
				result = append(result, exactFinding("WFV1-HANDOFF-ASSOCIATION", "warning", sessionLinkScope(read.Metadata.EffortID, handoff.SessionID, payload.TargetSessionID), sortedUnique(evidence),
					"a successful handoff has no validated same-effort association copy",
					"append an explicit association or detach correction, or approve the missing copy"))
			}
		}
	}
	return result
}

func exactFinding(code, severity, scope string, eventIDs []string, explanation, nextAction string) Finding {
	return Finding{
		Code: code, Type: "exact", Severity: severity, Scope: scope,
		Evidence:   FindingEvidence{EventIDs: sortedUnique(eventIDs), CounterIDs: []string{}},
		Confidence: "certain", Explanation: explanation, NextAction: nextAction, Waived: false,
	}
}

func applyExactWaiver(finding Finding, waivers []WaiverState) Finding {
	for _, waiver := range waivers {
		if string(waiver.RuleCode) != finding.Code || waiver.Scope != finding.Scope || !equalStringSets(waiver.EvidenceIDs, finding.Evidence.EventIDs) || !contains(descriptor.WaiverRules[finding.Code], string(waiver.ReasonCode)) {
			continue
		}
		finding.Severity = "informational"
		finding.Waived = true
		return finding
	}
	return finding
}

func transitionRepair(evidence []string, byID map[string]EventEnvelope) *ReconciliationProposal {
	if len(evidence) != 1 {
		return nil
	}
	event, ok := byID[evidence[0]]
	if !ok || event.Kind != "route_changed" {
		return nil
	}
	return proposal("supersede-event", evidence, "route_selected", event.Payload)
}

func concurrentRepair(evidence []string, byID map[string]EventEnvelope) *ReconciliationProposal {
	if len(evidence) < 2 {
		return nil
	}
	event, ok := byID[evidence[0]]
	if !ok || !descriptor.Payloads[string(event.Kind)].Repairable || event.Kind == "repair_applied" {
		return nil
	}
	return proposal("supersede-event", evidence, event.Kind, event.Payload)
}

func proposal(kind string, sources []string, eventKind EventKind, payload json.RawMessage) *ReconciliationProposal {
	return &ReconciliationProposal{
		Kind:           kind,
		SourceEventIDs: sortedUnique(sources),
		Replacement:    RepairReplacement{EventKind: eventKind, Payload: append(json.RawMessage(nil), payload...)},
	}
}

func findingMatchesSelection(finding Finding, selected map[string]bool, selector Selector) bool {
	if selector.SessionID == nil && selector.Phase == nil && selector.Since == nil && selector.Until == nil {
		return true
	}
	for _, eventID := range finding.Evidence.EventIDs {
		if selected[eventID] {
			return true
		}
	}
	return false
}

func stableFindings(findings []Finding) []Finding {
	seen := map[string]bool{}
	result := make([]Finding, 0, len(findings))
	for _, finding := range findings {
		finding.Evidence.EventIDs = sortedUnique(finding.Evidence.EventIDs)
		finding.Evidence.CounterIDs = sortedUnique(finding.Evidence.CounterIDs)
		key := finding.Code + "\x00" + finding.Scope + "\x00" + strings.Join(finding.Evidence.EventIDs, "\x00") + "\x00" + strings.Join(finding.Evidence.CounterIDs, "\x00")
		if !seen[key] {
			seen[key] = true
			result = append(result, finding)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		left := result[i].Code + "\x00" + result[i].Scope + "\x00" + strings.Join(result[i].Evidence.EventIDs, "\x00")
		right := result[j].Code + "\x00" + result[j].Scope + "\x00" + strings.Join(result[j].Evidence.EventIDs, "\x00")
		return left < right
	})
	return result
}

func eventsByID(events []EventEnvelope) map[string]EventEnvelope {
	result := make(map[string]EventEnvelope, len(events))
	for _, event := range events {
		result[event.EventID] = event
	}
	return result
}

func sortedUnique(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func equalStringSets(left, right []string) bool {
	left, right = sortedUnique(left), sortedUnique(right)
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func effortScope(effortID string) string { return boundedDiagnosticScope("effort:" + effortID) }
func terminalEpochScope(effortID string, epoch uint64) string {
	return boundedDiagnosticScope("effort:" + effortID + ":epoch:" + uintString(epoch))
}
func trajectoryScope(effortID, trajectoryID string) string {
	return boundedDiagnosticScope("effort:" + effortID + ":trajectory:" + trajectoryID)
}
func intervalScope(effortID, start, finish string) string {
	return boundedDiagnosticScope("effort:" + effortID + ":interval:" + start + ":" + finish)
}
func sessionLinkScope(effortID, source, target string) string {
	return boundedDiagnosticScope("effort:" + effortID + ":session-link:" + source + ":" + target)
}

func boundedDiagnosticScope(scope string) string {
	if len([]byte(scope)) <= descriptor.Limits.CategoryBytes {
		return scope
	}
	digest := sha256.Sum256([]byte(scope))
	return "scope-sha256:" + hex.EncodeToString(digest[:])
}

func uintString(value uint64) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	position := len(digits)
	for value > 0 {
		position--
		digits[position] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[position:])
}

func findingBelongsToEffort(finding Finding, reads []EffortRead, effortID string) bool {
	for _, read := range reads {
		if read.Metadata.EffortID != effortID {
			continue
		}
		byID := eventsByID(read.Events)
		for _, eventID := range finding.Evidence.EventIDs {
			if event, exists := byID[eventID]; exists && event.Kind == "handoff_observed" {
				return true
			}
		}
	}
	return false
}

func selectedForFinding(finding Finding, selectedByEffort map[string]map[string]bool) map[string]bool {
	result := map[string]bool{}
	for _, selected := range selectedByEffort {
		for _, eventID := range finding.Evidence.EventIDs {
			result[eventID] = result[eventID] || selected[eventID]
		}
	}
	return result
}

func lifecycleForFinding(reads []EffortRead, finding Finding) LifecycleProjection {
	for _, read := range reads {
		byID := eventsByID(read.Events)
		for _, eventID := range finding.Evidence.EventIDs {
			if _, exists := byID[eventID]; exists {
				return projectLifecycleFromRead(read)
			}
		}
	}
	return newLifecycleProjection()
}
