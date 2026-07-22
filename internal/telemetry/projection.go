package telemetry

import (
	"encoding/json"
	"sort"
)

type TrajectoryProjection struct {
	TrajectoryID       string
	ParentTrajectoryID string
	ForkAnchorID       string
	EventIDs           []string
	Discarded          bool
}

type WorkflowProjection struct {
	Metadata            EffortMetadata
	Lifecycle           LifecycleProjection
	ActiveAncestry      []string
	EvidenceEventIDs    []string
	EffectApplied       map[string]bool
	CurrentPathEventIDs []string
	AllWorkEventIDs     []string
	DiscardedEventIDs   []string
	Trajectories        []TrajectoryProjection
	DerivedEffortIDs    []string
	Origin              *OriginMetadata
	Integrity           []IntegrityIssue
}

func ProjectWorkflow(read EffortRead) WorkflowProjection {
	lifecycle := projectLifecycleFromRead(read)
	projection := WorkflowProjection{
		Metadata: read.Metadata, Lifecycle: lifecycle, Origin: read.Metadata.Origin,
		EvidenceEventIDs: []string{}, EffectApplied: map[string]bool{},
		CurrentPathEventIDs: []string{}, AllWorkEventIDs: []string{}, DiscardedEventIDs: []string{},
		Trajectories: []TrajectoryProjection{}, DerivedEffortIDs: []string{},
		Integrity: append(append([]IntegrityIssue(nil), read.Integrity...), lifecycle.Invalid...),
	}
	active := map[string]bool{}
	for trajectoryID := lifecycle.ActiveTrajectoryID; trajectoryID != ""; trajectoryID = lifecycle.Trajectories[trajectoryID].ParentTrajectoryID {
		if active[trajectoryID] { // coverage-ignore: validated trajectory creation and fork rules guarantee acyclic ancestry
			break
		}
		active[trajectoryID] = true
		projection.ActiveAncestry = append(projection.ActiveAncestry, trajectoryID)
	}
	for left, right := 0, len(projection.ActiveAncestry)-1; left < right; left, right = left+1, right-1 {
		projection.ActiveAncestry[left], projection.ActiveAncestry[right] = projection.ActiveAncestry[right], projection.ActiveAncestry[left]
	}
	byTrajectory := map[string][]string{}
	order, _ := BuildCausalOrder(read.Events)
	for _, event := range read.Events {
		projection.EvidenceEventIDs = append(projection.EvidenceEventIDs, event.EventID)
		applied := descriptor.Payloads[string(event.Kind)].Class == "passive" || lifecycle.EffectApplied[event.EventID]
		if recorded, exists := read.EffectApplied[event.EventID]; exists {
			applied = recorded
		}
		projection.EffectApplied[event.EventID] = applied
		if !applied {
			continue
		}
		projection.AllWorkEventIDs = append(projection.AllWorkEventIDs, event.EventID)
		trajectoryID := event.TrajectoryID
		if trajectoryID == "" {
			trajectoryID = causallyVisibleAssociationTrajectory(event, read.Events, order, lifecycle.EffectApplied)
		}
		if trajectoryID != "" {
			byTrajectory[trajectoryID] = append(byTrajectory[trajectoryID], event.EventID)
		}
		if trajectoryID == "" || active[trajectoryID] {
			projection.CurrentPathEventIDs = append(projection.CurrentPathEventIDs, event.EventID)
		} else {
			projection.DiscardedEventIDs = append(projection.DiscardedEventIDs, event.EventID)
		}
	}
	for trajectoryID, node := range lifecycle.Trajectories {
		eventIDs := append([]string(nil), byTrajectory[trajectoryID]...)
		sort.Strings(eventIDs)
		projection.Trajectories = append(projection.Trajectories, TrajectoryProjection{TrajectoryID: trajectoryID, ParentTrajectoryID: node.ParentTrajectoryID, ForkAnchorID: node.ForkAnchorID, EventIDs: eventIDs, Discarded: !active[trajectoryID]})
	}
	sort.Slice(projection.Trajectories, func(i, j int) bool {
		return projection.Trajectories[i].TrajectoryID < projection.Trajectories[j].TrajectoryID
	})
	sort.Strings(projection.EvidenceEventIDs)
	sort.Strings(projection.CurrentPathEventIDs)
	sort.Strings(projection.AllWorkEventIDs)
	sort.Strings(projection.DiscardedEventIDs)
	projection.Integrity = deduplicateIntegrity(projection.Integrity)
	return projection
}

func causallyVisibleAssociationTrajectory(event EventEnvelope, events []EventEnvelope, order *CausalOrder, effectApplied map[string]bool) string {
	var latest *EventEnvelope
	for index := range events {
		candidate := &events[index]
		if !effectApplied[candidate.EventID] || candidate.SessionID != event.SessionID || (candidate.Kind != "session_associated" && candidate.Kind != "session_detached") || !order.HappensBefore(candidate.EventID, event.EventID) {
			continue
		}
		if latest == nil || order.HappensBefore(latest.EventID, candidate.EventID) {
			latest = candidate
		}
	}
	if latest == nil || latest.Kind == "session_detached" {
		return ""
	}
	var payload SessionAssociatedPayload
	_ = json.Unmarshal(latest.Payload, &payload)
	return payload.TrajectoryID
}

// GroupDerivedFamilies annotates derived descendants. Each effort remains a
// separate projection, so callers can group lineage without adding parent work
// to a child and double-counting its totals.
func GroupDerivedFamilies(projections []WorkflowProjection) []WorkflowProjection {
	result := append([]WorkflowProjection(nil), projections...)
	index := map[string]int{}
	origins := map[string]string{}
	for i := range result {
		result[i].DerivedEffortIDs = []string{}
		index[result[i].Metadata.EffortID] = i
		if result[i].Origin != nil {
			origins[result[i].Metadata.EffortID] = result[i].Origin.EffortID
		}
	}
	for _, projection := range result {
		seen := map[string]bool{}
		for ancestorID := origins[projection.Metadata.EffortID]; ancestorID != "" && !seen[ancestorID]; ancestorID = origins[ancestorID] {
			if ancestorID == projection.Metadata.EffortID {
				break
			}
			seen[ancestorID] = true
			if ancestor, ok := index[ancestorID]; ok {
				result[ancestor].DerivedEffortIDs = append(result[ancestor].DerivedEffortIDs, projection.Metadata.EffortID)
			}
		}
	}
	for i := range result {
		sort.Strings(result[i].DerivedEffortIDs)
	}
	return result
}
