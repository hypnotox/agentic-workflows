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
	before := uni(
		[]adr.ADR{
			rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x")),
			rec("0138", "Accepted", 0, op(adr.OpUpdate, "d/t:x")),
		},
		prosed(claim("d/t:x", "0137"), "old"))
	after := uni(
		[]adr.ADR{
			rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x")),
			rec("0138", "Implemented", 2, op(adr.OpUpdate, "d/t:x")),
		},
		prosed(claim("d/t:x", "0137", "0138"), "new"))
	if f := currentstate.CheckPair(before, after, 137); len(f) != 0 {
		t.Fatalf("expected no findings, got:\n%s", messages(f))
	}
}

// TestCheckPairValidRemove accepts an Accepted->Implemented remove that retires
// the claim.
func TestCheckPairValidRemove(t *testing.T) {
	before := uni(
		[]adr.ADR{
			rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x")),
			rec("0139", "Accepted", 0, op(adr.OpRemove, "d/t:x")),
		},
		claim("d/t:x", "0137"))
	after := uni([]adr.ADR{
		rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x")),
		rec("0139", "Implemented", 2, op(adr.OpRemove, "d/t:x")),
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

// TestCheckPairIllegalTransition rejects an edge out of a terminal state.
func TestCheckPairIllegalTransition(t *testing.T) {
	before := uni([]adr.ADR{rec("0137", "Implemented", 0)})
	after := uni([]adr.ADR{rec("0137", "Abandoned", 0)})
	if f := currentstate.CheckPair(before, after, 137); !strings.Contains(messages(f), "ADR-0137 changed status from Implemented to Abandoned, which is not a legal") {
		t.Fatalf("illegal transition not reported:\n%s", messages(f))
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
