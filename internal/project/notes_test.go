package project

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// The completeness advisory keys on key presence, not value emptiness
// (ADR-0045 item 4 narrowed by ADR-0087): a present-but-empty or explicit-null
// key is an open to-do and notes; an absent key is the deliberate, deleted
// acknowledgement and stays silent - the standing-note regression this exists
// for.
// invariant: rendering/inplace-and-placeholders:absent-var-acknowledged
func TestUnsetVarNotesPresentKeySemantics(t *testing.T) {
	for name, tc := range map[string]struct {
		yaml     string
		wantNote bool
	}{
		"present-empty": {"prefix: example\nvars: {testCmd: go test ./..., gateCmd: \"\"}\nskills: [tdd]\nagents: []\n", true},
		"present-null":  {"prefix: example\nvars: {testCmd: go test ./..., gateCmd: null}\nskills: [tdd]\nagents: []\n", true},
		"absent":        {"prefix: example\nvars: {testCmd: go test ./...}\nskills: [tdd]\nagents: []\n", false},
	} {
		t.Run(name, func(t *testing.T) {
			p, err := Open(scaffold(t, tc.yaml))
			if err != nil {
				t.Fatal(err)
			}
			notes, err := p.AdvisoryNotes()
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
	p, err := Open(scaffold(t, "prefix: example\nvars: {gateCmd: \"\", testCmd: \"\"}\ntargets: [claude, cursor]\nskills: [tdd]\nagents: []\n"))
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.AdvisoryNotes()
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

// Base-shared artifacts (project-local skills all render from one base
// template id) must each report their own unset vars: the collapse key is the
// note itself, not the template id, or the second local artifact is silently
// skipped.
func TestUnsetVarNotesBaseSharedArtifactsReportIndependently(t *testing.T) {
	p, err := Open(scaffold(t, "prefix: example\nvars: {alpha: \"\", beta: \"\", gamma: \"\"}\nskills: []\nagents: []\n"))
	if err != nil {
		t.Fatal(err)
	}
	files := []RenderedFile{
		{Path: ".claude/skills/example-a/SKILL.md", TemplateID: baseSkillTID, assembled: "{{ .vars.alpha }}"},
		{Path: ".claude/skills/example-b/SKILL.md", TemplateID: baseSkillTID, assembled: "{{ .vars.beta }}"},
		{Path: ".cursor/skills/example-b/SKILL.md", TemplateID: baseSkillTID, assembled: "{{ .vars.beta }}"}, // adapter duplicate
		{Path: ".claude/agents/reviewer.md", TemplateID: baseAgentTID, assembled: "{{ .vars.gamma }}"},
	}
	notes := p.unsetVarNotes(files)
	joined := strings.Join(notes, "\n")
	for _, want := range []string{
		// Labels derive from the output path - a template-derived "skill _base"
		// could not say which local artifact a note is about.
		"skill example-a references unset vars: alpha",
		"skill example-b references unset vars: beta",
		"agent reviewer references unset vars: gamma",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing note %q, got %v", want, notes)
		}
	}
	if len(notes) != 3 {
		t.Errorf("adapter duplicate must still collapse to one note, got %d: %v", len(notes), notes)
	}
}

func TestUnsetVarNotesSurfacesRenderError(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nvars: {}\nskills: [tdd]\nagents: []\n",
		map[string]string{
			"skills/tdd.yaml": "data:\n  testSurfaces:\n    - {name: \"<no value>\", kind: k, location: l}\n",
		})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.AdvisoryNotes(); err == nil {
		t.Fatal("expected AdvisoryNotes to surface the render error")
	}
}

// AdvisoryNotes' generateDomainDocs input is the only path that parses ADRs -
// RenderAll never does - so a malformed ADR under a declared domain must surface
// as an error here rather than being swallowed.
func TestAdvisoryNotesSurfacesDomainDocError(t *testing.T) {
	root := scaffold(t, "prefix: example\nvars: {}\nskills: []\nagents: []\ndomains: [config]\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-bad.md"),
		"---\nstatus: {bad\n---\n# ADR-0001: Bad\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.AdvisoryNotes(); err == nil {
		t.Fatal("expected AdvisoryNotes to surface the domain-doc generation error")
	}
}

// Stub notes are keyed by output path, so the same stub-marked part reports once
// per adapter target (inv: stub-notes-path-keyed).
func TestStubNotesPathKeyedAcrossTargets(t *testing.T) {
	root := scaffoldFiles(t,
		"prefix: example\nvars: {testCmd: go test ./..., gateCmd: make gate, gateCmdFull: make gate full}\ntargets: [claude, cursor]\nskills: [tdd]\nagents: []\n",
		map[string]string{
			"skills/parts/tdd/notes.md": "<!-- awf:stub -->\nstarter notes\n",
		})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.AdvisoryNotes()
	if err != nil {
		t.Fatal(err)
	}
	var stub []string
	for _, n := range notes {
		if strings.Contains(n, "stub-marked parts: notes") {
			stub = append(stub, n)
		}
	}
	// invariant: rendering/doc-outputs:stub-notes-path-keyed
	if len(stub) != 2 {
		t.Fatalf("expected one stub note per target path, got %d: %v", len(stub), notes)
	}
	joined := strings.Join(stub, "\n")
	if !strings.Contains(joined, ".claude/") || !strings.Contains(joined, ".cursor/") {
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
	cfg := "prefix: example\nvars: {}\nskills: []\nagents: []\ndocs: [development]\n"
	p, err := Open(scaffold(t, cfg))
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.AdvisoryNotes()
	if err != nil {
		t.Fatal(err)
	}
	want := "docs/development.md has unauthored stub content: sections at stub default: setup, command-runner, dependencies"
	if joined := strings.Join(notes, "\n"); !strings.Contains(joined, want) {
		t.Errorf("missing defaults note %q, got:\n%s", want, joined)
	}
	p2, err := Open(scaffoldFiles(t, cfg, map[string]string{
		"docs/parts/development/setup.md": "<!-- awf:stub -->\nstarter setup\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	notes, err = p2.AdvisoryNotes()
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
	p, err := Open(scaffold(t, "prefix: example\nvars: {}\nskills: []\nagents: []\ndomains: [config]\n"))
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.AdvisoryNotes()
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
		"prefix: example\nvars: {testCmd: go test ./..., gateCmd: make gate, gateCmdFull: make gate full}\ntargets: [claude, cursor]\nskills: [tdd]\nagents: []\n",
		map[string]string{
			"skills/parts/tdd/notes.md": "some prose\n<!-- awf:section bogus -->\nmore prose\n",
		})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.AdvisoryNotes()
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
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.AdvisoryNotes()
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
	root := scaffoldFiles(t, "prefix: example\nvars: {}\nskills: []\nagents: []\ndomains: [config]\n",
		map[string]string{
			"domains/parts/config/current-state.md": "state prose\n<!-- awf:end -->\n",
		})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.AdvisoryNotes()
	if err != nil {
		t.Fatal(err)
	}
	want := "part .awf/domains/parts/config/current-state.md contains a marker-shaped line"
	if joined := strings.Join(notes, "\n"); !strings.Contains(joined, want) {
		t.Errorf("missing domain-part marker note %q, got:\n%s", want, joined)
	}
}

func TestUnsetVarNotesFullySetIsSilent(t *testing.T) {
	p, err := Open(scaffold(t, sampleYAML))
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.AdvisoryNotes()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range notes {
		if strings.Contains(n, "skill tdd") {
			t.Errorf("unexpected note for fully-set skill: %q", n)
		}
	}
}

// tagHealthNotes emits a frequency note for a tag over the 25% share of
// tag-bearing artifacts and a coverage note for a zero-tag artifact; a 25%-share
// tag (not strictly over) stays quiet.
func TestTagHealthNotes(t *testing.T) {
	root := scaffold(t, "prefix: awf\nskills: []\nagents: []\ndocs: []\ndomains: []\n"+
		"tags:\n  alpha: A\n  beta: B\n  gamma: C\n  delta: D\n  epsilon: E\n")
	// 0001/0002 also carry `bogus`, a non-vocabulary tag: it is excluded from the
	// frequency accounting (only vocabulary members count) so it never surfaces a
	// coarsening note, even though it would exceed 25% if counted.
	writeADR(t, root, "0001-a.md", testsupport.ADR("Implemented", testsupport.WithTitle("0001: A"), testsupport.WithTags("alpha", "beta", "bogus")))
	writeADR(t, root, "0002-b.md", testsupport.ADR("Implemented", testsupport.WithTitle("0002: B"), testsupport.WithTags("alpha", "gamma", "bogus")))
	writeADR(t, root, "0003-c.md", testsupport.ADR("Implemented", testsupport.WithTitle("0003: C"), testsupport.WithTags("delta")))
	writeADR(t, root, "0004-d.md", testsupport.ADR("Implemented", testsupport.WithTitle("0004: D"), testsupport.WithTags("epsilon")))
	writeADR(t, root, "0005-e.md", testsupport.ADR("Implemented", testsupport.WithTitle("0005: E")))
	// 0006 carries only `bogus` (non-vocabulary): it has tags (no coverage note),
	// but contributes to neither the numerator nor the denominator - proving the
	// denominator counts only vocabulary-tag-bearing artifacts (alpha stays 2/4).
	writeADR(t, root, "0006-f.md", testsupport.ADR("Implemented", testsupport.WithTitle("0006: F"), testsupport.WithTags("bogus")))
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.tagHealthNotes()
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(notes, "\n")
	// invariant: config/configuration:tag-frequency-note
	if !strings.Contains(joined, `tag "alpha" is on 2/4`) {
		t.Errorf("expected an alpha frequency note; got %v", notes)
	}
	// beta/delta each 1/4 (25%, not strictly over) - no note.
	if strings.Contains(joined, `tag "beta"`) || strings.Contains(joined, `tag "delta"`) {
		t.Errorf("did not expect a note for a 25%%-share tag; got %v", notes)
	}
	// `bogus` (non-vocabulary) is not counted → no coarsening note, and 0006
	// (bogus-only) is neither a coverage note nor part of the denominator.
	if strings.Contains(joined, `tag "bogus"`) {
		t.Errorf("a non-vocabulary tag must not surface a frequency note; got %v", notes)
	}
	if strings.Contains(joined, "0006-f.md carries no tags") {
		t.Errorf("a bogus-only artifact has tags - no coverage note expected; got %v", notes)
	}
	// invariant: config/configuration:tag-coverage-note
	if !strings.Contains(joined, "0005-e.md carries no tags") {
		t.Errorf("expected a coverage note for the untagged ADR; got %v", notes)
	}
}

func TestTagHealthNotesSkipGovernedADRs(t *testing.T) {
	root := scaffold(t, "prefix: awf\nskills: []\nagents: []\ndocs: []\ndomains: []\ntags:\n  tooling: Tooling\n")
	governedBody := "status: Proposed\ndate: 2026-07-20\n---\n# ADR-%s: A\n\n## Context\n\nC.\n\n## Decision\n\n1. D.\n\n## State changes\n\nNone.\n\n## Consequences\n\nC.\n\n## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-07-20: Proposed\n"
	writeADR(t, root, "0001-a.md", "---\nformat: current-state-v1\n"+fmt.Sprintf(governedBody, "0001"))
	writeADR(t, root, "0002-b.md", "---\nformat: current-state-v2\n"+fmt.Sprintf(governedBody, "0002"))
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.tagHealthNotes()
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 0 {
		t.Fatalf("governed current-state ADRs produced legacy tag-health notes: %v", notes)
	}
}

// An empty/absent vocabulary makes the whole tag-health producer inert - the
// example-adopter safety case (sundial carries free-form tags but no vocabulary).
func TestTagHealthNotesEmptyVocabInert(t *testing.T) {
	root := scaffold(t, "prefix: awf\nskills: []\nagents: []\ndocs: []\ndomains: []\n")
	writeADR(t, root, "0001-a.md", testsupport.ADR("Implemented", testsupport.WithTitle("0001: A")))
	p, err := Open(root)
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

// With a non-empty vocabulary but every artifact untagged, coverage notes fire and
// the frequency computation is skipped (empty-denominator guard, no divide-by-zero).
func TestTagHealthNotesEmptyDenominator(t *testing.T) {
	root := scaffold(t, "prefix: awf\nskills: []\nagents: []\ndocs: []\ndomains: []\ntags:\n  alpha: A\n")
	writeADR(t, root, "0001-a.md", testsupport.ADR("Implemented", testsupport.WithTitle("0001: A")))
	writeADR(t, root, "0002-b.md", testsupport.ADR("Implemented", testsupport.WithTitle("0002: B")))
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.tagHealthNotes()
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(notes, "\n")
	if !strings.Contains(joined, "0001-a.md carries no tags") || !strings.Contains(joined, "0002-b.md carries no tags") {
		t.Errorf("expected coverage notes for both untagged ADRs; got %v", notes)
	}
	for _, n := range notes {
		if strings.Contains(n, "coarsening") {
			t.Errorf("no frequency note expected with zero tagged artifacts; got %v", notes)
		}
	}
}

// A malformed ADR surfaces as an error from tagHealthNotes' adr.ParseDir.
func TestTagHealthNotesADRParseError(t *testing.T) {
	root := scaffold(t, "prefix: awf\nskills: []\nagents: []\ndocs: []\ndomains: []\ntags:\n  alpha: A\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-broken.md"),
		"---\nstatus: [unterminated\n---\n# ADR-0001: Broken\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.tagHealthNotes(); err == nil {
		t.Fatal("expected adr.ParseDir error, got nil")
	}
}

// A malformed pitfalls sidecar surfaces as an error from tagHealthNotes'
// pitfallTagEntries (only reached once the vocabulary is non-empty and the ADRs parse).
func TestTagHealthNotesPitfallError(t *testing.T) {
	root := scaffoldFiles(t, "prefix: awf\nskills: []\nagents: []\ndocs: [pitfalls]\ndomains: []\ntags:\n  alpha: A\n",
		map[string]string{"docs/pitfalls.yaml": "data:\n  pitfalls: just a string\n"})
	writeADR(t, root, "0001-a.md", testsupport.ADR("Implemented", testsupport.WithTitle("0001: A"), testsupport.WithTags("alpha")))
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.tagHealthNotes(); err == nil {
		t.Fatal("expected pitfallTagEntries structural error, got nil")
	}
}

// tagHealthNotes counts pitfall tags alongside ADR tags and flags an untagged
// pitfall - exercising the pitfall arm of the artifact scan.
func TestTagHealthNotesPitfalls(t *testing.T) {
	root := scaffoldFiles(t, "prefix: awf\nskills: []\nagents: []\ndocs: [pitfalls]\ndomains: []\ntags:\n  alpha: A\n",
		map[string]string{"docs/pitfalls.yaml": "data:\n  pitfalls:\n" +
			"    - title: Tagged\n      tags: [alpha]\n      body: ok\n" +
			"    - title: Untagged\n      body: ok\n"})
	writeADR(t, root, "0001-a.md", testsupport.ADR("Implemented", testsupport.WithTitle("0001: A"), testsupport.WithTags("alpha")))
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.tagHealthNotes()
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(notes, "\n")
	// alpha on the tagged pitfall + the ADR = 2/2 tagged artifacts (>25%).
	if !strings.Contains(joined, `tag "alpha" is on 2/2`) {
		t.Errorf("expected alpha frequency note counting the pitfall; got %v", notes)
	}
	if !strings.Contains(joined, "Untagged carries no tags") {
		t.Errorf("expected coverage note for the untagged pitfall; got %v", notes)
	}
}

// AdvisoryNotes surfaces a tagHealthNotes fault: with no domains (generateDomainDocs
// parses no ADRs) but a non-empty vocabulary, a malformed ADR fails inside
// tagHealthNotes, exercising AdvisoryNotes' propagation of that error.
func TestAdvisoryNotesSurfacesTagHealthError(t *testing.T) {
	root := scaffold(t, "prefix: awf\nskills: []\nagents: []\ndocs: []\ndomains: []\ntags:\n  alpha: A\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-broken.md"),
		"---\nstatus: [unterminated\n---\n# ADR-0001: Broken\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.AdvisoryNotes(); err == nil {
		t.Fatal("expected AdvisoryNotes to surface the tag-health ADR parse error")
	}
}
