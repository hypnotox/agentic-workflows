package render

import (
	"strings"
	"testing"
)

func TestParseSectionsSplitsLiteralAndSections(t *testing.T) {
	src := "# Title\n\n<!-- awf:section surfaces -->\nbody one\n<!-- awf:end -->\n\ntail\n"
	segs := ParseSections(src)
	if len(segs) != 3 {
		t.Fatalf("want 3 segments, got %d: %#v", len(segs), segs)
	}
	if segs[0].IsSection || segs[0].Text != "# Title\n\n" {
		t.Errorf("seg0 literal = %q (section=%v)", segs[0].Text, segs[0].IsSection)
	}
	if !segs[1].IsSection || segs[1].Name != "surfaces" || segs[1].Text != "body one" {
		t.Errorf("seg1 = %#v", segs[1])
	}
	if segs[2].IsSection || segs[2].Text != "\n\ntail\n" {
		t.Errorf("seg2 literal = %q", segs[2].Text)
	}
}

func TestParseSectionsNoSections(t *testing.T) {
	segs := ParseSections("plain text\n")
	if len(segs) != 1 || segs[0].IsSection || segs[0].Text != "plain text\n" {
		t.Errorf("got %#v", segs)
	}
}

func TestParseSectionsEmptyBody(t *testing.T) {
	src := "a\n<!-- awf:section empty -->\n<!-- awf:end -->\nb\n"
	segs := ParseSections(src)
	if len(segs) != 3 {
		t.Fatalf("want 3 segments, got %d: %#v", len(segs), segs)
	}
	if !segs[1].IsSection || segs[1].Name != "empty" || segs[1].Text != "" {
		t.Errorf("seg1 = %#v", segs[1])
	}
}

func TestParseSectionsEmptyInput(t *testing.T) {
	segs := ParseSections("")
	if len(segs) != 1 || segs[0].IsSection || segs[0].Text != "" {
		t.Errorf("empty input should yield one empty literal segment, got %#v", segs)
	}
}

func TestParseSectionsMultiLineBody(t *testing.T) {
	src := "<!-- awf:section multi -->\nline one\nline two\n<!-- awf:end -->\n"
	segs := ParseSections(src)
	var sec *Segment
	for i := range segs {
		if segs[i].IsSection {
			sec = &segs[i]
		}
	}
	if sec == nil {
		t.Fatalf("no section segment found: %#v", segs)
	}
	if sec.Name != "multi" || sec.Text != "line one\nline two" {
		t.Errorf("section = %#v", *sec)
	}
}

func TestParseSectionsStubAttribute(t *testing.T) {
	segs := ParseSections("<!-- awf:section a stub -->\nbody\n<!-- awf:end -->\n")
	if len(segs) < 1 || !segs[0].IsSection {
		t.Fatalf("want a section segment first, got %#v", segs)
	}
	if segs[0].Name != "a" || !segs[0].Stub || segs[0].Text != "body" {
		t.Errorf("stub section = %#v", segs[0])
	}
	plain := ParseSections("<!-- awf:section a -->\nbody\n<!-- awf:end -->\n")
	if !plain[0].IsSection || plain[0].Stub {
		t.Errorf("plain section must parse with Stub=false: %#v", plain[0])
	}
}

func TestParseSectionsUnknownAttributeDoesNotParse(t *testing.T) {
	src := "<!-- awf:section a bogus -->\nbody\n<!-- awf:end -->\n"
	segs := ParseSections(src)
	for _, s := range segs {
		if s.IsSection {
			t.Fatalf("unknown attribute must not parse as a section: %#v", segs)
		}
	}
	err := CheckResidualMarkers(src)
	if err == nil {
		t.Fatal("expected residual-marker error for the malformed marker, got nil")
	}
	if !strings.Contains(err.Error(), "malformed awf:section/awf:end marker") {
		t.Errorf("error missing malformed-marker context: %q", err.Error())
	}
}

func TestHasStubMarker(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"exact line", "<!-- awf:stub -->\nprose\n", true},
		{"surrounding whitespace", "  <!-- awf:stub -->  \nprose\n", true},
		{"quoted inline", "see `<!-- awf:stub -->` for details\n", false},
		{"absent", "just prose\n", false},
	}
	for _, c := range cases {
		if got := HasStubMarker(c.body); got != c.want {
			t.Errorf("%s: HasStubMarker = %v, want %v", c.name, got, c.want)
		}
	}
}

// HasMarkerLine's prefix anchor (ADR-0083): every whole-line residue shape
// fires, every inline quote stays silent, and awf:stub is out of scope.
func TestHasMarkerLine(t *testing.T) {
	cases := map[string]struct {
		body string
		want bool
	}{
		"closed section marker": {"<!-- awf:section foo -->\n", true},
		"end marker":            {"prose\n<!-- awf:end -->\n", true},
		"unclosed opener":       {"<!-- awf:section foo\n", true},
		"trailing text":         {"<!-- awf:section foo --> trailing\n", true},
		"indented marker":       {"   <!--  awf:end -->\n", true},
		"inline quote":          {"the `<!-- awf:section -->` form opens a section\n", false},
		"bare token":            {"a bare `awf:section` mention\n", false},
		"stub marker":           {"<!-- awf:stub -->\n", false},
		"other awf comment":     {"<!-- awf:edit x — default -->\n", false},
		"empty body":            {"", false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := HasMarkerLine(c.body); got != c.want {
				t.Errorf("HasMarkerLine(%q) = %v, want %v", c.body, got, c.want)
			}
		})
	}
}

func TestCheckResidualMarkersBareTokenLegal(t *testing.T) {
	if err := CheckResidualMarkers("A managed doc is a sequence of `awf:section` blocks.\n"); err != nil {
		t.Errorf("bare backtick-quoted token must be legal, got %v", err)
	}
	if err := CheckResidualMarkers("text\n<!-- awf:end -->\n"); err == nil {
		t.Error("stray awf:end comment must be a residual-marker error")
	}
	if err := CheckResidualMarkers("text\n<!--  awf:section x -->\n"); err == nil {
		t.Error("whitespace-padded awf:section comment must be a residual-marker error")
	}
}

func TestParseSectionsAdjacentSections(t *testing.T) {
	src := "<!-- awf:section one -->\nbody one\n<!-- awf:end -->\n<!-- awf:section two -->\nbody two\n<!-- awf:end -->\n"
	segs := ParseSections(src)
	var sections []Segment
	for _, s := range segs {
		if s.IsSection {
			sections = append(sections, s)
		} else if s.Text == "" {
			t.Errorf("unexpected empty literal segment between sections: %#v", segs)
		}
	}
	if len(sections) != 2 {
		t.Fatalf("want 2 section segments, got %d: %#v", len(sections), segs)
	}
	if sections[0].Name != "one" || sections[0].Text != "body one" {
		t.Errorf("section0 = %#v", sections[0])
	}
	if sections[1].Name != "two" || sections[1].Text != "body two" {
		t.Errorf("section1 = %#v", sections[1])
	}
}
