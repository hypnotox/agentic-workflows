package project

import (
	"strings"
	"testing"
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
			notes, err := p.UnsetVarNotes()
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
	notes, err := p.UnsetVarNotes()
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
	if _, err := p.UnsetVarNotes(); err == nil {
		t.Fatal("expected UnsetVarNotes to surface the render error")
	}
}

func TestUnsetVarNotesFullySetIsSilent(t *testing.T) {
	p, err := Open(scaffold(t, sampleYAML))
	if err != nil {
		t.Fatal(err)
	}
	notes, err := p.UnsetVarNotes()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range notes {
		if strings.Contains(n, "skill tdd") {
			t.Errorf("unexpected note for fully-set skill: %q", n)
		}
	}
}
