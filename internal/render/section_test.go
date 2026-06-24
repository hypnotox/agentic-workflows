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
