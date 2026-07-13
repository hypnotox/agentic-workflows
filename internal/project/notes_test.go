package project

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// The completeness advisory keys on key presence, not value emptiness
// (ADR-0045 item 4 narrowed by ADR-0087): a present-but-empty or explicit-null
// key is an open to-do and notes; an absent key is the deliberate, deleted
// acknowledgement and stays silent — the standing-note regression this exists
// for.
// invariant: absent-var-acknowledged
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
		// Labels derive from the output path — a template-derived "skill _base"
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

// AdvisoryNotes' generateDomainDocs input is the only path that parses ADRs —
// RenderAll never does — so a malformed ADR under a declared domain must surface
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
	// invariant: stub-notes-path-keyed
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
		"docs/a.md has unauthored stub content — sections at stub default: setup, deps",
		"docs/b.md has unauthored stub content — sections at stub default: overview; stub-marked parts: terms",
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
	want := "docs/development.md has unauthored stub content — sections at stub default: setup, command-runner, dependencies"
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
	want = "docs/development.md has unauthored stub content — sections at stub default: command-runner, dependencies; stub-marked parts: setup"
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
	want := "docs/domains/config.md has unauthored stub content — sections at stub default: current-state"
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
			want := "part .awf/skills/parts/tdd/notes.md contains a marker-shaped line — section markers have no effect inside convention parts; fence the example to silence this note"
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
