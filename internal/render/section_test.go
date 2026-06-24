package render

import "testing"

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
