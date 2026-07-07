package project

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// Missing vars and empty-string vars are equivalent for the completeness
// advisory: both count as unset (ADR-0045 item 4).
func TestUnsetVarNotesEmptyAndMissingEquivalent(t *testing.T) {
	for name, yaml := range map[string]string{
		"missing": "prefix: example\nvars: {testCmd: go test ./...}\nskills: [tdd]\nagents: []\n",
		"empty":   "prefix: example\nvars: {testCmd: go test ./..., gateCmd: \"\"}\nskills: [tdd]\nagents: []\n",
	} {
		t.Run(name, func(t *testing.T) {
			p, err := Open(scaffold(t, yaml))
			if err != nil {
				t.Fatal(err)
			}
			notes, err := p.AdvisoryNotes()
			if err != nil {
				t.Fatal(err)
			}
			joined := strings.Join(notes, "\n")
			if !strings.Contains(joined, "skill tdd references unset vars: gateCmd") {
				t.Errorf("expected a gateCmd note, got: %q", joined)
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
	p, err := Open(scaffold(t, "prefix: example\nvars: {}\ntargets: [claude, cursor]\nskills: [tdd]\nagents: []\n"))
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
