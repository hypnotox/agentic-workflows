package currentstate_test

import (
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
	before := uni([]adr.ADR{rec("0137", "Proposed", 0, op(adr.OpAdd, "d/t:new"))})
	after := uni([]adr.ADR{rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:new"))}, claim("d/t:new", "0137"))
	if f := currentstate.CheckPair(before, after, 137); len(f) != 0 {
		t.Fatalf("expected no findings, got:\n%s", messages(f))
	}
}

// TestCheckPairValidUpdate accepts an Accepted->Implemented update that preserves
// Origin, appends the updating ADR to Revised-by, and changes the prose.
func TestCheckPairValidUpdate(t *testing.T) {
	accepted := rec("0138", "Accepted", 0, op(adr.OpUpdate, "d/t:x"))
	implemented := rec("0138", "Implemented", 2, op(adr.OpUpdate, "d/t:x"))
	implemented.History = append(append([]adr.StatusEntry(nil), accepted.History...), implemented.History[len(implemented.History)-1])
	before := uni(
		[]adr.ADR{
			rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x")),
			accepted,
		},
		prosed(claim("d/t:x", "0137"), "old"))
	after := uni(
		[]adr.ADR{
			rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x")),
			implemented,
		},
		prosed(claim("d/t:x", "0137", "0138"), "new"))
	if f := currentstate.CheckPair(before, after, 137); len(f) != 0 {
		t.Fatalf("expected no findings, got:\n%s", messages(f))
	}
}

// TestCheckPairValidRemove accepts an Accepted->Implemented remove that retires
// the claim.
func TestCheckPairValidRemove(t *testing.T) {
	accepted := rec("0139", "Accepted", 0, op(adr.OpRemove, "d/t:x"))
	implemented := rec("0139", "Implemented", 2, op(adr.OpRemove, "d/t:x"))
	implemented.History = append(append([]adr.StatusEntry(nil), accepted.History...), implemented.History[len(implemented.History)-1])
	before := uni(
		[]adr.ADR{
			rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x")),
			accepted,
		},
		claim("d/t:x", "0137"))
	after := uni([]adr.ADR{
		rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x")),
		implemented,
	})
	if f := currentstate.CheckPair(before, after, 137); len(f) != 0 {
		t.Fatalf("expected no findings, got:\n%s", messages(f))
	}
}

// TestCheckPairBootstrapAddExempt accepts a claim first appearing with an Origin
// below cutoff and no add operation: the closed migration bootstrap.
func TestCheckPairBootstrapAddExempt(t *testing.T) {
	before := uni(nil)
	after := uni([]adr.ADR{{Number: "0100", Format: adr.Legacy, Status: "Implemented"}}, claim("d/t:legacy", "0100"))
	if f := currentstate.CheckPair(before, after, 137); len(f) != 0 {
		t.Fatalf("expected no findings for bootstrap add, got:\n%s", messages(f))
	}
}

// TestCheckPairUnchangedClaim accepts a claim that persists identically across
// the pair with no operation touching it.
func TestCheckPairUnchangedClaim(t *testing.T) {
	legacy := []adr.ADR{{Number: "0100", Format: adr.Legacy, Status: "Implemented"}}
	before := uni(legacy, prosed(claim("d/t:keep", "0100"), "steady"))
	after := uni(legacy, prosed(claim("d/t:keep", "0100"), "steady"))
	if f := currentstate.CheckPair(before, after, 137); len(f) != 0 {
		t.Fatalf("expected no findings, got:\n%s", messages(f))
	}
}

// TestCheckPairDeletedV1ADR rejects removal of a governed ADR record.
func TestCheckPairDeletedV1ADR(t *testing.T) {
	before := uni([]adr.ADR{rec("0137", "Implemented", 1)})
	if f := currentstate.CheckPair(before, uni(nil), 137); !strings.Contains(messages(f), "current-state-v1 ADR-0137 was deleted") {
		t.Fatalf("deleted ADR not reported:\n%s", messages(f))
	}
}

// TestCheckPairIllegalTransition rejects an edge out of a terminal state.
func TestCheckPairIllegalTransition(t *testing.T) {
	before := uni([]adr.ADR{rec("0137", "Implemented", 0)})
	after := uni([]adr.ADR{rec("0137", "Abandoned", 0)})
	if f := currentstate.CheckPair(before, after, 137); !strings.Contains(messages(f), "ADR-0137 changed status from Implemented to Abandoned, which is not a legal") {
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
			got := messages(currentstate.CheckPair(uni([]adr.ADR{tc.before}), uni([]adr.ADR{tc.after}), 137))
			if !strings.Contains(got, "ADR-0137") || !strings.Contains(got, tc.want) {
				t.Fatalf("want ADR number and %q in:\n%s", tc.want, got)
			}
		})
	}
	formatBefore := record("Proposed", "same", proposed)
	formatAfter := formatBefore
	formatAfter.Format = adr.CurrentStateV2
	if got := messages(currentstate.CheckPair(uni([]adr.ADR{formatBefore}), uni([]adr.ADR{formatAfter}), 137)); !strings.Contains(got, "changed governed format") {
		t.Fatalf("governed format mutation not rejected:\n%s", got)
	}
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
			if f := currentstate.CheckPair(uni([]adr.ADR{before}), uni([]adr.ADR{after}), 137); len(f) != 0 {
				t.Fatalf("expected no findings, got:\n%s", messages(f))
			}
		})
	}
}

func TestCheckPairV2IncrementalBatches(t *testing.T) {
	addX := op(adr.OpAdd, "d/t:x")
	updateX := op(adr.OpUpdate, "d/t:x")
	addA := op(adr.OpAdd, "d/t:a")
	addB := op(adr.OpAdd, "d/t:b")
	base := rec("0137", "Implemented", 1, addX)
	proposed := v2rec("0138", "Proposed", []adr.Operation{updateX, addA, addB}, v2status("Proposed"))
	first := proposed
	first.Status = "Implementing"
	first.History = append(append([]adr.HistoryEvent(nil), proposed.History...), v2status("Implementing"), v2batch(2, updateX))
	before := uni([]adr.ADR{base, proposed}, prosed(claim("d/t:x", "0137"), "old"))
	after := uni([]adr.ADR{base, first}, prosed(claim("d/t:x", "0137", "0138"), "new"))
	if f := currentstate.CheckPair(before, after, 137); len(f) != 0 {
		t.Fatalf("first batch pair rejected:\n%s", messages(f))
	}

	middle := first
	middle.History = append(append([]adr.HistoryEvent(nil), first.History...), v2batch(3, addA))
	middleAfter := uni([]adr.ADR{base, middle}, prosed(claim("d/t:x", "0137", "0138"), "new"), claim("d/t:a", "0138"))
	if f := currentstate.CheckPair(after, middleAfter, 137); len(f) != 0 {
		t.Fatalf("middle batch pair rejected:\n%s", messages(f))
	}

	done := middle
	done.Status = "Implemented"
	done.History = append(append([]adr.HistoryEvent(nil), middle.History...), v2batch(4, addB), v2status("Implemented"))
	doneAfter := uni([]adr.ADR{base, done}, prosed(claim("d/t:x", "0137", "0138"), "new"), claim("d/t:a", "0138"), claim("d/t:b", "0138"))
	if f := currentstate.CheckPair(middleAfter, doneAfter, 137); len(f) != 0 {
		t.Fatalf("final batch pair rejected:\n%s", messages(f))
	}

	abandoned := middle
	abandoned.Status = "Abandoned"
	abandoned.History = append(append([]adr.HistoryEvent(nil), middle.History...), v2status("Abandoned"))
	if f := currentstate.CheckPair(middleAfter, uni([]adr.ADR{base, abandoned}, prosed(claim("d/t:x", "0137", "0138"), "new"), claim("d/t:a", "0138")), 137); len(f) != 0 {
		t.Fatalf("terminal abandonment pair rejected:\n%s", messages(f))
	}

	deleted := first
	deleted.History = append([]adr.HistoryEvent(nil), first.History[:2]...)
	if got := messages(currentstate.CheckPair(after, uni([]adr.ADR{base, deleted}, prosed(claim("d/t:x", "0137", "0138"), "new")), 137)); !strings.Contains(got, "history-prefix rule") {
		t.Fatalf("Applied event deletion not rejected:\n%s", got)
	}
}

func TestCheckPairV2BatchSetRules(t *testing.T) {
	base := rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:base"))
	direct := func(num, id string, sequence int) (adr.ADR, adr.ADR) {
		operation := op(adr.OpAdd, id)
		before := v2rec(num, "Accepted", []adr.Operation{operation}, v2status("Proposed"), v2status("Accepted"))
		after := before
		after.Status = "Implemented"
		terminal := v2status("Implemented")
		terminal.Sequence, terminal.HasSequence = sequence, true
		after.History = append(append([]adr.HistoryEvent(nil), before.History...), terminal)
		return before, after
	}
	b1, a1 := direct("0138", "d/t:a", 2)
	b2, a2 := direct("0139", "d/t:b", 3)
	before := uni([]adr.ADR{base, b1, b2}, claim("d/t:base", "0137"))
	after := uni([]adr.ADR{base, a1, a2}, claim("d/t:base", "0137"), claim("d/t:a", "0138"), claim("d/t:b", "0139"))
	if f := currentstate.CheckPair(before, after, 137); len(f) != 0 {
		t.Fatalf("disjoint consecutive batches rejected:\n%s", messages(f))
	}

	_, duplicateTarget := direct("0140", "d/t:a", 3)
	if got := messages(currentstate.CheckPair(uni([]adr.ADR{base, b1, b2}), uni([]adr.ADR{base, a1, duplicateTarget}, claim("d/t:a", "0138")), 137)); !strings.Contains(got, "target of more than one operation") {
		t.Fatalf("cross-batch duplicate target not rejected:\n%s", got)
	}

	wrong := a2
	wrong.History[len(wrong.History)-1].Sequence = 4
	if got := messages(currentstate.CheckPair(before, uni([]adr.ADR{base, a1, wrong}, claim("d/t:base", "0137"), claim("d/t:a", "0138"), claim("d/t:b", "0139")), 137)); !strings.Contains(got, "expected next sequence 3") {
		t.Fatalf("nonconsecutive appended sequences not diagnosed:\n%s", got)
	}

	x, y, z := op(adr.OpAdd, "d/t:x"), op(adr.OpAdd, "d/t:y"), op(adr.OpAdd, "d/t:z")
	partial := v2rec("0141", "Implementing", []adr.Operation{x, y, z}, v2status("Proposed"), v2status("Implementing"), v2batch(2, x))
	two := partial
	two.History = append(append([]adr.HistoryEvent(nil), partial.History...), v2batch(3, y), v2batch(4, z))
	if got := messages(currentstate.CheckPair(uni([]adr.ADR{base, partial}, claim("d/t:base", "0137"), claim("d/t:x", "0141")), uni([]adr.ADR{base, two}, claim("d/t:base", "0137"), claim("d/t:x", "0141"), claim("d/t:y", "0141"), claim("d/t:z", "0141")), 137)); !strings.Contains(got, "at most one new batch") {
		t.Fatalf("same ADR duplicate batch not rejected:\n%s", got)
	}

	terminal := v2rec("0142", "Implemented", nil, v2status("Proposed"), v2status("Implemented"))
	illegal := terminal
	illegal.Status = "Abandoned"
	illegal.History = append(illegal.History, v2status("Abandoned"))
	if got := messages(currentstate.CheckPair(uni([]adr.ADR{terminal}), uni([]adr.ADR{illegal}), 137)); !strings.Contains(got, "legal current-state-v2 transition") {
		t.Fatalf("illegal V2 edge not format-attributed:\n%s", got)
	}

	invalid := v2rec("0143", "Implemented", []adr.Operation{op(adr.OpAdd, "d/t:invalid")})
	if got := messages(currentstate.CheckPair(uni([]adr.ADR{invalid}), uni([]adr.ADR{invalid}), 137)); !strings.Contains(got, "no Implemented status event") {
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
			before: uni([]adr.ADR{rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x"))}, claim("d/t:x", "0137")),
			after: uni([]adr.ADR{
				rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x")),
				rec("0138", "Implemented", 2, op(adr.OpAdd, "d/t:x")),
			}, claim("d/t:x", "0137")),
			cutoff: 137,
			want:   "ADR-0138 adds claim d/t:x, which already existed before this transition",
		},
		{
			name:   "remove of an absent claim",
			before: uni(nil),
			after:  uni([]adr.ADR{rec("0137", "Implemented", 1, op(adr.OpRemove, "d/t:x"))}),
			cutoff: 137,
			want:   "ADR-0137 removes claim d/t:x, which did not exist before this transition",
		},
		{
			name:   "update of a claim absent after",
			before: uni([]adr.ADR{{Number: "0100", Format: adr.Legacy, Status: "Implemented"}}, claim("d/t:x", "0100")),
			after:  uni([]adr.ADR{rec("0137", "Implemented", 1, op(adr.OpUpdate, "d/t:x"))}),
			cutoff: 137,
			want:   "ADR-0137 updates claim d/t:x, which is not present on both sides",
		},
		{
			name: "update with no canonical change",
			before: uni([]adr.ADR{rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x"))},
				prosed(claim("d/t:x", "0137"), "same")),
			after: uni([]adr.ADR{
				rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x")),
				rec("0138", "Implemented", 2, op(adr.OpUpdate, "d/t:x")),
			}, prosed(claim("d/t:x", "0137", "0138"), "same\n")),
			cutoff: 137,
			want:   "ADR-0138 updates claim d/t:x, but no canonical field changed",
		},
		{
			name: "update changing Origin",
			before: uni([]adr.ADR{rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x"))},
				prosed(claim("d/t:x", "0137"), "old")),
			after: uni([]adr.ADR{
				rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x")),
				rec("0138", "Implemented", 2, op(adr.OpUpdate, "d/t:x")),
			}, prosed(claim("d/t:x", "0199", "0138"), "new")),
			cutoff: 137,
			want:   "update of claim d/t:x changed its Origin from ADR-0137 to ADR-0199",
		},
		{
			name: "update not appending Revised-by",
			before: uni([]adr.ADR{rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x"))},
				prosed(claim("d/t:x", "0137"), "old")),
			after: uni([]adr.ADR{
				rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x")),
				rec("0138", "Implemented", 2, op(adr.OpUpdate, "d/t:x")),
			}, prosed(claim("d/t:x", "0137"), "new")),
			cutoff: 137,
			want:   "must extend Revised-by by exactly one entry",
		},
		{
			name: "update breaking the Revised-by prefix",
			before: uni([]adr.ADR{
				rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x")),
				rec("0138", "Implemented", 2, op(adr.OpUpdate, "d/t:x")),
			}, prosed(claim("d/t:x", "0137", "0138"), "v1")),
			after: uni([]adr.ADR{
				rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x")),
				rec("0138", "Implemented", 2, op(adr.OpUpdate, "d/t:x")),
				rec("0140", "Implemented", 3, op(adr.OpUpdate, "d/t:x")),
			}, prosed(claim("d/t:x", "0137", "0199", "0140"), "v2")),
			cutoff: 137,
			want:   "must keep the prior Revised-by list as an exact prefix",
		},
		{
			name: "update appending the wrong ADR",
			before: uni([]adr.ADR{rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x"))},
				prosed(claim("d/t:x", "0137"), "old")),
			after: uni([]adr.ADR{
				rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x")),
				rec("0140", "Implemented", 2, op(adr.OpUpdate, "d/t:x")),
			}, prosed(claim("d/t:x", "0137", "0199"), "new")),
			cutoff: 137,
			want:   "must append the updating ADR-0140 to Revised-by",
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
				rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x")),
				rec("0138", "Implemented", 2, op(adr.OpRemove, "d/t:x")),
			}),
			cutoff: 137,
			want:   "claim d/t:x is the target of more than one operation in this transition",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := currentstate.CheckPair(tc.before, tc.after, tc.cutoff)
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
