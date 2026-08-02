package currentstate_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// uni builds a Universe from ADR records and claims.
func uni(adrs []adr.ADR, cl ...topic.Claim) currentstate.Universe {
	return currentstate.Universe{ADRs: adrs, Topics: topics(cl...)}
}

// prose sets a claim's prose so a material change can be exercised.
func prosed(c topic.Claim, p string) topic.Claim { c.Prose = p; return c }

// TestCheckPairValidAdd accepts a Proposed->Implemented add: the claim appears
// with the adding ADR as its Origin and nothing else mutates.
func TestCheckPairValidAdd(t *testing.T) {
	before := uni([]adr.ADR{rec("0137", "Proposed", op(adr.OpAdd, "d/t:new"))})
	after := uni([]adr.ADR{rec("0137", "Implemented", op(adr.OpAdd, "d/t:new"))}, claim("d/t:new", "0137"))
	if f := currentstate.CheckPair(before, after, currentstate.AuthoredCommit); len(f) != 0 {
		t.Fatalf("expected no findings, got:\n%s", messages(f))
	}
}

// TestCheckPairValidUpdate accepts an Accepted->Implemented update that preserves
// Origin, appends the updating ADR to Revised-by, and changes the prose.
func TestCheckPairValidUpdate(t *testing.T) {
	accepted := rec("0138", "Accepted", op(adr.OpUpdate, "d/t:x"))
	implemented := rec("0138", "Implemented", op(adr.OpUpdate, "d/t:x"))
	implemented.History = append(append([]adr.StatusEntry(nil), accepted.History...), implemented.History[len(implemented.History)-1])
	before := uni(
		[]adr.ADR{
			rec("0137", "Implemented", op(adr.OpAdd, "d/t:x")),
			accepted,
		},
		prosed(claim("d/t:x", "0137"), "old"))
	after := uni(
		[]adr.ADR{
			rec("0137", "Implemented", op(adr.OpAdd, "d/t:x")),
			implemented,
		},
		prosed(claim("d/t:x", "0137", "0138"), "new"))
	if f := currentstate.CheckPair(before, after, currentstate.AuthoredCommit); len(f) != 0 {
		t.Fatalf("expected no findings, got:\n%s", messages(f))
	}
}

// TestCheckPairValidRemove accepts an Accepted->Implemented remove that retires
// the claim.
func TestCheckPairValidRemove(t *testing.T) {
	accepted := rec("0139", "Accepted", op(adr.OpRemove, "d/t:x"))
	implemented := rec("0139", "Implemented", op(adr.OpRemove, "d/t:x"))
	implemented.History = append(append([]adr.StatusEntry(nil), accepted.History...), implemented.History[len(implemented.History)-1])
	before := uni(
		[]adr.ADR{
			rec("0137", "Implemented", op(adr.OpAdd, "d/t:x")),
			accepted,
		},
		claim("d/t:x", "0137"))
	after := uni([]adr.ADR{
		rec("0137", "Implemented", op(adr.OpAdd, "d/t:x")),
		implemented,
	})
	if f := currentstate.CheckPair(before, after, currentstate.AuthoredCommit); len(f) != 0 {
		t.Fatalf("expected no findings, got:\n%s", messages(f))
	}
}

// TestCheckPairBootstrapAddExempt accepts a claim first appearing with an Origin
// below cutoff and no add operation: the closed migration bootstrap.
func TestCheckPairBootstrapAddExempt(t *testing.T) {
	before := uni(nil)
	after := uni([]adr.ADR{{Number: "0100", Format: adr.Legacy, Status: "Implemented"}}, claim("d/t:legacy", "0100"))
	if f := currentstate.CheckPair(before, after, currentstate.AuthoredCommit); len(f) != 0 {
		t.Fatalf("expected no findings for bootstrap add, got:\n%s", messages(f))
	}
}

// TestCheckPairUnchangedClaim accepts a claim that persists identically across
// the pair with no operation touching it.
func TestCheckPairUnchangedClaim(t *testing.T) {
	legacy := []adr.ADR{{Number: "0100", Format: adr.Legacy, Status: "Implemented"}}
	before := uni(legacy, prosed(claim("d/t:keep", "0100"), "steady"))
	after := uni(legacy, prosed(claim("d/t:keep", "0100"), "steady"))
	if f := currentstate.CheckPair(before, after, currentstate.AuthoredCommit); len(f) != 0 {
		t.Fatalf("expected no findings, got:\n%s", messages(f))
	}
}

// TestCheckPairDeletedV1ADR rejects removal of a governed ADR record.
func TestCheckPairDeletedV1ADR(t *testing.T) {
	before := uni([]adr.ADR{rec("0137", "Implemented")})
	if f := currentstate.CheckPair(before, uni(nil), currentstate.AuthoredCommit); !strings.Contains(messages(f), "current-state-v1 ADR-0137 was deleted") {
		t.Fatalf("deleted ADR not reported:\n%s", messages(f))
	}
}

// TestCheckPairIllegalTransition rejects an edge out of a terminal state.
func TestCheckPairIllegalTransition(t *testing.T) {
	before := uni([]adr.ADR{rec("0137", "Implemented")})
	after := uni([]adr.ADR{rec("0137", "Abandoned")})
	if f := currentstate.CheckPair(before, after, currentstate.AuthoredCommit); !strings.Contains(messages(f), "ADR-0137 changed status from Implemented to Abandoned, which is not a legal") {
		t.Fatalf("illegal transition not reported:\n%s", messages(f))
	}
}

// TestCheckPairFrozenAndHistoryRules rejects content rewrites after Proposed and
// any Status history change other than the one-entry append of a legal edge.
func TestCheckPairFrozenAndHistoryRules(t *testing.T) {
	entry := func(status, digest string) adr.StatusEntry {
		return adr.StatusEntry{Date: "2026-01-02", Status: status, Digest: digest}
	}
	record := func(status, body string, history ...adr.StatusEntry) adr.ADR {
		return adr.ADR{
			Number:   "0137",
			Format:   adr.CurrentStateV1,
			Status:   status,
			Sections: map[string]string{"Decision": body},
			History:  history,
		}
	}
	proposed := adr.StatusEntry{Date: "2026-01-01", Status: "Proposed"}

	cases := []struct {
		name          string
		before, after adr.ADR
		want          string
	}{
		{"same-status Accepted semantic rewrite", record("Accepted", "old", proposed, entry("Accepted", "old-digest")), record("Accepted", "new", proposed, entry("Accepted", "old-digest")), "frozen-content rule"},
		{"same-status Implemented semantic rewrite", record("Implemented", "old", proposed, entry("Implemented", "old-digest")), record("Implemented", "new", proposed, entry("Implemented", "old-digest")), "frozen-content rule"},
		{"same-status Abandoned semantic rewrite", record("Abandoned", "old", proposed, entry("Abandoned", "old-digest")), record("Abandoned", "new", proposed, entry("Abandoned", "old-digest")), "frozen-content rule"},
		{"recomputed digest rewrite", record("Accepted", "old", proposed, entry("Accepted", "old-digest")), record("Accepted", "new", proposed, entry("Accepted", "new-digest")), "frozen-content rule"},
		{"history truncation", record("Implemented", "same", proposed, entry("Accepted", "digest"), entry("Implemented", "digest")), record("Implemented", "same", proposed, entry("Implemented", "digest")), "history-prefix rule"},
		{"history replacement", record("Accepted", "same", proposed, entry("Accepted", "digest")), record("Accepted", "same", proposed, adr.StatusEntry{Date: "2026-01-09", Status: "Proposed"}, entry("Accepted", "digest")), "history-prefix rule"},
		{"legal transition rewrites earlier entry", record("Accepted", "same", proposed, entry("Accepted", "digest")), record("Implemented", "same", adr.StatusEntry{Date: "2026-01-03", Status: "Proposed"}, entry("Accepted", "digest"), entry("Implemented", "digest")), "history-prefix rule"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := messages(currentstate.CheckPair(uni([]adr.ADR{tc.before}), uni([]adr.ADR{tc.after}), currentstate.AuthoredCommit))
			if !strings.Contains(got, "ADR-0137") || !strings.Contains(got, tc.want) {
				t.Fatalf("want ADR number and %q in:\n%s", tc.want, got)
			}
		})
	}
	formatBefore := record("Proposed", "same", proposed)
	formatAfter := formatBefore
	formatAfter.Format = adr.CurrentStateV2
	if got := messages(currentstate.CheckPair(uni([]adr.ADR{formatBefore}), uni([]adr.ADR{formatAfter}), currentstate.AuthoredCommit)); !strings.Contains(got, "changed governed format") {
		t.Fatalf("governed format mutation not rejected:\n%s", got)
	}
}

// TestCheckPairAmendedContent covers the V2 amendability window across pairs:
// a non-terminal record's content may change when the commit appends exactly
// one Amended event, a terminal record stays frozen, and a merge aggregate
// accepts Amended events interleaved with Applied batches (ADR-0188).
func TestCheckPairAmendedContent(t *testing.T) {
	v2doc := func(status, body string, events ...adr.HistoryEvent) adr.ADR {
		return adr.ADR{Number: "0141", Format: adr.CurrentStateV2, Status: status,
			Sections: map[string]string{"Decision": body}, History: events}
	}
	proposed := v2status("Proposed")
	accepted := adr.HistoryEvent{Kind: adr.HistoryStatus, Date: "2026-01-02", Status: "Accepted", Digest: "old-digest"}
	amended := adr.HistoryEvent{Kind: adr.HistoryAmended, Date: "2026-01-03", Digest: "new-digest"}

	t.Run("accepted amendment with event is finding-free", func(t *testing.T) {
		before := uni([]adr.ADR{v2doc("Accepted", "old", proposed, accepted)})
		after := uni([]adr.ADR{v2doc("Accepted", "new", proposed, accepted, amended)})
		if f := currentstate.CheckPair(before, after, currentstate.AuthoredCommit); len(f) != 0 {
			t.Fatalf("amendment with Amended event must be finding-free:\n%s", messages(f))
		}
	})

	t.Run("terminal content change stays frozen", func(t *testing.T) {
		done := adr.HistoryEvent{Kind: adr.HistoryStatus, Date: "2026-01-04", Status: "Implemented", Digest: "old-digest"}
		before := uni([]adr.ADR{v2doc("Implemented", "old", proposed, accepted, done)})
		after := uni([]adr.ADR{v2doc("Implemented", "new", proposed, accepted, done)})
		got := messages(currentstate.CheckPair(before, after, currentstate.AuthoredCommit))
		if !strings.Contains(got, "frozen-content rule") || !strings.Contains(got, "changed after the record froze") {
			t.Fatalf("terminal rewrite not reported as frozen-content violation:\n%s", got)
		}
	})

	t.Run("merge aggregate interleaves amendments and batches", func(t *testing.T) {
		base := rec("0137", "Implemented", op(adr.OpAdd, "d/t:base"))
		x, y := op(adr.OpAdd, "d/t:x"), op(adr.OpAdd, "d/t:y")
		pending := op(adr.OpAdd, "d/t:pending")
		partial := v2doc("Implementing", "old", proposed, v2status("Implementing"), v2batch(x))
		partial.Operations = []adr.Operation{x, y, pending}
		merged := v2doc("Implementing", "new", proposed, v2status("Implementing"), v2batch(x), amended, v2batch(y))
		merged.Operations = partial.Operations
		before := uni([]adr.ADR{base, partial}, claim("d/t:base", "0137"), claim("d/t:x", "0141"))
		after := uni([]adr.ADR{base, merged}, claim("d/t:base", "0137"), claim("d/t:x", "0141"), claim("d/t:y", "0141"))
		if f := currentstate.CheckPair(before, after, currentstate.MergeAggregate); len(f) != 0 {
			t.Fatalf("aggregate with interleaved Amended events must be finding-free:\n%s", messages(f))
		}
	})
}

// invariant: adr-system/adr-lifecycle:corrective-reapplication (TestAuthoredCorrectiveReapplication)
func TestAuthoredCorrectiveReapplication(t *testing.T) {
	pending := op(adr.OpAdd, "d/t:pending")

	t.Run("update preserves canonical provenance and changes substance", func(t *testing.T) {
		update := op(adr.OpUpdate, "d/t:x")
		base := rec("0137", "Implemented", op(adr.OpAdd, "d/t:x"))
		beforeADR := v2rec("0141", "Implementing", []adr.Operation{update, pending},
			v2status("Proposed"), v2status("Implementing"), v2batch(update))
		afterADR := beforeADR
		afterADR.History = append(append([]adr.HistoryEvent(nil), beforeADR.History...), v2reapplied(update))
		before := uni([]adr.ADR{base, beforeADR}, prosed(claim("d/t:x", "0137", "0141"), "first revision"))
		afterClaim := prosed(claim("d/t:x", "0137", "0141"), "corrected revision")
		firstCorrection := uni([]adr.ADR{base, afterADR}, afterClaim)
		if f := currentstate.CheckPair(before, firstCorrection, currentstate.AuthoredCommit); len(f) != 0 {
			t.Fatalf("corrective update rejected:\n%s", messages(f))
		}
		secondADR := afterADR
		secondADR.History = append(append([]adr.HistoryEvent(nil), afterADR.History...), v2reapplied(update))
		secondClaim := prosed(claim("d/t:x", "0137", "0141"), "corrected revision again")
		if f := currentstate.CheckPair(firstCorrection, uni([]adr.ADR{base, secondADR}, secondClaim), currentstate.AuthoredCommit); len(f) != 0 {
			t.Fatalf("second corrective update rejected:\n%s", messages(f))
		}
		if got := messages(currentstate.CheckPair(before, uni([]adr.ADR{base, afterADR}, prosed(claim("d/t:x", "0137", "0141"), "first revision")), currentstate.AuthoredCommit)); !strings.Contains(got, "no canonical field changed") {
			t.Fatalf("non-material corrective update not rejected:\n%s", got)
		}
		if got := messages(currentstate.CheckPair(before, uni([]adr.ADR{base, afterADR}, prosed(claim("d/t:x", "0140", "0141"), "corrected revision")), currentstate.AuthoredCommit)); !strings.Contains(got, "must preserve Origin") {
			t.Fatalf("corrective update Origin change not rejected:\n%s", got)
		}
	})

	t.Run("add preserves origin and Revised-by", func(t *testing.T) {
		add := op(adr.OpAdd, "d/t:new")
		beforeADR := v2rec("0141", "Implementing", []adr.Operation{add, pending},
			v2status("Proposed"), v2status("Implementing"), v2batch(add))
		afterADR := beforeADR
		afterADR.History = append(append([]adr.HistoryEvent(nil), beforeADR.History...), v2reapplied(add))
		beforeClaim := prosed(claim("d/t:new", "0141"), "first wording")
		afterClaim := prosed(claim("d/t:new", "0141"), "corrected wording")
		before := uni([]adr.ADR{beforeADR}, beforeClaim)
		if f := currentstate.CheckPair(before, uni([]adr.ADR{afterADR}, afterClaim), currentstate.AuthoredCommit); len(f) != 0 {
			t.Fatalf("corrective add rejected:\n%s", messages(f))
		}
		if got := messages(currentstate.CheckPair(before, uni([]adr.ADR{afterADR}), currentstate.AuthoredCommit)); !strings.Contains(got, "not present on both sides") {
			t.Fatalf("corrective add missing endpoint not rejected:\n%s", got)
		}
		if got := messages(currentstate.CheckPair(uni([]adr.ADR{beforeADR}, beforeClaim), uni([]adr.ADR{afterADR}, prosed(claim("d/t:new", "0141"), "first wording")), currentstate.AuthoredCommit)); !strings.Contains(got, "no canonical field changed") {
			t.Fatalf("non-material corrective add not rejected:\n%s", got)
		}
		wrong := prosed(claim("d/t:new", "0140", "0141"), "corrected wording")
		got := messages(currentstate.CheckPair(uni([]adr.ADR{beforeADR}, beforeClaim), uni([]adr.ADR{afterADR}, wrong), currentstate.AuthoredCommit))
		if !strings.Contains(got, "must preserve Origin") || !strings.Contains(got, "must preserve Revised-by byte-identically") {
			t.Fatalf("corrective add provenance defects not rejected:\n%s", got)
		}
	})

	t.Run("remove is refused", func(t *testing.T) {
		remove := op(adr.OpRemove, "d/t:x")
		beforeADR := v2rec("0141", "Implementing", []adr.Operation{remove, pending},
			v2status("Proposed"), v2status("Implementing"), v2batch(remove))
		afterADR := beforeADR
		afterADR.History = append(append([]adr.HistoryEvent(nil), beforeADR.History...), v2reapplied(remove))
		got := messages(currentstate.CheckPair(uni([]adr.ADR{beforeADR}), uni([]adr.ADR{afterADR}), currentstate.AuthoredCommit))
		if !strings.Contains(got, "only add or update may be reapplied") {
			t.Fatalf("corrective remove not rejected:\n%s", got)
		}
	})
}

// TestCheckPairHistoryValid accepts Proposed edits before freezing and every
// legal edge when Status history appends exactly one entry.
func TestCheckPairHistoryValid(t *testing.T) {
	proposed := adr.StatusEntry{Date: "2026-01-01", Status: "Proposed"}
	cases := []struct {
		name, from, to string
	}{
		{"Proposed body edit", "Proposed", "Proposed"},
		{"Proposed to Accepted", "Proposed", "Accepted"},
		{"Proposed to Implemented", "Proposed", "Implemented"},
		{"Proposed to Abandoned", "Proposed", "Abandoned"},
		{"Accepted to Implemented", "Accepted", "Implemented"},
		{"Accepted to Abandoned", "Accepted", "Abandoned"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			beforeHistory := []adr.StatusEntry{proposed}
			if tc.from == "Accepted" {
				beforeHistory = append(beforeHistory, adr.StatusEntry{Date: "2026-01-02", Status: "Accepted"})
			}
			afterHistory := append([]adr.StatusEntry(nil), beforeHistory...)
			if tc.to != tc.from {
				afterHistory = append(afterHistory, adr.StatusEntry{Date: "2026-01-03", Status: tc.to})
			}
			before := adr.ADR{Number: "0137", Format: adr.CurrentStateV1, Status: tc.from, Sections: map[string]string{"Decision": "before"}, History: beforeHistory}
			afterDecision := "before"
			if tc.from == "Proposed" {
				afterDecision = "after"
			}
			after := adr.ADR{Number: "0137", Format: adr.CurrentStateV1, Status: tc.to, Sections: map[string]string{"Decision": afterDecision}, History: afterHistory}
			if f := currentstate.CheckPair(uni([]adr.ADR{before}), uni([]adr.ADR{after}), currentstate.AuthoredCommit); len(f) != 0 {
				t.Fatalf("expected no findings, got:\n%s", messages(f))
			}
		})
	}
}

// invariant: invariants/current-state-authority:state-impact-transition-atomic (TestCheckPairV2IncrementalBatches)
// invariant: invariants/current-state-authority:implemented-impact-bidirectional (TestCheckPairV2IncrementalBatches)
func TestCheckPairV2IncrementalBatches(t *testing.T) {
	addX := op(adr.OpAdd, "d/t:x")
	updateX := op(adr.OpUpdate, "d/t:x")
	addA := op(adr.OpAdd, "d/t:a")
	addB := op(adr.OpAdd, "d/t:b")
	base := rec("0137", "Implemented", addX)
	proposed := v2rec("0138", "Proposed", []adr.Operation{updateX, addA, addB}, v2status("Proposed"))
	first := proposed
	first.Status = "Implementing"
	first.History = append(append([]adr.HistoryEvent(nil), proposed.History...), v2status("Implementing"), v2batch(updateX))
	before := uni([]adr.ADR{base, proposed}, prosed(claim("d/t:x", "0137"), "old"))
	after := uni([]adr.ADR{base, first}, prosed(claim("d/t:x", "0137", "0138"), "new"))
	if f := currentstate.CheckPair(before, after, currentstate.AuthoredCommit); len(f) != 0 {
		t.Fatalf("first batch pair rejected:\n%s", messages(f))
	}

	middle := first
	middle.History = append(append([]adr.HistoryEvent(nil), first.History...), v2batch(addA))
	middleAfter := uni([]adr.ADR{base, middle}, prosed(claim("d/t:x", "0137", "0138"), "new"), claim("d/t:a", "0138"))
	if f := currentstate.CheckPair(after, middleAfter, currentstate.AuthoredCommit); len(f) != 0 {
		t.Fatalf("middle batch pair rejected:\n%s", messages(f))
	}

	done := middle
	done.Status = "Implemented"
	done.History = append(append([]adr.HistoryEvent(nil), middle.History...), v2batch(addB), v2status("Implemented"))
	doneAfter := uni([]adr.ADR{base, done}, prosed(claim("d/t:x", "0137", "0138"), "new"), claim("d/t:a", "0138"), claim("d/t:b", "0138"))
	if f := currentstate.CheckPair(middleAfter, doneAfter, currentstate.AuthoredCommit); len(f) != 0 {
		t.Fatalf("final batch pair rejected:\n%s", messages(f))
	}

	assertSplit := func(name string, before, after currentstate.Universe) {
		t.Helper()
		eventOnly := currentstate.Universe{ADRs: after.ADRs, Topics: before.Topics}
		if got := messages(currentstate.CheckPair(before, eventOnly, currentstate.AuthoredCommit)); got == "" || strings.Contains(got, "with no ADR") {
			t.Fatalf("%s event-without-mutation was not operation-attributed:\n%s", name, got)
		}
		mutationOnly := currentstate.Universe{ADRs: before.ADRs, Topics: after.Topics}
		if got := messages(currentstate.CheckPair(before, mutationOnly, currentstate.AuthoredCommit)); !strings.Contains(got, "with no ADR") {
			t.Fatalf("%s mutation-without-event was accepted:\n%s", name, got)
		}
	}
	assertSplit("first", before, after)
	assertSplit("middle", after, middleAfter)
	assertSplit("final", middleAfter, doneAfter)

	removeD := op(adr.OpRemove, "d/t:d")
	baseD := rec("0140", "Implemented", op(adr.OpAdd, "d/t:d"))
	directBeforeADR := v2rec("0141", "Accepted", []adr.Operation{removeD}, v2status("Proposed"), v2status("Accepted"))
	directAfterADR := directBeforeADR
	directAfterADR.Status = "Implemented"
	directAfterADR.History = append(append([]adr.HistoryEvent(nil), directBeforeADR.History...), v2status("Implemented"))
	directBefore := uni([]adr.ADR{baseD, directBeforeADR}, claim("d/t:d", "0140"))
	directAfter := uni([]adr.ADR{baseD, directAfterADR})
	if f := currentstate.CheckPair(directBefore, directAfter, currentstate.AuthoredCommit); len(f) != 0 {
		t.Fatalf("direct batch pair rejected:\n%s", messages(f))
	}
	assertSplit("direct", directBefore, directAfter)

	abandoned := middle
	abandoned.Status = "Abandoned"
	abandoned.History = append(append([]adr.HistoryEvent(nil), middle.History...), v2status("Abandoned"))
	if f := currentstate.CheckPair(middleAfter, uni([]adr.ADR{base, abandoned}, prosed(claim("d/t:x", "0137", "0138"), "new"), claim("d/t:a", "0138")), currentstate.AuthoredCommit); len(f) != 0 {
		t.Fatalf("terminal abandonment pair rejected:\n%s", messages(f))
	}

	deleted := first
	deleted.History = append([]adr.HistoryEvent(nil), first.History[:2]...)
	if got := messages(currentstate.CheckPair(after, uni([]adr.ADR{base, deleted}, prosed(claim("d/t:x", "0137", "0138"), "new")), currentstate.AuthoredCommit)); !strings.Contains(got, "history-prefix rule") {
		t.Fatalf("Applied event deletion not rejected:\n%s", got)
	}
}

func TestCheckPairV2BatchSetRules(t *testing.T) {
	base := rec("0137", "Implemented", op(adr.OpAdd, "d/t:base"))
	direct := func(num, id string) (adr.ADR, adr.ADR) {
		operation := op(adr.OpAdd, id)
		before := v2rec(num, "Accepted", []adr.Operation{operation}, v2status("Proposed"), v2status("Accepted"))
		after := before
		after.Status = "Implemented"
		after.History = append(append([]adr.HistoryEvent(nil), before.History...), v2status("Implemented"))
		return before, after
	}
	b1, a1 := direct("0138", "d/t:a")
	b2, a2 := direct("0139", "d/t:b")
	before := uni([]adr.ADR{base, b1, b2}, claim("d/t:base", "0137"))
	after := uni([]adr.ADR{base, a1, a2}, claim("d/t:base", "0137"), claim("d/t:a", "0138"), claim("d/t:b", "0139"))
	if f := currentstate.CheckPair(before, after, currentstate.AuthoredCommit); len(f) != 0 {
		t.Fatalf("disjoint consecutive batches rejected:\n%s", messages(f))
	}

	_, duplicateTarget := direct("0140", "d/t:a")
	if got := messages(currentstate.CheckPair(uni([]adr.ADR{base, b1, b2}), uni([]adr.ADR{base, a1, duplicateTarget}, claim("d/t:a", "0138")), currentstate.AuthoredCommit)); !strings.Contains(got, "target of more than one operation") {
		t.Fatalf("cross-batch duplicate target not rejected:\n%s", got)
	}

	x, y, z := op(adr.OpAdd, "d/t:x"), op(adr.OpAdd, "d/t:y"), op(adr.OpAdd, "d/t:z")
	partial := v2rec("0141", "Implementing", []adr.Operation{x, y, z}, v2status("Proposed"), v2status("Implementing"), v2batch(x))
	two := partial
	two.History = append(append([]adr.HistoryEvent(nil), partial.History...), v2batch(y), v2batch(z))
	if got := messages(currentstate.CheckPair(uni([]adr.ADR{base, partial}, claim("d/t:base", "0137"), claim("d/t:x", "0141")), uni([]adr.ADR{base, two}, claim("d/t:base", "0137"), claim("d/t:x", "0141"), claim("d/t:y", "0141"), claim("d/t:z", "0141")), currentstate.AuthoredCommit)); !strings.Contains(got, "at most one new batch") {
		t.Fatalf("same ADR duplicate batch not rejected:\n%s", got)
	}

	terminal := v2rec("0142", "Implemented", nil, v2status("Proposed"), v2status("Implemented"))
	illegal := terminal
	illegal.Status = "Abandoned"
	illegal.History = append(illegal.History, v2status("Abandoned"))
	if got := messages(currentstate.CheckPair(uni([]adr.ADR{terminal}), uni([]adr.ADR{illegal}), currentstate.AuthoredCommit)); !strings.Contains(got, "legal current-state-v2 transition") {
		t.Fatalf("illegal V2 edge not format-attributed:\n%s", got)
	}

	invalid := v2rec("0143", "Implemented", []adr.Operation{op(adr.OpAdd, "d/t:invalid")})
	if got := messages(currentstate.CheckPair(uni([]adr.ADR{invalid}), uni([]adr.ADR{invalid}), currentstate.AuthoredCommit)); !strings.Contains(got, "no Implemented status event") {
		t.Fatalf("invalid before/after projection not reported:\n%s", got)
	}
}

// TestCheckPairMismatches covers each way an operation and a claim mutation fail
// to correspond, asserting the pair-specific message even when the after-state
// static check also fires.
func TestCheckPairMismatches(t *testing.T) {
	cases := []struct {
		name          string
		before, after currentstate.Universe
		cutoff        int
		want          string
	}{
		{
			name:   "add of an existing claim",
			before: uni([]adr.ADR{rec("0137", "Implemented", op(adr.OpAdd, "d/t:x"))}, claim("d/t:x", "0137")),
			after: uni([]adr.ADR{
				rec("0137", "Implemented", op(adr.OpAdd, "d/t:x")),
				rec("0138", "Implemented", op(adr.OpAdd, "d/t:x")),
			}, claim("d/t:x", "0137")),
			cutoff: 137,
			want:   "ADR-0138 adds claim d/t:x, which already existed before this transition",
		},
		{
			name:   "remove of an absent claim",
			before: uni(nil),
			after:  uni([]adr.ADR{rec("0137", "Implemented", op(adr.OpRemove, "d/t:x"))}),
			cutoff: 137,
			want:   "ADR-0137 removes claim d/t:x, which did not exist before this transition",
		},
		{
			name:   "update of a claim absent after",
			before: uni([]adr.ADR{{Number: "0100", Format: adr.Legacy, Status: "Implemented"}}, claim("d/t:x", "0100")),
			after:  uni([]adr.ADR{rec("0137", "Implemented", op(adr.OpUpdate, "d/t:x"))}),
			cutoff: 137,
			want:   "ADR-0137 updates claim d/t:x, which is not present on both sides",
		},
		{
			name: "update with no canonical change",
			before: uni([]adr.ADR{rec("0137", "Implemented", op(adr.OpAdd, "d/t:x"))},
				prosed(claim("d/t:x", "0137"), "same")),
			after: uni([]adr.ADR{
				rec("0137", "Implemented", op(adr.OpAdd, "d/t:x")),
				rec("0138", "Implemented", op(adr.OpUpdate, "d/t:x")),
			}, prosed(claim("d/t:x", "0137", "0138"), "same\n")),
			cutoff: 137,
			want:   "ADR-0138 updates claim d/t:x, but no canonical field changed",
		},
		{
			name: "update changing Origin",
			before: uni([]adr.ADR{rec("0137", "Implemented", op(adr.OpAdd, "d/t:x"))},
				prosed(claim("d/t:x", "0137"), "old")),
			after: uni([]adr.ADR{
				rec("0137", "Implemented", op(adr.OpAdd, "d/t:x")),
				rec("0138", "Implemented", op(adr.OpUpdate, "d/t:x")),
			}, prosed(claim("d/t:x", "0199", "0138"), "new")),
			cutoff: 137,
			want:   "update of claim d/t:x changed its Origin from ADR-0137 to ADR-0199",
		},
		{
			name: "update not appending Revised-by",
			before: uni([]adr.ADR{rec("0137", "Implemented", op(adr.OpAdd, "d/t:x"))},
				prosed(claim("d/t:x", "0137"), "old")),
			after: uni([]adr.ADR{
				rec("0137", "Implemented", op(adr.OpAdd, "d/t:x")),
				rec("0138", "Implemented", op(adr.OpUpdate, "d/t:x")),
			}, prosed(claim("d/t:x", "0137"), "new")),
			cutoff: 137,
			want:   "must add the updating ADR-0138 to Revised-by",
		},
		{
			name: "update dropping a prior Revised-by entry",
			before: uni([]adr.ADR{
				rec("0137", "Implemented", op(adr.OpAdd, "d/t:x")),
				rec("0138", "Implemented", op(adr.OpUpdate, "d/t:x")),
			}, prosed(claim("d/t:x", "0137", "0138"), "v1")),
			after: uni([]adr.ADR{
				rec("0137", "Implemented", op(adr.OpAdd, "d/t:x")),
				rec("0138", "Implemented", op(adr.OpUpdate, "d/t:x")),
				rec("0140", "Implemented", op(adr.OpUpdate, "d/t:x")),
			}, prosed(claim("d/t:x", "0137", "0199", "0140"), "v2")),
			cutoff: 137,
			want:   "must preserve every prior Revised-by entry",
		},
		{
			name: "update appending the wrong ADR",
			before: uni([]adr.ADR{rec("0137", "Implemented", op(adr.OpAdd, "d/t:x"))},
				prosed(claim("d/t:x", "0137"), "old")),
			after: uni([]adr.ADR{
				rec("0137", "Implemented", op(adr.OpAdd, "d/t:x")),
				rec("0140", "Implemented", op(adr.OpUpdate, "d/t:x")),
			}, prosed(claim("d/t:x", "0137", "0199"), "new")),
			cutoff: 137,
			want:   "must add the updating ADR-0140 to Revised-by",
		},
		{
			name:   "added claim with no operation",
			before: uni(nil),
			after:  uni(nil, claim("d/t:x", "0137")),
			cutoff: 137,
			want:   "claim d/t:x was added with no ADR add operation in this transition",
		},
		{
			name:   "added claim with no operation and no cutoff",
			before: uni(nil),
			after:  uni(nil, claim("d/t:x", "0100")),
			cutoff: 0,
			want:   "claim d/t:x was added with no ADR add operation in this transition",
		},
		{
			name:   "removed claim with no operation",
			before: uni([]adr.ADR{{Number: "0100", Format: adr.Legacy, Status: "Implemented"}}, claim("d/t:x", "0100")),
			after:  uni([]adr.ADR{{Number: "0100", Format: adr.Legacy, Status: "Implemented"}}),
			cutoff: 137,
			want:   "claim d/t:x was removed with no ADR remove operation in this transition",
		},
		{
			name: "changed claim with no operation",
			before: uni([]adr.ADR{{Number: "0100", Format: adr.Legacy, Status: "Implemented"}},
				prosed(claim("d/t:x", "0100"), "old")),
			after: uni([]adr.ADR{{Number: "0100", Format: adr.Legacy, Status: "Implemented"}},
				prosed(claim("d/t:x", "0100"), "new")),
			cutoff: 137,
			want:   "claim d/t:x was changed with no ADR update operation in this transition",
		},
		{
			name:   "origin-only change with no operation",
			before: uni(nil, claim("d/t:x", "0100")),
			after:  uni(nil, claim("d/t:x", "0101")),
			cutoff: 200,
			want:   "claim d/t:x was changed with no ADR update operation in this transition",
		},
		{
			name:   "revised-by-only change with no operation",
			before: uni(nil, claim("d/t:x", "0100")),
			after:  uni(nil, claim("d/t:x", "0100", "0101")),
			cutoff: 200,
			want:   "claim d/t:x was changed with no ADR update operation in this transition",
		},
		{
			name:   "two operations on one claim",
			before: uni(nil),
			after: uni([]adr.ADR{
				rec("0137", "Implemented", op(adr.OpAdd, "d/t:x")),
				rec("0138", "Implemented", op(adr.OpRemove, "d/t:x")),
			}),
			cutoff: 137,
			want:   "claim d/t:x is the target of more than one operation in this transition",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := currentstate.CheckPair(tc.before, tc.after, currentstate.AuthoredCommit)
			if !strings.Contains(messages(f), tc.want) {
				t.Fatalf("want %q in:\n%s", tc.want, messages(f))
			}
		})
	}
}

// TestLoadedUniverse reduces a Loaded view to its before/after inputs.
func TestLoadedUniverse(t *testing.T) {
	u := currentstate.Loaded{ADRs: []adr.ADR{{Number: "0137"}}}.Universe()
	if len(u.ADRs) != 1 || u.ADRs[0].Number != "0137" || len(u.Topics) != 0 {
		t.Fatalf("unexpected universe: %+v", u)
	}
}

// TestAppliedRemoveAbsorbsConcurrentUpdate covers the absorbing tombstone
// (ADR-0191): update-then-remove and remove-then-dominated-update integration
// orders converge to the same attributed absence, the dominated batch is
// retained as history with an empty required mutation set, and a dominated
// chain whose claim survives is rejected.
// invariant: invariants/current-state-authority:applied-remove-absorbing-tombstone (TestAppliedRemoveAbsorbsConcurrentUpdate)
func TestAppliedRemoveAbsorbsConcurrentUpdate(t *testing.T) {
	add := op(adr.OpAdd, "d/t:c")
	update := op(adr.OpUpdate, "d/t:c")
	remove := op(adr.OpRemove, "d/t:c")
	origin := v2rec("0140", "Implemented", []adr.Operation{add},
		v2status("Proposed"), v2status("Implementing"), v2batch(add), v2status("Implemented"))
	updater := v2rec("0141", "Implemented", []adr.Operation{update},
		v2status("Proposed"), v2status("Implementing"), v2batch(update), v2status("Implemented"))
	remover := v2rec("0142", "Implemented", []adr.Operation{remove},
		v2status("Proposed"), v2status("Implementing"), v2batch(remove), v2status("Implemented"))

	t.Run("update integrates first, remove second", func(t *testing.T) {
		before := uni([]adr.ADR{origin, updater}, prosed(claim("d/t:c", "0140", "0141"), "revised"))
		after := uni([]adr.ADR{origin, updater, remover})
		if f := currentstate.CheckPair(before, after, currentstate.AuthoredCommit); len(f) != 0 {
			t.Fatalf("remove after an integrated update must be clean:\n%s", messages(f))
		}
	})

	t.Run("remove integrates first, update arrives dominated", func(t *testing.T) {
		// The legacy record and the record with an inconsistent progress are
		// skipped, not consulted, when deriving the applied-remove set.
		legacy := adr.ADR{Number: "0100", Format: adr.Legacy, Status: "Implemented"}
		broken := v2rec("0143", "Implementing", []adr.Operation{op(adr.OpAdd, "d/t:other")}, v2status("Proposed"))
		before := uni([]adr.ADR{legacy, origin, remover})
		after := uni([]adr.ADR{legacy, broken, origin, remover, updater})
		got := messages(currentstate.CheckPair(before, after, currentstate.MergeAggregate))
		if strings.Contains(got, "must stay absent") || strings.Contains(got, "updates claim d/t:c") {
			t.Fatalf("a dominated update after an integrated remove must be clean:\n%s", got)
		}
	})

	t.Run("both orders converge to attributed absence", func(t *testing.T) {
		full := []adr.ADR{origin, updater, remover}
		if f := currentstate.Check(full, topics()); len(f) != 0 {
			t.Fatalf("the converged corpus must be clean with the claim absent:\n%s", messages(f))
		}
	})

	t.Run("a net remove is attributed to the removing ADR", func(t *testing.T) {
		// The remover carries the LOWER number, so the dominated updater is
		// the chain's trailing element: the finding must still name the
		// remover, never chain[len-1]. The claim is absent on the before
		// side, which is what makes the net-remove finding fire.
		removerLow := v2rec("0141", "Implemented", []adr.Operation{remove},
			v2status("Proposed"), v2status("Implementing"), v2batch(remove), v2status("Implemented"))
		updaterHigh := v2rec("0142", "Implemented", []adr.Operation{update},
			v2status("Proposed"), v2status("Implementing"), v2batch(update), v2status("Implemented"))
		before := uni([]adr.ADR{origin})
		after := uni([]adr.ADR{origin, removerLow, updaterHigh})
		got := messages(currentstate.CheckPair(before, after, currentstate.MergeAggregate))
		if !strings.Contains(got, "ADR-0141 removes claim d/t:c, which did not exist before this transition") {
			t.Fatalf("the net remove must be attributed to the removing ADR-0141:\n%s", got)
		}
	})

	t.Run("a dominated chain must not resurrect the claim", func(t *testing.T) {
		before := uni([]adr.ADR{origin, remover})
		after := uni([]adr.ADR{origin, remover, updater}, prosed(claim("d/t:c", "0140", "0141"), "revived"))
		got := messages(currentstate.CheckPair(before, after, currentstate.MergeAggregate))
		if !strings.Contains(got, "claim d/t:c has only dominated updates in this transition, so it must stay absent") {
			t.Fatalf("a surviving claim under a dominated chain must be rejected:\n%s", got)
		}
	})
}

// TestRevisedByCanonicalReorderIsNotAMutation pins the deliberate set
// comparison in checkUnmatchedMutation: reordering a Revised-by list to its
// canonical ascending form with no ADR operation in the pair is not a change
// (migration 26 does exactly this), while a membership change still is.
func TestRevisedByCanonicalReorderIsNotAMutation(t *testing.T) {
	add := op(adr.OpAdd, "d/t:c")
	update := op(adr.OpUpdate, "d/t:c")
	origin := v2rec("0140", "Implemented", []adr.Operation{add},
		v2status("Proposed"), v2status("Implementing"), v2batch(add), v2status("Implemented"))
	first := v2rec("0141", "Implemented", []adr.Operation{update},
		v2status("Proposed"), v2status("Implementing"), v2batch(update), v2status("Implemented"))
	second := v2rec("0142", "Implemented", []adr.Operation{update},
		v2status("Proposed"), v2status("Implementing"), v2batch(update), v2status("Implemented"))
	records := []adr.ADR{origin, first, second}

	before := uni(records, claim("d/t:c", "0140", "0142", "0141"))
	if f := currentstate.CheckPair(before, uni(records, claim("d/t:c", "0140", "0141", "0142")), currentstate.AuthoredCommit); len(f) != 0 {
		t.Fatalf("a canonical reorder with no operation must not be a mutation:\n%s", messages(f))
	}
	got := messages(currentstate.CheckPair(before, uni(records, claim("d/t:c", "0140", "0141")), currentstate.AuthoredCommit))
	if !strings.Contains(got, "changed with no ADR update operation") {
		t.Fatalf("a membership change without an operation must stay a mutation finding:\n%s", got)
	}
}

// A renumbered intrinsically formatted ADR keeps the format in which it was
// authored. Changing V2 to V3 while moving it is therefore a format change, not
// part of the sanctioned slugless digest-paired rename.
func TestCheckPairRefusesFormatRetrofitDuringRenumber(t *testing.T) {
	sections := map[string]string{"Decision": "one body, two numbers"}
	history := []adr.StatusEntry{{Date: "2026-01-01", Status: "Proposed"}}
	before := adr.ADR{Number: "0199", Format: adr.CurrentStateV2, Status: "Proposed", Sections: sections, History: history}
	after := adr.ADR{Number: "0205", Format: adr.CurrentStateV3, Status: "Proposed", Sections: sections, History: history}
	got := messages(currentstate.CheckPair(uni([]adr.ADR{before}), uni([]adr.ADR{after}), currentstate.AuthoredCommit))
	if !strings.Contains(got, "changed governed format across this transition") {
		t.Fatalf("a slugless digest-paired format retrofit must be refused:\n%s", got)
	}
}

func TestOlderIntroductions(t *testing.T) {
	current := adr.ADR{Slug: "current", Format: adr.CurrentStateV3}
	if got := currentstate.OlderIntroductions(uni(nil), uni([]adr.ADR{current}), adr.CurrentStateV3); len(got) != 0 {
		t.Fatalf("current-format introduction = %#v, want none", got)
	}

	beforeV2 := adr.ADR{Number: "0004", Format: adr.CurrentStateV2, Status: "Accepted"}
	afterV2 := beforeV2
	afterV2.Status = "Implementing"
	if got := currentstate.OlderIntroductions(uni([]adr.ADR{beforeV2}), uni([]adr.ADR{afterV2}), adr.CurrentStateV3); len(got) != 0 {
		t.Fatalf("existing older lifecycle transition = %#v, want none", got)
	}

	renumberedBefore := adr.ADR{Number: "0005", Format: adr.CurrentStateV2, Sections: map[string]string{"Decision": "same record"}}
	renumberedAfter := renumberedBefore
	renumberedAfter.Number = "0008"
	if got := currentstate.OlderIntroductions(uni([]adr.ADR{renumberedBefore}), uni([]adr.ADR{renumberedAfter}), adr.CurrentStateV3); len(got) != 0 {
		t.Fatalf("existing older slugless renumber = %#v, want none", got)
	}

	after := uni([]adr.ADR{
		{Number: "0002", Format: adr.CurrentStateV1},
		{Number: "0001", Format: adr.Legacy},
		{Number: "0003", Format: adr.CurrentStateV2},
		current,
	})
	want := []currentstate.Introduction{
		{Identity: "0001", Format: adr.Legacy},
		{Identity: "0002", Format: adr.CurrentStateV1},
		{Identity: "0003", Format: adr.CurrentStateV2},
	}
	if got := currentstate.OlderIntroductions(uni(nil), after, adr.CurrentStateV3); !reflect.DeepEqual(got, want) {
		t.Fatalf("older introductions = %#v, want %#v", got, want)
	}
}

// A V3 record's slug is retained forever, so changing it at the same number is
// not a rename to be paired but a violation to be reported. The before side of
// the digest index stays slugless-only for exactly this reason: admitting a
// record whose old slug is absent from the after side would let the two ends
// pair by body and launder the slug change away.
func TestCheckPairRetainedSlugCannotChange(t *testing.T) {
	sections := map[string]string{"Decision": "one body under two slugs"}
	history := []adr.StatusEntry{{Date: "2026-01-01", Status: "Proposed"}}
	before := adr.ADR{Number: "0300", Slug: "the-original-slug", Format: adr.CurrentStateV3, Status: "Proposed", Sections: sections, History: history}
	after := before
	after.Slug = "a-renamed-slug"
	if got := messages(currentstate.CheckPair(uni([]adr.ADR{before}), uni([]adr.ADR{after}), currentstate.AuthoredCommit)); got == "" {
		t.Fatal("a retained slug changing at the same number must not be finding-free")
	}
}

// A pending record carries no number, so it can only be an addition and must
// never become the far end of a renumber. Without that exclusion a genuine
// deletion standing beside an unrelated pending addition whose canonical body
// coincides is laundered into a rename, which is the fail-closed promise
// ADR-0204 item 4 makes and the widening for the v3 retrofit could have broken.
func TestCheckPairPendingAdditionCannotLaunderADeletion(t *testing.T) {
	sections := map[string]string{"Decision": "a body two records happen to share"}
	history := []adr.StatusEntry{{Date: "2026-01-01", Status: "Proposed"}}
	deleted := adr.ADR{Number: "0100", Format: adr.CurrentStateV2, Status: "Implemented", Sections: sections,
		History: []adr.StatusEntry{{Date: "2026-01-01", Status: "Proposed"}, {Date: "2026-01-02", Status: "Implemented", Digest: "d"}}}
	pending := adr.ADR{Slug: "an-unrelated-new-record", Format: adr.CurrentStateV3, Status: "Proposed", Sections: sections, History: history}
	got := messages(currentstate.CheckPair(uni([]adr.ADR{deleted}), uni([]adr.ADR{pending}), currentstate.AuthoredCommit))
	if !strings.Contains(got, "was deleted across this transition") {
		t.Fatalf("a pending addition must not pair with a deleted record:\n%s", got)
	}
}
