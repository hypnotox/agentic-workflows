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
// transition check compares: the intrinsically formatted ADR records and the topic set. It
// is the loader-agnostic shape the working, index, and commit loaders each
// collapse to, mirroring Check's parsed-input contract so CheckPair reads a Git
// diff without knowing how either side was loaded. Loaded.Universe builds one.
type Universe struct {
	ADRs    []adr.ADR
	Topics  []topic.Topic
	Sources map[string][]byte
}

// Universe reduces a Loaded view to the before/after inputs CheckPair compares.
func (l Loaded) Universe() Universe {
	return Universe{ADRs: l.ADRs, Topics: l.Topics.All(), Sources: l.Sources}
}

// TransitionMode selects which contract CheckPair applies to a pair.
type TransitionMode int

const (
	// AuthoredCommit is the strict per-step contract: one commit is one
	// authoring step, so an ADR appends at most one batch, a claim is the
	// target of at most one operation, and a Status history grows by the fixed
	// one-or-two-event shape.
	AuthoredCommit TransitionMode = iota
	// MergeAggregate is the contract for a merge, which is one Git commit but
	// the aggregate of a branch's commits. Several batches, a claim's ordered
	// operation chain, and a multi-step Status history are each legal when they
	// form a legal ordered chain (ADR-0182).
	MergeAggregate
)

// CheckPair validates the current-state transition from the before universe to
// the after universe (ADR-0135): every current-state-v1 ADR status change across
// the pair is a legal lifecycle edge, and the claim add/update/remove mutations
// between the two topic corpora correspond exactly to the operations of the ADRs
// that reached Implemented across the pair. An update must preserve the claim
// Origin, grow its Revised-by to the duplicate-free union of the prior list and
// the updating ADRs (ADR-0191), and change a canonical field that is neither
// provenance nor formatting; a claim mutation with no matching operation and an
// operation with no matching mutation are both rejected, while a dominated
// update whose claim an applied remove already absorbed expects no mutation. It also runs the full
// after-state static Check, so a legal transition still lands in a valid state.
// Parsed record formats identify the closed migration bootstrap. Findings are
// returned sorted by message. The mode selects the per-step or aggregate
// contract; see TransitionMode.
func CheckPair(before, after Universe, mode TransitionMode) []Finding {
	var findings []Finding
	pairs := newPairing(before.ADRs, after.ADRs)
	findings = append(findings, Check(after.ADRs, after.Topics)...)
	findings = append(findings, checkTransitions(before.ADRs, after.ADRs, pairs, mode)...)
	findings = append(findings, checkMutations(before, after, pairs, mode)...)
	sort.Slice(findings, func(i, j int) bool { return findings[i].Message < findings[j].Message })
	return findings
}

// Introduction identifies an ADR that exists only in the result universe and
// carries a format older than the current authoring format.
type Introduction struct {
	Identity string
	Format   adr.Format
}

// OlderIntroductions returns provisional older-format ADR introductions in
// identity order. It consumes the same pairing resolution as CheckPair so a
// retained record, numbered pending record, or sanctioned slugless renumber is
// never misclassified as a new result record.
func OlderIntroductions(before, after Universe, current adr.Format) []Introduction {
	pairs := newPairing(before.ADRs, after.ADRs)
	var out []Introduction
	for _, record := range after.ADRs {
		if record.Format >= current {
			continue
		}
		if _, paired := pairs.before(record); paired {
			continue
		}
		out = append(out, Introduction{Identity: record.Identity(), Format: record.Format})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Identity < out[j].Identity })
	return out
}

// checkTransitions enforces frozen content, stable-history prefix preservation,
// and the format-specific event shape for every governed record pair.
func checkTransitions(before, after []adr.ADR, pairs pairing, mode TransitionMode) []Finding {
	var findings []Finding
	for _, b := range before {
		if b.IsGoverned() {
			if _, ok := pairs.after(b); !ok {
				findings = append(findings, Finding{fmt.Sprintf("%s ADR-%s was deleted across this transition", adr.FormatMarker(b.Format), b.Identity())})
			}
		}
	}
	for _, a := range after {
		if !a.IsGoverned() {
			continue
		}
		b, ok := pairs.before(a)
		if !ok || !b.IsGoverned() {
			continue
		}
		if b.Format != a.Format {
			findings = append(findings, Finding{fmt.Sprintf("ADR-%s changed governed format across this transition", a.Identity())})
			continue
		}
		if pairs.renumbered(a) {
			// The sanctioned renumber permits the number, filename, and heading
			// change and nothing else about the record. Its canonical content is
			// equal by construction, since that is the key the pair formed on, so
			// only the status and Status history need saying, and they are compared
			// for byte equality rather than through either append-tolerant variant.
			if !adr.HistoriesEqual(b, a) || !b.HasSameStatus(a) {
				findings = append(findings, Finding{fmt.Sprintf("ADR-%s violates the renumbering rule: renumbering ADR-%s must leave its status and Status history byte-identical", a.Number, b.Number)})
			}
			continue
		}
		if b.Number != "" && b.Number != a.Number {
			// Slug pairing makes a renumber or an un-numbering visible as one
			// pair instead of a delete plus an add, so the number-immutability
			// rule has to be stated here (ADR-0202 item 12).
			findings = append(findings, Finding{fmt.Sprintf("ADR-%s changed its assigned number to ADR-%s across this transition; an assigned ADR number never changes", b.Number, a.Identity())})
			continue
		}
		if !adr.FrozenContentEqual(b, a) {
			findings = append(findings, Finding{fmt.Sprintf("ADR-%s violates the frozen-content rule: canonical decision content changed after the record froze", a.Identity())})
		}
		if isNumberingPair(b, a) {
			// The sanctioned numbering transition permits the number, filename,
			// and heading gain and nothing else about the record, so its status
			// and Status history are compared for byte equality rather than
			// through either append-tolerant variant (ADR-0202 item 11).
			if !adr.HistoriesEqual(b, a) || !b.HasSameStatus(a) {
				findings = append(findings, Finding{fmt.Sprintf("ADR-%s violates the numbering-transition rule: numbering pending ADR-%s must leave its status and Status history byte-identical", a.Number, b.Slug)})
			}
			continue
		}
		if !historyTransitionValid(b, a, mode) {
			shape := "Status history must remain equal at the same status or append exactly one entry for a legal transition"
			if a.HasV2Semantics() {
				shape = "prior events must remain an exact prefix and the transition must append the required status/Applied event shape"
			}
			findings = append(findings, Finding{fmt.Sprintf("ADR-%s violates the history-prefix rule: %s", a.Identity(), shape)})
		}
		if !b.HasSameStatus(a) && !adr.TransitionLegal(b.Status, a.Status, a.Format) {
			findings = append(findings, Finding{fmt.Sprintf("ADR-%s changed status from %s to %s, which is not a legal %s transition", a.Identity(), b.Status, a.Status, adr.FormatMarker(a.Format))})
		}
	}
	return findings
}

// isNumberingPair reports whether a pair is the sanctioned numbering shape: the
// before record is pending and the after record is the same slug carrying its
// assigned number (ADR-0202 item 11).
func isNumberingPair(before, after adr.ADR) bool {
	return before.IsPending() && after.Number != ""
}

// pairOp is one operation an ADR reaching Implemented across the pair declares
// over a claim, tagged with the implementing ADR number.
type pairOp struct {
	verb adr.OpVerb
	adr  string
	// updaters lists the updating ADRs of a folded net-update chain, in
	// chain order. Empty for every other verb and for an authored commit,
	// where the single updating ADR is `adr`.
	updaters []string
}

// checkMutations reconciles the claim add/update/remove mutations between the two
// topic corpora against the operations of the ADRs that reached Implemented
// across the pair. Every union of an operation ID and a mutated claim ID is
// classified once, so an operation with no mutation and a mutation with no
// operation are both surfaced.
func checkMutations(before, after Universe, pairs pairing, mode TransitionMode) []Finding {
	ops, dups, rejected, batchFindings := pairOps(after.ADRs, pairs, mode)
	renames := numberingSubstitutions(before.ADRs, pairs)
	beforeClaims := claimMap(before.Topics)
	afterClaims := claimMap(after.Topics)

	findings := append([]Finding(nil), batchFindings...)
	for _, id := range dups {
		findings = append(findings, Finding{fmt.Sprintf("claim %s is the target of more than one operation in this transition", id)})
	}
	for _, id := range unionKeys(ops, beforeClaims, afterClaims) {
		op, hasOp := ops[id]
		bcl, hasBefore := beforeClaims[id]
		acl, hasAfter := afterClaims[id]
		switch {
		case hasOp && op.verb == adr.OpAdd:
			if hasBefore {
				findings = append(findings, Finding{fmt.Sprintf("ADR-%s adds claim %s, which already existed before this transition", op.adr, id)})
			}
		case hasOp && op.verb == adr.OpRemove:
			if !hasBefore {
				findings = append(findings, Finding{fmt.Sprintf("ADR-%s removes claim %s, which did not exist before this transition", op.adr, id)})
			}
		case hasOp && op.verb == adr.OpUpdate:
			if removedInUniverse(after.ADRs, id) {
				// An applied remove absorbed this claim (ADR-0191): the update
				// chain is dominated history with an empty mutation set.
				if hasBefore || hasAfter {
					findings = append(findings, Finding{fmt.Sprintf("claim %s has only dominated updates in this transition, so it must stay absent", id)})
				}
			} else {
				findings = append(findings, checkUpdate(op.adr, id, bcl, acl, hasBefore, hasAfter, renames, op.updaters)...)
			}
		case hasOp && op.verb == opNetNoop:
			// The chain both added and removed the claim, so the pair must show
			// it absent on both sides rather than reporting it as an unmatched
			// mutation, whose add/remove wording would deny the operations exist.
			if hasBefore || hasAfter {
				findings = append(findings, Finding{fmt.Sprintf("claim %s is added and removed within this transition, so it must be absent on both sides", id)})
			}
		case rejected[id]:
			// Its chain already produced a diagnosis; a second, contradictory
			// unmatched-mutation finding would only obscure it.
		default:
			findings = append(findings, checkUnmatchedMutation(after.ADRs, id, bcl, acl, hasBefore, hasAfter, renames)...)
		}
	}
	return findings
}

// opNetNoop is the net effect of a chain that both adds and removes a claim
// within one aggregate: the claim must be absent on both sides. It is internal
// to the fold and never appears in an ADR.
const opNetNoop adr.OpVerb = "net-noop"

type appendedBatch struct {
	adr string
	// order is the owning record's provenance rank: its number when numbered,
	// and one rank shared by every pending record, above every number. That
	// places a pending batch after every numbered one, which is the order
	// numbering will in fact assign it (ADR-0202 item 10). Pending records tie
	// with each other and the stable sort falls back to corpus order, which is
	// the authored order the provenance claim gives slug entries among
	// themselves; docs/roadmap.md records what that tie costs.
	order    int
	batchIdx int
	ops      []adr.Operation
}

// pairOps derives only newly appended batches. It validates one batch per ADR
// in authored mode and cross-batch target uniqueness; cross-ADR order is
// ascending ADR number then intra-ADR history position (ADR-0191).
func pairOps(after []adr.ADR, pairs pairing, mode TransitionMode) (map[string]pairOp, []string, map[string]bool, []Finding) {
	var batches []appendedBatch
	var findings []Finding
	for _, a := range after {
		if !a.IsGoverned() {
			continue
		}
		afterBatches, err := a.ApplicationBatches()
		if err != nil {
			continue
		}
		beforeCount := 0
		if b, ok := pairs.before(a); ok && b.IsGoverned() {
			beforeBatches, beforeErr := b.ApplicationBatches()
			if beforeErr == nil {
				beforeCount = len(beforeBatches)
			}
		}
		if len(afterBatches) < beforeCount {
			findings = append(findings, Finding{fmt.Sprintf("ADR-%s deleted a previously applied batch", a.Identity())})
			continue
		}
		added := afterBatches[beforeCount:]
		if len(added) > 1 && mode == AuthoredCommit {
			findings = append(findings, Finding{fmt.Sprintf("ADR-%s appends %d application batches; at most one new batch is allowed per transition", a.Identity(), len(added))})
		}
		for j, batch := range added {
			batches = append(batches, appendedBatch{adr: a.Identity(), order: adr.IdentityOrder(a.Identity()), batchIdx: beforeCount + j, ops: batch.Operations})
		}
	}
	sort.SliceStable(batches, func(i, j int) bool {
		if batches[i].order != batches[j].order {
			return batches[i].order < batches[j].order
		}
		return batches[i].batchIdx < batches[j].batchIdx
	})
	// Batches are in ascending ADR-number and intra-ADR history order, so
	// appending here yields each claim's canonical operation chain.
	chains := map[string][]pairOp{}
	order := []string{}
	for _, batch := range batches {
		for _, operation := range batch.ops {
			if _, seen := chains[operation.ID]; !seen {
				order = append(order, operation.ID)
			}
			chains[operation.ID] = append(chains[operation.ID], pairOp{verb: operation.Verb, adr: batch.adr})
		}
	}
	sort.Strings(order)
	ops := map[string]pairOp{}
	dups := []string{}
	// rejected carries every ID already diagnosed by this function, so
	// checkMutations does not add a contradictory unmatched-mutation finding on
	// top of a chain rejection or a duplicate-target report.
	rejected := map[string]bool{}
	for _, id := range order {
		chain := chains[id]
		if len(chain) == 1 {
			ops[id] = chain[0]
			continue
		}
		if mode == AuthoredCommit {
			dups = append(dups, id)
			rejected[id] = true
			continue
		}
		net, reason := foldChain(chain)
		if reason != "" {
			findings = append(findings, Finding{fmt.Sprintf("claim %s has an illegal operation chain in this transition: %s", id, reason)})
			rejected[id] = true
			continue
		}
		ops[id] = net
	}
	return ops, dups, rejected, findings
}

// historyTransitionValid applies the Status-history contract the mode selects:
// the fixed one-or-two-event shape for an authored commit, append-only prefix
// preservation for a merge, whose appended events were already proven a legal
// ordered chain when the record parsed (ADR-0182 item 7).
func historyTransitionValid(before, after adr.ADR, mode TransitionMode) bool {
	if mode == MergeAggregate {
		return adr.HistoryTransitionValidAggregate(before, after)
	}
	return adr.HistoryTransitionValid(before, after)
}

// foldChain reduces a claim's ordered operation chain to the net effect the pair
// must show, or names why the chain is illegal. Taken in canonical order a legal
// chain admits at most one add, which must be first, at most one remove, and any
// number of updates; an update after the remove is dominated history that joins
// no updaters list and never alters the net effect, and the remove is absorbing,
// so a net remove or net no-op is attributed to the removing ADR (ADR-0191).
// One ADR may not actively update the same claim twice, which
// update-requires-substance already forbids by requiring an update to add its
// ADR once (ADR-0182 item 6).
func foldChain(chain []pairOp) (pairOp, string) {
	var updaters []string
	var removeADR string
	hasAdd, hasRemove := false, false
	for i, step := range chain {
		switch step.verb {
		case adr.OpAdd:
			if i != 0 {
				return pairOp{}, "an add must be the first operation"
			}
			hasAdd = true
		case adr.OpRemove:
			if hasRemove {
				return pairOp{}, "at most one remove is allowed"
			}
			hasRemove = true
			removeADR = step.adr
		default:
			if hasRemove {
				// Dominated by the absorbing remove: retained history only.
				continue
			}
			if slices.Contains(updaters, step.adr) {
				return pairOp{}, fmt.Sprintf("ADR-%s updates it more than once", step.adr)
			}
			updaters = append(updaters, step.adr)
		}
	}
	last := chain[len(chain)-1]
	switch {
	case hasAdd && hasRemove:
		return pairOp{verb: opNetNoop, adr: removeADR}, ""
	case hasAdd:
		return pairOp{verb: adr.OpAdd, adr: chain[0].adr}, ""
	case hasRemove:
		return pairOp{verb: adr.OpRemove, adr: removeADR}, ""
	default:
		return pairOp{verb: adr.OpUpdate, adr: last.adr, updaters: updaters}, ""
	}
}

// removedInUniverse reports whether any governed record in the universe has an
// applied remove for id, the absorbing-tombstone fact that classifies a later
// update chain as dominated history (ADR-0191).
func removedInUniverse(records []adr.ADR, id string) bool {
	for _, a := range records {
		if !a.IsGoverned() {
			continue
		}
		progress, err := a.OperationProgress()
		if err != nil {
			continue
		}
		for _, applied := range progress.Applied {
			if applied.Operation.Verb == adr.OpRemove && applied.Operation.ID == id {
				return true
			}
		}
	}
	return false
}

// numberingSubstitutions maps each identity rewritten across this transition to
// the number it took: a slug to the number numbering assigned it (ADR-0202 item
// 9), and a renumbered slugless record's old number to its new one. It is empty
// for every transition that rewrites neither, which is what keeps the byte-exact
// provenance rules in force everywhere else.
func numberingSubstitutions(before []adr.ADR, pairs pairing) map[string]string {
	renames := pairs.renumbers()
	for _, b := range before {
		if !b.IsPending() {
			continue
		}
		if a, ok := pairs.after(b); ok && isNumberingPair(b, a) {
			renames[b.Slug] = a.Number
		}
	}
	return renames
}

// numberedProvenance rewrites a claim's authored provenance the way numbering
// does - each entry naming a slug numbered in this transition becomes that
// record's number, and a touched Revised-by list is canonicalized to the
// duplicate-free ascending order ADR-0191 requires - and reports whether any
// entry moved. A claim citing nothing numbered here is returned untouched, so
// the caller applies the ordinary rules to it.
func numberedProvenance(c topic.Claim, renames map[string]string) (topic.Claim, bool) {
	substituted := false
	if number, renamed := renames[c.Origin]; renamed {
		c.Origin, substituted = number, true
	}
	revised := slices.Clone(c.RevisedBy)
	touched := false
	for i, entry := range revised {
		if number, renamed := renames[entry]; renamed {
			revised[i], touched, substituted = number, true, true
		}
	}
	if touched {
		unique := make([]string, 0, len(revised))
		for _, entry := range revised {
			if !slices.Contains(unique, entry) {
				unique = append(unique, entry)
			}
		}
		slices.SortStableFunc(unique, func(x, y string) int { return adr.IdentityOrder(x) - adr.IdentityOrder(y) })
		revised = unique
	}
	c.RevisedBy = revised
	return c, substituted
}

// checkUpdate validates a declared update: the claim is present on both sides, a
// canonical non-provenance/non-formatting field changed, the Origin is
// preserved, and Revised-by grew to the duplicate-free union of the prior list
// and the updating ADRs (ADR-0191). When the transition also numbers a record
// the before claim cites, the preserve-Origin and Revised-by rules are applied
// to the substituted before claim, which is how the sanctioned numbering
// transition composes with a declared update in the same pair (ADR-0202 item 11).
func checkUpdate(adrNum, id string, before, after topic.Claim, hasBefore, hasAfter bool, renames map[string]string, updaters []string) []Finding {
	if len(updaters) == 0 {
		updaters = []string{adrNum}
	}
	if !hasBefore || !hasAfter {
		return []Finding{{fmt.Sprintf("ADR-%s updates claim %s, which is not present on both sides of this transition", adrNum, id)}}
	}
	before, _ = numberedProvenance(before, renames)
	var out []Finding
	if claimMateriallyEqual(before, after) {
		out = append(out, Finding{fmt.Sprintf("ADR-%s updates claim %s, but no canonical field changed (a provenance- or formatting-only edit is not an update)", adrNum, id)})
	}
	if before.Origin != after.Origin {
		out = append(out, Finding{fmt.Sprintf("ADR-%s update of claim %s changed its Origin from ADR-%s to ADR-%s; an update must preserve Origin", adrNum, id, before.Origin, after.Origin)})
	}
	if reason, ok := revisedByExtension(before, after, updaters); !ok {
		out = append(out, Finding{fmt.Sprintf("ADR-%s update of claim %s %s", adrNum, id, reason)})
	}
	return out
}

// checkUnmatchedMutation reports a claim add/removal/material change that no
// operation in this transition accounts for. A claim first appearing with a
// legacy Origin is the closed migration bootstrap and needs no add operation.
//
// A claim whose authored provenance cites a record numbered in this transition
// takes the sanctioned numbering contract instead (ADR-0202 item 11): its
// permitted delta is exactly the slug-to-number substitution with each touched
// list canonicalized, compared in order, and it declares no operation because
// numbering appends no application batch. This is the only rule that admits a
// provenance change with no update operation behind it, so it is stated
// exactly rather than by loosening the general one.
func checkUnmatchedMutation(records []adr.ADR, id string, before, after topic.Claim, hasBefore, hasAfter bool, renames map[string]string) []Finding {
	// The classification reaches here only for an ID some claim map holds, so
	// absence on one side means presence on the other and the two guards below
	// leave exactly the both-present case to fall through.
	switch {
	case !hasBefore:
		if legacyOrigin(records, after.Origin) {
			return nil
		}
		return []Finding{{fmt.Sprintf("claim %s was added with no ADR add operation in this transition", id)}}
	case !hasAfter:
		return []Finding{{fmt.Sprintf("claim %s was removed with no ADR remove operation in this transition", id)}}
	}
	if numbered, substituted := numberedProvenance(before, renames); substituted {
		if claimMateriallyEqual(before, after) && numbered.Origin == after.Origin && slices.Equal(numbered.RevisedBy, after.RevisedBy) {
			return nil
		}
		return []Finding{{fmt.Sprintf("claim %s must carry the numbering substitution exactly: Origin ADR-%s, Revised-by %s, and no other change", id, numbered.Origin, provenanceList(numbered.RevisedBy))}}
	}
	if !claimMateriallyEqual(before, after) || before.Origin != after.Origin || !sameRevisedBySet(before.RevisedBy, after.RevisedBy) {
		return []Finding{{fmt.Sprintf("claim %s was changed with no ADR update operation in this transition", id)}}
	}
	return nil
}

// provenanceList renders a Revised-by expectation in the authored spelling.
func provenanceList(entries []string) string {
	if len(entries) == 0 {
		return "(none)"
	}
	out := make([]string, len(entries))
	for i, entry := range entries {
		out[i] = "ADR-" + entry
	}
	return strings.Join(out, ", ")
}

// revisedByExtension validates that Revised-by grew to the duplicate-free union
// of the prior list and the updating ADRs; an insertion below an existing higher
// number is legal (ADR-0191 item 5). Ascending order over the result is owned by
// the static backward check, which runs over the whole after universe.
func revisedByExtension(before, after topic.Claim, adrNums []string) (string, bool) {
	afterSet := make(map[string]bool, len(after.RevisedBy))
	for _, num := range after.RevisedBy {
		afterSet[num] = true
	}
	for _, num := range before.RevisedBy {
		if !afterSet[num] {
			return "must preserve every prior Revised-by entry", false
		}
	}
	for _, num := range adrNums {
		if !afterSet[num] {
			return fmt.Sprintf("must add the updating ADR-%s to Revised-by", num), false
		}
	}
	expected := make(map[string]bool, len(before.RevisedBy)+len(adrNums))
	for _, num := range before.RevisedBy {
		expected[num] = true
	}
	for _, num := range adrNums {
		expected[num] = true
	}
	if len(after.RevisedBy) != len(expected) {
		return "must grow Revised-by to exactly the union of the prior list and the updating ADRs, duplicate-free", false
	}
	return "", true
}

// sameRevisedBySet compares Revised-by membership, not order: canonical order
// is derived (ascending ADR number, ADR-0191), so the migration's reordering of
// a legacy list to canonical form is not a change.
func sameRevisedBySet(a, b []string) bool {
	x := slices.Clone(a)
	y := slices.Clone(b)
	slices.Sort(x)
	slices.Sort(y)
	return slices.Equal(x, y)
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

// pairKey is the identity a transition pairs two universes' records on: the
// retained slug whenever the record carries one, and the number otherwise. It
// is deliberately not adr.ADR.Identity, which prefers the number: a pending
// record and the numbered successor numbering produced are the same record, and
// only the slug is stable across that rename. Keying on the number would read
// the rename as a delete plus an add, and would collide every pending record on
// the empty number (ADR-0202 item 11).
func pairKey(a adr.ADR) string {
	if a.Slug != "" {
		return a.Slug
	}
	return a.Number
}

// digestKeyPrefix namespaces a resolved digest key away from the number and slug
// forms, which are the only other keys a record can take.
const digestKeyPrefix = "content-sha256:"

// pairing is the record correspondence between one transition's two universes,
// resolved once so every check that pairs them consumes the same answer.
//
// A governed record's key is its retained slug, then its canonical content
// digest when a renumber moved it, then its assigned number. The digest step
// cannot be a function of one record the way pairKey is: it forms a pair only on
// a digest carried exactly once on each side, and only where that pair re-keys
// the record, so both universes have to be in hand at once
// (ADR-0204 item 11).
type pairing struct {
	beforeAlias map[string]string
	afterAlias  map[string]string
	beforeByKey map[string]adr.ADR
	afterByKey  map[string]adr.ADR
}

// newPairing resolves both universes' keys once.
func newPairing(before, after []adr.ADR) pairing {
	beforeAlias, afterAlias := renumberAliases(before, after)
	return pairing{
		beforeAlias: beforeAlias,
		afterAlias:  afterAlias,
		beforeByKey: byResolvedKey(before, beforeAlias),
		afterByKey:  byResolvedKey(after, afterAlias),
	}
}

// after returns the after-universe record continuing b, if the transition has one.
func (p pairing) after(b adr.ADR) (adr.ADR, bool) {
	a, ok := p.afterByKey[resolvedKey(b, p.beforeAlias)]
	return a, ok
}

// before returns the before-universe record a continues, if the transition has one.
func (p pairing) before(a adr.ADR) (adr.ADR, bool) {
	b, ok := p.beforeByKey[resolvedKey(a, p.afterAlias)]
	return b, ok
}

// renumbered reports whether a is the after side of a digest-paired renumber,
// the one pair whose assigned number may legally differ across the transition.
func (p pairing) renumbered(a adr.ADR) bool {
	_, ok := p.afterAlias[pairKey(a)]
	return ok
}

// renumbers maps each renumbered record's old number to its new one. A slugless
// record's pair key is its number, so the before-side alias keys are exactly the
// old numbers.
func (p pairing) renumbers() map[string]string {
	out := map[string]string{}
	for oldNumber, key := range p.beforeAlias {
		if a, ok := p.afterByKey[key]; ok {
			out[oldNumber] = a.Number
		}
	}
	return out
}

// renumberAliases resolves the digest step. A governed slugless record whose
// canonical digest is carried by exactly one slugless record on each side,
// where the two ends hold different numbers, is one record renumbered, and both
// ends take one shared key. A digest repeated on either side aliases nothing,
// so an ambiguous match leaves every record holding it on its number and the
// transition refuses the rename rather than guessing. A digest matching at the
// same number aliases nothing either, so an ordinary transition pairs exactly
// as it did before. A slug on either end excludes the record from this step;
// retained-slug pairing already owns those records, and a newly added slug must
// not widen the renumbering exception.
func renumberAliases(before, after []adr.ADR) (map[string]string, map[string]string) {
	beforeDigests := uniqueSluglessDigests(before)
	afterDigests := uniqueSluglessDigests(after)
	beforeAlias, afterAlias := map[string]string{}, map[string]string{}
	for digest, beforeKey := range beforeDigests {
		afterKey, matched := afterDigests[digest]
		if !matched || afterKey == beforeKey {
			continue
		}
		beforeAlias[beforeKey] = digestKeyPrefix + digest
		afterAlias[afterKey] = digestKeyPrefix + digest
	}
	return beforeAlias, afterAlias
}

// uniqueSluglessDigests indexes one universe's governed slugless records by
// canonical content digest, dropping every digest more than one record carries.
// A slug-carrying record is never the far end of a digest-paired renumber.
func uniqueSluglessDigests(records []adr.ADR) map[string]string {
	keys := map[string]string{}
	ambiguous := map[string]bool{}
	for _, a := range records {
		if !a.IsGoverned() || a.Slug != "" {
			continue
		}
		digest := a.CanonicalDigest()
		if _, seen := keys[digest]; seen {
			ambiguous[digest] = true
			continue
		}
		keys[digest] = pairKey(a)
	}
	for digest := range ambiguous {
		delete(keys, digest)
	}
	return keys
}

// resolvedKey returns a record's pairing key under its own universe's aliases.
func resolvedKey(a adr.ADR, alias map[string]string) string {
	key := pairKey(a)
	if resolved, aliased := alias[key]; aliased {
		return resolved
	}
	return key
}

// byResolvedKey indexes records by their resolved pairing key.
func byResolvedKey(records []adr.ADR, alias map[string]string) map[string]adr.ADR {
	out := make(map[string]adr.ADR, len(records))
	for _, a := range records {
		out[resolvedKey(a, alias)] = a
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
