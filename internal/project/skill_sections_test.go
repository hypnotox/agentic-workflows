package project

import (
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/topic"
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
	for _, s := range parseSections(string(src)) {
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

// TestSkillSectionParity asserts that for every catalog skill the set of
// awf:section markers in its template source equals its catalog declaration.
//
// invariant: rendering/catalog-and-targets:skill-section-parity (TestSkillSectionParity)
func TestSkillSectionParity(t *testing.T) {
	cat := catalog.Standard
	for name, spec := range cat.Skills {
		assertSectionParity(t, "skill "+name, fmt.Sprintf("skills/%s/SKILL.md.tmpl", name), spec.Sections)
	}
}

// TestTopicsSkillExamplesMatchParserContract applies every claim example taught
// by the awf-topics skill to the real current-state parser.
func TestTopicsSkillExamplesMatchParserContract(t *testing.T) {
	src, err := fs.ReadFile(templates.FS, "skills/awf-topics/SKILL.md.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	blocks := regexp.MustCompile("(?s)```markdown\\n(.*?)\\n```").FindAllSubmatch(src, -1)
	if len(blocks) != 3 {
		t.Fatalf("awf-topics claim examples = %d, want exactly 3", len(blocks))
	}
	for i, block := range blocks {
		part := append([]byte("Current behavior.\n\n## Claims\n\n"), block[1]...)
		if _, err := topic.ParsePart(topic.TopicID{Domain: "contract", Slug: "examples"}, "current-state.md", part); err != nil {
			t.Errorf("claim example %d violates parser contract: %v\n%s", i+1, err, block[1])
		}
	}
}
