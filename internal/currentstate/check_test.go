package currentstate_test

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// rec builds a current-state-v1 ADR record with a Status history whose terminal
// entry carries seq as its state-sequence when status is Implemented with ops.
func rec(num, status string, seq int, ops ...adr.Operation) adr.ADR {
	hist := []adr.StatusEntry{{Date: "2026-01-01", Status: "Proposed"}}
	if status != "Proposed" {
		e := adr.StatusEntry{Date: "2026-01-02", Status: status}
		if status == "Implemented" && len(ops) > 0 && seq > 0 {
			e.Sequence, e.HasSequence = seq, true
		}
		hist = append(hist, e)
	}
	return adr.ADR{Number: num, Format: adr.CurrentStateV1, Status: status, Operations: ops, History: hist}
}

func op(v adr.OpVerb, id string) adr.Operation { return adr.Operation{Verb: v, ID: id} }

func claim(id, origin string, revisedBy ...string) topic.Claim {
	return topic.Claim{ID: id, Origin: origin, RevisedBy: revisedBy}
}

func topics(cl ...topic.Claim) []topic.Topic {
	return []topic.Topic{{ID: topic.TopicID{Domain: "d", Slug: "t"}, Claims: cl}}
}

func otherTopic(cl ...topic.Claim) []topic.Topic {
	return []topic.Topic{{ID: topic.TopicID{Domain: "other", Slug: "topic"}, Claims: cl}}
}

// messages joins the findings for substring assertions.
func messages(f []currentstate.Finding) string {
	var b strings.Builder
	for _, x := range f {
		b.WriteString(x.Message)
		b.WriteByte('\n')
	}
	return b.String()
}

// TestCheckValid accepts a coherent corpus: an Implemented add, an Implemented
// update, an Implemented remove, a legacy Origin, a pending Accepted add, and a
// legacy record that filterV1 skips.
func TestCheckValid(t *testing.T) {
	records := []adr.ADR{
		rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:kept")),
		rec("0138", "Implemented", 2, op(adr.OpUpdate, "d/t:kept")),
		rec("0139", "Implemented", 3, op(adr.OpAdd, "d/t:gone")),
		rec("0140", "Implemented", 4, op(adr.OpUpdate, "d/t:gone")), // update of a later-removed claim
		rec("0141", "Implemented", 5, op(adr.OpRemove, "d/t:gone")),
		rec("0142", "Accepted", 0, op(adr.OpAdd, "d/t:pending")),
		rec("0143", "Abandoned", 0, op(adr.OpAdd, "d/t:never")), // unapplied
		{Number: "0100", Format: adr.Legacy, Status: "Implemented"},
	}
	tp := topics(
		claim("d/t:kept", "0137", "0138"),
		claim("d/t:legacy", "0100"), // Origin below cutoff: exempt
	)
	if f := currentstate.Check(records, tp); len(f) != 0 {
		t.Fatalf("expected no findings, got:\n%s", messages(f))
	}
}

// TestCheckSequences covers duplicate and non-contiguous sequences.
func TestCheckSequences(t *testing.T) {
	dup := currentstate.Check([]adr.ADR{
		rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:a")),
		rec("0138", "Implemented", 1, op(adr.OpAdd, "d/t:b")),
	}, topics(claim("d/t:a", "0137"), claim("d/t:b", "0138")))
	if !strings.Contains(messages(dup), "used by more than one ADR") {
		t.Errorf("duplicate sequence not reported:\n%s", messages(dup))
	}
	gap := currentstate.Check([]adr.ADR{
		rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:a")),
		rec("0138", "Implemented", 3, op(adr.OpAdd, "d/t:b")),
	}, topics(claim("d/t:a", "0137"), claim("d/t:b", "0138")))
	if !strings.Contains(messages(gap), "not contiguous") {
		t.Errorf("sequence gap not reported:\n%s", messages(gap))
	}
}

// TestCheckOperationHistory covers the per-identity add/update/remove ordering.
func TestCheckOperationHistory(t *testing.T) {
	cases := []struct {
		name    string
		records []adr.ADR
		want    string
	}{
		{"two adds", []adr.ADR{rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x")), rec("0138", "Implemented", 2, op(adr.OpAdd, "d/t:x"))}, "2 add operations"},
		{"update without add", []adr.ADR{rec("0137", "Implemented", 1, op(adr.OpUpdate, "d/t:x"))}, "does not begin with an add"},
		{"two removes", []adr.ADR{rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x")), rec("0138", "Implemented", 2, op(adr.OpRemove, "d/t:x")), rec("0139", "Implemented", 3, op(adr.OpRemove, "d/t:x"))}, "more than one remove"},
		{"op after remove", []adr.ADR{rec("0137", "Implemented", 1, op(adr.OpRemove, "d/t:x")), rec("0138", "Implemented", 2, op(adr.OpAdd, "d/t:x"))}, "operation after its remove"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Provide claims so backward/forward do not add unrelated noise we do not assert on.
			if f := currentstate.Check(tc.records, nil); !strings.Contains(messages(f), tc.want) {
				t.Errorf("want %q in:\n%s", tc.want, messages(f))
			}
		})
	}
}

func TestCheckOperationHistoryAllowsMigratedBaseline(t *testing.T) {
	records := []adr.ADR{
		{Number: "0100"},
		rec("0137", "Implemented", 1, op(adr.OpUpdate, "d/t:legacy")),
		rec("0138", "Implemented", 2, op(adr.OpRemove, "d/t:retired")),
	}
	tp := topics(claim("d/t:legacy", "0100", "0137"))
	if f := currentstate.Check(records, tp); len(f) != 0 {
		t.Fatalf("migrated baseline update/remove rejected:\n%s", messages(f))
	}
}

// TestCheckForward covers the pending, Implemented, and Abandoned operation
// outcomes against the current claim set.
func TestCheckForward(t *testing.T) {
	cases := []struct {
		name    string
		records []adr.ADR
		topics  []topic.Topic
		want    string
	}{
		{"pending add exists", []adr.ADR{rec("0137", "Accepted", 0, op(adr.OpAdd, "d/t:x"))}, topics(claim("d/t:x", "0100")), "already exists"},
		{"pending update missing", []adr.ADR{rec("0137", "Proposed", 0, op(adr.OpUpdate, "d/t:x"))}, nil, "updates missing claim"},
		{"implemented add missing", []adr.ADR{rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x"))}, nil, "has no active claim"},
		{"implemented add wrong origin", []adr.ADR{rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x"))}, topics(claim("d/t:x", "0199")), "Origin is ADR-0199"},
		{"implemented update not revised", []adr.ADR{rec("0137", "Implemented", 1, op(adr.OpUpdate, "d/t:x"))}, topics(claim("d/t:x", "0100")), "does not list updating ADR-0137"},
		{"implemented remove still present", []adr.ADR{rec("0137", "Implemented", 1, op(adr.OpRemove, "d/t:x"))}, topics(claim("d/t:x", "0100")), "still has an active claim"},
		{"abandoned add applied", []adr.ADR{rec("0137", "Abandoned", 0, op(adr.OpAdd, "d/t:x"))}, topics(claim("d/t:x", "0137")), "add for claim d/t:x was applied"},
		{"abandoned update applied", []adr.ADR{rec("0137", "Abandoned", 0, op(adr.OpUpdate, "d/t:x"))}, topics(claim("d/t:x", "0100", "0137")), "update for claim d/t:x was applied"},
		{"pending re-add of removed", []adr.ADR{rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x"), op(adr.OpRemove, "d/t:x")), rec("0138", "Proposed", 0, op(adr.OpAdd, "d/t:x"))}, nil, "may never be reused"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if f := currentstate.Check(tc.records, tc.topics); !strings.Contains(messages(f), tc.want) {
				t.Errorf("want %q in:\n%s", tc.want, messages(f))
			}
		})
	}
}

// invariant: invariants/current-state-authority:abandoned-remove-pair-attributed
func TestCheckAbandonedRemoveAttributedByPair(t *testing.T) {
	abandoned := rec("0137", "Abandoned", 0, op(adr.OpRemove, "d/t:x"))
	accepted := rec("0138", "Accepted", 0, op(adr.OpRemove, "d/t:x"))
	implemented := rec("0138", "Implemented", 1, op(adr.OpRemove, "d/t:x"))
	implemented.History = append(append([]adr.StatusEntry(nil), accepted.History...), implemented.History[len(implemented.History)-1])

	legacy := adr.ADR{Number: "0100"}
	records := []adr.ADR{legacy, abandoned, implemented}
	if f := currentstate.Check(records, topics()); len(f) != 0 {
		t.Fatalf("final absence statically attributed to Abandoned removal:\n%s", messages(f))
	}
	withoutImplemented := currentstate.CheckPair(
		uni([]adr.ADR{legacy, abandoned}, claim("d/t:x", "0100")),
		uni([]adr.ADR{legacy, abandoned}),
	)
	if got := messages(withoutImplemented); !strings.Contains(got, "claim d/t:x was removed with no ADR remove operation in this transition") {
		t.Fatalf("disappearance without an actual Implemented remover was accepted:\n%s", got)
	}

	before := uni([]adr.ADR{legacy, abandoned, accepted}, claim("d/t:x", "0100"))
	after := uni(records)
	if f := currentstate.CheckPair(before, after); len(f) != 0 {
		t.Fatalf("actual Implemented removal rejected by pair validation:\n%s", messages(f))
	}
}

func TestCheckDestinationTopic(t *testing.T) {
	for _, verb := range []adr.OpVerb{adr.OpAdd, adr.OpUpdate, adr.OpRemove} {
		t.Run("Accepted "+string(verb)+" missing", func(t *testing.T) {
			var tp []topic.Topic
			if verb != adr.OpAdd {
				tp = otherTopic(claim("d/t:x", "0100"))
			}
			got := messages(currentstate.Check([]adr.ADR{
				rec("0137", "Accepted", 0, op(verb, "d/t:x")),
			}, tp))
			if !strings.Contains(got, "ADR-0137 operation "+string(verb)+" targets missing topic d/t") {
				t.Fatalf("missing destination topic not reported:\n%s", got)
			}
		})

		t.Run("Accepted "+string(verb)+" empty shell", func(t *testing.T) {
			tp := topics()
			if verb != adr.OpAdd {
				tp[0].Claims = []topic.Claim{claim("d/t:x", "0100")}
			}
			got := messages(currentstate.Check([]adr.ADR{
				rec("0137", "Accepted", 0, op(verb, "d/t:x")),
			}, tp))
			if strings.Contains(got, "targets missing topic") {
				t.Fatalf("empty topic shell was treated as absent:\n%s", got)
			}
		})

		t.Run("Implemented direct "+string(verb), func(t *testing.T) {
			var tp []topic.Topic
			switch verb {
			case adr.OpAdd:
				tp = otherTopic(claim("d/t:x", "0137"))
			case adr.OpUpdate:
				tp = otherTopic(claim("d/t:x", "0100", "0137"))
			case adr.OpRemove:
				tp = nil
			}
			got := messages(currentstate.Check([]adr.ADR{
				rec("0137", "Implemented", 1, op(verb, "d/t:x")),
			}, tp))
			if !strings.Contains(got, "ADR-0137 operation "+string(verb)+" targets missing topic d/t") {
				t.Fatalf("missing destination topic not reported:\n%s", got)
			}
		})

		t.Run("Proposed "+string(verb)+" exempt", func(t *testing.T) {
			var tp []topic.Topic
			if verb != adr.OpAdd {
				tp = otherTopic(claim("d/t:x", "0100"))
			}
			got := messages(currentstate.Check([]adr.ADR{
				rec("0137", "Proposed", 0, op(verb, "d/t:x")),
			}, tp))
			if strings.Contains(got, "targets missing topic") {
				t.Fatalf("Proposed operation required destination metadata:\n%s", got)
			}
		})
	}
}

func TestCheckDestinationTopicAbandonedHistory(t *testing.T) {
	t.Run("Accepted then Abandoned requires topic", func(t *testing.T) {
		a := rec("0137", "Abandoned", 0, op(adr.OpAdd, "d/t:x"))
		a.History = []adr.StatusEntry{
			{Date: "2026-01-01", Status: "Proposed"},
			{Date: "2026-01-02", Status: "Accepted"},
			{Date: "2026-01-03", Status: "Abandoned"},
		}
		got := messages(currentstate.Check([]adr.ADR{a}, nil))
		if !strings.Contains(got, "ADR-0137 operation add targets missing topic d/t") {
			t.Fatalf("Accepted-then-Abandoned destination topic not reported:\n%s", got)
		}
	})

	t.Run("Proposed then Abandoned is exempt", func(t *testing.T) {
		got := messages(currentstate.Check([]adr.ADR{
			rec("0137", "Abandoned", 0, op(adr.OpAdd, "d/t:x")),
		}, nil))
		if strings.Contains(got, "targets missing topic") {
			t.Fatalf("directly Abandoned operation required destination metadata:\n%s", got)
		}
	})
}

// TestCheckBackward covers the claim-to-ADR handshake direction.
func TestCheckBackward(t *testing.T) {
	// Origin at/above cutoff whose ADR carries no add operation.
	noAdd := currentstate.Check(
		[]adr.ADR{rec("0137", "Implemented", 1, op(adr.OpUpdate, "d/t:other"))},
		topics(claim("d/t:x", "0137"), claim("d/t:other", "0137")))
	if !strings.Contains(messages(noAdd), "Origin ADR-0137, which has no matching add") {
		t.Errorf("missing add operation not reported:\n%s", messages(noAdd))
	}
	// Revised-by whose ADR carries no update operation.
	noUpdate := currentstate.Check(
		[]adr.ADR{rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x"))},
		topics(claim("d/t:x", "0137", "0199")))
	if !strings.Contains(messages(noUpdate), "Revised-by ADR-0199, which has no matching update") {
		t.Errorf("missing update operation not reported:\n%s", messages(noUpdate))
	}
	outOfOrder := currentstate.Check(
		[]adr.ADR{
			rec("0137", "Implemented", 1, op(adr.OpAdd, "d/t:x")),
			rec("0138", "Implemented", 3, op(adr.OpUpdate, "d/t:x")),
			rec("0139", "Implemented", 2, op(adr.OpUpdate, "d/t:x")),
		},
		topics(claim("d/t:x", "0137", "0138", "0139")))
	if !strings.Contains(messages(outOfOrder), "not in increasing State-sequence order at ADR-0139") {
		t.Errorf("out-of-order Revised-by not reported:\n%s", messages(outOfOrder))
	}
	beforeOrigin := currentstate.Check(
		[]adr.ADR{
			rec("0136", "Implemented", 1, op(adr.OpAdd, "d/t:other")),
			rec("0137", "Implemented", 3, op(adr.OpAdd, "d/t:x")),
			rec("0138", "Implemented", 2, op(adr.OpUpdate, "d/t:x")),
		},
		topics(claim("d/t:other", "0136"), claim("d/t:x", "0137", "0138")))
	if !strings.Contains(messages(beforeOrigin), "not in increasing State-sequence order at ADR-0138") {
		t.Errorf("Revised-by before Origin sequence not reported:\n%s", messages(beforeOrigin))
	}
}

func v2rec(num, status string, declarations []adr.Operation, events ...adr.HistoryEvent) adr.ADR {
	return adr.ADR{Number: num, Format: adr.CurrentStateV2, Status: status, Operations: declarations, History: events}
}

func v2status(status string) adr.HistoryEvent {
	return adr.HistoryEvent{Kind: adr.HistoryStatus, Date: "2026-01-01", Status: status}
}

func v2batch(sequence int, operations ...adr.Operation) adr.HistoryEvent {
	return adr.HistoryEvent{Kind: adr.HistoryApplied, Date: "2026-01-02", Sequence: sequence, HasSequence: true, Operations: operations}
}

func TestCheckV2AppliedAuthority(t *testing.T) {
	addX := op(adr.OpAdd, "d/t:x")
	updateX := op(adr.OpUpdate, "d/t:x")
	pending := op(adr.OpAdd, "d/t:pending")
	base := rec("0137", "Implemented", 1, addX)
	implementing := v2rec("0138", "Implementing", []adr.Operation{updateX, pending},
		v2status("Proposed"), v2status("Implementing"), v2batch(2, updateX))
	if f := currentstate.Check([]adr.ADR{base, implementing}, topics(claim("d/t:x", "0137", "0138"))); len(f) != 0 {
		t.Fatalf("valid interleaved Implementing state rejected:\n%s", messages(f))
	}

	abandoned := implementing
	abandoned.Status = "Abandoned"
	abandoned.History = append(append([]adr.HistoryEvent(nil), implementing.History...), v2status("Abandoned"))
	if f := currentstate.Check([]adr.ADR{{Number: "0100"}, base, abandoned}, topics(claim("d/t:x", "0137", "0138"), claim("d/t:pending", "0100"))); len(f) != 0 {
		t.Fatalf("canceled add imposed a pending/result precondition:\n%s", messages(f))
	}

	remove := op(adr.OpRemove, "d/t:x")
	cancel := op(adr.OpUpdate, "d/t:unused")
	partialRemove := v2rec("0139", "Abandoned", []adr.Operation{remove, cancel},
		v2status("Proposed"), v2status("Implementing"), v2batch(2, remove), v2status("Abandoned"))
	if f := currentstate.Check([]adr.ADR{base, partialRemove}, topics()); len(f) != 0 {
		t.Fatalf("partially Abandoned remove lost authority:\n%s", messages(f))
	}
	reuse := v2rec("0140", "Proposed", []adr.Operation{addX}, v2status("Proposed"))
	if got := messages(currentstate.Check([]adr.ADR{base, partialRemove, reuse}, topics())); !strings.Contains(got, "may never be reused") {
		t.Fatalf("removed ID reuse not rejected:\n%s", got)
	}
	directlyAbandonedReuse := v2rec("0140", "Abandoned", []adr.Operation{addX}, v2status("Proposed"), v2status("Abandoned"))
	if got := messages(currentstate.Check([]adr.ADR{base, partialRemove, directlyAbandonedReuse}, topics())); !strings.Contains(got, "may never be reused") {
		t.Fatalf("directly Abandoned V2 removed ID reuse not rejected:\n%s", got)
	}
	v1AbandonedReuse := rec("0140", "Abandoned", 0, addX)
	if got := messages(currentstate.Check([]adr.ADR{base, partialRemove, v1AbandonedReuse}, topics())); !strings.Contains(got, "may never be reused") {
		t.Fatalf("Abandoned V1 removed ID reuse not rejected:\n%s", got)
	}

	canceledWithoutPreconditions := []adr.Operation{
		op(adr.OpAdd, "d/t:active"),
		op(adr.OpUpdate, "d/t:missing-update"),
		op(adr.OpRemove, "d/t:missing-remove"),
	}
	directlyAbandoned := v2rec("0142", "Abandoned", canceledWithoutPreconditions, v2status("Proposed"), v2status("Abandoned"))
	if f := currentstate.Check([]adr.ADR{{Number: "0100"}, directlyAbandoned}, topics(claim("d/t:active", "0100"))); len(f) != 0 {
		t.Fatalf("directly Abandoned V2 operations imposed pending/result preconditions:\n%s", messages(f))
	}
	v1Abandoned := rec("0142", "Abandoned", 0, canceledWithoutPreconditions...)
	if f := currentstate.Check([]adr.ADR{{Number: "0100"}, v1Abandoned}, topics(claim("d/t:active", "0100"))); len(f) != 0 {
		t.Fatalf("Abandoned V1 operations changed equivalent canceled behavior:\n%s", messages(f))
	}

	remainingOrigin := v2rec("0141", "Proposed", []adr.Operation{op(adr.OpAdd, "d/t:new")}, v2status("Proposed"))
	if got := messages(currentstate.Check([]adr.ADR{remainingOrigin}, topics(claim("d/t:new", "0141")))); !strings.Contains(got, "no matching add operation applied") {
		t.Fatalf("remaining operation authorized inverse provenance:\n%s", got)
	}
	canceledOrigin := remainingOrigin
	canceledOrigin.Status = "Abandoned"
	canceledOrigin.History = append(canceledOrigin.History, v2status("Abandoned"))
	if got := messages(currentstate.Check([]adr.ADR{canceledOrigin}, topics(claim("d/t:new", "0141")))); !strings.Contains(got, "no matching add operation applied") {
		t.Fatalf("canceled operation authorized inverse provenance:\n%s", got)
	}
}

func TestCheckRejectsInvalidV2Projection(t *testing.T) {
	operation := op(adr.OpAdd, "d/t:x")
	invalid := v2rec("0137", "Implementing", []adr.Operation{operation}, v2status("Proposed"), v2status("Implementing"), v2batch(1, operation))
	if got := messages(currentstate.Check([]adr.ADR{invalid}, topics(claim("d/t:x", "0137")))); !strings.Contains(got, "requires applied and remaining operations") {
		t.Fatalf("invalid projection not reported:\n%s", got)
	}
}

func TestCheckV2BatchSequences(t *testing.T) {
	a := op(adr.OpAdd, "d/t:a")
	b := op(adr.OpAdd, "d/t:b")
	multi := v2rec("0137", "Implemented", []adr.Operation{a, b}, v2status("Proposed"), v2status("Implementing"), v2batch(1, a, b), v2status("Implemented"))
	if f := currentstate.Check([]adr.ADR{multi}, topics(claim("d/t:a", "0137"), claim("d/t:b", "0137"))); len(f) != 0 {
		t.Fatalf("multi-operation batch counted as duplicate sequence:\n%s", messages(f))
	}
	duplicate := v2rec("0138", "Implemented", []adr.Operation{op(adr.OpAdd, "d/t:c")}, v2status("Proposed"), v2status("Implemented"))
	duplicate.History[len(duplicate.History)-1].Sequence, duplicate.History[len(duplicate.History)-1].HasSequence = 1, true
	if got := messages(currentstate.Check([]adr.ADR{multi, duplicate}, topics(claim("d/t:a", "0137"), claim("d/t:b", "0137"), claim("d/t:c", "0138")))); !strings.Contains(got, "more than one ADR batch") {
		t.Fatalf("duplicate batch sequence not rejected:\n%s", got)
	}
	duplicate.History[len(duplicate.History)-1].Sequence = 3
	if got := messages(currentstate.Check([]adr.ADR{multi, duplicate}, topics(claim("d/t:a", "0137"), claim("d/t:b", "0137"), claim("d/t:c", "0138")))); !strings.Contains(got, "expected 2, found 3") {
		t.Fatalf("batch sequence gap not rejected:\n%s", got)
	}
}

// TestSeverityString covers both severities.
func TestSeverityString(t *testing.T) {
	if currentstate.Error.String() != "error" || currentstate.Warn.String() != "warn" {
		t.Fatalf("severity strings = %q, %q", currentstate.Error, currentstate.Warn)
	}
}

// TestParseRecordRouting covers cutoff-based legacy/v1 routing.
func TestParseRecordRouting(t *testing.T) {
	legacy := []byte("---\nstatus: Implemented\ndate: 2026-01-01\n---\n# ADR-0100: Legacy\n\n## Context\n\nx\n")
	a, err := adr.ParseRecord("0100-legacy.md", legacy, adr.FormatBoundaries{V1From: 137})
	if err != nil || a.IsV1() || a.Number != "0100" {
		t.Fatalf("legacy routing: %+v err=%v", a, err)
	}
	// A below-cutoff ADR that declares the v1 marker is rejected.
	strayV1 := []byte("---\nformat: current-state-v1\nstatus: Implemented\ndate: 2026-01-01\n---\n# ADR-0100: X\n")
	if _, err := adr.ParseRecord("0100-x.md", strayV1, adr.FormatBoundaries{V1From: 137}); err == nil || !strings.Contains(err.Error(), "below the format cutoff") {
		t.Fatalf("stray v1 marker below cutoff: err=%v", err)
	}
	// Cutoff of zero treats everything as legacy.
	if a, err := adr.ParseRecord("0200-x.md", legacy, adr.FormatBoundaries{}); err != nil || a.IsV1() {
		t.Fatalf("cutoff 0 routing: %+v err=%v", a, err)
	}
}
