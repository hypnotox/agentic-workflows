package adr

import (
	"fmt"
	"slices"
)

// ApplicationBatch is one implicit or explicit application of declared state
// operations. Operations are retained in declaration/event order.
type ApplicationBatch struct {
	Operations   []Operation
	Kind         HistoryEventKind
	HistoryIndex int
	Implicit     bool
}

// AppliedOperation is one applied declaration and the index of its batch in
// the owning ADR's history; per-claim cross-ADR order is ascending ADR number
// (ADR-0191), never derived from batch positions.
type AppliedOperation struct {
	Operation  Operation
	BatchIndex int
}

// OperationProgress partitions an ADR's declarations by application state.
type OperationProgress struct {
	Applied   []AppliedOperation
	Remaining []Operation
	Canceled  []Operation
}

// ApplicationBatches projects the application records owned by a governed ADR.
func (a ADR) ApplicationBatches() ([]ApplicationBatch, error) {
	if !a.IsGoverned() {
		return []ApplicationBatch{}, nil
	}
	batches := []ApplicationBatch{}
	for historyIndex, event := range a.History {
		if event.Kind == HistoryApplied || event.Kind == HistoryReapplied {
			if !a.HasV2Semantics() {
				return nil, fmt.Errorf("ADR-%s has an Applied event outside current-state-v2", a.Identity())
			}
			batches = append(batches, ApplicationBatch{
				Operations: slices.Clone(event.Operations),
				Kind:       event.Kind, HistoryIndex: historyIndex,
			})
		}
	}
	if len(batches) != 0 {
		return batches, nil
	}
	if !a.IsImplemented() || len(a.Operations) == 0 {
		return batches, nil
	}
	for i := len(a.History) - 1; i >= 0; i-- {
		event := a.History[i]
		if (event.Kind == HistoryStatus || (a.IsV1() && event.Kind == 0)) && event.Status == statusImplemented {
			return []ApplicationBatch{{
				Operations: slices.Clone(a.Operations), Kind: HistoryApplied,
				HistoryIndex: i, Implicit: true,
			}}, nil
		}
	}
	return nil, fmt.Errorf("ADR-%s has no Implemented status event", a.Identity())
}

// OperationProgress projects declared operations into applied, remaining, and
// canceled partitions without inferring removal from claim absence.
func (a ADR) OperationProgress() (OperationProgress, error) {
	progress := OperationProgress{Applied: []AppliedOperation{}, Remaining: []Operation{}, Canceled: []Operation{}}
	if !a.IsGoverned() {
		return progress, nil
	}
	batches, err := a.ApplicationBatches()
	if err != nil {
		return OperationProgress{}, err
	}
	declared := make(map[Operation]int, len(a.Operations))
	for i, op := range a.Operations {
		declared[op] = i
	}
	applied := make(map[Operation]bool, len(a.Operations))
	for i, batch := range batches {
		if len(batch.Operations) == 0 {
			return OperationProgress{}, fmt.Errorf("ADR-%s has an invalid application batch", a.Identity())
		}
		for _, op := range batch.Operations {
			if _, ok := declared[op]; !ok {
				return OperationProgress{}, fmt.Errorf("ADR-%s applies undeclared operation %s `%s`", a.Identity(), op.Verb, op.ID)
			}
			switch batch.Kind {
			case HistoryApplied:
				if applied[op] {
					return OperationProgress{}, fmt.Errorf("ADR-%s applies operation %s `%s` more than once; use a Reapplied event for a correction", a.Identity(), op.Verb, op.ID)
				}
				applied[op] = true
				progress.Applied = append(progress.Applied, AppliedOperation{Operation: op, BatchIndex: i})
			case HistoryReapplied:
				if !applied[op] {
					return OperationProgress{}, fmt.Errorf("ADR-%s reapplies operation %s `%s` without an earlier Applied occurrence", a.Identity(), op.Verb, op.ID)
				}
				if op.Verb == OpRemove {
					return OperationProgress{}, fmt.Errorf("ADR-%s reapplies remove operation `%s`; only add or update may be reapplied", a.Identity(), op.ID)
				}
			default: // coverage-ignore: ApplicationBatches emits only Applied or Reapplied kinds
				return OperationProgress{}, fmt.Errorf("ADR-%s has an invalid application batch kind", a.Identity())
			}
		}
	}
	var complement []Operation
	for _, op := range a.Operations {
		if !applied[op] {
			complement = append(complement, op)
		}
	}
	switch a.Status {
	case statusProposed, statusAccepted:
		if len(progress.Applied) != 0 {
			return OperationProgress{}, fmt.Errorf("ADR-%s status %s cannot have applied operations", a.Identity(), a.Status)
		}
		progress.Remaining = slices.Clone(a.Operations)
	case statusImplementing:
		if len(progress.Applied) == 0 || len(complement) == 0 {
			return OperationProgress{}, fmt.Errorf("ADR-%s Implementing status requires applied and remaining operations", a.Identity())
		}
		progress.Remaining = slices.Clone(complement)
	case statusImplemented:
		if len(complement) != 0 {
			return OperationProgress{}, fmt.Errorf("ADR-%s Implemented status has %d remaining operations", a.Identity(), len(complement))
		}
	case statusAbandoned:
		progress.Canceled = slices.Clone(complement)
	default:
		return OperationProgress{}, fmt.Errorf("ADR-%s has unsupported governed status %q", a.Identity(), a.Status)
	}
	return progress, nil
}
