// Package currentstate validates parsed ADR application authority and topics.
package currentstate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

type Severity int

const (
	Error Severity = iota
	Warn
)

func (s Severity) String() string {
	if s == Warn {
		return "warn"
	}
	return "error"
}

type Finding struct {
	Severity Severity
	Message  string
}

type projectedADR struct {
	record   adr.ADR
	batches  []adr.ApplicationBatch
	progress adr.OperationProgress
}

type operationAt struct {
	owner adr.ADR
	op    adr.Operation
	seq   int
}

// Check validates application sequences, operation history, forward results,
// and inverse provenance. Parsed record formats identify the legacy bootstrap.
func Check(records []adr.ADR, corpusTopics []topic.Topic) []Finding {
	claims := map[string]topic.Claim{}
	topics := map[string]bool{}
	for _, t := range corpusTopics {
		topics[t.ID.String()] = true
		for _, c := range t.Claims {
			claims[c.ID] = c
		}
	}
	projected, projectionFindings := projectADRs(records)
	applied := appliedOperations(projected)
	removed := removedSet(applied)
	findings := append([]Finding(nil), projectionFindings...)
	findings = append(findings, checkSequences(projected)...)
	findings = append(findings, checkOperationHistory(applied, hasLegacyRecord(records))...)
	findings = append(findings, checkForward(projected, claims, topics, removed)...)
	findings = append(findings, checkBackward(records, applied, claims)...)
	sort.Slice(findings, func(i, j int) bool { return findings[i].Message < findings[j].Message })
	return findings
}

func hasLegacyRecord(records []adr.ADR) bool {
	for _, record := range records {
		if !record.IsGoverned() {
			return true
		}
	}
	return false
}

func legacyOrigin(records []adr.ADR, number string) bool {
	for _, record := range records {
		if record.Number == number {
			return !record.IsGoverned()
		}
	}
	return false
}

func projectADRs(records []adr.ADR) ([]projectedADR, []Finding) {
	var projected []projectedADR
	var findings []Finding
	for _, a := range records {
		if !a.IsGoverned() {
			continue
		}
		batches, err := a.ApplicationBatches()
		if err != nil {
			findings = append(findings, Finding{Error, err.Error()})
			continue
		}
		progress, err := a.OperationProgress()
		if err != nil {
			findings = append(findings, Finding{Error, err.Error()})
			continue
		}
		projected = append(projected, projectedADR{record: a, batches: batches, progress: progress})
	}
	return projected, findings
}

func appliedOperations(projected []projectedADR) []operationAt {
	var out []operationAt
	for _, p := range projected {
		for _, applied := range p.progress.Applied {
			out = append(out, operationAt{owner: p.record, op: applied.Operation, seq: applied.Sequence})
		}
	}
	return out
}

func checkSequences(projected []projectedADR) []Finding {
	bySeq := map[int][]string{}
	for _, p := range projected {
		for _, batch := range p.batches {
			bySeq[batch.Sequence] = append(bySeq[batch.Sequence], p.record.Number)
		}
	}
	var findings []Finding
	seqs := make([]int, 0, len(bySeq))
	for seq, nums := range bySeq {
		seqs = append(seqs, seq)
		if len(nums) > 1 {
			sort.Strings(nums)
			findings = append(findings, Finding{Error, fmt.Sprintf("state-sequence %d is used by more than one ADR batch: %v", seq, nums)})
		}
	}
	sort.Ints(seqs)
	for i, seq := range seqs {
		if seq != i+1 {
			findings = append(findings, Finding{Error, fmt.Sprintf("state-sequence values are not contiguous from 1: expected %d, found %d", i+1, seq)})
		}
	}
	return findings
}

func checkOperationHistory(applied []operationAt, hasLegacy bool) []Finding {
	byID := map[string][]operationAt{}
	for _, operation := range applied {
		byID[operation.op.ID] = append(byID[operation.op.ID], operation)
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var findings []Finding
	for _, id := range ids {
		ops := byID[id]
		sort.SliceStable(ops, func(i, j int) bool { return ops[i].seq < ops[j].seq })
		adds, removeIdx := 0, -1
		for i, operation := range ops {
			switch operation.op.Verb {
			case adr.OpAdd:
				adds++
			case adr.OpRemove:
				if removeIdx >= 0 {
					findings = append(findings, Finding{Error, fmt.Sprintf("claim %s has more than one remove", id)})
				}
				removeIdx = i
			case adr.OpUpdate:
				// Updates are legal between the add/baseline and terminal remove.
			}
		}
		legacyBaseline := hasLegacy && adds == 0 && ops[0].op.Verb != adr.OpAdd
		if adds != 1 && !legacyBaseline {
			findings = append(findings, Finding{Error, fmt.Sprintf("claim %s has %d add operations; require exactly one", id, adds)})
		}
		if ops[0].op.Verb != adr.OpAdd && !legacyBaseline {
			findings = append(findings, Finding{Error, fmt.Sprintf("claim %s history does not begin with an add", id)})
		}
		if removeIdx >= 0 && removeIdx != len(ops)-1 {
			findings = append(findings, Finding{Error, fmt.Sprintf("claim %s has an operation after its remove", id)})
		}
	}
	return findings
}

func removedSet(applied []operationAt) map[string]bool {
	removed := map[string]bool{}
	for _, operation := range applied {
		if operation.op.Verb == adr.OpRemove {
			removed[operation.op.ID] = true
		}
	}
	return removed
}

func checkForward(projected []projectedADR, claims map[string]topic.Claim, topics map[string]bool, removed map[string]bool) []Finding {
	var findings []Finding
	for _, p := range projected {
		needsTopic := p.record.ReachedAccepted() || len(p.progress.Applied) != 0 || p.record.IsImplemented()
		for _, applied := range p.progress.Applied {
			op := applied.Operation
			if needsTopic {
				findings = append(findings, checkTopic(p.record, op, topics)...)
			}
			claim, present := claims[op.ID]
			findings = append(findings, checkAppliedOp(p.record, op, claim, present, removed[op.ID])...)
		}
		for _, op := range p.progress.Canceled {
			if op.Verb == adr.OpAdd && removed[op.ID] {
				findings = append(findings, Finding{Error, fmt.Sprintf("ADR-%s adds removed claim %s, which may never be reused", p.record.Number, op.ID)})
			}
			if p.record.IsV1() && p.record.IsAbandoned() {
				claim, present := claims[op.ID]
				findings = append(findings, checkAbandonedOp(p.record, op, claim, present)...)
				if p.record.ReachedAccepted() {
					findings = append(findings, checkTopic(p.record, op, topics)...)
				}
			}
		}
		for _, op := range p.progress.Remaining {
			if needsTopic {
				findings = append(findings, checkTopic(p.record, op, topics)...)
			}
			if op.Verb == adr.OpAdd && removed[op.ID] {
				findings = append(findings, Finding{Error, fmt.Sprintf("ADR-%s adds removed claim %s, which may never be reused", p.record.Number, op.ID)})
			}
			_, present := claims[op.ID]
			findings = append(findings, checkPendingOp(p.record, op, present)...)
		}
	}
	return findings
}

func checkTopic(a adr.ADR, op adr.Operation, topics map[string]bool) []Finding {
	topicID, _, _ := strings.Cut(op.ID, ":")
	if !topics[topicID] {
		return []Finding{{Error, fmt.Sprintf("ADR-%s operation %s targets missing topic %s", a.Number, op.Verb, topicID)}}
	}
	return nil
}

func checkPendingOp(a adr.ADR, op adr.Operation, present bool) []Finding {
	if op.Verb == adr.OpAdd && present {
		return []Finding{{Error, fmt.Sprintf("pending ADR-%s adds claim %s, which already exists", a.Number, op.ID)}}
	}
	if op.Verb != adr.OpAdd && !present {
		return []Finding{{Error, fmt.Sprintf("pending ADR-%s %ss missing claim %s", a.Number, op.Verb, op.ID)}}
	}
	return nil
}

func checkAbandonedOp(a adr.ADR, op adr.Operation, claim topic.Claim, present bool) []Finding {
	switch op.Verb {
	case adr.OpAdd:
		if present && claim.Origin == a.Number {
			return []Finding{{Error, fmt.Sprintf("Abandoned ADR-%s add for claim %s was applied; it must be reverted", a.Number, op.ID)}}
		}
	case adr.OpUpdate:
		if present && contains(claim.RevisedBy, a.Number) {
			return []Finding{{Error, fmt.Sprintf("Abandoned ADR-%s update for claim %s was applied; it must be reverted", a.Number, op.ID)}}
		}
	case adr.OpRemove:
		// V1 removal attribution remains a CheckPair responsibility.
	}
	return nil
}

func checkAppliedOp(a adr.ADR, op adr.Operation, claim topic.Claim, present, wasRemoved bool) []Finding {
	label := a.Status
	switch op.Verb {
	case adr.OpAdd:
		if wasRemoved {
			return nil
		}
		if !present {
			return []Finding{{Error, fmt.Sprintf("%s ADR-%s adds claim %s, which has no active claim", label, a.Number, op.ID)}}
		}
		if claim.Origin != a.Number {
			return []Finding{{Error, fmt.Sprintf("claim %s Origin is ADR-%s, not the adding ADR-%s", op.ID, claim.Origin, a.Number)}}
		}
	case adr.OpUpdate:
		if wasRemoved {
			return nil
		}
		if !present {
			return []Finding{{Error, fmt.Sprintf("%s ADR-%s updates claim %s, which has no active claim", label, a.Number, op.ID)}}
		}
		if !contains(claim.RevisedBy, a.Number) {
			return []Finding{{Error, fmt.Sprintf("claim %s does not list updating ADR-%s in Revised-by", op.ID, a.Number)}}
		}
	case adr.OpRemove:
		if present {
			return []Finding{{Error, fmt.Sprintf("%s ADR-%s removes claim %s, which still has an active claim", label, a.Number, op.ID)}}
		}
	}
	return nil
}

func checkBackward(records []adr.ADR, applied []operationAt, claims map[string]topic.Claim) []Finding {
	byOperation := map[string]operationAt{}
	for _, operation := range applied {
		key := operation.owner.Number + "\x00" + string(operation.op.Verb) + "\x00" + operation.op.ID
		byOperation[key] = operation
	}
	byNum := map[string]adr.ADR{}
	for _, a := range records {
		byNum[a.Number] = a
	}
	ids := make([]string, 0, len(claims))
	for id := range claims {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var findings []Finding
	for _, id := range ids {
		claim := claims[id]
		originKey := claim.Origin + "\x00" + string(adr.OpAdd) + "\x00" + id
		origin, hasOrigin := byOperation[originKey]
		if !legacyOrigin(records, claim.Origin) && !hasOrigin {
			findings = append(findings, Finding{Error, fmt.Sprintf("claim %s names Origin ADR-%s, which has no matching add operation applied", id, claim.Origin)})
		}
		lastSequence := 0
		if hasOrigin {
			lastSequence = origin.seq
		} else if _, ok := byNum[claim.Origin]; ok {
			lastSequence = 0
		}
		for _, rev := range claim.RevisedBy {
			key := rev + "\x00" + string(adr.OpUpdate) + "\x00" + id
			operation, ok := byOperation[key]
			if !ok {
				findings = append(findings, Finding{Error, fmt.Sprintf("claim %s names Revised-by ADR-%s, which has no matching update operation applied", id, rev)})
				continue
			}
			if operation.seq <= lastSequence {
				findings = append(findings, Finding{Error, fmt.Sprintf("claim %s Revised-by entries are not in increasing State-sequence order at ADR-%s", id, rev)})
			}
			lastSequence = operation.seq
		}
	}
	return findings
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
