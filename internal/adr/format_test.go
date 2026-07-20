package adr_test

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
)

// build assembles a current-state-v1 ADR document from its varying parts. The
// Context, Consequences, and Alternatives Considered bodies are fixed; only the
// status, date, Decision items, State changes, and Status history vary.
func build(status, date, decision, stateChanges, history string) string {
	return "---\nformat: current-state-v1\nstatus: " + status + "\ndate: " + date + "\n---\n" +
		"# ADR-0137: Test Decision\n\n" +
		"## Context\n\nBackground prose.\n\n" +
		"## Decision\n\n" + decision + "\n\n" +
		"## State changes\n\n" + stateChanges + "\n\n" +
		"## Consequences\n\nConsequence prose.\n\n" +
		"## Alternatives Considered\n\nNone considered.\n\n" +
		"## Status history\n\n" + history + "\n"
}

const oneDecision = "1. The only decision."

// TestFrozenContentEqual permits Proposed drafting and freezes canonical
// decision content at every later status.
func TestFrozenContentEqual(t *testing.T) {
	record := func(status, decision string) adr.ADR {
		return adr.ADR{Status: status, Sections: map[string]string{"Decision": decision}}
	}
	cases := []struct {
		name          string
		before, after adr.ADR
		want          bool
	}{
		{"Proposed rewrite", record("Proposed", "old"), record("Proposed", "new"), true},
		{"Accepted unchanged", record("Accepted", "same"), record("Accepted", "same"), true},
		{"Accepted rewrite", record("Accepted", "old"), record("Accepted", "new"), false},
		{"Implemented rewrite", record("Implemented", "old"), record("Implemented", "new"), false},
		{"Abandoned rewrite", record("Abandoned", "old"), record("Abandoned", "new"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := adr.FrozenContentEqual(tc.before, tc.after); got != tc.want {
				t.Fatalf("FrozenContentEqual = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestHistoryTransitionValid requires equality without a status change and an
// exact one-entry extension for every legal status edge.
func TestHistoryTransitionValid(t *testing.T) {
	p := adr.StatusEntry{Date: "2026-01-01", Status: "Proposed"}
	a := adr.StatusEntry{Date: "2026-01-02", Status: "Accepted", Digest: "digest"}
	i := adr.StatusEntry{Date: "2026-01-03", Status: "Implemented", Digest: "digest"}
	record := func(status string, history ...adr.StatusEntry) adr.ADR {
		return adr.ADR{Status: status, History: history}
	}
	cases := []struct {
		name          string
		before, after adr.ADR
		want          bool
	}{
		{"same status equal", record("Accepted", p, a), record("Accepted", p, a), true},
		{"same status replacement", record("Accepted", p, a), record("Accepted", p, adr.StatusEntry{Date: "2026-01-09", Status: "Accepted"}), false},
		{"legal exact append", record("Accepted", p, a), record("Implemented", p, a, i), true},
		{"legal append after rewritten prefix", record("Accepted", p, a), record("Implemented", adr.StatusEntry{Date: "2026-01-09", Status: "Proposed"}, a, i), false},
		{"legal edge missing append", record("Accepted", p, a), record("Implemented", p, a), false},
		{"legal edge two appends", record("Proposed", p), record("Implemented", p, a, i), false},
		{"illegal edge", record("Implemented", p, i), record("Abandoned", p, i, adr.StatusEntry{Date: "2026-01-04", Status: "Abandoned"}), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := adr.HistoryTransitionValid(tc.before, tc.after); got != tc.want {
				t.Fatalf("HistoryTransitionValid = %v, want %v", got, tc.want)
			}
		})
	}
}

// digestFor returns the content-sha256 an ADR with these Decision and State
// changes bodies must record, computed from a Proposed scaffold that shares the
// five canonical sections byte-for-byte.
func digestFor(t *testing.T, stateChanges string) string {
	t.Helper()
	a, err := adr.ParseV1("0137-x.md", []byte(build("Proposed", "2026-07-20", oneDecision, stateChanges, "- 2026-07-20: Proposed")))
	if err != nil {
		t.Fatalf("scaffold parse for digest: %v", err)
	}
	return adr.ContentDigest(a.Sections)
}

// TestParseV1Valid covers every legal lifecycle shape end to end.
func TestParseV1Valid(t *testing.T) {
	noneDigest := digestFor(t, "None.")
	opsChanges := "- add `tooling/cli:new-flag`\n- update `config/configuration:strict-scalars`"
	opsDigest := digestFor(t, opsChanges)

	cases := []struct {
		name    string
		doc     string
		status  string
		wantOps int
		none    bool
	}{
		{"proposed scaffold", build("Proposed", "2026-07-20", oneDecision, "None.", "- 2026-07-20: Proposed"), "Proposed", 0, true},
		{"accepted", build("Accepted", "2026-07-21", oneDecision, opsChanges,
			"- 2026-07-20: Proposed\n- 2026-07-21: Accepted; content-sha256: "+opsDigest), "Accepted", 2, false},
		{"implemented none", build("Implemented", "2026-07-21", oneDecision, "None.",
			"- 2026-07-20: Proposed\n- 2026-07-21: Implemented; content-sha256: "+noneDigest), "Implemented", 0, true},
		{"implemented with ops and sequence", build("Implemented", "2026-07-22", oneDecision, opsChanges,
			"- 2026-07-20: Proposed\n- 2026-07-22: Implemented; content-sha256: "+opsDigest+"; state-sequence: 7"), "Implemented", 2, false},
		{"accepted then implemented", build("Implemented", "2026-07-23", oneDecision, opsChanges,
			"- 2026-07-20: Proposed\n- 2026-07-21: Accepted; content-sha256: "+opsDigest+"\n- 2026-07-23: Implemented; content-sha256: "+opsDigest+"; state-sequence: 3"), "Implemented", 2, false},
		{"abandoned with rationale", build("Abandoned", "2026-07-24", oneDecision, opsChanges,
			"- 2026-07-20: Proposed\n- 2026-07-24: Abandoned; content-sha256: "+opsDigest+"; rationale: never built the seam"), "Abandoned", 2, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, err := adr.ParseV1("0137-test-decision.md", []byte(tc.doc))
			if err != nil {
				t.Fatalf("ParseV1: %v", err)
			}
			if !a.IsV1() || a.Status != tc.status || a.Number != "0137" {
				t.Fatalf("record = {v1:%v status:%q number:%q}, want v1 status:%q number:0137", a.IsV1(), a.Status, a.Number, tc.status)
			}
			if a.Title != "ADR-0137: Test Decision" {
				t.Errorf("title = %q", a.Title)
			}
			if len(a.Operations) != tc.wantOps || a.NoneState != tc.none {
				t.Errorf("ops = %d none = %v; want %d, %v", len(a.Operations), a.NoneState, tc.wantOps, tc.none)
			}
		})
	}
}

// TestParseV1FencedHeadingIgnored proves a `## ` inside a fenced block does not
// count as a section heading.
func TestParseV1FencedHeadingIgnored(t *testing.T) {
	decision := "1. Only decision.\n\n```\n## Not a section\n```"
	doc := build("Proposed", "2026-07-20", decision, "None.", "- 2026-07-20: Proposed")
	if _, err := adr.ParseV1("0137-x.md", []byte(doc)); err != nil {
		t.Fatalf("fenced heading should be ignored: %v", err)
	}
}

// TestParseV1Errors covers each validation failure.
// invariant: adr-system/adr-lifecycle:decision-items-enumerable
func TestParseV1Errors(t *testing.T) {
	d := digestFor(t, "None.")
	cases := []struct {
		name, doc, want string
	}{
		{"no frontmatter", "# ADR-0137: X\n\n## Context\n", "missing frontmatter"},
		{"unknown frontmatter key", "---\nformat: current-state-v1\nstatus: Proposed\ndate: 2026-07-20\ntags: [x]\n---\n# X\n", "frontmatter:"},
		{"wrong format marker", "---\nformat: legacy\nstatus: Proposed\ndate: 2026-07-20\n---\n# X\n", "format must be"},
		{"invalid status", build("Bogus", "2026-07-20", oneDecision, "None.", "- 2026-07-20: Proposed"), "invalid status"},
		{"bad date", build("Proposed", "2026-13-40", oneDecision, "None.", "- 2026-13-40: Proposed"), "invalid date"},
		{"wrong section order", strings.Replace(build("Proposed", "2026-07-20", oneDecision, "None.", "- 2026-07-20: Proposed"), "## Context\n\nBackground prose.\n\n", "", 1), "sections must be exactly"},
		{"no decision items", build("Proposed", "2026-07-20", "Just prose, no items.", "None.", "- 2026-07-20: Proposed"), "no numbered items"},
		{"non-sequential decision items", build("Proposed", "2026-07-20", "1. One.\n3. Three.", "None.", "- 2026-07-20: Proposed"), "sequential from 1"},
		{"empty state changes", build("Proposed", "2026-07-20", oneDecision, "", "- 2026-07-20: Proposed"), "state changes is empty"},
		{"malformed state change", build("Proposed", "2026-07-20", oneDecision, "- add tooling/cli:x", "- 2026-07-20: Proposed"), "malformed State changes"},
		{"duplicate claim id", build("Proposed", "2026-07-20", oneDecision, "- add `tooling/cli:x`\n- update `tooling/cli:x`", "- 2026-07-20: Proposed"), "more than once"},
		{"empty status history", build("Proposed", "2026-07-20", oneDecision, "None.", ""), "status history is empty"},
		{"malformed history line", build("Proposed", "2026-07-20", oneDecision, "None.", "- proposed today"), "malformed Status history"},
		{"bare rationale segment", build("Abandoned", "2026-07-21", oneDecision, "None.", "- 2026-07-20: Proposed\n- 2026-07-21: Abandoned; content-sha256: "+d+"; rationale:"), "unknown metadata segment"},
		{"bad digest hex", build("Implemented", "2026-07-21", oneDecision, "None.", "- 2026-07-20: Proposed\n- 2026-07-21: Implemented; content-sha256: zzz"), "64-hex"},
		{"duplicate content-sha256", build("Accepted", "2026-07-21", oneDecision, "None.", "- 2026-07-20: Proposed\n- 2026-07-21: Accepted; content-sha256: "+d+"; content-sha256: "+d), "duplicated or out of order"},
		{"sequence before digest", build("Implemented", "2026-07-21", oneDecision, "None.", "- 2026-07-20: Proposed\n- 2026-07-21: Implemented; state-sequence: 1; content-sha256: "+d), "duplicated or out of order"},
		{"non-positive sequence", build("Implemented", "2026-07-21", oneDecision, "None.", "- 2026-07-20: Proposed\n- 2026-07-21: Implemented; content-sha256: "+d+"; state-sequence: 0"), "positive integer"},
		{"duplicate sequence", build("Implemented", "2026-07-21", oneDecision, "None.", "- 2026-07-20: Proposed\n- 2026-07-21: Implemented; content-sha256: "+d+"; state-sequence: 1; state-sequence: 2"), "state-sequence is duplicated"},
		{"unknown segment", build("Accepted", "2026-07-21", oneDecision, "None.", "- 2026-07-20: Proposed\n- 2026-07-21: Accepted; content-sha256: "+d+"; mystery: x"), "unknown metadata segment"},
		{"malformed metadata segment", build("Accepted", "2026-07-21", oneDecision, "None.", "- 2026-07-20: Proposed\n- 2026-07-21: Accepted;content-sha256: "+d), "malformed metadata segment"},
		{"first not proposed", build("Accepted", "2026-07-21", oneDecision, "None.", "- 2026-07-21: Accepted; content-sha256: "+d), "must be the `- <date>: Proposed` scaffold"},
		{"illegal transition", build("Accepted", "2026-07-21", oneDecision, "None.", "- 2026-07-20: Proposed\n- 2026-07-20: Proposed\n- 2026-07-21: Accepted; content-sha256: "+d), "illegal Status history transition"},
		{"descending dates", build("Accepted", "2026-07-19", oneDecision, "None.", "- 2026-07-20: Proposed\n- 2026-07-19: Accepted; content-sha256: "+d), "must not descend"},
		{"accepted with sequence", build("Accepted", "2026-07-21", oneDecision, "None.", "- 2026-07-20: Proposed\n- 2026-07-21: Accepted; content-sha256: "+d+"; state-sequence: 1"), "sequence or rationale it must not"},
		{"implemented with rationale", build("Implemented", "2026-07-21", oneDecision, "None.", "- 2026-07-20: Proposed\n- 2026-07-21: Implemented; content-sha256: "+d+"; rationale: x"), "must not carry a rationale"},
		{"implemented ops missing sequence", build("Implemented", "2026-07-21", oneDecision, "- add `a/b:c`", "- 2026-07-20: Proposed\n- 2026-07-21: Implemented; content-sha256: "+digestFor(t, "- add `a/b:c`")), "must record a state-sequence"},
		{"implemented none with sequence", build("Implemented", "2026-07-21", oneDecision, "None.", "- 2026-07-20: Proposed\n- 2026-07-21: Implemented; content-sha256: "+d+"; state-sequence: 1"), "must not record a state-sequence"},
		{"abandoned missing rationale", build("Abandoned", "2026-07-21", oneDecision, "None.", "- 2026-07-20: Proposed\n- 2026-07-21: Abandoned; content-sha256: "+d), "must end with a nonempty rationale"},
		{"abandoned with sequence", build("Abandoned", "2026-07-21", oneDecision, "None.", "- 2026-07-20: Proposed\n- 2026-07-21: Abandoned; content-sha256: "+d+"; state-sequence: 1"), "abandoned entry must not record a state-sequence"},
		{"digest mismatch", build("Accepted", "2026-07-21", oneDecision, "None.", "- 2026-07-20: Proposed\n- 2026-07-21: Accepted; content-sha256: "+strings.Repeat("a", 64)), "does not match the computed digest"},
		{"final status mismatch", build("Implemented", "2026-07-21", oneDecision, "None.", "- 2026-07-20: Proposed"), "does not match frontmatter status"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := adr.ParseV1("0137-x.md", []byte(tc.doc))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v; want containing %q", err, tc.want)
			}
		})
	}
}
