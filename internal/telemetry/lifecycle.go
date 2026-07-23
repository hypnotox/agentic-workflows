package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

type EffortState string

const (
	EffortDiscovery EffortState = "discovery"
	EffortActive    EffortState = "active"
	EffortCompleted EffortState = "completed"
	EffortAbandoned EffortState = "abandoned"
)

type PhaseInterval struct {
	Phase         Phase
	StartEventID  string
	FinishEventID string
	TrajectoryID  string
	TerminalEpoch uint64
}

type RouteSelection struct {
	Route   Route
	EventID string
}

type WaiverState struct {
	EventID     string
	RuleCode    DiagnosticRuleCode
	Scope       string
	EvidenceIDs []string
	ReasonCode  WaiverReasonCode
}

type RepairState struct {
	EventID        string
	ProposalKind   ProposalKind
	SourceEventIDs []string
	Replacement    RepairReplacement
}

type LifecycleProjection struct {
	State              EffortState
	Route              Route
	RouteHistory       []RouteSelection
	TerminalEpoch      uint64
	ActiveTrajectoryID string
	Associations       map[string]Association
	OpenPhases         map[string]PhaseInterval
	PhaseIntervals     []PhaseInterval
	Trajectories       map[string]TrajectoryNode
	Waivers            []WaiverState
	Repairs            []RepairState
	SupersededEventIDs map[string]string
	AppliedEventIDs    []string
	EffectApplied      map[string]bool
	closedTrajectories map[string]bool
	Invalid            []IntegrityIssue
}

var routeRequirements = map[Route][]Phase{
	"direct":             {"brainstorming", "implementation", "implementation-review", "retrospective"},
	"adr":                {"brainstorming", "adr-authoring", "adr-review", "implementation", "implementation-review", "retrospective"},
	"plan":               {"brainstorming", "planning", "plan-review", "implementation", "implementation-review", "retrospective"},
	"adr-plan":           {"brainstorming", "adr-authoring", "adr-review", "planning", "plan-review", "adr-plan-resync", "implementation", "implementation-review", "retrospective"},
	"bugfix":             {"brainstorming", "implementation", "implementation-review", "retrospective"},
	"investigation-only": {"investigation", "retrospective"},
}

func ProjectLifecycle(events []EventEnvelope) LifecycleProjection {
	return projectLifecycle(events, nil)
}

func projectLifecycle(events []EventEnvelope, excluded map[string]bool) LifecycleProjection {
	projection := newLifecycleProjection()
	order, causalIssues := BuildCausalOrder(events)
	projection.Invalid = append(projection.Invalid, causalIssues...)
	invalid := map[string]bool{}
	for _, issue := range causalIssues {
		for _, eventID := range issue.EventIDs {
			if event, exists := order.events[eventID]; exists && descriptor.Payloads[string(event.Kind)].Class == "lifecycle" {
				invalid[eventID] = true
			}
		}
	}

	// Incomparable competing mutations cannot be made sequential by file or
	// event-ID enumeration. Passive predecessor tips may differ while carrying
	// the same effective lifecycle frontier, so direct frontier equality is not
	// required. Keep every event as evidence and apply neither effect.
	for left := range events {
		for right := left + 1; right < len(events); right++ {
			if excluded[events[left].EventID] || excluded[events[right].EventID] || !lifecycleEventsConflict(events[left], events[right]) {
				continue
			}
			if order.Concurrent(events[left].EventID, events[right].EventID) {
				invalid[events[left].EventID], invalid[events[right].EventID] = true, true
				projection.Invalid = append(projection.Invalid, integrity("concurrent-state", events[left].EffortID, 0, []string{events[left].EventID, events[right].EventID}, "competing lifecycle mutations share a causal frontier"))
			}
		}
	}

	// A valid repair supersedes named evidence without deleting it. Determine
	// suppression before state application so a repaired source never leaks an
	// effect merely because its stream happened to sort first.
	byID := map[string]EventEnvelope{}
	repairPayloads := map[string]RepairAppliedPayload{}
	for _, event := range events {
		byID[event.EventID] = event
	}
	for _, event := range events {
		if event.Kind != "repair_applied" || invalid[event.EventID] {
			continue
		}
		var payload RepairAppliedPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			invalid[event.EventID] = true
			continue
		}
		if err := validateRepair(event, payload, byID, order); err != nil {
			invalid[event.EventID] = true
			projection.Invalid = append(projection.Invalid, transitionIssue(event, err))
			continue
		}
		repairPayloads[event.EventID] = payload
		for _, source := range payload.SourceEventIDs {
			projection.SupersededEventIDs[source] = event.EventID
		}
	}

	pending := order.ordered(events)
	replacementApplied := map[string]bool{}
	for _, event := range pending {
		projection.EffectApplied[event.EventID] = false
		sourceRejected := invalid[event.EventID] || excluded[event.EventID]
		if repairID := projection.SupersededEventIDs[event.EventID]; repairID != "" {
			if replacementApplied[repairID] {
				continue
			}
			payload := repairPayloads[repairID]
			replacement := event
			replacement.Kind = payload.Replacement.EventKind
			replacement.Payload = payload.Replacement.Payload
			if err := projection.apply(replacement, order); err != nil {
				invalid[repairID] = true
				projection.Invalid = append(projection.Invalid, transitionIssue(byID[repairID], fmt.Errorf("repair replacement: %w", err)))
				for source, owner := range projection.SupersededEventIDs {
					if owner == repairID {
						delete(projection.SupersededEventIDs, source)
					}
				}
				if !sourceRejected {
					if err := projection.apply(event, order); err != nil {
						invalid[event.EventID] = true
						projection.Invalid = append(projection.Invalid, transitionIssue(event, err))
					} else {
						projection.EffectApplied[event.EventID] = true
						projection.AppliedEventIDs = append(projection.AppliedEventIDs, event.EventID)
					}
				}
			} else {
				replacementApplied[repairID] = true
			}
			continue
		}
		if sourceRejected {
			continue
		}
		if payload, repair := repairPayloads[event.EventID]; repair {
			if !replacementApplied[event.EventID] { // coverage-ignore: validateRepair requires every source to happen before the repair, so ordered projection always attempts its replacement first
				continue
			}
			projection.Repairs = append(projection.Repairs, RepairState{EventID: event.EventID, ProposalKind: payload.ProposalKind, SourceEventIDs: append([]string(nil), payload.SourceEventIDs...), Replacement: payload.Replacement})
			projection.EffectApplied[event.EventID] = true
			projection.AppliedEventIDs = append(projection.AppliedEventIDs, event.EventID)
			continue
		}
		if err := projection.apply(event, order); err != nil {
			invalid[event.EventID] = true
			projection.Invalid = append(projection.Invalid, transitionIssue(event, err))
			continue
		}
		projection.EffectApplied[event.EventID] = true
		projection.AppliedEventIDs = append(projection.AppliedEventIDs, event.EventID)
	}
	projection.Invalid = deduplicateIntegrity(projection.Invalid)
	return projection
}

func newLifecycleProjection() LifecycleProjection {
	return LifecycleProjection{
		State:              "",
		TerminalEpoch:      0,
		Associations:       map[string]Association{},
		OpenPhases:         map[string]PhaseInterval{},
		Trajectories:       map[string]TrajectoryNode{},
		SupersededEventIDs: map[string]string{},
		RouteHistory:       []RouteSelection{},
		PhaseIntervals:     []PhaseInterval{},
		Waivers:            []WaiverState{},
		Repairs:            []RepairState{},
		AppliedEventIDs:    []string{},
		EffectApplied:      map[string]bool{},
		Invalid:            []IntegrityIssue{},
		closedTrajectories: map[string]bool{},
	}
}

func (p *LifecycleProjection) apply(event EventEnvelope, order *CausalOrder) error {
	if descriptor.Payloads[string(event.Kind)].Class != "lifecycle" {
		return nil
	}
	if p.State == "" && event.Kind != "effort_created" {
		return errors.New("mutation requires an existing effort")
	}
	if (p.State == EffortCompleted || p.State == EffortAbandoned) && event.Kind != "finding_waived" && event.Kind != "repair_applied" && event.Kind != "effort_reopened" && (p.State != EffortAbandoned || event.Kind != "session_detached") {
		return fmt.Errorf("%s is not legal after terminal state %s", event.Kind, p.State)
	}
	switch event.Kind {
	case "effort_created":
		if len(p.AppliedEventIDs) != 0 {
			return errors.New("effort already exists")
		}
		p.State = EffortDiscovery
		p.TerminalEpoch = 1
	case "session_associated":
		var payload SessionAssociatedPayload
		_ = json.Unmarshal(event.Payload, &payload)
		node, exists := p.Trajectories[payload.TrajectoryID]
		if !exists || !order.HappensBefore(node.EventID, event.EventID) {
			return errors.New("session association requires a causally visible trajectory")
		}
		p.Associations[event.SessionID] = Association{EffortID: event.EffortID, SessionID: event.SessionID, TrajectoryID: payload.TrajectoryID, AssociationOrigin: payload.AssociationOrigin}
	case "session_detached":
		delete(p.Associations, event.SessionID)
	case "route_selected":
		if p.State != EffortDiscovery {
			return errors.New("route selection requires discovery state")
		}
		var payload RoutePayload
		_ = json.Unmarshal(event.Payload, &payload)
		if payload.Route == "investigation-only" && p.hasPhase("implementation") {
			return errors.New("investigation-only cannot follow implementation")
		}
		p.Route, p.State = payload.Route, EffortActive
		p.RouteHistory = append(p.RouteHistory, RouteSelection{Route: payload.Route, EventID: event.EventID})
	case "route_changed":
		if p.State != EffortActive || p.Route == "" {
			return errors.New("route change requires an active selected route")
		}
		var payload RoutePayload
		_ = json.Unmarshal(event.Payload, &payload)
		if payload.Route == "investigation-only" && p.hasPhase("implementation") {
			return errors.New("investigation-only cannot include implementation")
		}
		p.Route = payload.Route
		p.RouteHistory = append(p.RouteHistory, RouteSelection{Route: payload.Route, EventID: event.EventID})
	case "phase_started":
		if len(p.OpenPhases) != 0 {
			return errors.New("a top-level phase is already open")
		}
		var payload PhaseStartedPayload
		_ = json.Unmarshal(event.Payload, &payload)
		p.OpenPhases[event.EventID] = PhaseInterval{Phase: payload.Phase, StartEventID: event.EventID, TrajectoryID: event.TrajectoryID, TerminalEpoch: p.TerminalEpoch}
	case "phase_finished":
		var payload PhaseFinishedPayload
		_ = json.Unmarshal(event.Payload, &payload)
		interval, ok := p.OpenPhases[payload.StartEventID]
		if !ok || interval.Phase != payload.Phase {
			return errors.New("phase finish does not name an unmatched matching start")
		}
		if !order.HappensBefore(payload.StartEventID, event.EventID) {
			return errors.New("phase start is not causally visible to finish")
		}
		interval.FinishEventID = event.EventID
		delete(p.OpenPhases, payload.StartEventID)
		p.PhaseIntervals = append(p.PhaseIntervals, interval)
	case "phase_transitioned":
		var payload PhaseTransitionedPayload
		_ = json.Unmarshal(event.Payload, &payload)
		interval, ok := p.OpenPhases[payload.StartEventID]
		if !ok || interval.Phase != payload.Phase {
			return errors.New("phase transition does not name an unmatched matching start")
		}
		if !order.HappensBefore(payload.StartEventID, event.EventID) {
			return errors.New("phase start is not causally visible to transition")
		}
		switch payload.RouteAction {
		case "":
		case "select":
			if p.State != EffortDiscovery {
				return errors.New("route selection requires discovery state")
			}
			p.Route, p.State = payload.Route, EffortActive
			p.RouteHistory = append(p.RouteHistory, RouteSelection{Route: payload.Route, EventID: event.EventID})
		case "change":
			if p.State != EffortActive || p.Route == "" {
				return errors.New("route change requires an active selected route")
			}
			p.Route = payload.Route
			p.RouteHistory = append(p.RouteHistory, RouteSelection{Route: payload.Route, EventID: event.EventID})
		}
		if payload.Route == "investigation-only" && p.hasPhase("implementation") {
			return errors.New("investigation-only cannot include implementation")
		}
		interval.FinishEventID = event.EventID
		delete(p.OpenPhases, payload.StartEventID)
		p.PhaseIntervals = append(p.PhaseIntervals, interval)
		p.OpenPhases[event.EventID] = PhaseInterval{Phase: payload.NextPhase, StartEventID: event.EventID, TrajectoryID: event.TrajectoryID, TerminalEpoch: p.TerminalEpoch}
	case "trajectory_started", "trajectory_resumed", "trajectory_closed":
		var payload TrajectoryPayload
		_ = json.Unmarshal(event.Payload, &payload)
		if event.Kind == "trajectory_started" {
			if node, exists := p.Trajectories[payload.TrajectoryID]; exists && node.EventID != event.EventID && order.HappensBefore(node.EventID, event.EventID) {
				return errors.New("trajectory already exists")
			}
			p.Trajectories[payload.TrajectoryID] = TrajectoryNode{ID: payload.TrajectoryID, EventID: event.EventID}
			p.closedTrajectories[payload.TrajectoryID] = false
		}
		if event.Kind == "trajectory_resumed" {
			node, exists := p.Trajectories[payload.TrajectoryID]
			if !exists || !order.HappensBefore(node.EventID, event.EventID) {
				return errors.New("cannot resume a causally unknown trajectory")
			}
			p.ActiveTrajectoryID = payload.TrajectoryID
			p.closedTrajectories[payload.TrajectoryID] = false
			if association, ok := p.Associations[event.SessionID]; ok {
				if !trajectoryContains(p.Trajectories, payload.TrajectoryID, association.TrajectoryID) {
					delete(p.Associations, event.SessionID)
				}
			}
		}
		if event.Kind == "trajectory_started" {
			p.ActiveTrajectoryID = payload.TrajectoryID
		}
		if event.Kind == "trajectory_closed" {
			node, exists := p.Trajectories[payload.TrajectoryID]
			if !exists || !order.HappensBefore(node.EventID, event.EventID) {
				return errors.New("cannot close unknown trajectory")
			}
			if p.closedTrajectories[payload.TrajectoryID] {
				return errors.New("cannot close inactive trajectory")
			}
			if p.ActiveTrajectoryID != payload.TrajectoryID {
				return errors.New("cannot close a non-active trajectory")
			}
			p.closedTrajectories[payload.TrajectoryID] = true
			p.ActiveTrajectoryID = ""
		}
	case "trajectory_forked":
		var payload TrajectoryForkedPayload
		_ = json.Unmarshal(event.Payload, &payload)
		if _, exists := p.Trajectories[payload.TrajectoryID]; exists {
			return errors.New("fork trajectory already exists")
		}
		parent, exists := p.Trajectories[payload.ParentTrajectoryID]
		if !exists || !order.HappensBefore(parent.EventID, event.EventID) {
			return errors.New("fork parent trajectory is not causally visible")
		}
		p.Trajectories[payload.TrajectoryID] = TrajectoryNode{ID: payload.TrajectoryID, ParentTrajectoryID: payload.ParentTrajectoryID, ForkAnchorID: payload.ForkAnchorID, EventID: event.EventID}
		p.closedTrajectories[payload.TrajectoryID] = false
		p.ActiveTrajectoryID = payload.TrajectoryID
	case "effort_completed":
		if p.State != EffortActive || p.Route == "" {
			return errors.New("completion requires active state and route")
		}
		var payload EffortTerminalPayload
		_ = json.Unmarshal(event.Payload, &payload)
		if payload.TerminalEpoch != p.TerminalEpoch {
			return errors.New("completion terminal epoch does not match active epoch")
		}
		if len(p.OpenPhases) != 0 {
			return errors.New("completion cannot leave a phase open")
		}
		if err := p.validateRouteCompletion(event, order); err != nil {
			return err
		}
		p.State = EffortCompleted
	case "effort_abandoned":
		var payload EffortTerminalPayload
		_ = json.Unmarshal(event.Payload, &payload)
		if payload.TerminalEpoch != p.TerminalEpoch {
			return errors.New("abandonment terminal epoch does not match active epoch")
		}
		p.State = EffortAbandoned
	case "effort_reopened":
		if p.State != EffortCompleted {
			return errors.New("only a completed effort can reopen")
		}
		var payload EffortReopenedPayload
		_ = json.Unmarshal(event.Payload, &payload)
		if payload.TerminalEpoch != p.TerminalEpoch+1 {
			return errors.New("reopen must create the next terminal epoch")
		}
		if _, exists := p.Trajectories[payload.TrajectoryID]; exists {
			return errors.New("reopen must create a new trajectory")
		}
		p.TerminalEpoch = payload.TerminalEpoch
		p.Trajectories[payload.TrajectoryID] = TrajectoryNode{ID: payload.TrajectoryID, EventID: event.EventID}
		p.ActiveTrajectoryID, p.State = payload.TrajectoryID, EffortActive
	case "finding_waived":
		var payload FindingWaivedPayload
		_ = json.Unmarshal(event.Payload, &payload)
		if !contains(descriptor.WaiverRules[string(payload.RuleCode)], string(payload.ReasonCode)) {
			return errors.New("rule and reason code are not waiver eligible")
		}
		p.Waivers = append(p.Waivers, WaiverState{EventID: event.EventID, RuleCode: payload.RuleCode, Scope: string(payload.Scope), EvidenceIDs: append([]string(nil), payload.EvidenceIDs...), ReasonCode: payload.ReasonCode})
	case "repair_applied":
		// The caller applies the typed replacement at the repair's causal point
		// and records the repair itself after that replacement succeeds.
	}
	return nil
}

func (p LifecycleProjection) hasPhase(phase Phase) bool {
	for _, interval := range p.PhaseIntervals {
		if interval.Phase == phase {
			return true
		}
	}
	for _, interval := range p.OpenPhases {
		if interval.Phase == phase {
			return true
		}
	}
	return false
}

func (p LifecycleProjection) validateRouteCompletion(terminal EventEnvelope, order *CausalOrder) error {
	required := routeRequirements[p.Route]
	last := ""
	for _, phase := range required {
		selected := ""
		for _, interval := range p.PhaseIntervals {
			if interval.TerminalEpoch != p.TerminalEpoch || interval.Phase != phase || interval.FinishEventID == "" || !order.HappensBefore(interval.FinishEventID, terminal.EventID) {
				continue
			}
			if last != "" && last != interval.StartEventID && !order.HappensBefore(last, interval.StartEventID) {
				continue
			}
			if selected == "" {
				selected = interval.FinishEventID
			}
		}
		if selected == "" {
			return fmt.Errorf("route %s lacks fresh ordered phase %s", p.Route, phase)
		}
		last = selected
	}
	latest := func(phase Phase) string {
		candidate := ""
		for _, interval := range p.PhaseIntervals {
			if interval.TerminalEpoch != p.TerminalEpoch || interval.Phase != phase || !order.HappensBefore(interval.FinishEventID, terminal.EventID) {
				continue
			}
			if candidate == "" || order.HappensBefore(candidate, interval.FinishEventID) {
				candidate = interval.FinishEventID
			}
		}
		return candidate
	}
	requireFresh := func(upstream, downstream Phase) error {
		upstreamID := latest(upstream)
		if upstreamID == "" {
			return nil
		}
		downstreamID := latest(downstream)
		if downstreamID == "" || !order.HappensBefore(upstreamID, downstreamID) {
			return fmt.Errorf("phase %s invalidates stale %s evidence", upstream, downstream)
		}
		return nil
	}
	if err := requireFresh("adr-authoring", "adr-review"); err != nil {
		return err
	}
	if err := requireFresh("planning", "plan-review"); err != nil {
		return err
	}
	if p.Route == "adr-plan" {
		resync := latest("adr-plan-resync")
		for _, phase := range []Phase{"adr-review", "plan-review"} {
			upstream := latest(phase)
			if upstream != "" && (resync == "" || !order.HappensBefore(upstream, resync)) {
				return fmt.Errorf("phase %s invalidates stale adr-plan-resync evidence", phase)
			}
		}
	}
	if err := requireFresh("implementation", "implementation-review"); err != nil {
		return err
	}
	return nil
}

func lifecycleEventsConflict(left, right EventEnvelope) bool {
	if transitionHasRouteEffect(left) && (right.Kind == "route_selected" || right.Kind == "route_changed") || transitionHasRouteEffect(right) && (left.Kind == "route_selected" || left.Kind == "route_changed") {
		return true
	}
	if left.Kind == "repair_applied" && right.Kind == "repair_applied" {
		return repairSourcesOverlap(left, right)
	}
	leftKey, rightKey := lifecycleConflictKey(left), lifecycleConflictKey(right)
	if leftKey != "" && leftKey == rightKey {
		return true
	}
	return isTerminalMutation(left.Kind) && isMutableLifecycle(right.Kind) || isTerminalMutation(right.Kind) && isMutableLifecycle(left.Kind)
}

func transitionHasRouteEffect(event EventEnvelope) bool {
	if event.Kind != "phase_transitioned" {
		return false
	}
	var payload PhaseTransitionedPayload
	return json.Unmarshal(event.Payload, &payload) == nil && payload.RouteAction != ""
}

func lifecycleConflictKey(event EventEnvelope) string {
	switch event.Kind {
	case "route_selected", "route_changed", "effort_completed", "effort_abandoned", "effort_reopened":
		return "effort-state"
	case "phase_started", "phase_transitioned":
		return "phase-open"
	case "phase_finished":
		var payload PhaseFinishedPayload
		_ = json.Unmarshal(event.Payload, &payload)
		return "phase-finish:" + payload.StartEventID
	case "session_associated", "session_detached":
		return "association:" + event.SessionID
	case "trajectory_started", "trajectory_resumed", "trajectory_closed", "trajectory_forked":
		return "trajectory-active"
	case "repair_applied":
		return ""
	}
	return ""
}

func repairSourcesOverlap(left, right EventEnvelope) bool {
	var leftPayload, rightPayload RepairAppliedPayload
	_ = json.Unmarshal(left.Payload, &leftPayload)
	_ = json.Unmarshal(right.Payload, &rightPayload)
	sources := make(map[string]bool, len(leftPayload.SourceEventIDs))
	for _, source := range leftPayload.SourceEventIDs {
		sources[source] = true
	}
	for _, source := range rightPayload.SourceEventIDs {
		if sources[source] {
			return true
		}
	}
	return false
}

func isTerminalMutation(kind EventKind) bool {
	return kind == "effort_completed" || kind == "effort_abandoned" || kind == "effort_reopened"
}

func isMutableLifecycle(kind EventKind) bool {
	return descriptor.Payloads[string(kind)].Class == "lifecycle" && kind != "effort_created" && kind != "finding_waived" && kind != "repair_applied"
}

func validateRepair(event EventEnvelope, payload RepairAppliedPayload, byID map[string]EventEnvelope, order *CausalOrder) error {
	if len(payload.SourceEventIDs) == 0 {
		return errors.New("repair requires source evidence")
	}
	for _, sourceID := range payload.SourceEventIDs {
		source, exists := byID[sourceID]
		if !exists {
			return fmt.Errorf("repair source %s does not exist", sourceID)
		}
		if !order.HappensBefore(sourceID, event.EventID) {
			return fmt.Errorf("repair source %s is not causally visible", sourceID)
		}
		if descriptor.Payloads[string(source.Kind)].Class != "lifecycle" {
			return fmt.Errorf("repair source %s is not lifecycle evidence", sourceID)
		}
	}
	kind := payload.Replacement.EventKind
	if kind == "repair_applied" || !descriptor.Payloads[string(kind)].Repairable {
		return errors.New("repair replacement is not a repairable lifecycle event")
	}
	allowed := false
	switch payload.ProposalKind {
	case "supersede-event":
		allowed = true
	case "correct-phase":
		allowed = kind == "phase_started" || kind == "phase_transitioned" || kind == "phase_finished"
	case "correct-association":
		allowed = kind == "session_associated" || kind == "session_detached"
	case "correct-trajectory":
		allowed = kind == "trajectory_started" || kind == "trajectory_resumed" || kind == "trajectory_closed" || kind == "trajectory_forked"
	}
	if !allowed {
		return errors.New("repair proposal kind does not match replacement event")
	}
	return nil
}

func transitionIssue(event EventEnvelope, err error) IntegrityIssue {
	return integrity("invalid-transition", event.EffortID, 0, []string{event.EventID}, err.Error())
}

func trajectoryContains(nodes map[string]TrajectoryNode, trajectoryID, ancestorID string) bool {
	seen := map[string]bool{}
	for trajectoryID != "" && !seen[trajectoryID] { // coverage-ignore: validated trajectory creation and fork rules guarantee acyclic ancestry
		if trajectoryID == ancestorID {
			return true
		}
		seen[trajectoryID] = true
		trajectoryID = nodes[trajectoryID].ParentTrajectoryID
	}
	return false
}

func (l *Ledger) ApplyLifecycle(ctx context.Context, request LifecycleRequest) (AppendResult, error) {
	raw, metadata, creating, err := lifecycleRequestEvent(request)
	if err != nil {
		return AppendResult{}, err
	}
	if creating {
		result, createErr := l.CreateEffort(metadata, raw)
		if createErr != nil {
			return AppendResult{}, createErr
		}
		return l.verifyLifecycleResult(result)
	}
	event, err := ValidateEvent(raw)
	if err != nil {
		return AppendResult{}, err
	}
	leasePath := l.paths.appendLease(event.EffortID)
	nonce, err := l.acquireLease(ctx, leasePath)
	if err != nil {
		return AppendResult{}, err
	}
	stopHeartbeat, heartbeatDone := l.startHeartbeat(ctx, leasePath, nonce)
	heldContext := context.WithValue(ctx, heldEffortLeaseKey{}, event.EffortID)
	result, operationErr := l.applyLifecycleUnderLease(heldContext, event, raw)
	if operationErr == nil {
		result, operationErr = l.verifyLifecycleResult(result)
	}
	heartbeatErr := finishHeartbeat(stopHeartbeat, heartbeatDone)
	releaseErr := l.releaseLease(leasePath, nonce)
	if operationErr != nil {
		return AppendResult{}, operationErr
	}
	if heartbeatErr != nil {
		return AppendResult{}, heartbeatErr
	}
	if releaseErr != nil {
		return AppendResult{}, releaseErr
	}
	return result, nil
}

func (l *Ledger) verifyLifecycleResult(result AppendResult) (AppendResult, error) {
	read, err := l.ReadEffort(result.Event.EffortID)
	if err != nil {
		return AppendResult{}, fmt.Errorf("verify lifecycle append: %w", err)
	}
	projections := GroupDerivedFamilies([]WorkflowProjection{ProjectWorkflow(read)})
	if len(projections) != 1 || !projections[0].EffectApplied[result.Event.EventID] { // coverage-ignore: held-lease prevalidation and append guarantee the just-written lifecycle effect is applied
		return AppendResult{}, errors.New("durable lifecycle append has no applied effect")
	}
	return result, nil
}

func (l *Ledger) applyLifecycleUnderLease(ctx context.Context, event EventEnvelope, raw json.RawMessage) (AppendResult, error) {
	read, err := l.ReadEffort(event.EffortID)
	if err != nil {
		return AppendResult{}, err
	}
	for _, record := range read.Records {
		if record.Event == nil || !sameContractIdentity(*record.Event, event) {
			continue
		}
		prior := *record.Event
		equal, compareErr := lifecycleRequestMatchesEvent(event, raw, prior)
		if compareErr != nil || !equal {
			return AppendResult{}, errors.New("conflicting lifecycle idempotency key")
		}
		if !record.Applied {
			return AppendResult{}, errors.New("existing lifecycle retry has no applied effect")
		}
		return AppendResult{Event: prior, Idempotent: true}, nil
	}
	current := projectLifecycleFromRead(read)
	if event.TrajectoryID == "" {
		if association, exists := current.Associations[event.SessionID]; exists {
			event.TrajectoryID = association.TrajectoryID
			raw = eventToRaw(event)
		}
	}
	if event.Kind == "effort_completed" || event.Kind == "effort_abandoned" {
		event.Payload, _ = json.Marshal(EffortTerminalPayload{TerminalEpoch: current.TerminalEpoch})
		raw = eventToRaw(event)
	}
	if event.Kind == "effort_reopened" {
		var payload EffortReopenedPayload
		_ = json.Unmarshal(event.Payload, &payload)
		payload.TerminalEpoch = current.TerminalEpoch + 1
		event.Payload, _ = json.Marshal(payload)
		raw = eventToRaw(event)
	}
	projection := projectLifecycle(append(append([]EventEnvelope(nil), read.Events...), event), lifecycleExclusions(read))
	for _, issue := range projection.Invalid {
		if containsString(issue.EventIDs, event.EventID) {
			return AppendResult{}, fmt.Errorf("invalid lifecycle transition: %s", issue.Detail)
		}
	}
	return l.Append(ctx, raw)
}

func lifecycleExclusions(read EffortRead) map[string]bool {
	excluded := make(map[string]bool, len(read.RejectedEffects))
	for eventID := range read.RejectedEffects {
		excluded[eventID] = true
	}
	return excluded
}

func projectLifecycleFromRead(read EffortRead) LifecycleProjection {
	if len(read.RejectedEffects) == 0 {
		return ProjectLifecycle(read.Events)
	}
	return projectLifecycle(read.Events, lifecycleExclusions(read))
}

func lifecycleRequestMatchesEvent(requested EventEnvelope, raw json.RawMessage, prior EventEnvelope) (bool, error) {
	if requested.EventID != prior.EventID || requested.EffortID != prior.EffortID || requested.SessionID != prior.SessionID || requested.Timestamp != prior.Timestamp || requested.Kind != prior.Kind || !equalStrings(requested.Predecessors, prior.Predecessors) {
		return false, nil
	}
	switch requested.Kind {
	case "effort_completed", "effort_abandoned":
		return true, nil
	case "effort_reopened":
		var requestedPayload, priorPayload EffortReopenedPayload
		if err := json.Unmarshal(requested.Payload, &requestedPayload); err != nil { // coverage-ignore: lifecycle request validation proved the payload
			return false, err
		}
		if err := json.Unmarshal(prior.Payload, &priorPayload); err != nil { // coverage-ignore: ledger event validation proved the payload
			return false, err
		}
		return requestedPayload.TrajectoryID == priorPayload.TrajectoryID && requestedPayload.AnchorID == priorPayload.AnchorID, nil
	default:
		if requested.Kind != "session_associated" && requested.Kind != "trajectory_started" && requested.Kind != "trajectory_resumed" && requested.Kind != "trajectory_closed" && requested.Kind != "trajectory_forked" {
			prior.TrajectoryID = ""
		}
		return eventsEqual(prior, raw)
	}
}

func lifecycleRequestEvent(request LifecycleRequest) (json.RawMessage, EffortMetadata, bool, error) {
	base, kind, payload, metadata, creating, err := lifecycleRequestParts(request)
	if err != nil {
		return nil, EffortMetadata{}, false, err
	}
	predecessors := append([]string{}, base.Predecessors...)
	envelope := EventEnvelope{Version: descriptor.Version, EventID: base.EventID, IdempotencyKey: base.IdempotencyKey, EffortID: base.EffortID, SessionID: base.SessionID, Timestamp: base.Timestamp, Kind: kind, Predecessors: predecessors, Payload: payload}
	requestValue := reflect.ValueOf(request)
	if requestValue.Kind() == reflect.Pointer {
		request = requestValue.Elem().Interface().(LifecycleRequest)
	}
	switch typed := request.(type) {
	case AssociateLifecycleRequest:
		envelope.TrajectoryID = typed.TrajectoryID
	case TrajectoryLifecycleRequest:
		envelope.TrajectoryID = typed.TrajectoryID
	case ForkTrajectoryLifecycleRequest:
		envelope.TrajectoryID, envelope.ParentTrajectoryID, envelope.ForkAnchorID = typed.TrajectoryID, typed.ParentTrajectoryID, typed.ForkAnchorID
	case ReopenLifecycleRequest:
		envelope.TrajectoryID = typed.TrajectoryID
	}
	raw, err := json.Marshal(envelope)
	return raw, metadata, creating, err
}

func lifecycleRequestParts(request LifecycleRequest) (LifecycleRequestBase, EventKind, json.RawMessage, EffortMetadata, bool, error) {
	value := reflect.ValueOf(request)
	if value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return LifecycleRequestBase{}, "", nil, EffortMetadata{}, false, errors.New("nil lifecycle request")
		}
		request = value.Elem().Interface().(LifecycleRequest)
	}
	marshal := func(value any) json.RawMessage { raw, _ := json.Marshal(value); return raw }
	var base LifecycleRequestBase
	var kind EventKind
	var payload json.RawMessage
	var metadata EffortMetadata
	creating := false
	switch typed := request.(type) {
	case CreateLifecycleRequest:
		base, kind, creating = typed.LifecycleRequestBase, "effort_created", true
		origin := OriginMetadata{}
		if typed.Origin != nil {
			origin = *typed.Origin
		}
		payload = marshal(EffortCreatedPayload{CreationMode: typed.CreationMode, OriginEffortID: origin.EffortID, OriginTrajectoryID: origin.TrajectoryID, OriginAnchorID: origin.AnchorID})
		metadata = EffortMetadata{EffortID: base.EffortID, CreatedAt: base.Timestamp, CreationMode: typed.CreationMode, Origin: typed.Origin}
	case AssociateLifecycleRequest:
		base, kind = typed.LifecycleRequestBase, "session_associated"
		payload = marshal(SessionAssociatedPayload{AssociationOrigin: typed.AssociationOrigin, TrajectoryID: typed.TrajectoryID, HandoffEventID: typed.HandoffEventID})
	case DetachLifecycleRequest:
		base, kind = typed.LifecycleRequestBase, "session_detached"
		payload = marshal(SessionDetachedPayload{Reason: typed.Reason})
	case RouteLifecycleRequest:
		base = typed.LifecycleRequestBase
		switch typed.Action {
		case "select-route":
			kind = "route_selected"
		case "change-route":
			kind = "route_changed"
		}
		payload = marshal(RoutePayload{Route: typed.Route})
	case StartPhaseLifecycleRequest:
		base, kind = typed.LifecycleRequestBase, "phase_started"
		payload = marshal(PhaseStartedPayload{Phase: typed.Phase, Activity: typed.Activity, ImplementationMode: typed.ImplementationMode})
	case FinishPhaseLifecycleRequest:
		base, kind = typed.LifecycleRequestBase, "phase_finished"
		payload = marshal(PhaseFinishedPayload{Phase: typed.Phase, StartEventID: typed.StartEventID, Outcome: typed.Outcome})
	case TransitionPhaseLifecycleRequest:
		base, kind = typed.LifecycleRequestBase, "phase_transitioned"
		payload = marshal(PhaseTransitionedPayload{Phase: typed.Phase, StartEventID: typed.StartEventID, NextPhase: typed.NextPhase, Outcome: typed.Outcome, Activity: typed.Activity, ImplementationMode: typed.ImplementationMode, RouteAction: typed.RouteAction, Route: typed.Route})
	case TrajectoryLifecycleRequest:
		base = typed.LifecycleRequestBase
		kind = map[string]EventKind{"start-trajectory": "trajectory_started", "resume-trajectory": "trajectory_resumed", "close-trajectory": "trajectory_closed"}[typed.Action]
		payload = marshal(TrajectoryPayload{TrajectoryID: typed.TrajectoryID, AnchorID: typed.AnchorID, Reason: typed.Reason})
	case ForkTrajectoryLifecycleRequest:
		base, kind = typed.LifecycleRequestBase, "trajectory_forked"
		payload = marshal(TrajectoryForkedPayload{TrajectoryID: typed.TrajectoryID, ParentTrajectoryID: typed.ParentTrajectoryID, ForkAnchorID: typed.ForkAnchorID})
	case TerminalLifecycleRequest:
		base, kind = typed.LifecycleRequestBase, "effort_completed"
		payload = marshal(EffortTerminalPayload{TerminalEpoch: 1})
	case AbandonLifecycleRequest:
		base, kind = typed.LifecycleRequestBase, "effort_abandoned"
		payload = marshal(EffortAbandonedPayload{TerminalEpoch: 1, Reason: typed.Reason})
	case ReopenLifecycleRequest:
		base, kind = typed.LifecycleRequestBase, "effort_reopened"
		payload = marshal(EffortReopenedPayload{TerminalEpoch: 2, TrajectoryID: typed.TrajectoryID, AnchorID: typed.AnchorID})
	case WaiveLifecycleRequest:
		base, kind = typed.LifecycleRequestBase, "finding_waived"
		payload = marshal(FindingWaivedPayload{RuleCode: typed.RuleCode, Scope: typed.Scope, EvidenceIDs: typed.EvidenceIDs, ReasonCode: typed.ReasonCode})
	case RepairLifecycleRequest:
		base, kind = typed.LifecycleRequestBase, "repair_applied"
		payload = marshal(RepairAppliedPayload{ProposalKind: typed.Proposal.Kind, SourceEventIDs: typed.Proposal.SourceEventIDs, Replacement: typed.Proposal.Replacement})
	default:
		return base, "", nil, metadata, false, errors.New("unknown lifecycle request type")
	}
	if kind == "" || base.Action != request.lifecycleAction() {
		return base, "", nil, metadata, false, errors.New("invalid lifecycle action for request type")
	}
	return base, kind, payload, metadata, creating, nil
}
