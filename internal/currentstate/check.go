// Package currentstate validates parsed ADR application authority and topics.
package currentstate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// Finding is one current-state claim-handshake violation. It carries no rank:
// every provenance and transition violation is structural and blocking, so a
// rank field would have exactly one representable useful value (ADR-0183 item 5).
type Finding struct {
	Message string
}

type projectedADR struct {
	record   adr.ADR
	batches  []adr.ApplicationBatch
	progress adr.OperationProgress
}

type operationAt struct {
	owner adr.ADR
	op    adr.Operation
	// order is the owning record's provenance rank: its number when numbered,
	// and a rank above every number when pending, because a pending record
	// takes the corpus's next numbers at integration (ADR-0194 item 10).
	order    int
	batchIdx int
}

// Check validates retired-segment absence, operation history, forward
// results, and inverse provenance. Parsed record formats identify the legacy
// bootstrap.
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
	retiredTopics := retiredTopicOperations(applied, topics)
	findings := append([]Finding(nil), projectionFindings...)
	findings = append(findings, checkLegacySegments(records)...)
	findings = append(findings, checkOperationHistory(applied, hasLegacyRecord(records))...)
	findings = append(findings, checkForward(projected, claims, topics, retiredTopics, removed)...)
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
		if record.Identity() == number {
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
			findings = append(findings, Finding{err.Error()})
			continue
		}
		progress, err := a.OperationProgress()
		if err != nil {
			findings = append(findings, Finding{err.Error()})
			continue
		}
		projected = append(projected, projectedADR{record: a, batches: batches, progress: progress})
	}
	return projected, findings
}

func appliedOperations(projected []projectedADR) []operationAt {
	var out []operationAt
	for _, p := range projected {
		order := adr.IdentityOrder(p.record.Identity())
		for _, applied := range p.progress.Applied {
			out = append(out, operationAt{owner: p.record, op: applied.Operation, order: order, batchIdx: applied.BatchIndex})
		}
	}
	return out
}

// checkLegacySegments reports every tolerated-and-discarded state-sequence
// segment: the namespace is retired (ADR-0191) and the migration strips the
// encoding, so a surviving segment means the tree needs awf upgrade.
func checkLegacySegments(records []adr.ADR) []Finding {
	var findings []Finding
	for _, record := range records {
		for _, event := range record.History {
			if event.LegacySequence {
				findings = append(findings, Finding{fmt.Sprintf("ADR-%s carries a retired state-sequence segment; run awf upgrade", record.Identity())})
				break
			}
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
		sort.SliceStable(ops, func(i, j int) bool {
			if ops[i].order != ops[j].order {
				return ops[i].order < ops[j].order
			}
			return ops[i].batchIdx < ops[j].batchIdx
		})
		adds, removeIdx := 0, -1
		for i, operation := range ops {
			switch operation.op.Verb {
			case adr.OpAdd:
				adds++
			case adr.OpRemove:
				if removeIdx >= 0 {
					findings = append(findings, Finding{fmt.Sprintf("claim %s has more than one remove", id)})
				}
				removeIdx = i
			case adr.OpUpdate:
				// Updates are legal between the add/baseline and terminal remove.
			}
		}
		legacyBaseline := hasLegacy && adds == 0 && ops[0].op.Verb != adr.OpAdd
		if adds != 1 && !legacyBaseline {
			findings = append(findings, Finding{fmt.Sprintf("claim %s has %d add operations; require exactly one", id, adds)})
		}
		if ops[0].op.Verb != adr.OpAdd && !legacyBaseline {
			findings = append(findings, Finding{fmt.Sprintf("claim %s history does not begin with an add", id)})
		}
		for i := removeIdx + 1; removeIdx >= 0 && i < len(ops); i++ {
			if ops[i].op.Verb == adr.OpAdd {
				findings = append(findings, Finding{fmt.Sprintf("claim %s has an add after its remove; a removed claim id is never reused", id)})
			}
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

func checkForward(projected []projectedADR, claims map[string]topic.Claim, topics, retiredTopics map[string]bool, removed map[string]bool) []Finding {
	var findings []Finding
	for _, p := range projected {
		needsTopic := p.record.ReachedAccepted() || len(p.progress.Applied) != 0 || p.record.IsImplemented()
		for _, applied := range p.progress.Applied {
			op := applied.Operation
			if needsTopic {
				findings = append(findings, checkTopic(p.record, op, topics, retiredTopics[op.ID])...)
			}
			claim, present := claims[op.ID]
			findings = append(findings, checkAppliedOp(p.record, op, claim, present, removed[op.ID])...)
		}
		for _, op := range p.progress.Canceled {
			if op.Verb == adr.OpAdd && removed[op.ID] {
				findings = append(findings, Finding{fmt.Sprintf("ADR-%s adds removed claim %s, which may never be reused", p.record.Identity(), op.ID)})
			}
			if p.record.IsV1() && p.record.IsAbandoned() {
				claim, present := claims[op.ID]
				findings = append(findings, checkAbandonedOp(p.record, op, claim, present)...)
				if p.record.ReachedAccepted() {
					findings = append(findings, checkTopic(p.record, op, topics, false)...)
				}
			}
		}
		for _, op := range p.progress.Remaining {
			if needsTopic {
				findings = append(findings, checkTopic(p.record, op, topics, false)...)
			}
			if op.Verb == adr.OpAdd && removed[op.ID] {
				findings = append(findings, Finding{fmt.Sprintf("ADR-%s adds removed claim %s, which may never be reused", p.record.Identity(), op.ID)})
			}
			_, present := claims[op.ID]
			findings = append(findings, checkPendingOp(p.record, op, present)...)
		}
	}
	return findings
}

func checkTopic(a adr.ADR, op adr.Operation, topics map[string]bool, retired bool) []Finding {
	topicID, _, _ := strings.Cut(op.ID, ":")
	if !topics[topicID] && !retired {
		return []Finding{{fmt.Sprintf("ADR-%s operation %s targets missing topic %s", a.Identity(), op.Verb, topicID)}}
	}
	return nil
}

// retiredTopicOperations identifies applied claims that fully document a topic
// retired with its final claim. Their historical operations require no active
// topic metadata, unlike pending or canceled operations.
func retiredTopicOperations(applied []operationAt, topics map[string]bool) map[string]bool {
	byTopic := map[string]map[string][]operationAt{}
	for _, operation := range applied {
		topicID, _, _ := strings.Cut(operation.op.ID, ":")
		if topics[topicID] {
			continue
		}
		if byTopic[topicID] == nil {
			byTopic[topicID] = map[string][]operationAt{}
		}
		byTopic[topicID][operation.op.ID] = append(byTopic[topicID][operation.op.ID], operation)
	}
	retired := map[string]bool{}
	for _, histories := range byTopic {
		complete := true
		for _, history := range histories {
			sort.SliceStable(history, func(i, j int) bool {
				if history[i].order != history[j].order {
					return history[i].order < history[j].order
				}
				return history[i].batchIdx < history[j].batchIdx
			})
			adds, removes := 0, 0
			for _, operation := range history {
				if operation.op.Verb == adr.OpAdd {
					adds++
				}
				if operation.op.Verb == adr.OpRemove {
					removes++
				}
			}
			// With exactly one add first and one remove, any trailing
			// operation is necessarily a dominated update, which retirement
			// tolerates (ADR-0191).
			if history[0].op.Verb != adr.OpAdd || removes != 1 || adds != 1 {
				complete = false
			}
		}
		if complete {
			for _, history := range histories {
				for _, operation := range history {
					retired[operation.op.ID] = true
				}
			}
		}
	}
	return retired
}

func checkPendingOp(a adr.ADR, op adr.Operation, present bool) []Finding {
	if op.Verb == adr.OpAdd && present {
		return []Finding{{fmt.Sprintf("pending ADR-%s adds claim %s, which already exists", a.Identity(), op.ID)}}
	}
	if op.Verb != adr.OpAdd && !present {
		return []Finding{{fmt.Sprintf("pending ADR-%s %ss missing claim %s", a.Identity(), op.Verb, op.ID)}}
	}
	return nil
}

func checkAbandonedOp(a adr.ADR, op adr.Operation, claim topic.Claim, present bool) []Finding {
	switch op.Verb {
	case adr.OpAdd:
		if present && claim.Origin == a.Identity() {
			return []Finding{{fmt.Sprintf("Abandoned ADR-%s add for claim %s was applied; it must be reverted", a.Identity(), op.ID)}}
		}
	case adr.OpUpdate:
		if present && contains(claim.RevisedBy, a.Identity()) {
			return []Finding{{fmt.Sprintf("Abandoned ADR-%s update for claim %s was applied; it must be reverted", a.Identity(), op.ID)}}
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
			return []Finding{{fmt.Sprintf("%s ADR-%s adds claim %s, which has no active claim", label, a.Identity(), op.ID)}}
		}
		if claim.Origin != a.Identity() {
			return []Finding{{fmt.Sprintf("claim %s Origin is ADR-%s, not the adding ADR-%s", op.ID, claim.Origin, a.Identity())}}
		}
	case adr.OpUpdate:
		if wasRemoved {
			return nil
		}
		if !present {
			return []Finding{{fmt.Sprintf("%s ADR-%s updates claim %s, which has no active claim", label, a.Identity(), op.ID)}}
		}
		if !contains(claim.RevisedBy, a.Identity()) {
			return []Finding{{fmt.Sprintf("claim %s does not list updating ADR-%s in Revised-by", op.ID, a.Identity())}}
		}
	case adr.OpRemove:
		if present {
			return []Finding{{fmt.Sprintf("%s ADR-%s removes claim %s, which still has an active claim", label, a.Identity(), op.ID)}}
		}
	}
	return nil
}

// checkBackward validates each claim's authored provenance against the applied
// operation set. The index key is the owning record's identity: its number when
// numbered, its slug while pending (ADR-0194 item 4).
//
// A slug-form entry resolves only against a pending record's slug, never
// against the retained slug of an already-numbered one, which is what forces
// numbering's substitution to be complete: a leftover slug reference is an
// error the moment its record takes a number. Order: numeric entries stay
// strictly ascending, a slug entry is legal only after every numeric entry, and
// slug entries compare in authored list order among themselves. When the Origin
// is itself a slug, its greater-than-Origin comparison is deferred to numbering,
// which the command's add-before-revise refusal guarantees (ADR-0194 item 10).
func checkBackward(records []adr.ADR, applied []operationAt, claims map[string]topic.Claim) []Finding {
	byOperation := map[string]operationAt{}
	for _, operation := range applied {
		key := operation.owner.Identity() + "\x00" + string(operation.op.Verb) + "\x00" + operation.op.ID
		byOperation[key] = operation
	}
	pendingSlugs := map[string]bool{}
	for _, record := range records {
		if record.IsPending() {
			pendingSlugs[record.Slug] = true
		}
	}
	ids := make([]string, 0, len(claims))
	for id := range claims {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var findings []Finding
	for _, id := range ids {
		claim := claims[id]
		originIsSlug := adr.IsSlugIdentity(claim.Origin)
		var origin operationAt
		var hasOrigin bool
		if originIsSlug && !pendingSlugs[claim.Origin] {
			findings = append(findings, Finding{fmt.Sprintf("claim %s cites pending ADR-%s which is not in the corpus", id, claim.Origin)})
		} else {
			origin, hasOrigin = byOperation[claim.Origin+"\x00"+string(adr.OpAdd)+"\x00"+id]
			if !legacyOrigin(records, claim.Origin) && !hasOrigin {
				findings = append(findings, Finding{fmt.Sprintf("claim %s names Origin ADR-%s, which has no matching add operation applied", id, claim.Origin)})
			}
		}
		last := 0
		if hasOrigin && !originIsSlug {
			last = origin.order
		}
		seenSlug := originIsSlug
		for _, rev := range claim.RevisedBy {
			revIsSlug := adr.IsSlugIdentity(rev)
			if revIsSlug && !pendingSlugs[rev] {
				findings = append(findings, Finding{fmt.Sprintf("claim %s cites pending ADR-%s which is not in the corpus", id, rev)})
				continue
			}
			operation, ok := byOperation[rev+"\x00"+string(adr.OpUpdate)+"\x00"+id]
			if !ok {
				findings = append(findings, Finding{fmt.Sprintf("claim %s names Revised-by ADR-%s, which has no matching update operation applied", id, rev)})
				continue
			}
			if revIsSlug {
				seenSlug = true
				continue
			}
			if seenSlug {
				findings = append(findings, Finding{fmt.Sprintf("claim %s lists numbered Revised-by ADR-%s after a pending entry", id, rev)})
			}
			if operation.order <= last {
				findings = append(findings, Finding{fmt.Sprintf("claim %s Revised-by entries are not in ascending ADR-number order at ADR-%s", id, rev)})
			}
			last = operation.order
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
