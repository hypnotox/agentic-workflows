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
	removeX := op(adr.OpRemove, "d/t:x")
	removeY := op(adr.OpRemove, "d/t:y")
	baseX := rec("0136", "Implemented", 1, op(adr.OpAdd, "d/t:x"))
	baseY := rec("0137", "Implemented", 2, op(adr.OpAdd, "d/t:y"))
	proposed := v2rec("0138", "Proposed", []adr.Operation{removeX, removeY}, v2status("Proposed"))
	implementing := proposed
	implementing.Status = "Implementing"
	implementing.History = append(append([]adr.HistoryEvent(nil), proposed.History...), v2status("Implementing"), v2batch(3, removeX))

	before := uni([]adr.ADR{baseX, baseY, proposed}, claim("d/t:x", "0136"), claim("d/t:y", "0137"))
	afterApplied := uni([]adr.ADR{baseX, baseY, implementing}, claim("d/t:y", "0137"))
	if f := currentstate.CheckPair(before, afterApplied); len(f) != 0 {
		t.Fatalf("applied V2 remove pair rejected:\n%s", messages(f))
	}
	if got := messages(currentstate.CheckPair(before, uni([]adr.ADR{baseX, baseY, implementing}, claim("d/t:x", "0136"), claim("d/t:y", "0137")))); !strings.Contains(got, "removes claim d/t:x") {
		t.Fatalf("Applied event without required removal was accepted:\n%s", got)
	}

	abandoned := implementing
	abandoned.Status = "Abandoned"
	abandoned.History = append(append([]adr.HistoryEvent(nil), implementing.History...), v2status("Abandoned"))
	afterAbandoned := uni([]adr.ADR{baseX, baseY, abandoned}, claim("d/t:y", "0137"))
	if f := currentstate.CheckPair(afterApplied, afterAbandoned); len(f) != 0 {
		t.Fatalf("abandonment after applied remove rejected:\n%s", messages(f))
	}
	if f := currentstate.Check([]adr.ADR{baseX, baseY, abandoned}, topics(claim("d/t:y", "0137"))); len(f) != 0 {
		t.Fatalf("applied removal lost authority or canceled removal imposed absence:\n%s", messages(f))
	}
	if got := messages(currentstate.Check([]adr.ADR{baseX, baseY, abandoned}, topics())); !strings.Contains(got, "has no active claim") {
		t.Fatalf("canceled remaining remove attributed y absence:\n%s", got)
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

// invariant: invariants/current-state-authority:implemented-impact-bidirectional
// invariant: invariants/current-state-authority:removed-claim-id-not-reused
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

	implementedRemove := v2rec("0141", "Implemented", []adr.Operation{op(adr.OpRemove, "d/t:implemented-gone")}, v2status("Proposed"), func() adr.HistoryEvent {
		e := v2status("Implemented")
		e.Sequence, e.HasSequence = 1, true
		return e
	}())
	implementedReuse := v2rec("0142", "Proposed", []adr.Operation{op(adr.OpAdd, "d/t:implemented-gone")}, v2status("Proposed"))
	if got := messages(currentstate.Check([]adr.ADR{{Number: "0100"}, implementedRemove, implementedReuse}, topics())); !strings.Contains(got, "may never be reused") {
		t.Fatalf("ID removed by Implemented V2 ADR was reusable:\n%s", got)
	}

	remainingOrigin := v2rec("0143", "Proposed", []adr.Operation{op(adr.OpAdd, "d/t:new")}, v2status("Proposed"))
	if got := messages(currentstate.Check([]adr.ADR{remainingOrigin}, topics(claim("d/t:new", "0141")))); !strings.Contains(got, "no matching add operation applied") {
		t.Fatalf("remaining operation authorized inverse provenance:\n%s", got)
	}
	canceledOrigin := remainingOrigin
	canceledOrigin.Status = "Abandoned"
	canceledOrigin.History = append(canceledOrigin.History, v2status("Abandoned"))
	if got := messages(currentstate.Check([]adr.ADR{canceledOrigin}, topics(claim("d/t:new", "0143")))); !strings.Contains(got, "no matching add operation applied") {
		t.Fatalf("canceled operation authorized inverse provenance:\n%s", got)
	}

	for _, tc := range []struct {
		name    string
		records []adr.ADR
		topics  []topic.Topic
		want    string
	}{
		{"applied add missing result", []adr.ADR{rec("0150", "Implemented", 1, op(adr.OpAdd, "d/t:add"))}, topics(), "has no active claim"},
		{"applied update missing result", []adr.ADR{{Number: "0100"}, rec("0150", "Implemented", 1, op(adr.OpUpdate, "d/t:update"))}, topics(claim("d/t:update", "0100")), "does not list updating ADR-0150"},
		{"applied remove missing result", []adr.ADR{{Number: "0100"}, rec("0150", "Implemented", 1, op(adr.OpRemove, "d/t:remove"))}, topics(claim("d/t:remove", "0100")), "still has an active claim"},
		{"inverse add missing", []adr.ADR{rec("0150", "Implemented", 1, op(adr.OpUpdate, "d/t:other"))}, topics(claim("d/t:add", "0150"), claim("d/t:other", "0100", "0150")), "no matching add"},
		{"inverse update missing", []adr.ADR{rec("0150", "Implemented", 1, op(adr.OpAdd, "d/t:update"))}, topics(claim("d/t:update", "0150", "0151")), "no matching update"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := messages(currentstate.Check(tc.records, tc.topics))
			if tc.want != "" && !strings.Contains(got, tc.want) {
				t.Fatalf("want %q in:\n%s", tc.want, got)
			}
		})
	}
}

func TestCheckRejectsInvalidV2Projection(t *testing.T) {
	operation := op(adr.OpAdd, "d/t:x")
	invalid := v2rec("0137", "Implementing", []adr.Operation{operation}, v2status("Proposed"), v2status("Implementing"), v2batch(1, operation))
	if got := messages(currentstate.Check([]adr.ADR{invalid}, topics(claim("d/t:x", "0137")))); !strings.Contains(got, "requires applied and remaining operations") {
		t.Fatalf("invalid projection not reported:\n%s", got)
	}
}

// invariant: invariants/current-state-authority:application-batch-sequence-order
func TestCheckV2BatchSequences(t *testing.T) {
	addX := op(adr.OpAdd, "d/t:x")
	updateX := op(adr.OpUpdate, "d/t:x")
	v1Implicit := rec("0137", "Implemented", 1, addX)
	v2Implicit := v2rec("0138", "Implemented", []adr.Operation{updateX}, v2status("Proposed"), v2status("Implemented"))
	v2Implicit.History[len(v2Implicit.History)-1].Sequence, v2Implicit.History[len(v2Implicit.History)-1].HasSequence = 2, true
	pending := op(adr.OpAdd, "d/t:pending")
	v2Explicit := v2rec("0139", "Implementing", []adr.Operation{updateX, pending}, v2status("Proposed"), v2status("Implementing"), v2batch(3, updateX))
	records := []adr.ADR{v1Implicit, v2Implicit, v2Explicit}
	claims := topics(claim("d/t:x", "0137", "0138", "0139"))
	if f := currentstate.Check(records, claims); len(f) != 0 {
		t.Fatalf("interleaved V1 implicit, V2 implicit, and V2 explicit sequences rejected:\n%s", messages(f))
	}
	for i, record := range records {
		progress, err := record.OperationProgress()
		if err != nil || len(progress.Applied) != 1 || progress.Applied[0].Sequence != i+1 {
			t.Fatalf("ADR-%s inherited sequence = %#v, err=%v", record.Number, progress.Applied, err)
		}
	}
	if got := claims[0].Claims[0].RevisedBy; len(got) != 2 || got[0] != "0138" || got[1] != "0139" {
		t.Fatalf("topic provenance order = %v", got)
	}

	duplicate := v2Implicit
	duplicate.History[len(duplicate.History)-1].Sequence = 1
	if got := messages(currentstate.Check([]adr.ADR{v1Implicit, duplicate}, topics(claim("d/t:x", "0137", "0138")))); !strings.Contains(got, "more than one ADR batch") {
		t.Fatalf("duplicate batch sequence not rejected:\n%s", got)
	}
	duplicate.History[len(duplicate.History)-1].Sequence = 3
	if got := messages(currentstate.Check([]adr.ADR{v1Implicit, duplicate}, topics(claim("d/t:x", "0137", "0138")))); !strings.Contains(got, "expected 2, found 3") {
		t.Fatalf("batch sequence gap not rejected:\n%s", got)
	}
}

// TestSeverityString covers both severities.
func TestSeverityString(t *testing.T) {
	if currentstate.Error.String() != "error" || currentstate.Warn.String() != "warn" {
		t.Fatalf("severity strings = %q, %q", currentstate.Error, currentstate.Warn)
	}
}

// TestParseRecordRouting covers cutoff-based legacy/V1/V2 routing.
// invariant: adr-system/adr-lifecycle:fresh-adoption-v1-cutoff
// invariant: adr-system/adr-lifecycle:adr-status-enum-and-matrix
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
	governed := []byte("---\nformat: current-state-v1\nstatus: Proposed\ndate: 2026-01-01\n---\n# ADR-0137: Governed\n\n## Context\n\nx\n\n## Decision\n\n1. Decide.\n\n## State changes\n\nNone.\n\n## Consequences\n\nx\n\n## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-01-01: Proposed\n")
	v1, err := adr.ParseRecord("0137-governed.md", governed, adr.FormatBoundaries{V1From: 137, V2From: 138})
	if err != nil || !v1.IsV1() || v1.IsV2() {
		t.Fatalf("V1 routing: %+v err=%v", v1, err)
	}
	v2Bytes := []byte(strings.Replace(string(governed), adr.V1FormatMarker, adr.V2FormatMarker, 1))
	v2, err := adr.ParseRecord("0138-governed.md", v2Bytes, adr.FormatBoundaries{V1From: 137, V2From: 138})
	if err != nil || !v2.IsV2() || v2.IsV1() {
		t.Fatalf("V2 routing: %+v err=%v", v2, err)
	}
	if _, err := adr.ParseRecord("0138-wrong-v1.md", governed, adr.FormatBoundaries{V1From: 137, V2From: 138}); err == nil || !strings.Contains(err.Error(), adr.V2FormatMarker) {
		t.Fatalf("V1 record in V2 region accepted: %v", err)
	}
	// Cutoff of zero treats everything as legacy.
	if a, err := adr.ParseRecord("0200-x.md", legacy, adr.FormatBoundaries{}); err != nil || a.IsGoverned() {
		t.Fatalf("cutoff 0 routing: %+v err=%v", a, err)
	}
}
