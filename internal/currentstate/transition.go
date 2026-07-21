package currentstate

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// Universe is one parsed current-state view reduced to the two inputs the
// transition check compares: the cutoff-aware ADR records and the topic set. It
// is the loader-agnostic shape the working, index, and commit loaders each
// collapse to, mirroring Check's parsed-input contract so CheckPair reads a Git
// diff without knowing how either side was loaded. Loaded.Universe builds one.
type Universe struct {
	ADRs   []adr.ADR
	Topics []topic.Topic
}

// Universe reduces a Loaded view to the before/after inputs CheckPair compares.
func (l Loaded) Universe() Universe {
	return Universe{ADRs: l.ADRs, Topics: l.Topics.All()}
}

// CheckPair validates the current-state transition from the before universe to
// the after universe (ADR-0135): every current-state-v1 ADR status change across
// the pair is a legal lifecycle edge, and the claim add/update/remove mutations
// between the two topic corpora correspond exactly to the operations of the ADRs
// that reached Implemented across the pair. An update must preserve the claim
// Origin, extend its Revised-by by exactly the updating ADR while keeping the
// prior list as an exact prefix, and change a canonical field that is neither
// provenance nor formatting; a claim mutation with no matching operation and an
// operation with no matching mutation are both rejected. It also runs the full
// after-state static Check, so a legal transition still lands in a valid state.
// Parsed record formats identify the closed migration bootstrap. Findings are
// returned sorted by message.
func CheckPair(before, after Universe) []Finding {
	var findings []Finding
	findings = append(findings, Check(after.ADRs, after.Topics)...)
	findings = append(findings, checkTransitions(before.ADRs, after.ADRs)...)
	findings = append(findings, checkMutations(before, after)...)
	sort.Slice(findings, func(i, j int) bool { return findings[i].Message < findings[j].Message })
	return findings
}

// checkTransitions enforces frozen content, stable-history prefix preservation,
// and the format-specific event shape for every governed record pair.
func checkTransitions(before, after []adr.ADR) []Finding {
	beforeByNum := byNumber(before)
	afterByNum := byNumber(after)
	var findings []Finding
	for _, b := range before {
		if b.IsGoverned() {
			if _, ok := afterByNum[b.Number]; !ok {
				marker := "current-state-v1"
				if b.IsV2() {
					marker = "current-state-v2"
				}
				findings = append(findings, Finding{Error, fmt.Sprintf("%s ADR-%s was deleted across this transition", marker, b.Number)})
			}
		}
	}
	for _, a := range after {
		if !a.IsGoverned() {
			continue
		}
		b, ok := beforeByNum[a.Number]
		if !ok || !b.IsGoverned() {
			continue
		}
		if b.Format != a.Format {
			findings = append(findings, Finding{Error, fmt.Sprintf("ADR-%s changed governed format across this transition", a.Number)})
			continue
		}
		if !adr.FrozenContentEqual(b, a) {
			findings = append(findings, Finding{Error, fmt.Sprintf("ADR-%s violates the frozen-content rule: canonical decision content changed after Proposed", a.Number)})
		}
		if !adr.HistoryTransitionValid(b, a) {
			shape := "Status history must remain equal at the same status or append exactly one entry for a legal transition"
			if a.IsV2() {
				shape = "prior events must remain an exact prefix and the transition must append the required status/Applied event shape"
			}
			findings = append(findings, Finding{Error, fmt.Sprintf("ADR-%s violates the history-prefix rule: %s", a.Number, shape)})
		}
		if !b.HasSameStatus(a) && !adr.TransitionLegal(b.Status, a.Status, a.Format) {
			marker := "current-state-v1"
			if a.IsV2() {
				marker = "current-state-v2"
			}
			findings = append(findings, Finding{Error, fmt.Sprintf("ADR-%s changed status from %s to %s, which is not a legal %s transition", a.Number, b.Status, a.Status, marker)})
		}
	}
	return findings
}

// pairOp is one operation an ADR reaching Implemented across the pair declares
// over a claim, tagged with the implementing ADR number.
type pairOp struct {
	verb adr.OpVerb
	adr  string
}

// checkMutations reconciles the claim add/update/remove mutations between the two
// topic corpora against the operations of the ADRs that reached Implemented
// across the pair. Every union of an operation ID and a mutated claim ID is
// classified once, so an operation with no mutation and a mutation with no
// operation are both surfaced.
func checkMutations(before, after Universe) []Finding {
	ops, dups, batchFindings := pairOps(before.ADRs, after.ADRs)
	beforeClaims := claimMap(before.Topics)
	afterClaims := claimMap(after.Topics)

	findings := append([]Finding(nil), batchFindings...)
	for _, id := range dups {
		findings = append(findings, Finding{Error, fmt.Sprintf("claim %s is the target of more than one operation in this transition", id)})
	}
	for _, id := range unionKeys(ops, beforeClaims, afterClaims) {
		op, hasOp := ops[id]
		bcl, hasBefore := beforeClaims[id]
		acl, hasAfter := afterClaims[id]
		switch {
		case hasOp && op.verb == adr.OpAdd:
			if hasBefore {
				findings = append(findings, Finding{Error, fmt.Sprintf("ADR-%s adds claim %s, which already existed before this transition", op.adr, id)})
			}
		case hasOp && op.verb == adr.OpRemove:
			if !hasBefore {
				findings = append(findings, Finding{Error, fmt.Sprintf("ADR-%s removes claim %s, which did not exist before this transition", op.adr, id)})
			}
		case hasOp && op.verb == adr.OpUpdate:
			findings = append(findings, checkUpdate(op.adr, id, bcl, acl, hasBefore, hasAfter)...)
		default:
			findings = append(findings, checkUnmatchedMutation(after.ADRs, id, bcl, acl, hasBefore, hasAfter)...)
		}
	}
	return findings
}

type appendedBatch struct {
	adr      string
	sequence int
	ops      []adr.Operation
}

// pairOps derives only newly appended batches. It validates one batch per ADR,
// cross-batch target uniqueness, and next-consecutive global sequencing.
func pairOps(before, after []adr.ADR) (map[string]pairOp, []string, []Finding) {
	beforeByNum := byNumber(before)
	var batches []appendedBatch
	var findings []Finding
	maxBefore := 0
	for _, b := range before {
		if !b.IsGoverned() {
			continue
		}
		projected, err := b.ApplicationBatches()
		if err != nil {
			continue
		}
		for _, batch := range projected {
			if batch.Sequence > maxBefore {
				maxBefore = batch.Sequence
			}
		}
	}
	for _, a := range after {
		if !a.IsGoverned() {
			continue
		}
		afterBatches, err := a.ApplicationBatches()
		if err != nil {
			continue
		}
		beforeCount := 0
		if b, ok := beforeByNum[a.Number]; ok && b.IsGoverned() {
			beforeBatches, beforeErr := b.ApplicationBatches()
			if beforeErr == nil {
				beforeCount = len(beforeBatches)
			}
		}
		if len(afterBatches) < beforeCount {
			findings = append(findings, Finding{Error, fmt.Sprintf("ADR-%s deleted a previously applied batch", a.Number)})
			continue
		}
		added := afterBatches[beforeCount:]
		if len(added) > 1 {
			findings = append(findings, Finding{Error, fmt.Sprintf("ADR-%s appends %d application batches; at most one new batch is allowed per transition", a.Number, len(added))})
		}
		for _, batch := range added {
			batches = append(batches, appendedBatch{adr: a.Number, sequence: batch.Sequence, ops: batch.Operations})
		}
	}
	sort.SliceStable(batches, func(i, j int) bool { return batches[i].sequence < batches[j].sequence })
	for i, batch := range batches {
		expected := maxBefore + i + 1
		if batch.sequence != expected {
			findings = append(findings, Finding{Error, fmt.Sprintf("ADR-%s application batch has state-sequence %d; expected next sequence %d", batch.adr, batch.sequence, expected)})
		}
	}
	ops := map[string]pairOp{}
	dupSet := map[string]bool{}
	for _, batch := range batches {
		for _, operation := range batch.ops {
			if _, exists := ops[operation.ID]; exists {
				dupSet[operation.ID] = true
				continue
			}
			ops[operation.ID] = pairOp{verb: operation.Verb, adr: batch.adr}
		}
	}
	dups := make([]string, 0, len(dupSet))
	for id := range dupSet {
		dups = append(dups, id)
	}
	sort.Strings(dups)
	return ops, dups, findings
}

// checkUpdate validates a declared update: the claim is present on both sides, a
// canonical non-provenance/non-formatting field changed, the Origin is
// preserved, and Revised-by grew by exactly the updating ADR with the prior list
// as an exact prefix.
func checkUpdate(adrNum, id string, before, after topic.Claim, hasBefore, hasAfter bool) []Finding {
	if !hasBefore || !hasAfter {
		return []Finding{{Error, fmt.Sprintf("ADR-%s updates claim %s, which is not present on both sides of this transition", adrNum, id)}}
	}
	var out []Finding
	if claimMateriallyEqual(before, after) {
		out = append(out, Finding{Error, fmt.Sprintf("ADR-%s updates claim %s, but no canonical field changed (a provenance- or formatting-only edit is not an update)", adrNum, id)})
	}
	if before.Origin != after.Origin {
		out = append(out, Finding{Error, fmt.Sprintf("ADR-%s update of claim %s changed its Origin from ADR-%s to ADR-%s; an update must preserve Origin", adrNum, id, before.Origin, after.Origin)})
	}
	if reason, ok := revisedByExtension(before, after, adrNum); !ok {
		out = append(out, Finding{Error, fmt.Sprintf("ADR-%s update of claim %s %s", adrNum, id, reason)})
	}
	return out
}

// checkUnmatchedMutation reports a claim add/removal/material change that no
// operation in this transition accounts for. A claim first appearing with a
// legacy Origin is the closed migration bootstrap and needs no add operation.
func checkUnmatchedMutation(records []adr.ADR, id string, before, after topic.Claim, hasBefore, hasAfter bool) []Finding {
	switch {
	case !hasBefore && hasAfter:
		if legacyOrigin(records, after.Origin) {
			return nil
		}
		return []Finding{{Error, fmt.Sprintf("claim %s was added with no ADR add operation in this transition", id)}}
	case hasBefore && !hasAfter:
		return []Finding{{Error, fmt.Sprintf("claim %s was removed with no ADR remove operation in this transition", id)}}
	case hasBefore && hasAfter && (!claimMateriallyEqual(before, after) || before.Origin != after.Origin || !slices.Equal(before.RevisedBy, after.RevisedBy)):
		return []Finding{{Error, fmt.Sprintf("claim %s was changed with no ADR update operation in this transition", id)}}
	}
	return nil
}

// revisedByExtension reports whether after.RevisedBy is before.RevisedBy with
// exactly the updating ADR appended, returning the reason it is not otherwise.
func revisedByExtension(before, after topic.Claim, adrNum string) (string, bool) {
	if len(after.RevisedBy) != len(before.RevisedBy)+1 {
		return "must extend Revised-by by exactly one entry, the updating ADR", false
	}
	for i := range before.RevisedBy {
		if after.RevisedBy[i] != before.RevisedBy[i] {
			return "must keep the prior Revised-by list as an exact prefix", false
		}
	}
	if after.RevisedBy[len(after.RevisedBy)-1] != adrNum {
		return fmt.Sprintf("must append the updating ADR-%s to Revised-by", adrNum), false
	}
	return "", true
}

// claimMateriallyEqual reports whether two claims carry the same canonical
// content: type, whitespace-trimmed prose, backing, verify note, and references.
// Origin and Revised-by (provenance) and surrounding whitespace (formatting) are
// deliberately excluded, so only a substantive edit counts as a change.
func claimMateriallyEqual(a, b topic.Claim) bool {
	return a.Type == b.Type &&
		strings.TrimSpace(a.Prose) == strings.TrimSpace(b.Prose) &&
		a.Backing == b.Backing &&
		a.Verify == b.Verify &&
		slices.Equal(a.References, b.References)
}

// byNumber indexes records by their ADR number.
func byNumber(records []adr.ADR) map[string]adr.ADR {
	out := make(map[string]adr.ADR, len(records))
	for _, a := range records {
		out[a.Number] = a
	}
	return out
}

// claimMap indexes every claim of every topic by its full ID.
func claimMap(topics []topic.Topic) map[string]topic.Claim {
	out := map[string]topic.Claim{}
	for _, t := range topics {
		for _, c := range t.Claims {
			out[c.ID] = c
		}
	}
	return out
}

// unionKeys returns the sorted union of the operation IDs and the before/after
// claim IDs, so each ID is classified exactly once.
func unionKeys(ops map[string]pairOp, before, after map[string]topic.Claim) []string {
	set := map[string]bool{}
	for id := range ops {
		set[id] = true
	}
	for id := range before {
		set[id] = true
	}
	for id := range after {
		set[id] = true
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
