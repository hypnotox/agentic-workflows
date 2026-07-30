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
// invariant: invariants/current-state-authority:merge-transition-ordered-aggregate
func TestMergeAggregateAcceptsSeveralBatchesFromOneADR(t *testing.T) {
	base := rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:base"))
	x, y, z := op(adr.OpAdd, "d/t:x"), op(adr.OpAdd, "d/t:y"), op(adr.OpAdd, "d/t:z")
	// A fourth declared operation stays unapplied so the record is legally
	// Implementing at both ends of the pair, which requires applied AND
	// remaining operations.
	pending := op(adr.OpAdd, "d/t:pending")
	partial := v2rec("0141", "Implementing", []adr.Operation{x, y, z, pending}, v2status("Proposed"), v2status("Implementing"), v2batch(2, x))
	three := partial
	three.History = append(append([]adr.HistoryEvent(nil), partial.History...), v2batch(3, y), v2batch(4, z))

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
// invariant: invariants/current-state-authority:merge-transition-ordered-aggregate
func TestMergeAggregateFoldsClaimChains(t *testing.T) {
	base := rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:base"))
	baseClaim := claim("d/t:base", "0137")

	// pair builds a two-ADR aggregate whose batches carry ops in sequence order.
	pair := func(first, second adr.Operation, afterClaims ...topic.Claim) []currentstate.Finding {
		a := v2rec("0141", "Implemented", []adr.Operation{first},
			v2status("Proposed"), v2status("Implementing"), v2batch(2, first), v2status("Implemented"))
		b := v2rec("0142", "Implemented", []adr.Operation{second},
			v2status("Proposed"), v2status("Implementing"), v2batch(3, second), v2status("Implemented"))
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

	t.Run("update then remove is a net remove", func(t *testing.T) {
		// The claim exists before and is gone after, so the chain's net effect is
		// the remove. baseClaim must not survive into the after universe here.
		a := v2rec("0141", "Implemented", []adr.Operation{op(adr.OpUpdate, "d/t:base")},
			v2status("Proposed"), v2status("Implementing"), v2batch(2, op(adr.OpUpdate, "d/t:base")), v2status("Implemented"))
		b := v2rec("0142", "Implemented", []adr.Operation{op(adr.OpRemove, "d/t:base")},
			v2status("Proposed"), v2status("Implementing"), v2batch(3, op(adr.OpRemove, "d/t:base")), v2status("Implemented"))
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
			v2status("Proposed"), v2status("Implementing"), v2batch(2, op(adr.OpUpdate, "d/t:base")), v2status("Implemented"))
		b := v2rec("0142", "Implemented", []adr.Operation{op(adr.OpUpdate, "d/t:base")},
			v2status("Proposed"), v2status("Implementing"), v2batch(3, op(adr.OpUpdate, "d/t:base")), v2status("Implemented"))
		if f := currentstate.CheckPair(before, uni([]adr.ADR{base, a, b}, revised), currentstate.MergeAggregate); len(f) != 0 {
			t.Fatalf("two updates must fold to a net update:\n%s", messages(f))
		}
	})

	t.Run("a second add is still illegal", func(t *testing.T) {
		got := messages(pair(op(adr.OpAdd, "d/t:c"), op(adr.OpAdd, "d/t:c"), claim("d/t:c", "0141")))
		if !strings.Contains(got, "illegal operation chain") || !strings.Contains(got, "an add must be the first operation") {
			t.Fatalf("a repeated add must stay illegal:\n%s", got)
		}
	})

	t.Run("an operation after a remove is still illegal", func(t *testing.T) {
		got := messages(pair(op(adr.OpRemove, "d/t:base"), op(adr.OpUpdate, "d/t:base")))
		if !strings.Contains(got, "no operation may follow a remove") {
			t.Fatalf("an op after a remove must stay illegal:\n%s", got)
		}
	})
}

// TestMergeAggregateRequiresRevisedByToGrowByEveryUpdater covers the generalized
// provenance rule: a folded net-update chain must append each updating ADR, so a
// Revised-by that grew by fewer entries than the chain has updaters is rejected.
// invariant: invariants/current-state-authority:merge-transition-ordered-aggregate
func TestMergeAggregateRequiresRevisedByToGrowByEveryUpdater(t *testing.T) {
	base := rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:base"))
	update := op(adr.OpUpdate, "d/t:base")
	a := v2rec("0141", "Implemented", []adr.Operation{update},
		v2status("Proposed"), v2status("Implementing"), v2batch(2, update), v2status("Implemented"))
	b := v2rec("0142", "Implemented", []adr.Operation{update},
		v2status("Proposed"), v2status("Implementing"), v2batch(3, update), v2status("Implemented"))

	// Two updaters, but only one appended to Revised-by.
	got := messages(currentstate.CheckPair(
		uni([]adr.ADR{base}, prosed(claim("d/t:base", "0137"), "original")),
		uni([]adr.ADR{base, a, b}, prosed(claim("d/t:base", "0137", "0141"), "revised")),
		currentstate.MergeAggregate))
	if !strings.Contains(got, "must extend Revised-by by exactly 2 entries") {
		t.Fatalf("a short Revised-by must be rejected:\n%s", got)
	}
}

// TestMergeAggregateRejectsRepeatedUpdateByOneADR covers the one clause the fold
// adds beyond ordering: update-requires-substance says an update appends its ADR
// once, so one ADR may not update a claim twice within one aggregate.
// invariant: invariants/current-state-authority:merge-transition-ordered-aggregate
func TestMergeAggregateRejectsRepeatedUpdateByOneADR(t *testing.T) {
	base := rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x"))
	update := op(adr.OpUpdate, "d/t:x")
	twice := v2rec("0141", "Implemented", []adr.Operation{update},
		v2status("Proposed"), v2status("Implementing"), v2batch(2, update), v2batch(3, update), v2status("Implemented"))

	got := messages(currentstate.CheckPair(
		uni([]adr.ADR{base}, prosed(claim("d/t:x", "0137"), "old")),
		uni([]adr.ADR{base, twice}, prosed(claim("d/t:x", "0137", "0141"), "new")),
		currentstate.MergeAggregate))
	if !strings.Contains(got, "updates it more than once") {
		t.Fatalf("one ADR updating a claim twice must be rejected:\n%s", got)
	}
}

// TestMergeAggregateAcceptsMultiStepStatusHistory covers the third relaxed rule.
// An ADR the target already carries advancing Proposed -> Implementing -> Applied
// -> Implemented appends four events, which the fixed one-or-two-event shape
// refuses; the aggregate requires only that the prior history is an exact prefix.
// invariant: invariants/current-state-authority:merge-transition-ordered-aggregate
func TestMergeAggregateAcceptsMultiStepStatusHistory(t *testing.T) {
	addX := op(adr.OpAdd, "d/t:x")
	proposed := v2rec("0141", "Proposed", []adr.Operation{addX, op(adr.OpAdd, "d/t:y")}, v2status("Proposed"))
	advanced := v2rec("0141", "Implemented", []adr.Operation{addX, op(adr.OpAdd, "d/t:y")},
		v2status("Proposed"), v2status("Implementing"), v2batch(1, addX), v2batch(2, op(adr.OpAdd, "d/t:y")), v2status("Implemented"))

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
// invariant: invariants/current-state-authority:merge-transition-ordered-aggregate
func TestMergeAggregateStillRequiresAnExactHistoryPrefix(t *testing.T) {
	addX := op(adr.OpAdd, "d/t:x")
	before := v2rec("0141", "Implementing", []adr.Operation{addX, op(adr.OpAdd, "d/t:y")},
		v2status("Proposed"), v2status("Implementing"), v2batch(1, addX))
	rewritten := v2rec("0141", "Implementing", []adr.Operation{addX, op(adr.OpAdd, "d/t:y")},
		v2status("Proposed"), v2status("Accepted"), v2status("Implementing"), v2batch(1, addX))

	got := messages(currentstate.CheckPair(
		uni([]adr.ADR{before}, claim("d/t:x", "0141")),
		uni([]adr.ADR{rewritten}, claim("d/t:x", "0141")),
		currentstate.MergeAggregate))
	if !strings.Contains(got, "history-prefix rule") {
		t.Fatalf("a rewritten retained event must still be rejected:\n%s", got)
	}
}

// TestMergeAggregateKeepsSequenceContiguity covers the rule ADR-0182 deliberately
// retains: a branch numbered before the target advanced still collides and must
// renumber before integrating.
// invariant: invariants/current-state-authority:merge-transition-ordered-aggregate
func TestMergeAggregateKeepsSequenceContiguity(t *testing.T) {
	base := rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:base"))
	addX := op(adr.OpAdd, "d/t:x")
	collide := v2rec("0141", "Implemented", []adr.Operation{addX},
		v2status("Proposed"), v2status("Implementing"), v2batch(5, addX), v2status("Implemented"))

	got := messages(currentstate.CheckPair(
		uni([]adr.ADR{base}, claim("d/t:base", "0137")),
		uni([]adr.ADR{base, collide}, claim("d/t:base", "0137"), claim("d/t:x", "0141")),
		currentstate.MergeAggregate))
	if !strings.Contains(got, "expected next sequence 2") {
		t.Fatalf("a merge must still require contiguous sequences:\n%s", got)
	}
}
