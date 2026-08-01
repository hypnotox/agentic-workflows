package adr_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// invariant: adr-system/adr-lifecycle:intrinsic-format-routing (TestParseRecordRoutesByIntrinsicFormat)
func TestParseRecordRoutesByIntrinsicFormat(t *testing.T) {
	_, body, found := strings.Cut(adrTemplateFixture, "---\n")
	if !found {
		t.Fatal("fixture frontmatter delimiter missing")
	}
	v1 := strings.Replace("---\n"+body, "YYYY-MM-DD", "2026-07-21", 2)
	v1 = strings.Replace(v1, "ADR-NNNN", "ADR-0005", 1)
	v2 := strings.Replace(v1, adr.V1FormatMarker, adr.V2FormatMarker, 1)
	legacy := testsupport.ADR("Accepted", testsupport.WithDate("2026-07-21"), testsupport.WithTitle("0001: Legacy"))
	for _, tc := range []struct {
		name, file, doc string
		want            adr.Format
	}{
		{"markerless before former cutoffs", "0001-legacy.md", legacy, adr.Legacy},
		{"V1 after former cutoffs", "9999-v1.md", strings.Replace(v1, "ADR-0005", "ADR-9999", 1), adr.CurrentStateV1},
		{"V2 before former cutoffs", "0001-v2.md", strings.Replace(v2, "ADR-0005", "ADR-0001", 1), adr.CurrentStateV2},
		{"V3 after former cutoffs", "9999-v3.md", buildV3("9999", "v3"), adr.CurrentStateV3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := adr.ParseRecord(tc.file, []byte(tc.doc))
			if err != nil || a.Format != tc.want {
				t.Fatalf("record=%#v err=%v", a, err)
			}
		})
	}
	if pending, err := adr.ParseRecord("pending.md", []byte(pendingFixture("pending"))); err != nil || pending.Format != adr.CurrentFormat() {
		t.Fatalf("pending current format = %#v err=%v", pending, err)
	}
	for _, tc := range []struct{ name, doc string }{
		{"unknown", strings.Replace(v1, adr.V1FormatMarker, "current-state-v99", 1)},
		{"duplicate", strings.Replace(v1, "format: "+adr.V1FormatMarker, "format: "+adr.V1FormatMarker+"\nformat: "+adr.V1FormatMarker, 1)},
		{"malformed YAML", strings.Replace(v1, "format: "+adr.V1FormatMarker, "format: [", 1)},
		{"unterminated frontmatter", "---\nformat: current-state-v1\nstatus: Proposed\n"},
		{"non-mapping frontmatter", "---\nscalar\n---\n# ADR-0001: Invalid\n"},
		{"non-scalar marker", strings.Replace(v1, "format: "+adr.V1FormatMarker, "format: []", 1)},
		{"empty", strings.Replace(v1, "format: "+adr.V1FormatMarker, "format: ", 1)},
		{"strict V3 frontmatter", strings.Replace(buildV3("0001", "invalid"), "status: Proposed", "extra: nope\nstatus: Proposed", 1)},
		{"invalid legacy frontmatter value", "---\nstatus: Accepted\nrelated: nope\n---\n# ADR-0001: Invalid\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := adr.ParseRecord("0001-invalid.md", []byte(tc.doc)); err == nil {
				t.Fatal("expected routing refusal")
			}
		})
	}
	for _, tc := range []struct{ name, doc string }{
		{"markerless", legacy},
		{"V1", v1},
		{"V2", v2},
	} {
		t.Run("pending "+tc.name, func(t *testing.T) {
			if _, err := adr.ParseRecord("pending.md", []byte(tc.doc)); err == nil {
				t.Fatal("expected pending refusal")
			}
		})
	}

	dir := t.TempDir()
	writeTemplateFixture(t, dir)
	swapNow(t, fixedNow)
	for _, tc := range []struct {
		name     string
		scaffold func(string, string) (string, error)
	}{
		{"numbered", adr.NewFile},
		{"pending", adr.NewPendingFile},
	} {
		t.Run("registry scaffold "+tc.name, func(t *testing.T) {
			path, err := tc.scaffold(dir, "Registry "+tc.name)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), "format: "+adr.CurrentFormatMarker()) {
				t.Fatalf("scaffold marker:\n%s", data)
			}
			record, err := adr.ParseRecord(filepath.Base(path), data)
			if err != nil || record.Format != adr.CurrentFormat() {
				t.Fatalf("scaffold record = %#v err=%v", record, err)
			}
		})
	}
}

func TestFormatAtGeneration(t *testing.T) {
	cases := []struct {
		generation int
		want       adr.Format
		ok         bool
	}{
		{13, adr.Legacy, false},
		{14, adr.CurrentStateV1, true},
		{15, adr.CurrentStateV2, true},
		{28, adr.CurrentStateV2, true},
		{29, adr.CurrentStateV3, true},
		{31, adr.CurrentStateV3, true},
	}
	for _, tc := range cases {
		got, ok := adr.FormatAtGeneration(tc.generation)
		if got != tc.want || ok != tc.ok {
			t.Errorf("FormatAtGeneration(%d) = %v, %t; want %v, %t", tc.generation, got, ok, tc.want, tc.ok)
		}
	}
}

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
	v2 := func(status, decision string) adr.ADR {
		return adr.ADR{Format: adr.CurrentStateV2, Status: status, Sections: map[string]string{"Decision": decision}}
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
		{"V2 Proposed rewrite", v2("Proposed", "old"), v2("Proposed", "new"), true},
		{"V2 Accepted rewrite", v2("Accepted", "old"), v2("Accepted", "new"), true},
		{"V2 Implementing rewrite", v2("Implementing", "old"), v2("Implementing", "new"), true},
		{"V2 Implemented unchanged", v2("Implemented", "same"), v2("Implemented", "same"), true},
		{"V2 Implemented rewrite", v2("Implemented", "old"), v2("Implemented", "new"), false},
		{"V2 Abandoned unchanged", v2("Abandoned", "same"), v2("Abandoned", "same"), true},
		{"V2 Abandoned rewrite", v2("Abandoned", "old"), v2("Abandoned", "new"), false},
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
		{"implemented with ops", build("Implemented", "2026-07-22", oneDecision, opsChanges,
			"- 2026-07-20: Proposed\n- 2026-07-22: Implemented; content-sha256: "+opsDigest), "Implemented", 2, false},
		{"accepted then implemented", build("Implemented", "2026-07-23", oneDecision, opsChanges,
			"- 2026-07-20: Proposed\n- 2026-07-21: Accepted; content-sha256: "+opsDigest+"\n- 2026-07-23: Implemented; content-sha256: "+opsDigest), "Implemented", 2, false},
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
// invariant: adr-system/adr-lifecycle:decision-items-enumerable (TestParseV1Errors)
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
		{"accepted with rationale", build("Accepted", "2026-07-21", oneDecision, "None.", "- 2026-07-20: Proposed\n- 2026-07-21: Accepted; content-sha256: "+d+"; rationale: nope"), "carries a rationale it must not"},
		{"malformed metadata segment", build("Accepted", "2026-07-21", oneDecision, "None.", "- 2026-07-20: Proposed\n- 2026-07-21: Accepted;content-sha256: "+d), "malformed metadata segment"},
		{"first not proposed", build("Accepted", "2026-07-21", oneDecision, "None.", "- 2026-07-21: Accepted; content-sha256: "+d), "must be the `- <date>: Proposed` scaffold"},
		{"illegal transition", build("Accepted", "2026-07-21", oneDecision, "None.", "- 2026-07-20: Proposed\n- 2026-07-20: Proposed\n- 2026-07-21: Accepted; content-sha256: "+d), "illegal Status history transition"},
		{"descending dates", build("Accepted", "2026-07-19", oneDecision, "None.", "- 2026-07-20: Proposed\n- 2026-07-19: Accepted; content-sha256: "+d), "must not descend"},
		{"implemented with rationale", build("Implemented", "2026-07-21", oneDecision, "None.", "- 2026-07-20: Proposed\n- 2026-07-21: Implemented; content-sha256: "+d+"; rationale: x"), "must not carry a rationale"},
		{"abandoned missing rationale", build("Abandoned", "2026-07-21", oneDecision, "None.", "- 2026-07-20: Proposed\n- 2026-07-21: Abandoned; content-sha256: "+d), "must end with a nonempty rationale"},
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

// invariant: adr-system/adr-lifecycle:adr-status-enum-and-matrix (TestParseV2LifecycleAndApplications)
func TestParseV2LifecycleAndApplications(t *testing.T) {
	changes := "- add `a/b:first`\n- update `a/b:second`\n- remove `a/b:third`"
	digest := v2DigestFor(t, changes)
	p := "- 2026-07-20: Proposed"
	a := "- 2026-07-21: Accepted; content-sha256: " + digest
	i := "- 2026-07-22: Implementing; content-sha256: " + digest
	first := "- 2026-07-22: Applied; operations: add `a/b:first`"
	middle := "- 2026-07-23: Applied; operations: update `a/b:second`"
	final := "- 2026-07-24: Applied; operations: remove `a/b:third`"
	implemented := "- 2026-07-24: Implemented; content-sha256: " + digest
	abandoned := "- 2026-07-24: Abandoned; content-sha256: " + digest + "; rationale: stopped; safely"
	cases := []struct {
		name, status, history string
		wantEvents            int
	}{
		{"proposed", "Proposed", p, 1},
		{"proposed accepted", "Accepted", p + "\n" + a, 2},
		{"whitespace-only blank line", "Accepted", p + "\n \t \n" + a, 2},
		{"proposed direct implemented", "Implemented", p + "\n- 2026-07-22: Implemented; content-sha256: " + digest, 2},
		{"proposed abandoned", "Abandoned", p + "\n" + abandoned, 2},
		{"proposed implementing first", "Implementing", p + "\n" + i + "\n" + first, 3},
		{"accepted implementing middle", "Implementing", p + "\n" + a + "\n" + i + "\n" + first + "\n" + middle, 5},
		{"accepted direct implemented", "Implemented", p + "\n" + a + "\n- 2026-07-22: Implemented; content-sha256: " + digest, 3},
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

// TestParseV2AppliedLegacySequenceTolerated proves a stale Applied line
// carrying a retired state-sequence segment still parses: the segment is
// tolerated and discarded, recorded only as LegacySequence, with no other
// effect on the parsed event (ADR-0191).
func TestParseV2AppliedLegacySequenceTolerated(t *testing.T) {
	changes := "- add `a/b:first`\n- update `a/b:second`"
	digest := v2DigestFor(t, changes)
	history := "- 2026-07-20: Proposed" +
		"\n- 2026-07-21: Implementing; content-sha256: " + digest +
		"\n- 2026-07-21: Applied; state-sequence: 1; operations: add `a/b:first`"
	record, err := adr.ParseV2("0137-test.md", []byte(buildV2("Implementing", changes, history)))
	if err != nil {
		t.Fatalf("ParseV2: %v", err)
	}
	applied := record.History[2]
	wantOp := adr.Operation{Verb: adr.OpAdd, ID: "a/b:first", Slug: "first"}
	if applied.Kind != adr.HistoryApplied || applied.Date != "2026-07-21" || !applied.LegacySequence ||
		len(applied.Operations) != 1 || applied.Operations[0] != wantOp {
		t.Fatalf("legacy Applied event = %#v", applied)
	}
}

// invariant: adr-system/adr-lifecycle:applied-history-events-append-only (TestParseV2RejectsInvalidHistory)
func TestParseV2RejectsInvalidHistory(t *testing.T) {
	changes := "- add `a/b:first`\n- update `a/b:second`"
	digest := v2DigestFor(t, changes)
	p := "- 2026-07-20: Proposed"
	i := "- 2026-07-21: Implementing; content-sha256: " + digest
	first := "- 2026-07-21: Applied; operations: add `a/b:first`"
	cases := []struct{ name, status, changes, history, want string }{
		{"v1 excludes implementing", "Implementing", changes, p + "\n" + i, ""},
		{"first not proposed", "Accepted", changes, "- 2026-07-20: Accepted; content-sha256: " + digest, "first Status history"},
		{"repeated proposed", "Proposed", changes, p + "\n" + p, "illegal Status history transition"},
		{"implementing rationale", "Implementing", changes, p + "\n- 2026-07-21: Implementing; content-sha256: " + digest + "; rationale: no\n" + first, "carries a rationale it must not"},
		{"implemented rationale", "Implemented", changes, p + "\n- 2026-07-21: Implemented; content-sha256: " + digest + "; rationale: no", "must not carry a rationale"},
		{"abandoned missing rationale", "Abandoned", changes, p + "\n- 2026-07-21: Abandoned; content-sha256: " + digest, "nonempty rationale"},
		{"implementing none", "Implementing", "None.", p + "\n" + i + "\n" + first, "not declared"},
		{"implementing one op", "Implementing", "- add `a/b:first`", p + "\n- 2026-07-21: Implementing; content-sha256: " + v2DigestFor(t, "- add `a/b:first`") + "\n" + first, "at least two"},
		{"missing first application", "Implementing", changes, p + "\n" + i, "followed by"},
		{"all applied while implementing", "Implementing", changes, p + "\n" + i + "\n- 2026-07-21: Applied; operations: add `a/b:first`, update `a/b:second`", "one remaining"},
		{"applied before implementing", "Proposed", changes, p + "\n" + first, "only while Implementing"},
		{"undeclared verb", "Implementing", changes, p + "\n" + i + "\n- 2026-07-21: Applied; operations: remove `a/b:first`", "not declared"},
		{"undeclared id", "Implementing", changes, p + "\n" + i + "\n- 2026-07-21: Applied; operations: add `a/b:other`", "not declared"},
		{"duplicate in batch", "Implementing", changes, p + "\n" + i + "\n- 2026-07-21: Applied; operations: add `a/b:first`, add `a/b:first`", "duplicated"},
		{"duplicate across batches", "Implementing", changes, p + "\n" + i + "\n" + first + "\n- 2026-07-22: Applied; operations: add `a/b:first`", "already applied"},
		{"declaration order", "Implementing", "- add `a/b:first`\n- update `a/b:second`\n- remove `a/b:third`", p + "\n- 2026-07-21: Implementing; content-sha256: " + v2DigestFor(t, "- add `a/b:first`\n- update `a/b:second`\n- remove `a/b:third`") + "\n- 2026-07-21: Applied; operations: update `a/b:second`, add `a/b:first`", "declaration order"},
		{"bad separator", "Implementing", changes, p + "\n" + i + "\n- 2026-07-21: Applied; operations: add `a/b:first`,update `a/b:second`", "malformed Applied operation"},
		{"bad code span", "Implementing", changes, p + "\n" + i + "\n- 2026-07-21: Applied; operations: add a/b:first", "malformed Applied operation"},
		{"bad id", "Implementing", changes, p + "\n" + i + "\n- 2026-07-21: Applied; operations: add `A/b:first`", "malformed Applied operation"},
		{"zero sequence", "Implementing", changes, p + "\n" + i + "\n- 2026-07-21: Applied; state-sequence: 0; operations: add `a/b:first`", "malformed Status history"},
		{"metadata order", "Implementing", changes, p + "\n" + i + "\n- 2026-07-21: Applied; operations: add `a/b:first`; state-sequence: 1", "malformed Applied operation"},
		{"empty application", "Implementing", changes, p + "\n" + i + "\n- 2026-07-21: Applied; operations: ", "malformed Status history"},
		{"leading status whitespace", "Proposed", changes, " " + p, "malformed Status history"},
		{"trailing status whitespace", "Proposed", changes, p + " ", "malformed Status history"},
		{"leading applied whitespace", "Implementing", changes, p + "\n" + i + "\n " + first, "malformed Status history"},
		{"trailing applied whitespace", "Implementing", changes, p + "\n" + i + "\n" + first + " ", "malformed Applied operation"},
		{"missing final application", "Implemented", changes, p + "\n" + i + "\n" + first + "\n- 2026-07-22: Implemented; content-sha256: " + digest, "every declared"},
		{"incomplete implemented", "Implemented", "- add `a/b:first`\n- update `a/b:second`\n- remove `a/b:third`", p + "\n- 2026-07-21: Implementing; content-sha256: " + v2DigestFor(t, "- add `a/b:first`\n- update `a/b:second`\n- remove `a/b:third`") + "\n" + first + "\n- 2026-07-22: Applied; operations: update `a/b:second`\n- 2026-07-22: Implemented; content-sha256: " + v2DigestFor(t, "- add `a/b:first`\n- update `a/b:second`\n- remove `a/b:third`"), "every declared"},
		{"fully applied abandoned", "Abandoned", changes, p + "\n" + i + "\n" + first + "\n- 2026-07-22: Applied; operations: update `a/b:second`\n- 2026-07-23: Abandoned; content-sha256: " + digest + "; rationale: stopped", "canceled"},
		{"descending applied date", "Implementing", changes, p + "\n" + i + "\n- 2026-07-19: Applied; operations: add `a/b:first`", "must not descend"},
		{"digest mismatch", "Accepted", changes, p + "\n- 2026-07-21: Accepted; content-sha256: " + strings.Repeat("a", 64), "does not match"},
		{"implemented breaks accepted stamp chain", "Implemented", changes, p + "\n- 2026-07-21: Accepted; content-sha256: " + strings.Repeat("a", 64) + "\n- 2026-07-22: Implemented; content-sha256: " + digest, "does not repeat the preceding stamp"},
		{"implemented breaks implementing stamp chain", "Implemented", changes, p + "\n- 2026-07-21: Implementing; content-sha256: " + strings.Repeat("a", 64) + "\n" + first + "\n- 2026-07-22: Applied; operations: update `a/b:second`\n- 2026-07-22: Implemented; content-sha256: " + digest, "does not repeat the preceding stamp"},
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

// TestParseV2StampChain covers the Amended event grammar and the digest stamp
// chain: only an Amended event introduces a new digest, a status event repeats
// the preceding stamp or establishes the first, and the latest stamp must equal
// the computed content digest (ADR-0188).
// invariant: adr-system/adr-lifecycle:adr-status-enum-and-matrix (TestParseV2StampChain)
// invariant: adr-system/adr-lifecycle:adr-amendable-until-terminal (TestParseV2StampChain)
func TestParseV2StampChain(t *testing.T) {
	changes := "- add `a/b:first`\n- update `a/b:second`"
	wide := "- add `a/b:first`\n- update `a/b:second`\n- remove `a/b:third`"
	d := v2DigestFor(t, changes)
	dWide := v2DigestFor(t, wide)
	old := strings.Repeat("b", 64)
	other := strings.Repeat("c", 64)
	p := "- 2026-07-20: Proposed"
	amend := func(date, digest string) string { return "- " + date + ": Amended; content-sha256: " + digest }
	cases := []struct{ name, status, changes, history, want string }{
		{"amended while accepted", "Accepted", changes,
			p + "\n- 2026-07-21: Accepted; content-sha256: " + old + "\n" + amend("2026-07-22", d), ""},
		{"amended while implementing", "Implementing", changes,
			p + "\n- 2026-07-21: Implementing; content-sha256: " + old + "\n- 2026-07-21: Applied; operations: add `a/b:first`\n" + amend("2026-07-22", d), ""},
		{"amended between batches", "Implementing", wide,
			p + "\n- 2026-07-21: Implementing; content-sha256: " + old + "\n- 2026-07-21: Applied; operations: add `a/b:first`\n" + amend("2026-07-22", dWide) + "\n- 2026-07-23: Applied; operations: update `a/b:second`", ""},
		{"amended then implemented", "Implemented", changes,
			p + "\n- 2026-07-21: Accepted; content-sha256: " + old + "\n" + amend("2026-07-22", d) + "\n- 2026-07-23: Implemented; content-sha256: " + d, ""},
		{"two amendments", "Accepted", changes,
			p + "\n- 2026-07-21: Accepted; content-sha256: " + old + "\n" + amend("2026-07-22", other) + "\n" + amend("2026-07-23", d), ""},
		{"malformed amended digest", "Accepted", changes,
			p + "\n- 2026-07-21: Accepted; content-sha256: " + d + "\n- 2026-07-22: Amended; content-sha256: zzz", "malformed Status history"},
		{"amended while proposed", "Proposed", changes,
			p + "\n" + amend("2026-07-21", d), "only while Accepted or Implementing"},
		{"amended after terminal", "Implemented", changes,
			p + "\n- 2026-07-21: Implemented; content-sha256: " + d + "\n" + amend("2026-07-22", old), "only while Accepted or Implementing"},
		{"amended repeats stamp", "Accepted", changes,
			p + "\n- 2026-07-21: Accepted; content-sha256: " + d + "\n" + amend("2026-07-22", d), "digest different from the preceding stamp"},
		{"amended before first applied", "Implementing", changes,
			p + "\n- 2026-07-21: Implementing; content-sha256: " + old + "\n" + amend("2026-07-22", d) + "\n- 2026-07-23: Applied; operations: add `a/b:first`", "followed by the first Applied"},
		{"amended before explicit implemented", "Implemented", changes,
			p + "\n- 2026-07-21: Implementing; content-sha256: " + old + "\n- 2026-07-21: Applied; operations: add `a/b:first`\n- 2026-07-22: Applied; operations: update `a/b:second`\n" + amend("2026-07-23", d) + "\n- 2026-07-23: Implemented; content-sha256: " + d, "final Applied event immediately before it"},
		{"latest stamp mismatch", "Accepted", changes,
			p + "\n- 2026-07-21: Accepted; content-sha256: " + old + "\n" + amend("2026-07-22", other), "does not match the computed digest"},
		{"status event missing stamp", "Accepted", changes,
			p + "\n- 2026-07-21: Accepted", "must carry a content-sha256"},
		{"status event introduces a digest", "Implemented", changes,
			p + "\n- 2026-07-21: Accepted; content-sha256: " + old + "\n- 2026-07-22: Implemented; content-sha256: " + d, "does not repeat the preceding stamp"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := adr.ParseV2("0137-test.md", []byte(buildV2(tc.status, tc.changes, tc.history)))
			if tc.want == "" {
				if err != nil {
					t.Fatalf("ParseV2: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want containing %q", err, tc.want)
			}
		})
	}
}

// invariant: adr-system/adr-lifecycle:adr-status-enum-and-matrix (TestFormatSpecificTransitionMatrices)
func TestFormatSpecificTransitionMatrices(t *testing.T) {
	statuses := []string{"Proposed", "Accepted", "Implementing", "Implemented", "Abandoned"}
	for _, tc := range []struct {
		name   string
		format adr.Format
		legal  map[string]bool
	}{
		{"v1", adr.CurrentStateV1, map[string]bool{
			"Proposed>Accepted": true, "Proposed>Implemented": true, "Proposed>Abandoned": true,
			"Accepted>Implemented": true, "Accepted>Abandoned": true,
		}},
		{"v2", adr.CurrentStateV2, map[string]bool{
			"Proposed>Accepted": true, "Proposed>Implementing": true, "Proposed>Implemented": true, "Proposed>Abandoned": true,
			"Accepted>Implementing": true, "Accepted>Implemented": true, "Accepted>Abandoned": true,
			"Implementing>Implemented": true, "Implementing>Abandoned": true,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, from := range statuses {
				for _, to := range statuses {
					key := from + ">" + to
					if got := adr.TransitionLegal(from, to, tc.format); got != tc.legal[key] {
						t.Errorf("%s %s = %v, want %v", tc.name, key, got, tc.legal[key])
					}
				}
			}
		})
	}
}

// invariant: adr-system/adr-lifecycle:applied-history-events-append-only (TestV2HistoryTransitionPrefixAndShapes)
func TestV2HistoryTransitionPrefixAndShapes(t *testing.T) {
	status := func(value string) adr.HistoryEvent { return adr.HistoryEvent{Kind: adr.HistoryStatus, Status: value} }
	applied := adr.HistoryEvent{Kind: adr.HistoryApplied, Operations: []adr.Operation{{Verb: adr.OpAdd, ID: "a/b:c", Slug: "c"}}}
	appliedNext := adr.HistoryEvent{Kind: adr.HistoryApplied, Operations: []adr.Operation{{Verb: adr.OpUpdate, ID: "a/b:d", Slug: "d"}}}
	amended := adr.HistoryEvent{Kind: adr.HistoryAmended, Digest: "new-digest"}
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
		{"middle batch", record("Implementing", p, i, applied), record("Implementing", p, i, applied, appliedNext), true},
		{"finish", record("Implementing", p, i, applied), record("Implemented", p, i, applied, appliedNext, done), true},
		{"direct implementation with crossed applied shape", record("Accepted", p, accepted), record("Implemented", p, accepted, applied, done), false},
		{"implementing finish with crossed terminal-only shape", record("Implementing", p, i, applied), record("Implemented", p, i, applied, done), false},
		{"abandon", record("Implementing", p, i, applied), record("Abandoned", p, i, applied, abandoned), true},
		{"prefix deletion", record("Implementing", p, i, applied), record("Implemented", p, i, done), false},
		{"prefix mutation", record("Implementing", p, i, applied), record("Abandoned", status("Accepted"), i, applied, abandoned), false},
		{"same non-implementing append", record("Accepted", p, accepted), record("Accepted", p, accepted, applied), false},
		{"illegal status edge", record("Accepted", p, accepted), record("Proposed", p, accepted, p), false},
		{"amend while accepted", record("Accepted", p, accepted), record("Accepted", p, accepted, amended), true},
		{"amend while implementing", record("Implementing", p, i, applied), record("Implementing", p, i, applied, amended), true},
		{"amend plus second event", record("Implementing", p, i, applied), record("Implementing", p, i, applied, amended, appliedNext), false},
		{"amend while proposed", record("Proposed", p), record("Proposed", p, amended), false},
		{"amend after implemented", record("Implemented", p, accepted, done), record("Implemented", p, accepted, done, amended), false},
		{"amend after abandoned", record("Abandoned", p, accepted, abandoned), record("Abandoned", p, accepted, abandoned, amended), false},
		{"amend riding a flip", record("Accepted", p, accepted), record("Implemented", p, accepted, amended, done), false},
		{"same-status extra status event", record("Accepted", p, accepted), record("Accepted", p, accepted, status("Accepted")), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := adr.HistoryTransitionValid(tc.before, tc.after); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
