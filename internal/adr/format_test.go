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

func buildV2(status, stateChanges, history string) string {
	return strings.Replace(build(status, "2026-07-25", oneDecision, stateChanges, history), "format: current-state-v1", "format: current-state-v2", 1)
}

func v2DigestFor(t *testing.T, stateChanges string) string {
	t.Helper()
	a, err := adr.ParseV2("0137-x.md", []byte(buildV2("Proposed", stateChanges, "- 2026-07-20: Proposed")))
	if err != nil {
		t.Fatalf("V2 scaffold parse for digest: %v", err)
	}
	return adr.ContentDigest(a.Sections)
}

func TestParseV2LifecycleAndApplications(t *testing.T) {
	changes := "- add `a/b:first`\n- update `a/b:second`\n- remove `a/b:third`"
	digest := v2DigestFor(t, changes)
	p := "- 2026-07-20: Proposed"
	a := "- 2026-07-21: Accepted; content-sha256: " + digest
	i := "- 2026-07-22: Implementing; content-sha256: " + digest
	first := "- 2026-07-22: Applied; state-sequence: 4; operations: add `a/b:first`"
	middle := "- 2026-07-23: Applied; state-sequence: 7; operations: update `a/b:second`"
	final := "- 2026-07-24: Applied; state-sequence: 9; operations: remove `a/b:third`"
	implemented := "- 2026-07-24: Implemented; content-sha256: " + digest
	abandoned := "- 2026-07-24: Abandoned; content-sha256: " + digest + "; rationale: stopped; safely"
	cases := []struct {
		name, status, history string
		wantEvents            int
	}{
		{"proposed", "Proposed", p, 1},
		{"proposed accepted", "Accepted", p + "\n" + a, 2},
		{"proposed direct implemented", "Implemented", p + "\n- 2026-07-22: Implemented; content-sha256: " + digest + "; state-sequence: 2", 2},
		{"proposed abandoned", "Abandoned", p + "\n" + abandoned, 2},
		{"proposed implementing first", "Implementing", p + "\n" + i + "\n" + first, 3},
		{"accepted implementing middle", "Implementing", p + "\n" + a + "\n" + i + "\n" + first + "\n" + middle, 5},
		{"accepted direct implemented", "Implemented", p + "\n" + a + "\n- 2026-07-22: Implemented; content-sha256: " + digest + "; state-sequence: 3", 3},
		{"accepted abandoned", "Abandoned", p + "\n" + a + "\n" + abandoned, 3},
		{"implementing implemented", "Implemented", p + "\n" + i + "\n" + first + "\n" + middle + "\n" + final + "\n" + implemented, 6},
		{"partial abandoned", "Abandoned", p + "\n" + i + "\n" + first + "\n" + abandoned, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record, err := adr.ParseV2("0137-test.md", []byte(buildV2(tc.status, changes, tc.history)))
			if err != nil {
				t.Fatalf("ParseV2: %v", err)
			}
			if !record.IsV2() || record.IsV1() || len(record.History) != tc.wantEvents {
				t.Fatalf("record format/history = %v/%d", record.Format, len(record.History))
			}
		})
	}

	noneDigest := v2DigestFor(t, "None.")
	if _, err := adr.ParseV2("0138-none.md", []byte(buildV2("Implemented", "None.", p+"\n- 2026-07-22: Implemented; content-sha256: "+noneDigest))); err != nil {
		t.Fatalf("direct None implementation: %v", err)
	}
}

func TestParseV2RejectsInvalidHistory(t *testing.T) {
	changes := "- add `a/b:first`\n- update `a/b:second`"
	digest := v2DigestFor(t, changes)
	p := "- 2026-07-20: Proposed"
	i := "- 2026-07-21: Implementing; content-sha256: " + digest
	first := "- 2026-07-21: Applied; state-sequence: 1; operations: add `a/b:first`"
	cases := []struct{ name, status, changes, history, want string }{
		{"v1 excludes implementing", "Implementing", changes, p + "\n" + i, ""},
		{"first not proposed", "Accepted", changes, "- 2026-07-20: Accepted; content-sha256: " + digest, "first Status history"},
		{"repeated proposed", "Proposed", changes, p + "\n" + p, "illegal Status history transition"},
		{"proposed metadata", "Proposed", changes, p + "; state-sequence: 1", "first Status history"},
		{"accepted sequence", "Accepted", changes, p + "\n- 2026-07-21: Accepted; content-sha256: " + digest + "; state-sequence: 1", "sequence or rationale"},
		{"implementing rationale", "Implementing", changes, p + "\n- 2026-07-21: Implementing; content-sha256: " + digest + "; rationale: no\n" + first, "sequence or rationale"},
		{"implemented rationale", "Implemented", changes, p + "\n- 2026-07-21: Implemented; content-sha256: " + digest + "; state-sequence: 1; rationale: no", "must not carry a rationale"},
		{"abandoned sequence", "Abandoned", changes, p + "\n- 2026-07-21: Abandoned; content-sha256: " + digest + "; state-sequence: 1; rationale: no", "must not record a state-sequence"},
		{"abandoned missing rationale", "Abandoned", changes, p + "\n- 2026-07-21: Abandoned; content-sha256: " + digest, "nonempty rationale"},
		{"implicit operations missing sequence", "Implemented", changes, p + "\n- 2026-07-21: Implemented; content-sha256: " + digest, "must record a state-sequence"},
		{"implicit None with sequence", "Implemented", "None.", p + "\n- 2026-07-21: Implemented; content-sha256: " + v2DigestFor(t, "None.") + "; state-sequence: 1", "must not record a state-sequence"},
		{"implementing none", "Implementing", "None.", p + "\n" + i + "\n" + first, "not declared"},
		{"implementing one op", "Implementing", "- add `a/b:first`", p + "\n- 2026-07-21: Implementing; content-sha256: " + v2DigestFor(t, "- add `a/b:first`") + "\n" + first, "at least two"},
		{"missing first application", "Implementing", changes, p + "\n" + i, "followed by"},
		{"all applied while implementing", "Implementing", changes, p + "\n" + i + "\n- 2026-07-21: Applied; state-sequence: 1; operations: add `a/b:first`, update `a/b:second`", "one remaining"},
		{"applied before implementing", "Proposed", changes, p + "\n" + first, "only while Implementing"},
		{"undeclared verb", "Implementing", changes, p + "\n" + i + "\n- 2026-07-21: Applied; state-sequence: 1; operations: remove `a/b:first`", "not declared"},
		{"undeclared id", "Implementing", changes, p + "\n" + i + "\n- 2026-07-21: Applied; state-sequence: 1; operations: add `a/b:other`", "not declared"},
		{"duplicate in batch", "Implementing", changes, p + "\n" + i + "\n- 2026-07-21: Applied; state-sequence: 1; operations: add `a/b:first`, add `a/b:first`", "duplicated"},
		{"duplicate across batches", "Implementing", changes, p + "\n" + i + "\n" + first + "\n- 2026-07-22: Applied; state-sequence: 2; operations: add `a/b:first`", "already applied"},
		{"declaration order", "Implementing", "- add `a/b:first`\n- update `a/b:second`\n- remove `a/b:third`", p + "\n- 2026-07-21: Implementing; content-sha256: " + v2DigestFor(t, "- add `a/b:first`\n- update `a/b:second`\n- remove `a/b:third`") + "\n- 2026-07-21: Applied; state-sequence: 1; operations: update `a/b:second`, add `a/b:first`", "declaration order"},
		{"bad separator", "Implementing", changes, p + "\n" + i + "\n- 2026-07-21: Applied; state-sequence: 1; operations: add `a/b:first`,update `a/b:second`", "malformed Applied operation"},
		{"bad code span", "Implementing", changes, p + "\n" + i + "\n- 2026-07-21: Applied; state-sequence: 1; operations: add a/b:first", "malformed Applied operation"},
		{"bad id", "Implementing", changes, p + "\n" + i + "\n- 2026-07-21: Applied; state-sequence: 1; operations: add `A/b:first`", "malformed Applied operation"},
		{"zero sequence", "Implementing", changes, p + "\n" + i + "\n- 2026-07-21: Applied; state-sequence: 0; operations: add `a/b:first`", "malformed Status history"},
		{"metadata order", "Implementing", changes, p + "\n" + i + "\n- 2026-07-21: Applied; operations: add `a/b:first`; state-sequence: 1", "malformed Status history"},
		{"empty application", "Implementing", changes, p + "\n" + i + "\n- 2026-07-21: Applied; state-sequence: 1; operations: ", "malformed Status history"},
		{"mixed sequencing", "Implemented", changes, p + "\n" + i + "\n" + first + "\n- 2026-07-22: Applied; state-sequence: 2; operations: update `a/b:second`\n- 2026-07-22: Implemented; content-sha256: " + digest + "; state-sequence: 3", "mix"},
		{"missing final application", "Implemented", changes, p + "\n" + i + "\n" + first + "\n- 2026-07-22: Implemented; content-sha256: " + digest, "every declared"},
		{"incomplete implemented", "Implemented", "- add `a/b:first`\n- update `a/b:second`\n- remove `a/b:third`", p + "\n- 2026-07-21: Implementing; content-sha256: " + v2DigestFor(t, "- add `a/b:first`\n- update `a/b:second`\n- remove `a/b:third`") + "\n" + first + "\n- 2026-07-22: Applied; state-sequence: 2; operations: update `a/b:second`\n- 2026-07-22: Implemented; content-sha256: " + v2DigestFor(t, "- add `a/b:first`\n- update `a/b:second`\n- remove `a/b:third`"), "every declared"},
		{"fully applied abandoned", "Abandoned", changes, p + "\n" + i + "\n" + first + "\n- 2026-07-22: Applied; state-sequence: 2; operations: update `a/b:second`\n- 2026-07-23: Abandoned; content-sha256: " + digest + "; rationale: stopped", "canceled"},
		{"descending applied date", "Implementing", changes, p + "\n" + i + "\n- 2026-07-19: Applied; state-sequence: 1; operations: add `a/b:first`", "must not descend"},
		{"digest mismatch", "Accepted", changes, p + "\n- 2026-07-21: Accepted; content-sha256: " + strings.Repeat("a", 64), "does not match"},
		{"latest status mismatch", "Implemented", changes, p + "\n- 2026-07-21: Accepted; content-sha256: " + digest, "does not match frontmatter"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			if tc.name == "v1 excludes implementing" {
				_, err = adr.ParseV1("0137-test.md", []byte(build("Implementing", "2026-07-25", oneDecision, changes, tc.history)))
				tc.want = "invalid status"
			} else {
				_, err = adr.ParseV2("0137-test.md", []byte(buildV2(tc.status, tc.changes, tc.history)))
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestV2TransitionMatrix(t *testing.T) {
	legal := map[string]bool{
		"Proposed>Accepted": true, "Proposed>Implementing": true, "Proposed>Implemented": true, "Proposed>Abandoned": true,
		"Accepted>Implementing": true, "Accepted>Implemented": true, "Accepted>Abandoned": true,
		"Implementing>Implemented": true, "Implementing>Abandoned": true,
	}
	statuses := []string{"Proposed", "Accepted", "Implementing", "Implemented", "Abandoned"}
	for _, from := range statuses {
		for _, to := range statuses {
			key := from + ">" + to
			if got := adr.TransitionLegal(from, to, adr.CurrentStateV2); got != legal[key] {
				t.Errorf("%s = %v, want %v", key, got, legal[key])
			}
		}
	}
}

func TestV2HistoryTransitionPrefixAndShapes(t *testing.T) {
	status := func(value string) adr.HistoryEvent { return adr.HistoryEvent{Kind: adr.HistoryStatus, Status: value} }
	applied := adr.HistoryEvent{Kind: adr.HistoryApplied, Sequence: 1, HasSequence: true, Operations: []adr.Operation{{Verb: adr.OpAdd, ID: "a/b:c", Slug: "c"}}}
	record := func(front string, events ...adr.HistoryEvent) adr.ADR {
		return adr.ADR{Format: adr.CurrentStateV2, Status: front, History: events}
	}
	p, accepted, i, done, abandoned := status("Proposed"), status("Accepted"), status("Implementing"), status("Implemented"), status("Abandoned")
	for _, tc := range []struct {
		name          string
		before, after adr.ADR
		want          bool
	}{
		{"accept", record("Proposed", p), record("Accepted", p, accepted), true},
		{"direct implementation", record("Accepted", p, accepted), record("Implemented", p, accepted, done), true},
		{"enter implementing", record("Proposed", p), record("Implementing", p, i, applied), true},
		{"middle batch", record("Implementing", p, i, applied), record("Implementing", p, i, applied, applied), true},
		{"finish", record("Implementing", p, i, applied), record("Implemented", p, i, applied, applied, done), true},
		{"abandon", record("Implementing", p, i, applied), record("Abandoned", p, i, applied, abandoned), true},
		{"prefix deletion", record("Implementing", p, i, applied), record("Implemented", p, i, done), false},
		{"prefix mutation", record("Implementing", p, i, applied), record("Abandoned", status("Accepted"), i, applied, abandoned), false},
		{"same non-implementing append", record("Accepted", p, accepted), record("Accepted", p, accepted, applied), false},
		{"illegal status edge", record("Accepted", p, accepted), record("Proposed", p, accepted, p), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := adr.HistoryTransitionValid(tc.before, tc.after); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
