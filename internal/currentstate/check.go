// Package currentstate runs the static current-state authority checks over an
// already-parsed ADR record set and topic set: the current-state-v1 ADR
// lifecycle and operation graph, and the bidirectional ADR-to-claim handshake
// (ADR-0135). It consumes parsed inputs rather than a filesystem or Git tree, so
// the same checker serves the working-tree, index, and range loaders.
package currentstate

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// Severity ranks a finding. Error findings make a check exit nonzero; Warn
// findings are reported without failing.
type Severity int

const (
	// Error is a blocking finding.
	Error Severity = iota
	// Warn is a non-blocking finding.
	Warn
)

// String names the severity for rendering.
func (s Severity) String() string {
	if s == Warn {
		return "warn"
	}
	return "error"
}

// Finding is one current-state check result.
type Finding struct {
	Severity Severity
	Message  string
}

// Check validates the current-state-v1 lifecycle and operation facts over the
// parsed records and topics. cutoff is the lock's adrFormatV1From boundary: a
// claim Origin below it is the migration bootstrap exemption. Provenance
// existence, references, and backing are validated when the topic corpus loads;
// Check adds the operation graph and the bidirectional handshake. It returns
// only Error findings, sorted deterministically by message.
func Check(records []adr.ADR, topics []topic.Topic, cutoff int) []Finding {
	claims := map[string]topic.Claim{}
	for _, t := range topics {
		for _, c := range t.Claims {
			claims[c.ID] = c
		}
	}
	v1 := filterV1(records)
	removed := removedSet(v1)

	var findings []Finding
	findings = append(findings, checkSequences(v1)...)
	findings = append(findings, checkOperationHistory(v1)...)
	findings = append(findings, checkForward(v1, claims, removed)...)
	findings = append(findings, checkBackward(records, claims, cutoff)...)
	sort.Slice(findings, func(i, j int) bool { return findings[i].Message < findings[j].Message })
	return findings
}

// filterV1 returns the current-state-v1 records.
func filterV1(records []adr.ADR) []adr.ADR {
	var out []adr.ADR
	for _, a := range records {
		if a.IsV1() {
			out = append(out, a)
		}
	}
	return out
}

// terminalSequence returns the state-sequence recorded on an ADR's final Status
// history entry.
func terminalSequence(a adr.ADR) int {
	return a.History[len(a.History)-1].Sequence
}

// mutationADRs returns the Implemented v1 ADRs that carry operations, each with
// its state-sequence.
func mutationADRs(v1 []adr.ADR) []adr.ADR {
	var out []adr.ADR
	for _, a := range v1 {
		if a.IsImplemented() && len(a.Operations) > 0 {
			out = append(out, a)
		}
	}
	return out
}

// checkSequences requires the state-sequence values of Implemented mutation
// ADRs to be unique and contiguous from 1 (ADR-0135 item 7).
func checkSequences(v1 []adr.ADR) []Finding {
	bySeq := map[int][]string{}
	for _, a := range mutationADRs(v1) {
		seq := terminalSequence(a)
		bySeq[seq] = append(bySeq[seq], a.Number)
	}
	var findings []Finding
	seqs := make([]int, 0, len(bySeq))
	for seq, nums := range bySeq {
		seqs = append(seqs, seq)
		if len(nums) > 1 {
			sort.Strings(nums)
			findings = append(findings, Finding{Error, fmt.Sprintf("state-sequence %d is used by more than one ADR: %v", seq, nums)})
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

// checkOperationHistory validates, for each claim ID touched by an Implemented
// operation, that its sequence-ordered history is exactly one add, then ordered
// updates, then at most one terminal remove, with nothing after the remove
// (ADR-0135 items 3 and 7).
func checkOperationHistory(v1 []adr.ADR) []Finding {
	type opAt struct {
		seq  int
		verb adr.OpVerb
	}
	byID := map[string][]opAt{}
	for _, a := range mutationADRs(v1) {
		seq := terminalSequence(a)
		for _, op := range a.Operations {
			byID[op.ID] = append(byID[op.ID], opAt{seq, op.Verb})
		}
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var findings []Finding
	for _, id := range ids {
		ops := byID[id]
		sort.Slice(ops, func(i, j int) bool { return ops[i].seq < ops[j].seq })
		adds, removeIdx := 0, -1
		for i, o := range ops {
			switch o.verb {
			case adr.OpAdd:
				adds++
			case adr.OpRemove:
				if removeIdx >= 0 {
					findings = append(findings, Finding{Error, fmt.Sprintf("claim %s has more than one remove", id)})
				}
				removeIdx = i
			case adr.OpUpdate:
				// An update is legal anywhere between the add and the terminal
				// remove; the add-count and remove-position checks below govern
				// the sequence, so an update imposes no additional constraint here.
			}
		}
		if adds != 1 {
			findings = append(findings, Finding{Error, fmt.Sprintf("claim %s has %d add operations; require exactly one", id, adds)})
		}
		if ops[0].verb != adr.OpAdd {
			findings = append(findings, Finding{Error, fmt.Sprintf("claim %s history does not begin with an add", id)})
		}
		if removeIdx >= 0 && removeIdx != len(ops)-1 {
			findings = append(findings, Finding{Error, fmt.Sprintf("claim %s has an operation after its remove", id)})
		}
	}
	return findings
}

// removedSet returns the claim IDs an Implemented ADR has removed.
func removedSet(v1 []adr.ADR) map[string]bool {
	removed := map[string]bool{}
	for _, a := range mutationADRs(v1) {
		for _, op := range a.Operations {
			if op.Verb == adr.OpRemove {
				removed[op.ID] = true
			}
		}
	}
	return removed
}

// checkForward validates each v1 ADR's operations against the current claim set
// for its status (ADR-0135 items 8): pending adds absent and updates/removes
// present; Implemented results applied; Abandoned results unapplied; and no add
// reuses a removed identity.
func checkForward(v1 []adr.ADR, claims map[string]topic.Claim, removed map[string]bool) []Finding {
	var findings []Finding
	for _, a := range v1 {
		for _, op := range a.Operations {
			claim, present := claims[op.ID]
			if op.Verb == adr.OpAdd && removed[op.ID] && !a.IsImplemented() {
				findings = append(findings, Finding{Error, fmt.Sprintf("ADR-%s adds removed claim %s, which may never be reused", a.Number, op.ID)})
			}
			switch {
			case a.IsInflight(): // Proposed or Accepted: the operation is pending
				findings = append(findings, checkPendingOp(a, op, present)...)
			case a.IsImplemented():
				findings = append(findings, checkImplementedOp(a, op, claim, present, removed[op.ID])...)
			case a.IsAbandoned():
				findings = append(findings, checkAbandonedOp(a, op, claim, present)...)
			}
		}
	}
	return findings
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

func checkImplementedOp(a adr.ADR, op adr.Operation, claim topic.Claim, present, wasRemoved bool) []Finding {
	switch op.Verb {
	case adr.OpAdd:
		if wasRemoved {
			return nil // a later remove retired it; checkForward handles the remove separately
		}
		if !present {
			return []Finding{{Error, fmt.Sprintf("Implemented ADR-%s adds claim %s, which has no active claim", a.Number, op.ID)}}
		}
		if claim.Origin != a.Number {
			return []Finding{{Error, fmt.Sprintf("claim %s Origin is ADR-%s, not the adding ADR-%s", op.ID, claim.Origin, a.Number)}}
		}
	case adr.OpUpdate:
		if wasRemoved {
			return nil
		}
		if !present {
			return []Finding{{Error, fmt.Sprintf("Implemented ADR-%s updates claim %s, which has no active claim", a.Number, op.ID)}}
		}
		if !contains(claim.RevisedBy, a.Number) {
			return []Finding{{Error, fmt.Sprintf("claim %s does not list updating ADR-%s in Revised-by", op.ID, a.Number)}}
		}
	case adr.OpRemove:
		if present {
			return []Finding{{Error, fmt.Sprintf("Implemented ADR-%s removes claim %s, which still has an active claim", a.Number, op.ID)}}
		}
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
		if !present {
			return []Finding{{Error, fmt.Sprintf("Abandoned ADR-%s remove for claim %s was applied; it must be reverted", a.Number, op.ID)}}
		}
	}
	return nil
}

// checkBackward validates that every active claim's provenance has the inverse
// current-state-v1 operation (ADR-0135 item 8): each Origin at or above cutoff
// is an Implemented add, and each Revised-by is an Implemented update. Origins
// below cutoff are the closed migration bootstrap exemption.
func checkBackward(records []adr.ADR, claims map[string]topic.Claim, cutoff int) []Finding {
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
		if num, _ := strconv.Atoi(claim.Origin); num >= cutoff {
			if !hasOperation(byNum, claim.Origin, adr.OpAdd, id) {
				findings = append(findings, Finding{Error, fmt.Sprintf("claim %s names Origin ADR-%s, which has no matching add operation", id, claim.Origin)})
			}
		}
		for _, rev := range claim.RevisedBy {
			if !hasOperation(byNum, rev, adr.OpUpdate, id) {
				findings = append(findings, Finding{Error, fmt.Sprintf("claim %s names Revised-by ADR-%s, which has no matching update operation", id, rev)})
			}
		}
	}
	return findings
}

// hasOperation reports whether the named ADR is an Implemented v1 ADR carrying
// the given verb over id.
func hasOperation(byNum map[string]adr.ADR, num string, verb adr.OpVerb, id string) bool {
	a, ok := byNum[num]
	if !ok || !a.IsV1() || !a.IsImplemented() {
		return false
	}
	for _, op := range a.Operations {
		if op.Verb == verb && op.ID == id {
			return true
		}
	}
	return false
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
