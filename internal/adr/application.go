package adr

import (
	"fmt"
	"slices"
)

// ApplicationBatch is one implicit or explicit application of declared state
// operations. Operations are retained in declaration/event order.
type ApplicationBatch struct {
	Sequence   int
	Operations []Operation
	Implicit   bool
}

// AppliedOperation is one applied declaration and its inherited batch sequence.
type AppliedOperation struct {
	Operation Operation
	Sequence  int
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
	for _, event := range a.History {
		if event.Kind == HistoryApplied {
			if !a.IsV2() {
				return nil, fmt.Errorf("ADR-%s has an Applied event outside current-state-v2", a.Number)
			}
			batches = append(batches, ApplicationBatch{
				Sequence: event.Sequence, Operations: slices.Clone(event.Operations),
			})
		}
	}
	if len(batches) != 0 {
		if a.IsImplemented() {
			for _, event := range a.History {
				if event.Kind == HistoryStatus && event.Status == statusImplemented && event.HasSequence {
					return nil, fmt.Errorf("ADR-%s mixes explicit Applied events with implicit terminal sequencing", a.Number)
				}
			}
		}
		return batches, nil
	}
	if !a.IsImplemented() || len(a.Operations) == 0 {
		return batches, nil
	}
	for i := len(a.History) - 1; i >= 0; i-- {
		event := a.History[i]
		if (event.Kind == HistoryStatus || (a.IsV1() && event.Kind == 0)) && event.Status == statusImplemented {
			if !event.HasSequence {
				return nil, fmt.Errorf("ADR-%s Implemented status has no state-sequence", a.Number)
			}
			return []ApplicationBatch{{Sequence: event.Sequence, Operations: slices.Clone(a.Operations), Implicit: true}}, nil
		}
	}
	return nil, fmt.Errorf("ADR-%s has no Implemented status event", a.Number)
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
	for _, batch := range batches {
		if batch.Sequence < 1 || len(batch.Operations) == 0 {
			return OperationProgress{}, fmt.Errorf("ADR-%s has an invalid application batch", a.Number)
		}
		for _, op := range batch.Operations {
			if _, ok := declared[op]; !ok {
				return OperationProgress{}, fmt.Errorf("ADR-%s applies undeclared operation %s `%s`", a.Number, op.Verb, op.ID)
			}
			if applied[op] {
				return OperationProgress{}, fmt.Errorf("ADR-%s applies operation %s `%s` more than once", a.Number, op.Verb, op.ID)
			}
			applied[op] = true
			progress.Applied = append(progress.Applied, AppliedOperation{Operation: op, Sequence: batch.Sequence})
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
			return OperationProgress{}, fmt.Errorf("ADR-%s status %s cannot have applied operations", a.Number, a.Status)
		}
		progress.Remaining = slices.Clone(a.Operations)
	case statusImplementing:
		if len(progress.Applied) == 0 || len(complement) == 0 {
			return OperationProgress{}, fmt.Errorf("ADR-%s Implementing status requires applied and remaining operations", a.Number)
		}
		progress.Remaining = slices.Clone(complement)
	case statusImplemented:
		if len(complement) != 0 {
			return OperationProgress{}, fmt.Errorf("ADR-%s Implemented status has %d remaining operations", a.Number, len(complement))
		}
	case statusAbandoned:
		progress.Canceled = slices.Clone(complement)
	default:
		return OperationProgress{}, fmt.Errorf("ADR-%s has unsupported governed status %q", a.Number, a.Status)
	}
	return progress, nil
}
