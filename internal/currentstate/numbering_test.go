package currentstate_test

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// v3rec builds a current-state-v3 record: numbered when number is non-empty and
// pending otherwise, at status, with ops applied through one Applied history
// event.
func v3record(number, slug, status string, ops ...adr.Operation) adr.ADR {
	history := []adr.StatusEntry{{Kind: adr.HistoryStatus, Date: "2026-07-31", Status: "Proposed"}}
	if status != "Proposed" {
		history = append(history, adr.StatusEntry{Kind: adr.HistoryStatus, Date: "2026-07-31", Status: status})
	}
	if len(ops) != 0 {
		history = append(history, adr.StatusEntry{Kind: adr.HistoryApplied, Date: "2026-07-31", Operations: ops})
	}
	return adr.ADR{
		Number: number, Slug: slug, Format: adr.CurrentStateV3, Status: status,
		Operations: ops, History: history, Sections: map[string]string{"Decision": "1. Body."},
	}
}

// numberingBefore and numberingAfter are the two sides of one legal numbering
// transition. Three records carry the claim so the substitution has a plural
// list to canonicalize: ADR-0001 added it, pending ADR-beta and numbered
// ADR-0400 each revised it, and numbering gives beta the number 0002 - which
// lands numerically below an entry already in the list, so the after list is a
// re-sort rather than an append.
func numberingBefore() currentstate.Universe {
	return uni([]adr.ADR{
		v3record("0001", "alpha", "Implemented", op(adr.OpAdd, "d/t:x")),
		v3record("", "beta", "Implemented", op(adr.OpUpdate, "d/t:x")),
		v3record("0400", "gamma", "Implemented", op(adr.OpUpdate, "d/t:x")),
	}, claim("d/t:x", "0001", "0400", "beta"))
}

func numberingAfter() currentstate.Universe {
	return uni([]adr.ADR{
		v3record("0001", "alpha", "Implemented", op(adr.OpAdd, "d/t:x")),
		v3record("0002", "beta", "Implemented", op(adr.OpUpdate, "d/t:x")),
		v3record("0400", "gamma", "Implemented", op(adr.OpUpdate, "d/t:x")),
	}, claim("d/t:x", "0001", "0002", "0400"))
}

// withRecord replaces the universe's record carrying the replacement's slug.
func withRecord(u currentstate.Universe, replacement adr.ADR) currentstate.Universe {
	records := append([]adr.ADR(nil), u.ADRs...)
	for i, a := range records {
		if a.Slug == replacement.Slug {
			records[i] = replacement
		}
	}
	return currentstate.Universe{ADRs: records, Topics: u.Topics}
}

// withoutRecord drops the universe's record carrying slug.
func withoutRecord(u currentstate.Universe, slug string) currentstate.Universe {
	var records []adr.ADR
	for _, a := range u.ADRs {
		if a.Slug != slug {
			records = append(records, a)
		}
	}
	return currentstate.Universe{ADRs: records, Topics: u.Topics}
}

// withClaim replaces the universe's single claim.
func withClaim(u currentstate.Universe, c topic.Claim) currentstate.Universe {
	return currentstate.Universe{ADRs: u.ADRs, Topics: topics(c)}
}

// The sanctioned numbering transition admits exactly one shape and nothing
// wider. The accept case proves the whole permitted delta at once: the pending
// record pairs with its numbered successor by slug rather than reading as a
// delete plus an add, and the claim's Revised-by list takes the substitution
// canonicalized into ascending order. Every refusal below is a single-field
// departure from that same pair, so each one is what the accept case would
// otherwise have hidden.
// invariant: adr-system/adr-lifecycle:numbering-transition-mode
func TestCheckPairNumberingTransition(t *testing.T) {
	for _, mode := range []currentstate.TransitionMode{currentstate.AuthoredCommit, currentstate.MergeAggregate} {
		if f := currentstate.CheckPair(numberingBefore(), numberingAfter(), mode); len(f) != 0 {
			t.Fatalf("legal numbering pair rejected in mode %d:\n%s", mode, messages(f))
		}
	}

	numbered := v3record("0002", "beta", "Implemented", op(adr.OpUpdate, "d/t:x"))
	amended := numbered
	amended.History = append(append([]adr.StatusEntry(nil), numbered.History...),
		adr.StatusEntry{Kind: adr.HistoryAmended, Date: "2026-08-01", Digest: "sha256:amended"})
	restamped := numbered
	restamped.History = append([]adr.StatusEntry(nil), numbered.History...)
	restamped.History[0].Date = "2026-08-01"
	advanced := numbered
	advanced.Status = "Abandoned"
	rewritten := numbered
	rewritten.Sections = map[string]string{"Decision": "1. A different body."}

	for _, tc := range []struct {
		name  string
		after currentstate.Universe
		want  string
	}{
		{"an appended history event", withRecord(numberingAfter(), amended),
			"ADR-0002 violates the numbering-transition rule"},
		{"a rewritten history event", withRecord(numberingAfter(), restamped),
			"ADR-0002 violates the numbering-transition rule"},
		{"a changed status", withRecord(numberingAfter(), advanced),
			"ADR-0002 violates the numbering-transition rule"},
		{"a rewritten body", withRecord(numberingAfter(), rewritten),
			"ADR-0002 violates the frozen-content rule"},
		{"a non-canonical Revised-by list", withClaim(numberingAfter(), claim("d/t:x", "0001", "0400", "0002")),
			"claim d/t:x must carry the numbering substitution exactly: Origin ADR-0001, Revised-by ADR-0002, ADR-0400, and no other change"},
		{"a substituted Origin", withClaim(withRecord(numberingAfter(), numbered), claim("d/t:x", "0002", "0002", "0400")),
			"claim d/t:x must carry the numbering substitution exactly: Origin ADR-0001, Revised-by ADR-0002, ADR-0400, and no other change"},
		{"a rewritten claim body", withClaim(numberingAfter(), prosed(claim("d/t:x", "0001", "0002", "0400"), "rewritten")),
			"claim d/t:x must carry the numbering substitution exactly: Origin ADR-0001, Revised-by ADR-0002, ADR-0400, and no other change"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := messages(currentstate.CheckPair(numberingBefore(), tc.after, currentstate.AuthoredCommit))
			if !strings.Contains(got, tc.want) {
				t.Errorf("expected %q, got:\n%s", tc.want, got)
			}
		})
	}
}

// Two provenance shapes the three-record fixture cannot reach. A Revised-by
// list naming both a numbered record and the slug that took that same number
// collapses to one entry, because the canonical form is duplicate-free as well
// as ascending. A claim with no revisions at all still has its Origin checked,
// and the refusal names the empty expectation rather than a blank.
// invariant: adr-system/adr-lifecycle:numbering-transition-mode
func TestCheckPairNumberingSubstitutionEdgeShapes(t *testing.T) {
	records := func(number string) []adr.ADR {
		return []adr.ADR{
			v3record("0001", "alpha", "Implemented", op(adr.OpAdd, "d/t:x")),
			v3record(number, "beta", "Implemented", op(adr.OpUpdate, "d/t:x")),
		}
	}
	before := uni(records(""), claim("d/t:x", "0001", "0002", "beta"))
	after := uni(records("0002"), claim("d/t:x", "0001", "0002"))
	if f := currentstate.CheckPair(before, after, currentstate.AuthoredCommit); len(f) != 0 {
		t.Fatalf("a substitution collapsing onto an entry already listed must be clean:\n%s", messages(f))
	}

	originOnly := []adr.ADR{v3record("", "beta", "Implemented", op(adr.OpAdd, "d/t:y"))}
	numberedOnly := []adr.ADR{v3record("0002", "beta", "Implemented", op(adr.OpAdd, "d/t:y"))}
	got := messages(currentstate.CheckPair(
		uni(originOnly, claim("d/t:y", "beta")),
		uni(numberedOnly, claim("d/t:y", "0003")),
		currentstate.AuthoredCommit))
	if !strings.Contains(got, "claim d/t:y must carry the numbering substitution exactly: Origin ADR-0002, Revised-by (none), and no other change") {
		t.Errorf("a revision-free claim must name its empty expectation, got:\n%s", got)
	}
}

// A provenance substitution is legal only as the tail of a numbering that
// actually happened. With the record left pending, the very same claim edit is
// an unexplained mutation - which is what stops the relaxation from becoming a
// standing licence to rewrite Origin.
// invariant: adr-system/adr-lifecycle:numbering-transition-mode
func TestCheckPairSubstitutionRequiresThePairedNumbering(t *testing.T) {
	stillPending := withRecord(numberingAfter(), v3record("", "beta", "Implemented", op(adr.OpUpdate, "d/t:x")))
	got := messages(currentstate.CheckPair(numberingBefore(), stillPending, currentstate.AuthoredCommit))
	if !strings.Contains(got, "claim d/t:x was changed with no ADR update operation in this transition") {
		t.Errorf("substitution without the numbering must be an unmatched mutation, got:\n%s", got)
	}
}

// Deleting a pending record is a deletion, not a numbering: slug pairing must
// not turn a vanished record into a silently tolerated rename.
// invariant: adr-system/adr-lifecycle:numbering-transition-mode
func TestCheckPairDeletedPendingADR(t *testing.T) {
	got := messages(currentstate.CheckPair(numberingBefore(), withoutRecord(numberingAfter(), "beta"), currentstate.AuthoredCommit))
	if !strings.Contains(got, "current-state-v3 ADR-beta was deleted across this transition") {
		t.Errorf("deleted pending record not reported:\n%s", got)
	}
}

// A number once assigned never changes. Slug pairing is what makes this
// statable: before it, a renumber read as one record deleted and another added,
// so the rule had no pair to attach to. Both directions are refused - taking a
// different number, and losing the number altogether.
// invariant: adr-system/adr-lifecycle:adr-number-immutable
func TestCheckPairRefusesAReassignedNumber(t *testing.T) {
	before := uni([]adr.ADR{v3record("0002", "beta", "Implemented")})
	for _, tc := range []struct {
		name  string
		after adr.ADR
		want  string
	}{
		{"renumbered", v3record("0003", "beta", "Implemented"),
			"ADR-0002 changed its assigned number to ADR-0003 across this transition; an assigned ADR number never changes"},
		{"unnumbered", v3record("", "beta", "Implemented"),
			"ADR-0002 changed its assigned number to ADR-beta across this transition; an assigned ADR number never changes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := messages(currentstate.CheckPair(before, uni([]adr.ADR{tc.after}), currentstate.AuthoredCommit))
			if !strings.Contains(got, tc.want) {
				t.Errorf("expected %q, got:\n%s", tc.want, got)
			}
		})
	}
}

// A pending record's newly applied batch must rank after every numbered record's
// in the aggregate chain, because that is the order numbering will assign it.
// Ranking it by its empty number would have sorted it first and read the chain
// as an update preceding its own add.
// invariant: adr-system/adr-lifecycle:numbering-transition-mode
func TestCheckPairRanksPendingBatchesLast(t *testing.T) {
	// The pending record is listed first, so universe order alone would rank its
	// add before the numbered update and read the chain as legal. Only ranking by
	// the identity - which places a pending record after every number, matching
	// what numbering will assign it - exposes the inversion.
	before := uni([]adr.ADR{
		v3record("", "beta", "Accepted"),
		v3record("0400", "gamma", "Accepted"),
	})
	before.ADRs[0].Operations = []adr.Operation{op(adr.OpAdd, "d/t:x")}
	before.ADRs[1].Operations = []adr.Operation{op(adr.OpUpdate, "d/t:x")}
	after := uni([]adr.ADR{
		v3record("", "beta", "Implemented", op(adr.OpAdd, "d/t:x")),
		v3record("0400", "gamma", "Implemented", op(adr.OpUpdate, "d/t:x")),
	}, claim("d/t:x", "beta", "0400"))

	got := messages(currentstate.CheckPair(before, after, currentstate.MergeAggregate))
	if !strings.Contains(got, "an add must be the first operation") {
		t.Errorf("a pending add ranked after a numbered update must be reported as an illegal chain, got:\n%s", got)
	}
}
