package project

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// templateMarkers returns the awf:section marker names declared in a template
// source (template order).
func templateMarkers(t *testing.T, tid string) []string {
	t.Helper()
	src, err := fs.ReadFile(templates.FS, tid)
	if err != nil {
		t.Fatalf("read %s: %v", tid, err)
	}
	var markers []string
	for _, s := range render.ParseSections(string(src)) {
		if s.IsSection {
			markers = append(markers, s.Name)
		}
	}
	return markers
}

// assertSectionParity fails if the template's awf:section marker set differs
// from the catalog-declared section set (order-independent).
func assertSectionParity(t *testing.T, label, tid string, sections []string) {
	t.Helper()
	want := append([]string(nil), sections...)
	got := append([]string(nil), templateMarkers(t, tid)...)
	sort.Strings(want)
	sort.Strings(got)
	if strings.Join(want, ",") != strings.Join(got, ",") {
		t.Errorf("%s: section mismatch: catalog %v vs template markers %v", label, want, got)
	}
}

// TestSkillAndAgentSectionParity asserts that for every catalog skill and agent
// the set of awf:section markers in its template source equals its
// catalog-declared sections list. Without this guard a section-slug rename that
// updates the template but not the catalog Standard value (or vice versa) renders
// green with a blank-path provenance pointer that no other gate catches (ADR-0054).
//
// invariant: skill-section-parity
func TestSkillAndAgentSectionParity(t *testing.T) {
	cat := catalog.Standard
	for name, spec := range cat.Skills {
		assertSectionParity(t, "skill "+name, fmt.Sprintf("skills/%s/SKILL.md.tmpl", name), spec.Sections)
	}
	for name, spec := range cat.Agents {
		assertSectionParity(t, "agent "+name, fmt.Sprintf("agents/%s.md.tmpl", name), spec.Sections)
	}
}
