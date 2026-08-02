package currentstate_test

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// TestMergeAggregateAcceptsSeveralBatchesFromOneADR covers the per-ADR batch cap
// relaxing for a merge. The same pair is rejected as an authored commit, which is
// what keeps the two contracts distinguishable rather than one being dead.
// invariant: invariants/current-state-authority:merge-transition-ordered-aggregate (TestMergeAggregateAcceptsSeveralBatchesFromOneADR)
func TestMergeAggregateAcceptsSeveralBatchesFromOneADR(t *testing.T) {
	base := rec("0137", "Implemented", op(adr.OpAdd, "d/t:base"))
	x, y, z := op(adr.OpAdd, "d/t:x"), op(adr.OpAdd, "d/t:y"), op(adr.OpAdd, "d/t:z")
	// A fourth declared operation stays unapplied so the record is legally
	// Implementing at both ends of the pair, which requires applied AND
	// remaining operations.
	pending := op(adr.OpAdd, "d/t:pending")
	partial := v2rec("0141", "Implementing", []adr.Operation{x, y, z, pending}, v2status("Proposed"), v2status("Implementing"), v2batch(x))
	three := partial
	three.History = append(append([]adr.HistoryEvent(nil), partial.History...), v2batch(y), v2batch(z))

	before := uni([]adr.ADR{base, partial}, claim("d/t:base", "0137"), claim("d/t:x", "0141"))
	after := uni([]adr.ADR{base, three},
		claim("d/t:base", "0137"), claim("d/t:x", "0141"), claim("d/t:y", "0141"), claim("d/t:z", "0141"))

	if f := currentstate.CheckPair(before, after, currentstate.MergeAggregate); len(f) != 0 {
		t.Fatalf("a merge must accept several batches from one ADR:\n%s", messages(f))
	}
	if got := messages(currentstate.CheckPair(before, after, currentstate.AuthoredCommit)); !strings.Contains(got, "at most one new batch") {
		t.Fatalf("an authored commit must still cap batches:\n%s", got)
	}
}

// TestMergeAggregateFoldsClaimChains covers the ordered-chain fold: the net
// effect of each legal chain, and the chains that stay illegal. add-then-update
// is the shape a follow-up ADR revising an earlier ADR's claim produces, which is
// what refused the severity effort's integration.
// invariant: invariants/current-state-authority:merge-transition-ordered-aggregate (TestMergeAggregateFoldsClaimChains)
func TestMergeAggregateFoldsClaimChains(t *testing.T) {
	base := rec("0137", "Implemented", op(adr.OpAdd, "d/t:base"))
	baseClaim := claim("d/t:base", "0137")

	// pair builds a two-ADR aggregate whose batches carry ops in sequence order.
	pair := func(first, second adr.Operation, afterClaims ...topic.Claim) []currentstate.Finding {
		a := v2rec("0141", "Implemented", []adr.Operation{first},
			v2status("Proposed"), v2status("Implementing"), v2batch(first), v2status("Implemented"))
		b := v2rec("0142", "Implemented", []adr.Operation{second},
			v2status("Proposed"), v2status("Implementing"), v2batch(second), v2status("Implemented"))
		return currentstate.CheckPair(
			uni([]adr.ADR{base}, baseClaim),
			uni([]adr.ADR{base, a, b}, append([]topic.Claim{baseClaim}, afterClaims...)...),
			currentstate.MergeAggregate)
	}

	t.Run("add then update is a net add", func(t *testing.T) {
		revised := prosed(claim("d/t:c", "0141", "0142"), "revised")
		if f := pair(op(adr.OpAdd, "d/t:c"), op(adr.OpUpdate, "d/t:c"), revised); len(f) != 0 {
			t.Fatalf("add-then-update must fold to a net add:\n%s", messages(f))
		}
	})

	t.Run("add then remove leaves the claim absent", func(t *testing.T) {
		if f := pair(op(adr.OpAdd, "d/t:c"), op(adr.OpRemove, "d/t:c")); len(f) != 0 {
			t.Fatalf("add-then-remove must fold to a net no-op:\n%s", messages(f))
		}
	})

	t.Run("an authored commit still refuses a two-operation chain", func(t *testing.T) {
		a := v2rec("0141", "Implemented", []adr.Operation{op(adr.OpAdd, "d/t:c")},
			v2status("Proposed"), v2status("Implementing"), v2batch(op(adr.OpAdd, "d/t:c")), v2status("Implemented"))
		b := v2rec("0142", "Implemented", []adr.Operation{op(adr.OpUpdate, "d/t:c")},
			v2status("Proposed"), v2status("Implementing"), v2batch(op(adr.OpUpdate, "d/t:c")), v2status("Implemented"))
		got := messages(currentstate.CheckPair(
			uni([]adr.ADR{base}, baseClaim),
			uni([]adr.ADR{base, a, b}, baseClaim, prosed(claim("d/t:c", "0141", "0142"), "revised")),
			currentstate.AuthoredCommit))
		if !strings.Contains(got, "target of more than one operation") {
			t.Fatalf("one operation per claim must still hold for an authored commit:\n%s", got)
		}
		if strings.Contains(got, "with no ADR add operation") {
			t.Fatalf("the duplicate report must not be joined by a contradictory one:\n%s", got)
		}
	})

	t.Run("update then remove is a net remove", func(t *testing.T) {
		// The claim exists before and is gone after, so the chain's net effect is
		// the remove. baseClaim must not survive into the after universe here.
		a := v2rec("0141", "Implemented", []adr.Operation{op(adr.OpUpdate, "d/t:base")},
			v2status("Proposed"), v2status("Implementing"), v2batch(op(adr.OpUpdate, "d/t:base")), v2status("Implemented"))
		b := v2rec("0142", "Implemented", []adr.Operation{op(adr.OpRemove, "d/t:base")},
			v2status("Proposed"), v2status("Implementing"), v2batch(op(adr.OpRemove, "d/t:base")), v2status("Implemented"))
		if f := currentstate.CheckPair(
			uni([]adr.ADR{base}, baseClaim),
			uni([]adr.ADR{base, a, b}),
			currentstate.MergeAggregate); len(f) != 0 {
			t.Fatalf("update-then-remove must fold to a net remove:\n%s", messages(f))
		}
	})

	t.Run("two updates by different ADRs are a net update", func(t *testing.T) {
		revised := prosed(claim("d/t:base", "0137", "0141", "0142"), "revised twice")
		before := uni([]adr.ADR{base}, prosed(baseClaim, "original"))
		a := v2rec("0141", "Implemented", []adr.Operation{op(adr.OpUpdate, "d/t:base")},
			v2status("Proposed"), v2status("Implementing"), v2batch(op(adr.OpUpdate, "d/t:base")), v2status("Implemented"))
		b := v2rec("0142", "Implemented", []adr.Operation{op(adr.OpUpdate, "d/t:base")},
			v2status("Proposed"), v2status("Implementing"), v2batch(op(adr.OpUpdate, "d/t:base")), v2status("Implemented"))
		if f := currentstate.CheckPair(before, uni([]adr.ADR{base, a, b}, revised), currentstate.MergeAggregate); len(f) != 0 {
			t.Fatalf("two updates must fold to a net update:\n%s", messages(f))
		}
	})

	t.Run("a second add is still illegal", func(t *testing.T) {
		got := messages(pair(op(adr.OpAdd, "d/t:c"), op(adr.OpAdd, "d/t:c"), claim("d/t:c", "0141")))
		if !strings.Contains(got, "illegal operation chain") || !strings.Contains(got, "an add must be the first operation") {
			t.Fatalf("a repeated add must stay illegal:\n%s", got)
		}
		// The chain diagnosis must not be joined by a contradictory
		// unmatched-mutation finding denying the operations exist.
		if strings.Contains(got, "was added with no ADR add operation") {
			t.Fatalf("a rejected chain must not also be reported as an unmatched mutation:\n%s", got)
		}
	})

	t.Run("a dominated update after a remove is legal", func(t *testing.T) {
		// The remove absorbs the concurrent update (ADR-0191): the chain is a
		// net remove and the update is retained as dominated history, so the
		// claim must be absent on the after side.
		a := v2rec("0141", "Implemented", []adr.Operation{op(adr.OpRemove, "d/t:base")},
			v2status("Proposed"), v2status("Implementing"), v2batch(op(adr.OpRemove, "d/t:base")), v2status("Implemented"))
		b := v2rec("0142", "Implemented", []adr.Operation{op(adr.OpUpdate, "d/t:base")},
			v2status("Proposed"), v2status("Implementing"), v2batch(op(adr.OpUpdate, "d/t:base")), v2status("Implemented"))
		if f := currentstate.CheckPair(
			uni([]adr.ADR{base}, baseClaim),
			uni([]adr.ADR{base, a, b}),
			currentstate.MergeAggregate); len(f) != 0 {
			t.Fatalf("a dominated update after a remove must be legal:\n%s", messages(f))
		}
	})

	t.Run("two removes are still illegal", func(t *testing.T) {
		got := messages(pair(op(adr.OpRemove, "d/t:base"), op(adr.OpRemove, "d/t:base")))
		if !strings.Contains(got, "at most one remove is allowed") {
			t.Fatalf("a second remove must stay illegal:\n%s", got)
		}
	})
}

// TestMergeAggregateRequiresRevisedByToGrowByEveryUpdater covers the generalized
// provenance rule: a folded net-update chain must append each updating ADR, so a
// Revised-by that grew by fewer entries than the chain has updaters is rejected.
// invariant: invariants/current-state-authority:merge-transition-ordered-aggregate (TestMergeAggregateRequiresRevisedByToGrowByEveryUpdater)
func TestMergeAggregateRequiresRevisedByToGrowByEveryUpdater(t *testing.T) {
	base := rec("0137", "Implemented", op(adr.OpAdd, "d/t:base"))
	update := op(adr.OpUpdate, "d/t:base")
	a := v2rec("0141", "Implemented", []adr.Operation{update},
		v2status("Proposed"), v2status("Implementing"), v2batch(update), v2status("Implemented"))
	b := v2rec("0142", "Implemented", []adr.Operation{update},
		v2status("Proposed"), v2status("Implementing"), v2batch(update), v2status("Implemented"))

	// Two updaters, but only one appended to Revised-by.
	got := messages(currentstate.CheckPair(
		uni([]adr.ADR{base}, prosed(claim("d/t:base", "0137"), "original")),
		uni([]adr.ADR{base, a, b}, prosed(claim("d/t:base", "0137", "0141"), "revised")),
		currentstate.MergeAggregate))
	if !strings.Contains(got, "must add the updating ADR-0142 to Revised-by") {
		t.Fatalf("a short Revised-by must be rejected:\n%s", got)
	}
}

// invariant: invariants/current-state-authority:merge-transition-ordered-aggregate (TestAggregateCorrectiveReapplication)
func TestAggregateCorrectiveReapplication(t *testing.T) {
	pending := op(adr.OpAdd, "d/t:pending")

	t.Run("repeated updates contribute one updater and a material endpoint", func(t *testing.T) {
		base := rec("0137", "Implemented", op(adr.OpAdd, "d/t:x"))
		update := op(adr.OpUpdate, "d/t:x")
		proposed := v2rec("0141", "Proposed", []adr.Operation{update, pending}, v2status("Proposed"))
		corrected := v2rec("0141", "Implementing", []adr.Operation{update, pending},
			v2status("Proposed"), v2status("Implementing"), v2batch(update), v2reapplied(update), v2reapplied(update))
		before := uni([]adr.ADR{base, proposed}, prosed(claim("d/t:x", "0137"), "old"))
		after := uni([]adr.ADR{base, corrected}, prosed(claim("d/t:x", "0137", "0141"), "corrected twice"))
		if f := currentstate.CheckPair(before, after, currentstate.MergeAggregate); len(f) != 0 {
			t.Fatalf("aggregate corrective updates rejected:\n%s", messages(f))
		}
		if got := messages(currentstate.CheckPair(before, after, currentstate.AuthoredCommit)); !strings.Contains(got, "at most one new batch") {
			t.Fatalf("one authored commit accepted several application occurrences:\n%s", got)
		}
		canceling := uni([]adr.ADR{base, corrected}, prosed(claim("d/t:x", "0137", "0141"), "old"))
		if got := messages(currentstate.CheckPair(before, canceling, currentstate.MergeAggregate)); !strings.Contains(got, "no canonical field changed") {
			t.Fatalf("canceling corrective update endpoint not rejected:\n%s", got)
		}
	})

	t.Run("second Applied update remains illegal", func(t *testing.T) {
		base := rec("0137", "Implemented", op(adr.OpAdd, "d/t:x"))
		update := op(adr.OpUpdate, "d/t:x")
		ordinaryTwice := v2rec("0141", "Implementing", []adr.Operation{update, pending},
			v2status("Proposed"), v2status("Implementing"), v2batch(update), v2batch(update))
		got := messages(currentstate.CheckPair(
			uni([]adr.ADR{base}, prosed(claim("d/t:x", "0137"), "old")),
			uni([]adr.ADR{base, ordinaryTwice}, prosed(claim("d/t:x", "0137", "0141"), "new")),
			currentstate.MergeAggregate))
		if !strings.Contains(got, "updates it more than once without a corrective Reapplied event") {
			t.Fatalf("second Applied update not rejected:\n%s", got)
		}
	})

	t.Run("repeated adds fold to one endpoint add", func(t *testing.T) {
		add := op(adr.OpAdd, "d/t:new")
		proposed := v2rec("0141", "Proposed", []adr.Operation{add, pending}, v2status("Proposed"))
		corrected := v2rec("0141", "Implementing", []adr.Operation{add, pending},
			v2status("Proposed"), v2status("Implementing"), v2batch(add), v2reapplied(add), v2reapplied(add))
		afterClaim := prosed(claim("d/t:new", "0141"), "corrected twice")
		if f := currentstate.CheckPair(uni([]adr.ADR{proposed}), uni([]adr.ADR{corrected}, afterClaim), currentstate.MergeAggregate); len(f) != 0 {
			t.Fatalf("aggregate corrective adds rejected:\n%s", messages(f))
		}
	})

	t.Run("correction-only repeated adds require a material endpoint", func(t *testing.T) {
		add := op(adr.OpAdd, "d/t:new")
		originBefore := v2rec("0141", "Implementing", []adr.Operation{add, pending},
			v2status("Proposed"), v2status("Implementing"), v2batch(add))
		originAfter := originBefore
		originAfter.History = append(append([]adr.HistoryEvent(nil), originBefore.History...), v2reapplied(add), v2reapplied(add))
		beforeClaim := prosed(claim("d/t:new", "0141"), "original")
		afterClaim := prosed(claim("d/t:new", "0141"), "corrected twice")
		if f := currentstate.CheckPair(
			uni([]adr.ADR{originBefore}, beforeClaim),
			uni([]adr.ADR{originAfter}, afterClaim),
			currentstate.MergeAggregate); len(f) != 0 {
			t.Fatalf("correction-only repeated adds rejected:\n%s", messages(f))
		}
	})

	t.Run("correction-only add composes with a later update", func(t *testing.T) {
		add := op(adr.OpAdd, "d/t:new")
		update := op(adr.OpUpdate, "d/t:new")
		originBefore := v2rec("0141", "Implementing", []adr.Operation{add, pending},
			v2status("Proposed"), v2status("Implementing"), v2batch(add))
		originAfter := originBefore
		originAfter.History = append(append([]adr.HistoryEvent(nil), originBefore.History...), v2reapplied(add))
		updater := v2rec("0142", "Implemented", []adr.Operation{update},
			v2status("Proposed"), v2status("Implemented"))
		updater.History = []adr.HistoryEvent{v2status("Proposed"), v2status("Implementing"), v2batch(update), v2status("Implemented")}
		beforeClaim := prosed(claim("d/t:new", "0141"), "original")
		afterClaim := prosed(claim("d/t:new", "0141", "0142"), "corrected then revised")
		if f := currentstate.CheckPair(
			uni([]adr.ADR{originBefore}, beforeClaim),
			uni([]adr.ADR{originAfter, updater}, afterClaim),
			currentstate.MergeAggregate); len(f) != 0 {
			t.Fatalf("correction-only add plus update rejected:\n%s", messages(f))
		}
	})

	t.Run("correction-only add composes with a later remove", func(t *testing.T) {
		add := op(adr.OpAdd, "d/t:new")
		remove := op(adr.OpRemove, "d/t:new")
		originBefore := v2rec("0141", "Implementing", []adr.Operation{add, pending},
			v2status("Proposed"), v2status("Implementing"), v2batch(add))
		originAfter := originBefore
		originAfter.History = append(append([]adr.HistoryEvent(nil), originBefore.History...), v2reapplied(add))
		remover := v2rec("0142", "Implemented", []adr.Operation{remove},
			v2status("Proposed"), v2status("Implementing"), v2batch(remove), v2status("Implemented"))
		if f := currentstate.CheckPair(
			uni([]adr.ADR{originBefore}, prosed(claim("d/t:new", "0141"), "original")),
			uni([]adr.ADR{originAfter, remover}),
			currentstate.MergeAggregate); len(f) != 0 {
			t.Fatalf("correction-only add plus remove rejected:\n%s", messages(f))
		}
	})
}

// TestMergeAggregateAcceptsMultiStepStatusHistory covers the third relaxed rule.
// An ADR the target already carries advancing Proposed -> Implementing -> Applied
// -> Implemented appends four events, which the fixed one-or-two-event shape
// refuses; the aggregate requires only that the prior history is an exact prefix.
// invariant: invariants/current-state-authority:merge-transition-ordered-aggregate (TestMergeAggregateAcceptsMultiStepStatusHistory)
func TestMergeAggregateAcceptsMultiStepStatusHistory(t *testing.T) {
	addX := op(adr.OpAdd, "d/t:x")
	proposed := v2rec("0141", "Proposed", []adr.Operation{addX, op(adr.OpAdd, "d/t:y")}, v2status("Proposed"))
	advanced := v2rec("0141", "Implemented", []adr.Operation{addX, op(adr.OpAdd, "d/t:y")},
		v2status("Proposed"), v2status("Implementing"), v2batch(addX), v2batch(op(adr.OpAdd, "d/t:y")), v2status("Implemented"))

	before := uni([]adr.ADR{proposed})
	after := uni([]adr.ADR{advanced}, claim("d/t:x", "0141"), claim("d/t:y", "0141"))

	if f := currentstate.CheckPair(before, after, currentstate.MergeAggregate); len(f) != 0 {
		t.Fatalf("a merge must accept a multi-step Status history:\n%s", messages(f))
	}
	if got := messages(currentstate.CheckPair(before, after, currentstate.AuthoredCommit)); !strings.Contains(got, "history-prefix rule") {
		t.Fatalf("an authored commit must still hold the fixed event shape:\n%s", got)
	}
}

// TestMergeAggregateStillRequiresAnExactHistoryPrefix covers the obligation the
// aggregate keeps: rewriting a retained event is not an append, in either mode.
// invariant: invariants/current-state-authority:merge-transition-ordered-aggregate (TestMergeAggregateStillRequiresAnExactHistoryPrefix)
func TestMergeAggregateStillRequiresAnExactHistoryPrefix(t *testing.T) {
	addX := op(adr.OpAdd, "d/t:x")
	before := v2rec("0141", "Implementing", []adr.Operation{addX, op(adr.OpAdd, "d/t:y")},
		v2status("Proposed"), v2status("Implementing"), v2batch(addX))
	rewritten := v2rec("0141", "Implementing", []adr.Operation{addX, op(adr.OpAdd, "d/t:y")},
		v2status("Proposed"), v2status("Accepted"), v2status("Implementing"), v2batch(addX))

	got := messages(currentstate.CheckPair(
		uni([]adr.ADR{before}, claim("d/t:x", "0141")),
		uni([]adr.ADR{rewritten}, claim("d/t:x", "0141")),
		currentstate.MergeAggregate))
	if !strings.Contains(got, "history-prefix rule") {
		t.Fatalf("a rewritten retained event must still be rejected:\n%s", got)
	}
}

// TestMergeAggregateOrdersBatchesByADRNumber covers the replacement ordering
// (ADR-0191): cross-ADR batch order is ascending ADR number, so a chain whose
// add is owned by the higher-numbered ADR is taken update-first and rejected,
// while the mirrored assignment is legal. No global counter is consulted.
// invariant: invariants/current-state-authority:merge-transition-ordered-aggregate (TestMergeAggregateOrdersBatchesByADRNumber)
func TestMergeAggregateOrdersBatchesByADRNumber(t *testing.T) {
	base := rec("0137", "Implemented", op(adr.OpAdd, "d/t:base"))
	baseClaim := claim("d/t:base", "0137")
	pair := func(firstNum string, first adr.Operation, secondNum string, second adr.Operation, afterClaims ...topic.Claim) []currentstate.Finding {
		a := v2rec(firstNum, "Implemented", []adr.Operation{first},
			v2status("Proposed"), v2status("Implementing"), v2batch(first), v2status("Implemented"))
		b := v2rec(secondNum, "Implemented", []adr.Operation{second},
			v2status("Proposed"), v2status("Implementing"), v2batch(second), v2status("Implemented"))
		return currentstate.CheckPair(
			uni([]adr.ADR{base}, baseClaim),
			uni([]adr.ADR{base, a, b}, append([]topic.Claim{baseClaim}, afterClaims...)...),
			currentstate.MergeAggregate)
	}

	added := prosed(claim("d/t:c", "0141", "0142"), "revised")
	if f := pair("0141", op(adr.OpAdd, "d/t:c"), "0142", op(adr.OpUpdate, "d/t:c"), added); len(f) != 0 {
		t.Fatalf("add by the lower ADR number must order before the update:\n%s", messages(f))
	}
	got := messages(pair("0142", op(adr.OpAdd, "d/t:c"), "0141", op(adr.OpUpdate, "d/t:c"), prosed(claim("d/t:c", "0142", "0141"), "revised")))
	if !strings.Contains(got, "an add must be the first operation") {
		t.Fatalf("an add owned by the higher ADR number must be taken after the update and rejected:\n%s", got)
	}

	// Within one ADR the tiebreak is history position: batch 0 adds and batch 1
	// removes, which is legal exactly because position orders the chain.
	addC, removeC := op(adr.OpAdd, "d/t:c"), op(adr.OpRemove, "d/t:c")
	oneADR := v2rec("0141", "Implemented", []adr.Operation{addC, removeC},
		v2status("Proposed"), v2status("Implementing"), v2batch(addC), v2batch(removeC), v2status("Implemented"))
	if f := currentstate.CheckPair(
		uni([]adr.ADR{base}, baseClaim),
		uni([]adr.ADR{base, oneADR}, baseClaim),
		currentstate.MergeAggregate); len(f) != 0 {
		t.Fatalf("intra-ADR batches must order by history position:\n%s", messages(f))
	}
}

// TestMergeAggregateNetNoopMustLeaveTheClaimAbsent covers the fold's one
// explicitly-checked net effect: a chain that both adds and removes a claim must
// leave it absent on both sides, reported in the chain's own terms rather than as
// an unmatched mutation that would deny the operations exist.
// invariant: invariants/current-state-authority:merge-transition-ordered-aggregate (TestMergeAggregateNetNoopMustLeaveTheClaimAbsent)
func TestMergeAggregateNetNoopMustLeaveTheClaimAbsent(t *testing.T) {
	base := rec("0137", "Implemented", op(adr.OpAdd, "d/t:base"))
	a := v2rec("0141", "Implemented", []adr.Operation{op(adr.OpAdd, "d/t:c")},
		v2status("Proposed"), v2status("Implementing"), v2batch(op(adr.OpAdd, "d/t:c")), v2status("Implemented"))
	b := v2rec("0142", "Implemented", []adr.Operation{op(adr.OpRemove, "d/t:c")},
		v2status("Proposed"), v2status("Implementing"), v2batch(op(adr.OpRemove, "d/t:c")), v2status("Implemented"))

	got := messages(currentstate.CheckPair(
		uni([]adr.ADR{base}, claim("d/t:base", "0137")),
		// The claim survives into the after universe, contradicting the net no-op.
		uni([]adr.ADR{base, a, b}, claim("d/t:base", "0137"), claim("d/t:c", "0141")),
		currentstate.MergeAggregate))
	if !strings.Contains(got, "added and removed within this transition") {
		t.Fatalf("a surviving net-noop claim must be reported in the chain's terms:\n%s", got)
	}
}

// TestMergeAggregateUnionsRevisedBy covers the union rule (ADR-0191 item 5):
// an updater inserting below an existing higher number is legal, a dropped
// prior entry is rejected, and an extra entry beyond the union is rejected.
// invariant: invariants/current-state-authority:merge-transition-ordered-aggregate (TestMergeAggregateUnionsRevisedBy)
func TestMergeAggregateUnionsRevisedBy(t *testing.T) {
	add := op(adr.OpAdd, "d/t:c")
	update := op(adr.OpUpdate, "d/t:c")
	origin := v2rec("0140", "Implemented", []adr.Operation{add},
		v2status("Proposed"), v2status("Implementing"), v2batch(add), v2status("Implemented"))
	high := v2rec("0142", "Implemented", []adr.Operation{update},
		v2status("Proposed"), v2status("Implementing"), v2batch(update), v2status("Implemented"))
	low := v2rec("0141", "Implemented", []adr.Operation{update},
		v2status("Proposed"), v2status("Implementing"), v2batch(update), v2status("Implemented"))
	before := uni([]adr.ADR{origin, high}, prosed(claim("d/t:c", "0140", "0142"), "revised"))

	pairWith := func(after topic.Claim) []currentstate.Finding {
		return currentstate.CheckPair(before,
			uni([]adr.ADR{origin, high, low}, after), currentstate.MergeAggregate)
	}

	if f := pairWith(prosed(claim("d/t:c", "0140", "0141", "0142"), "revised again")); len(f) != 0 {
		t.Fatalf("an updater inserting below a higher number must be legal:\n%s", messages(f))
	}
	if got := messages(pairWith(prosed(claim("d/t:c", "0140", "0141"), "revised again"))); !strings.Contains(got, "must preserve every prior Revised-by entry") {
		t.Fatalf("a dropped prior entry must be rejected:\n%s", got)
	}
	if got := messages(pairWith(prosed(claim("d/t:c", "0140", "0139", "0141", "0142"), "revised again"))); !strings.Contains(got, "must grow Revised-by to exactly the union") {
		t.Fatalf("an entry beyond the union must be rejected:\n%s", got)
	}
}
