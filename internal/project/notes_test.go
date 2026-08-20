package project

import (
	"fmt"
	"maps"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// The completeness advisory keys on key presence, not value emptiness
// (ADR-0045 item 4 narrowed by ADR-0087): a present-but-empty or explicit-null
// key is an open to-do and notes; an absent key is the deliberate, deleted
// acknowledgement and stays silent - the standing-note regression this exists
// for.
// invariant: rendering/inplace-and-placeholders:absent-var-acknowledged (TestUnsetVarNotesPresentKeySemantics)
func TestUnsetVarNotesPresentKeySemantics(t *testing.T) {
	for name, tc := range map[string]struct {
		yaml     string
		wantNote bool
	}{
		"present-empty": {"prefix: example\nprofile: full\nintegrationBranch: main\nvars: {testCmd: go test ./..., gateCmd: \"\"}\n", true},
		"present-null":  {"prefix: example\nprofile: full\nintegrationBranch: main\nvars: {testCmd: go test ./..., gateCmd: null}\n", true},
		"absent":        {"prefix: example\nprofile: full\nintegrationBranch: main\nvars: {testCmd: go test ./...}\n", false},
	} {
		t.Run(name, func(t *testing.T) {
			p, err := Open(testContext(t), scaffold(t, tc.yaml))
			if err != nil {
				t.Fatal(err)
			}
			notes, err := p.AdvisoryNotes(testContext(t))
			if err != nil {
				t.Fatal(err)
			}
			joined := strings.Join(notes, "\n")
			if got := strings.Contains(joined, "skill tdd references unset vars: gateCmd"); got != tc.wantNote {
				t.Errorf("gateCmd note presence = %v, want %v; notes: %q", got, tc.wantNote, joined)
			}
			if tc.wantNote && !strings.Contains(joined, "delete the key to accept the generic prose") {
				t.Errorf("note must advertise the deletion exit, got: %q", joined)
			}
			if strings.Contains(joined, "testCmd") {
				t.Errorf("testCmd is set and must not be reported: %q", joined)
			}
		})
	}
}

// Adapter duplicates collapse: with two targets the same skill renders twice
// under one template id and must produce a single note.
func TestUnsetVarNotesCollapsesAdapterDuplicates(t *testing.T) {
	p, err := Open(testContext(t), scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {gateCmd: \"\", testCmd: \"\"}\n"))
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.AdvisoryNotes(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, n := range notes {
		if strings.Contains(n, "skill tdd") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one tdd note across two targets, got %d: %v", count, notes)
	}
}

func TestUnsetVarNotesSurfacesRenderError(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {}\n",
		map[string]string{
			"skills/tdd.yaml": "data:\n  testSurfaces:\n    - {name: \"<no value>\", kind: k, location: l}\n",
		})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.AdvisoryNotes(testContext(t)); err == nil {
		t.Fatal("expected AdvisoryNotes to surface the render error")
	}
}

// AdvisoryNotes' generateDomainDocs input is the only path that parses ADRs -
// RenderAll never does - so a malformed ADR under a declared domain must surface
// as an error here rather than being swallowed.
func TestAdvisoryNotesSurfacesDomainDocError(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {}\ndomains: [config]\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-bad.md"),
		"---\nstatus: {bad\n---\n# ADR-0001: Bad\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.AdvisoryNotes(testContext(t)); err == nil {
		t.Fatal("expected AdvisoryNotes to surface the domain-doc generation error")
	}
}

// Stub notes are keyed by output path, so the same stub-marked part reports once
// per adapter target (inv: stub-notes-path-keyed).
func TestStubNotesPathKeyedAcrossTargets(t *testing.T) {
	root := scaffoldFiles(t,
		"prefix: example\nprofile: full\nintegrationBranch: main\nvars: {testCmd: go test ./..., gateCmd: make gate, gateCmdFull: make gate full}\n",
		map[string]string{
			"skills/parts/tdd/notes.md": "<!-- awf:stub -->\nstarter notes\n",
		})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.AdvisoryNotes(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	var stub []string
	for _, n := range notes {
		if strings.Contains(n, "stub-marked parts: notes") {
			stub = append(stub, n)
		}
	}
	// invariant: rendering/doc-outputs:stub-notes-path-keyed (TestStubNotesPathKeyedAcrossTargets)
	if len(stub) != 2 {
		t.Fatalf("expected one stub note per target path, got %d: %v", len(stub), notes)
	}
	joined := strings.Join(stub, "\n")
	if !strings.Contains(joined, ".claude/") || !strings.Contains(joined, ".pi/") {
		t.Errorf("stub notes must name both adapter paths: %q", joined)
	}
}

// Direct unit test of stubNotes over hand-built values: covers the defaults
// clause (unreachable via fixtures until the template sweep), the combined
// two-clause line format, and the no-stub silence.
func TestStubNotesDefaultsClauseUnit(t *testing.T) {
	notes := stubNotes([]RenderedFile{
		{Path: "docs/a.md", stubDefaults: []string{"setup", "deps"}},
		{Path: "docs/b.md", stubDefaults: []string{"overview"}, stubParts: []string{"terms"}},
		{Path: "docs/c.md"},
	})
	want := []string{
		"docs/a.md has unauthored stub content: sections at stub default: setup, deps",
		"docs/b.md has unauthored stub content: sections at stub default: overview; stub-marked parts: terms",
	}
	if len(notes) != len(want) {
		t.Fatalf("notes = %#v, want %#v", notes, want)
	}
	for i := range want {
		if notes[i] != want[i] {
			t.Errorf("note[%d] = %q, want %q", i, notes[i], want[i])
		}
	}
}

// A doc whose stub-attributed sections render their defaults reports one note
// line, sections in template order; a stub-marked part moves its section into
// the parts clause.
func TestStubNotesReportsDefaultsAndParts(t *testing.T) {
	cfg := "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {}\n"
	p, err := Open(testContext(t), scaffold(t, cfg))
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.AdvisoryNotes(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	want := "docs/development.md has unauthored stub content: sections at stub default: setup, command-runner, dependencies"
	if joined := strings.Join(notes, "\n"); !strings.Contains(joined, want) {
		t.Errorf("missing defaults note %q, got:\n%s", want, joined)
	}
	p2, err := Open(testContext(t), scaffoldFiles(t, cfg, map[string]string{
		"docs/parts/development/setup.md": "<!-- awf:stub -->\nstarter setup\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	notes, err = p2.AdvisoryNotes(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	want = "docs/development.md has unauthored stub content: sections at stub default: command-runner, dependencies; stub-marked parts: setup"
	if joined := strings.Join(notes, "\n"); !strings.Contains(joined, want) {
		t.Errorf("missing combined note %q, got:\n%s", want, joined)
	}
}

// Domain docs render outside RenderAll; their stub current-state default must
// still reach the advisory.
func TestStubNotesDomainDocs(t *testing.T) {
	p, err := Open(testContext(t), scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {}\ndomains: [config]\n"))
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.AdvisoryNotes(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	want := "docs/domains/config.md has unauthored stub content: sections at stub default: current-state"
	if joined := strings.Join(notes, "\n"); !strings.Contains(joined, want) {
		t.Errorf("missing domain-doc note %q, got:\n%s", want, joined)
	}
}

// The ADR-0083 part-marker advisory is part-keyed and deduplicated: a
// whole-line marker in a part consumed by two adapter targets notes exactly
// once, under the part path, with the fencing remedy in the note text.
func TestMarkerNotesPartKeyedAndDeduplicated(t *testing.T) {
	root := scaffoldFiles(t,
		"prefix: example\nprofile: full\nintegrationBranch: main\nvars: {testCmd: go test ./..., gateCmd: make gate, gateCmdFull: make gate full}\n",
		map[string]string{
			"skills/parts/tdd/notes.md": "some prose\n<!-- awf:section bogus -->\nmore prose\n",
		})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.AdvisoryNotes(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, n := range notes {
		if strings.Contains(n, "marker-shaped line") {
			count++
			want := "part .awf/skills/parts/tdd/notes.md contains a marker-shaped line: section markers have no effect inside convention parts; fence the example to silence this note"
			if n != want {
				t.Errorf("note = %q, want %q", n, want)
			}
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one part-keyed note across two targets, got %d: %v", count, notes)
	}
}

// Inline prose quoting the marker form and a fenced whole-line example must
// stay silent (inv: part-marker-advisory's negative cases).
func TestMarkerNotesInlineAndFencedSilent(t *testing.T) {
	root := scaffoldFiles(t, sampleYAML, map[string]string{
		"skills/parts/tdd/notes.md": "the `<!-- awf:section x -->` form opens a section\n```\n<!-- awf:section demo -->\nbody\n<!-- awf:end -->\n```\n",
	})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.AdvisoryNotes(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range notes {
		if strings.Contains(n, "marker-shaped line") {
			t.Errorf("inline quote / fenced example must not note: %q", n)
		}
	}
}

// Domain docs render outside RenderAll; a marker line in a domain part must
// still reach the advisory (ADR-0083 Decision 4).
func TestMarkerNotesDomainDocParts(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {}\ndomains: [config]\n",
		map[string]string{
			"domains/parts/config/current-state.md": "state prose\n<!-- awf:end -->\n",
		})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.AdvisoryNotes(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	want := "part .awf/domains/parts/config/current-state.md contains a marker-shaped line"
	if joined := strings.Join(notes, "\n"); !strings.Contains(joined, want) {
		t.Errorf("missing domain-part marker note %q, got:\n%s", want, joined)
	}
}

func TestUnsetVarNotesFullySetIsSilent(t *testing.T) {
	p, err := Open(testContext(t), scaffold(t, sampleYAML))
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.AdvisoryNotes(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range notes {
		if strings.Contains(n, "skill tdd") {
			t.Errorf("unexpected note for fully-set skill: %q", n)
		}
	}
}

// tagHealthNotes evaluates current pitfalls only. It emits a frequency note for
// a tag over the 25% share of tag-bearing pitfalls and a coverage note for an
// untagged pitfall; a 25%-share tag stays quiet and legacy ADRs do not affect
// either population.
func TestTagHealthNotes(t *testing.T) {
	root := scaffoldFiles(t, "prefix: awf\nintegrationBranch: main\ndomains: []\n"+
		"tags:\n  alpha: A\n  beta: B\n  gamma: C\n  delta: D\n  epsilon: E\n", map[string]string{
		"docs/pitfalls/a.md": pitfallSource("A", "tags: [alpha, beta, bogus]\n", "ok\n"),
		"docs/pitfalls/b.md": pitfallSource("B", "tags: [alpha, gamma, bogus]\n", "ok\n"),
		"docs/pitfalls/c.md": pitfallSource("C", "tags: [delta]\n", "ok\n"),
		"docs/pitfalls/d.md": pitfallSource("D", "tags: [epsilon]\n", "ok\n"),
		"docs/pitfalls/e.md": pitfallSource("E", "", "ok\n"),
		"docs/pitfalls/f.md": pitfallSource("F", "tags: [bogus]\n", "ok\n"),
	})
	writeADR(t, root, "0001-a.md", testsupport.ADR("Implemented", testsupport.WithTitle("0001: A"), testsupport.WithTags("alpha")))
	writeADR(t, root, "0002-b.md", testsupport.ADR("Implemented", testsupport.WithTitle("0002: B")))
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.tagHealthNotes()
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(notes, "\n")
	// invariant: config/configuration:tag-frequency-note (TestTagHealthNotes)
	if !strings.Contains(joined, `tag "alpha" is on 2/4`) {
		t.Errorf("expected an alpha frequency note over tagged pitfalls only; got %v", notes)
	}
	if strings.Contains(joined, `tag "beta"`) || strings.Contains(joined, `tag "delta"`) {
		t.Errorf("did not expect a note for a 25%%-share pitfall tag; got %v", notes)
	}
	if strings.Contains(joined, `tag "bogus"`) {
		t.Errorf("a non-vocabulary pitfall tag must not surface a frequency note; got %v", notes)
	}
	if strings.Contains(joined, "docs/pitfalls/f.md carries no tags") {
		t.Errorf("a bogus-only pitfall has tags, so no coverage note is expected; got %v", notes)
	}
	// invariant: config/configuration:tag-coverage-note (TestTagHealthNotes)
	if !strings.Contains(joined, ".awf/docs/pitfalls/e.md carries no tags") {
		t.Errorf("expected a coverage note for the untagged pitfall; got %v", notes)
	}
	if strings.Contains(joined, "docs/decisions/") {
		t.Errorf("legacy ADRs must not enter current tag-health notes; got %v", notes)
	}
}

func TestTagHealthNotesIgnoresAllADRs(t *testing.T) {
	root := scaffold(t, "prefix: awf\nintegrationBranch: main\ndomains: []\ntags:\n  tooling: Tooling\n")
	writeADR(t, root, "0001-legacy.md", testsupport.ADR("Implemented", testsupport.WithTitle("0001: Legacy")))
	governedBody := "status: Proposed\ndate: 2026-07-20\n---\n# ADR-%s: A\n\n## Context\n\nC.\n\n## Decision\n\n1. D.\n\n## State changes\n\nNone.\n\n## Consequences\n\nC.\n\n## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-07-20: Proposed\n"
	writeADR(t, root, "0002-a.md", "---\nformat: current-state-v1\n"+fmt.Sprintf(governedBody, "0002"))
	writeADR(t, root, "0003-b.md", "---\nformat: current-state-v2\n"+fmt.Sprintf(governedBody, "0003"))
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.tagHealthNotes()
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 0 {
		t.Fatalf("legacy and governed ADRs produced current tag-health notes: %v", notes)
	}
}

// An empty/absent vocabulary makes the whole tag-health producer inert - the
// empty-vocabulary safety case (a fixture carries free-form tags but no vocabulary).
func TestTagHealthNotesEmptyVocabInert(t *testing.T) {
	root := scaffold(t, "prefix: awf\nintegrationBranch: main\ndomains: []\n")
	writeADR(t, root, "0001-a.md", testsupport.ADR("Implemented", testsupport.WithTitle("0001: A")))
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.tagHealthNotes()
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 0 {
		t.Errorf("empty vocabulary must be inert; got %v", notes)
	}
}

// With a non-empty vocabulary but every pitfall untagged, coverage notes fire and
// the frequency computation is skipped (empty-denominator guard, no divide-by-zero).
func TestTagHealthNotesEmptyDenominator(t *testing.T) {
	root := scaffoldFiles(t, "prefix: awf\nintegrationBranch: main\ndomains: []\ntags:\n  alpha: A\n", map[string]string{
		"docs/pitfalls/a.md": pitfallSource("A", "", "ok\n"),
		"docs/pitfalls/b.md": pitfallSource("B", "", "ok\n"),
	})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.tagHealthNotes()
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(notes, "\n")
	if !strings.Contains(joined, "pitfalls/a.md carries no tags") || !strings.Contains(joined, "pitfalls/b.md carries no tags") {
		t.Errorf("expected coverage notes for both untagged pitfalls; got %v", notes)
	}
	for _, n := range notes {
		if strings.Contains(n, "coarsening") {
			t.Errorf("no frequency note expected with zero tagged pitfalls; got %v", notes)
		}
	}
}

// A malformed pitfall source surfaces from direct tag-health projection.
func TestTagHealthNotesPitfallError(t *testing.T) {
	root := scaffoldFiles(t, "prefix: awf\nintegrationBranch: main\ndomains: []\ntags:\n  alpha: A\n",
		map[string]string{"docs/pitfalls/bad.md": "---\ntitle: Bad\nunknown: value\n---\nbody\n"})
	writeADR(t, root, "0001-a.md", testsupport.ADR("Implemented", testsupport.WithTitle("0001: A"), testsupport.WithTags("alpha")))
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.tagHealthNotes(); err == nil {
		t.Fatal("expected pitfall corpus structural error, got nil")
	}
}

// tagHealthNotes counts pitfall tags without legacy ADR participation and flags
// an untagged pitfall.
func TestTagHealthNotesPitfalls(t *testing.T) {
	root := scaffoldFiles(t, "prefix: awf\nintegrationBranch: main\ndomains: []\ntags:\n  alpha: A\n",
		map[string]string{
			"docs/pitfalls/tagged.md":   pitfallSource("Tagged", "tags: [alpha]\n", "ok\n"),
			"docs/pitfalls/untagged.md": pitfallSource("Untagged", "", "ok\n"),
		})
	writeADR(t, root, "0001-a.md", testsupport.ADR("Implemented", testsupport.WithTitle("0001: A"), testsupport.WithTags("alpha")))
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.tagHealthNotes()
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(notes, "\n")
	// The legacy ADR is excluded, so alpha is on the one tagged pitfall only.
	if !strings.Contains(joined, `tag "alpha" is on 1/1`) {
		t.Errorf("expected alpha frequency note counting pitfalls only; got %v", notes)
	}
	if !strings.Contains(joined, ".awf/docs/pitfalls/untagged.md carries no tags") {
		t.Errorf("expected coverage note for the untagged pitfall; got %v", notes)
	}
}

// AdvisoryNotes surfaces a tagHealthNotes fault: with no domains (generateDomainDocs
// parses no ADRs) but a non-empty vocabulary, a malformed ADR fails inside
// tagHealthNotes, exercising AdvisoryNotes' propagation of that error.
func TestAdvisoryNotesSurfacesTagHealthError(t *testing.T) {
	root := scaffold(t, "prefix: awf\nintegrationBranch: main\ndomains: []\ntags:\n  alpha: A\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-broken.md"),
		"---\nstatus: [unterminated\n---\n# ADR-0001: Broken\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.AdvisoryNotes(testContext(t)); err == nil {
		t.Fatal("expected AdvisoryNotes to surface the tag-health ADR parse error")
	}
}

// The terseness advisory notes one merged term per over-length meaning, naming
// the term and its length, and stays silent for a meaning at or under the
// threshold. It evaluates the merged set, so a shipped standard term is bound
// by the same guideline as an authored one.
// invariant: rendering/doc-outputs:glossary-terseness-advisory (TestGlossaryTersenessNotes)
func TestGlossaryTersenessNotes(t *testing.T) {
	long := strings.Repeat("x", glossaryMeaningMax+1)
	atLimit := strings.Repeat("y", glossaryMeaningMax)
	root := scaffoldFiles(t, "prefix: awf\nintegrationBranch: main\nvars: {}\n",
		map[string]string{"docs/glossary.yaml": "data:\n  terms:\n" +
			"    - term: bloated\n      meaning: \"" + long + "\"\n" +
			"    - term: exactly-at-limit\n      meaning: \"" + atLimit + "\"\n" +
			"    - term: terse\n      meaning: short and clear\n"})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.glossaryTersenessNotes()
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 {
		t.Fatalf("want exactly one note (only the over-length term), got %v", notes)
	}
	if !strings.Contains(notes[0], `"bloated"`) ||
		!strings.Contains(notes[0], fmt.Sprintf("%d characters", glossaryMeaningMax+1)) ||
		!strings.Contains(notes[0], glossarySidecarPath) {
		t.Errorf("note must name the sidecar, the term, and its length, got %q", notes[0])
	}
}

// The threshold counts runes, not bytes, so a meaning of accented letters that
// reads short is not reported merely for encoding wider. Accented letters are
// ordinary text, so an adopter can genuinely hit this.
func TestGlossaryTersenessNotesCountsRunesNotBytes(t *testing.T) {
	// 200 runes, 400 bytes: under the threshold read, over it counted as bytes.
	wide := strings.Repeat("é", 200)
	if len(wide) <= glossaryMeaningMax || utf8.RuneCountInString(wide) > glossaryMeaningMax {
		t.Fatalf("fixture must be over the threshold in bytes (%d) and under it in runes (%d)", len(wide), utf8.RuneCountInString(wide))
	}
	root := scaffoldFiles(t, "prefix: awf\nintegrationBranch: main\nvars: {}\n",
		map[string]string{"docs/glossary.yaml": "data:\n  terms:\n    - term: accented\n      meaning: \"" + wide + "\"\n"})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.glossaryTersenessNotes()
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 0 {
		t.Fatalf("a meaning under the threshold in runes must not be reported, got %v", notes)
	}
}

// The producer is inert when the glossary doc is disabled, mirroring the other
// doc-scoped families, so a project that renders no glossary is never nagged.
func TestGlossaryTersenessNotesDisabled(t *testing.T) {
	p, err := Open(testContext(t), scaffold(t, "prefix: awf\nintegrationBranch: main\nvars: {}\n"))
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.glossaryTersenessNotes()
	if err != nil || notes != nil {
		t.Errorf("disabled glossary must yield no notes, got %v / %v", notes, err)
	}
}

// The shipped layer is inside the threshold's reach: the producer wraps the
// sidecar in the catalog default before merging, so the standard vocabulary is
// bound by the same guideline as an authored term. The on-disk sidecar never
// carries standardTerms, so dropping that wrap would silently exempt the whole
// shipped layer.
func TestGlossaryTersenessNotesCoversShippedLayer(t *testing.T) {
	cfg := "prefix: awf\nintegrationBranch: main\nvars: {}\n"

	t.Run("the real shipped vocabulary is under the threshold", func(t *testing.T) {
		p, err := Open(testContext(t), scaffold(t, cfg))
		if err != nil {
			t.Fatal(err)
		}
		notes, err := p.glossaryTersenessNotes()
		if err != nil {
			t.Fatal(err)
		}
		if len(notes) != 0 {
			t.Fatalf("the shipped vocabulary must itself be under the threshold, got %v", notes)
		}
	})

	t.Run("an over-length shipped term is reported", func(t *testing.T) {
		p, err := Open(testContext(t), scaffold(t, cfg))
		if err != nil {
			t.Fatal(err)
		}
		// p.catalog()'s Docs map is this project's own clone, but the rest of this
		// Project reads it, so swap in a local copy rather than mutating the
		// project's catalog mid-test.
		e := p.catalog().Docs["glossary"]
		e.Data = map[string]any{"standardTerms": []any{
			map[string]any{"term": "shipped-bloat", "meaning": strings.Repeat("x", glossaryMeaningMax+1)},
		}}
		cp := *p.catalog()
		cp.Docs = maps.Clone(p.catalog().Docs)
		cp.Docs["glossary"] = e
		p.cat = catalog.NewView(&cp).Catalog()
		notes, err := p.glossaryTersenessNotes()
		if err != nil {
			t.Fatal(err)
		}
		if len(notes) != 1 || !strings.Contains(notes[0], "shipped-bloat") {
			t.Fatalf("the shipped layer must be inside the threshold's reach, got %v", notes)
		}
	})
}
