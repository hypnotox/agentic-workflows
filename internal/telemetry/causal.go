package telemetry

import (
	"encoding/json"
	"fmt"
	"sort"
)

// CausalOrder is the protocol partial order. It deliberately has no timestamp
// edge: timestamps measure causally linked intervals but never order writers.
type CausalOrder struct {
	events       map[string]EventEnvelope
	ancestors    map[string]map[string]bool
	frontiers    map[string][]string
	trajectories map[string]TrajectoryNode
}

type TrajectoryNode struct {
	ID                 string
	ParentTrajectoryID string
	ForkAnchorID       string
	EventID            string
}

func BuildCausalOrder(events []EventEnvelope) (*CausalOrder, []IntegrityIssue) {
	order := &CausalOrder{
		events:       make(map[string]EventEnvelope, len(events)),
		ancestors:    make(map[string]map[string]bool, len(events)),
		frontiers:    make(map[string][]string, len(events)),
		trajectories: make(map[string]TrajectoryNode),
	}
	issues := []IntegrityIssue{}
	previousSession := map[string]string{}
	// Anchor claims come only from the descriptor's anchorClaimKinds, keyed on the
	// payload anchor; the envelope piAnchorId is observation-location metadata and
	// never participates in resolution.
	claimKinds := stringSet(descriptor.Vocabularies["anchorClaimKinds"])
	anchorEvents := map[string]string{}
	ambiguousAnchors := map[string]bool{}
	trajectoryOrigins := map[string]string{}
	pendingAnchors := map[string]string{}
	for _, event := range events {
		order.events[event.EventID] = event
		if claimKinds[string(event.Kind)] {
			var claim struct {
				AnchorID string `json:"anchorId"`
			}
			if json.Unmarshal(event.Payload, &claim) == nil && claim.AnchorID != "" {
				if prior := anchorEvents[claim.AnchorID]; prior != "" && prior != event.EventID {
					ambiguousAnchors[claim.AnchorID] = true
				} else {
					anchorEvents[claim.AnchorID] = event.EventID
				}
			}
		}
		switch event.Kind {
		case "trajectory_started":
			var payload TrajectoryPayload
			if json.Unmarshal(event.Payload, &payload) == nil && trajectoryOrigins[payload.TrajectoryID] == "" {
				trajectoryOrigins[payload.TrajectoryID] = event.EventID
			}
		case "trajectory_forked":
			var payload TrajectoryForkedPayload
			if json.Unmarshal(event.Payload, &payload) == nil && trajectoryOrigins[payload.TrajectoryID] == "" {
				trajectoryOrigins[payload.TrajectoryID] = event.EventID
			}
		case "effort_reopened":
			var payload EffortReopenedPayload
			if json.Unmarshal(event.Payload, &payload) == nil && trajectoryOrigins[payload.TrajectoryID] == "" {
				trajectoryOrigins[payload.TrajectoryID] = event.EventID
			}
		}
	}
	for anchor := range ambiguousAnchors {
		delete(anchorEvents, anchor)
		issues = append(issues, integrity("ambiguous-anchor", anchor, 0, nil, "multiple events claim the same Pi anchor"))
	}
	for _, event := range events {
		frontier := append([]string(nil), event.Predecessors...)
		if prior := previousSession[event.SessionID]; prior != "" && !containsString(frontier, prior) {
			frontier = append(frontier, prior)
		}
		if event.Kind == "session_associated" {
			var payload SessionAssociatedPayload
			if json.Unmarshal(event.Payload, &payload) == nil && payload.HandoffEventID != "" && !containsString(frontier, payload.HandoffEventID) {
				frontier = append(frontier, payload.HandoffEventID)
			}
		}
		switch event.Kind {
		case "trajectory_resumed":
			var payload TrajectoryPayload
			if json.Unmarshal(event.Payload, &payload) == nil {
				if anchorEvent := anchorEvents[payload.AnchorID]; anchorEvent != "" && anchorEvent != event.EventID {
					pendingAnchors[event.EventID] = anchorEvent
				}
			}
		case "trajectory_forked":
			var payload TrajectoryForkedPayload
			if json.Unmarshal(event.Payload, &payload) == nil {
				if origin := trajectoryOrigins[payload.ParentTrajectoryID]; origin != "" && origin != event.EventID && !containsString(frontier, origin) {
					frontier = append(frontier, origin)
				}
				if anchorEvent := anchorEvents[payload.ForkAnchorID]; anchorEvent != "" && anchorEvent != event.EventID {
					pendingAnchors[event.EventID] = anchorEvent
				}
			}
		case "effort_reopened":
			var payload EffortReopenedPayload
			if json.Unmarshal(event.Payload, &payload) == nil {
				if anchorEvent := anchorEvents[payload.AnchorID]; anchorEvent != "" && anchorEvent != event.EventID {
					pendingAnchors[event.EventID] = anchorEvent
				}
			}
		}
		sort.Strings(frontier)
		order.frontiers[event.EventID] = frontier
		previousSession[event.SessionID] = event.EventID
	}
	computeAncestors := func(record bool) {
		order.ancestors = make(map[string]map[string]bool, len(order.events))
		visiting := map[string]bool{}
		visited := map[string]bool{}
		var visit func(string) map[string]bool
		visit = func(id string) map[string]bool {
			if visited[id] {
				return order.ancestors[id]
			}
			if visiting[id] {
				if record {
					issues = append(issues, integrity("causal-cycle", id, 0, []string{id}, "causal predecessor cycle"))
				}
				return map[string]bool{}
			}
			visiting[id] = true
			ancestors := map[string]bool{}
			for _, predecessor := range order.frontiers[id] {
				if _, ok := order.events[predecessor]; !ok {
					if record {
						issues = append(issues, integrity("broken-predecessor", id, 0, []string{id, predecessor}, "causal predecessor does not exist"))
					}
					continue
				}
				ancestors[predecessor] = true
				for ancestor := range visit(predecessor) {
					ancestors[ancestor] = true
				}
			}
			delete(visiting, id)
			visited[id] = true
			order.ancestors[id] = ancestors
			return ancestors
		}
		for _, event := range events {
			visit(event.EventID)
		}
	}
	// Anchor references resolve causally forward only: an edge lands only when the
	// base order (everything except anchor edges) does not already place the claim
	// after the referencing event, so a later close at a revisited entry cannot
	// invert real order into a manufactured cycle.
	computeAncestors(false)
	for eventID, anchorEvent := range pendingAnchors {
		if !order.HappensBefore(eventID, anchorEvent) && !containsString(order.frontiers[eventID], anchorEvent) {
			order.frontiers[eventID] = append(order.frontiers[eventID], anchorEvent)
			sort.Strings(order.frontiers[eventID])
		}
	}
	computeAncestors(true)
	for _, event := range events {
		switch event.Kind {
		case "trajectory_started", "trajectory_resumed", "trajectory_closed":
			var payload TrajectoryPayload
			if json.Unmarshal(event.Payload, &payload) == nil {
				if _, exists := order.trajectories[payload.TrajectoryID]; !exists {
					order.trajectories[payload.TrajectoryID] = TrajectoryNode{ID: payload.TrajectoryID, EventID: event.EventID}
				}
			}
		case "trajectory_forked":
			var payload TrajectoryForkedPayload
			if json.Unmarshal(event.Payload, &payload) == nil {
				order.trajectories[payload.TrajectoryID] = TrajectoryNode{ID: payload.TrajectoryID, ParentTrajectoryID: payload.ParentTrajectoryID, ForkAnchorID: payload.ForkAnchorID, EventID: event.EventID}
			}
		case "effort_reopened":
			var payload EffortReopenedPayload
			if json.Unmarshal(event.Payload, &payload) == nil {
				order.trajectories[payload.TrajectoryID] = TrajectoryNode{ID: payload.TrajectoryID, EventID: event.EventID}
			}
		}
	}
	return order, deduplicateIntegrity(issues)
}

func (o *CausalOrder) HappensBefore(left, right string) bool {
	return left != right && o.ancestors[right][left]
}

func (o *CausalOrder) Comparable(left, right string) bool {
	return left == right || o.HappensBefore(left, right) || o.HappensBefore(right, left)
}

func (o *CausalOrder) Concurrent(left, right string) bool {
	_, leftOK := o.events[left]
	_, rightOK := o.events[right]
	return leftOK && rightOK && left != right && !o.Comparable(left, right)
}

func (o *CausalOrder) ordered(events []EventEnvelope) []EventEnvelope {
	byID := make(map[string]EventEnvelope, len(events))
	remaining := make(map[string]int, len(events))
	children := make(map[string][]string, len(events))
	for _, event := range events {
		byID[event.EventID] = event
		remaining[event.EventID] = 0
		for _, predecessor := range o.frontiers[event.EventID] {
			if _, exists := o.events[predecessor]; exists {
				remaining[event.EventID]++
				children[predecessor] = append(children[predecessor], event.EventID)
			}
		}
	}
	ready := []string{}
	for id, count := range remaining {
		if count == 0 {
			ready = append(ready, id)
		}
	}
	result := make([]EventEnvelope, 0, len(events))
	for len(ready) != 0 {
		sort.Strings(ready)
		id := ready[0]
		ready = ready[1:]
		result = append(result, byID[id])
		for _, child := range children[id] {
			remaining[child]--
			if remaining[child] == 0 {
				ready = append(ready, child)
			}
		}
	}
	// Cyclic evidence was already reported. Keep it deterministic and available
	// to validation, where its claimed effect will be rejected.
	if len(result) != len(events) {
		seen := map[string]bool{}
		for _, event := range result {
			seen[event.EventID] = true
		}
		left := []string{}
		for id := range byID {
			if !seen[id] {
				left = append(left, id)
			}
		}
		sort.Strings(left)
		for _, id := range left {
			result = append(result, byID[id])
		}
	}
	return result
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func equalStrings(left, right []string) bool {
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

func deduplicateIntegrity(issues []IntegrityIssue) []IntegrityIssue {
	seen := map[string]bool{}
	result := make([]IntegrityIssue, 0, len(issues))
	for _, issue := range issues {
		key := issue.Code + "\x00" + issue.Scope + "\x00" + fmt.Sprint(issue.EventIDs)
		if !seen[key] {
			seen[key] = true
			result = append(result, issue)
		}
	}
	return result
}
