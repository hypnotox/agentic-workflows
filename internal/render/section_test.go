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

// invariant: rendering/render-engine:structural-heading-owned (TestStructuralHeadingPolicyAndAssembly)
func TestStructuralHeadingPolicyAndAssembly(t *testing.T) {
	src := "<!-- awf:section headed -->\n## Owned {{ .missing }}\nbody\n<!-- awf:end -->\n"
	markdown := ParseSections(src, true)
	if markdown[0].Heading != "## Owned {{ .missing }}" || markdown[0].Text != "body" {
		t.Fatalf("Markdown heading split = %#v", markdown[0])
	}
	nonMarkdown := ParseSections(src, false)
	if nonMarkdown[0].Heading != "" || nonMarkdown[0].Text != "## Owned {{ .missing }}\nbody" {
		t.Fatalf("non-Markdown must keep hash line as body: %#v", nonMarkdown[0])
	}
	asm, parts := Assemble(markdown, map[string]SectionPlan{"headed": {HasPart: true, PartBody: "part body", EditPath: ".awf/part.md"}}, HTMLComment)
	out, err := Execute(asm, map[string]any{"missing": ""}, parts, "heading")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "## Owned ") || !strings.Contains(out, "part body") || strings.Contains(out, "<no value>") {
		t.Fatalf("headed part output = %q", out)
	}
	inPlace, inPlaceParts := Assemble(markdown, map[string]SectionPlan{"headed": {InPlace: true, InPlaceFound: true, InPlaceBody: "preserved"}}, HTMLComment)
	inPlaceOut, err := Execute(inPlace, map[string]any{"missing": ""}, inPlaceParts, "heading")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inPlaceOut, "the heading immediately below is awf-owned") {
		t.Fatalf("headed in-place pointer = %q", inPlaceOut)
	}
	dropped, _ := Assemble(markdown, map[string]SectionPlan{"headed": {Drop: true}}, HTMLComment)
	if strings.Contains(dropped, "Owned") || strings.Contains(dropped, "body") || strings.Contains(dropped, "awf:edit") {
		t.Fatalf("drop must omit pointer, heading, and body: %q", dropped)
	}
}

// invariant: rendering/render-engine:section-edit-pointer (TestStructuralHeadingAssemblyOrdering)
// invariant: rendering/render-engine:section-default-splice (TestStructuralHeadingAssemblyOrdering)
// invariant: rendering/render-engine:structural-heading-owned (TestStructuralHeadingAssemblyOrdering)
func TestStructuralHeadingAssemblyOrdering(t *testing.T) {
	segs := ParseSections("<!-- awf:section s -->\n## Owned\nDEFAULT\n<!-- awf:end -->\n")
	cases := []struct {
		name string
		plan SectionPlan
		want string
	}{
		{"default", SectionPlan{EditPath: "part.md"}, "<!-- awf:edit s: default; create part.md to override -->\n## Owned\nDEFAULT\n"},
		{"part", SectionPlan{HasPart: true, PartBody: "PART", EditPath: "part.md"}, "<!-- awf:edit s: from part.md -->\n## Owned\nPART\n"},
		{"reinjection", SectionPlan{HasPart: true, PartBody: "before " + SectionDefaultSentinel + " after", EditPath: "part.md"}, "<!-- awf:edit s: from part.md -->\n## Owned\nbefore DEFAULT after\n"},
		{"in-place", SectionPlan{InPlace: true, InPlaceFound: true, InPlaceBody: "BODY", EditPath: "part.md"}, "<!-- awf:edit-in-place s: the heading immediately below is awf-owned; only the body after it is preserved across syncs -->\n## Owned\nBODY\n"},
		{"drop", SectionPlan{Drop: true}, "\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			asm, parts := Assemble(segs, map[string]SectionPlan{"s": tc.plan}, HTMLComment)
			got, err := Execute(asm, nil, parts, tc.name)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("ordering changed:\ngot  %q\nwant %q", got, tc.want)
			}
		})
	}
	stub := ParseSections("<!-- awf:section s stub -->\n## Owned\nPROMPT\n<!-- awf:end -->")
	asm, parts := Assemble(stub, map[string]SectionPlan{"s": {EditPath: "part.md"}}, HTMLComment)
	got, err := Execute(asm, nil, parts, "stub")
	if err != nil || got != "<!-- awf:edit s: stub; replace by creating part.md -->\n## Owned\nPROMPT" {
		t.Fatalf("headed stub = %q, %v", got, err)
	}
	headingless := ParseSections("<!-- awf:section s -->\nBODY\n<!-- awf:end -->")
	asm, parts = Assemble(headingless, map[string]SectionPlan{"s": {EditPath: "part.md"}}, HTMLComment)
	got, err = Execute(asm, nil, parts, "headingless")
	if err != nil || got != "<!-- awf:edit s: default; create part.md to override -->\nBODY" {
		t.Fatalf("headingless = %q, %v", got, err)
	}
}

func TestSourceSectionsAndAssemblyRetainLiteralAndDefaultSpans(t *testing.T) {
	src := SourceText{Root: "guide.md.tmpl", Spans: []SourceSpan{
		{Source: "guide.md.tmpl", Text: "before\n"},
		{Source: "partials/spine.md", Text: "included\n"},
		{Source: "guide.md.tmpl", Text: "<!-- awf:section body -->\nDEFAULT\n<!-- awf:end -->\nafter\n"},
	}}
	segs := ParseSourceSections(src)
	if got, want := segs[0].Source.Spans[1].Source, "partials/spine.md"; got != want {
		t.Fatalf("included literal source = %q, want %q", got, want)
	}
	if got, want := segs[1].SectionSource, "guide.md.tmpl"; got != want {
		t.Fatalf("section source = %q, want root %q", got, want)
	}
	assembled, parts := AssembleSourceWithTemplateSource(segs, map[string]SectionPlan{"body": {HasPart: true, PartBody: "PRE" + SectionDefaultSentinel + "POST"}}, HTMLComment, TemplateSource{})
	out, err := Execute(assembled.AuthoredText(), nil, parts, "source")
	if err != nil {
		t.Fatal(err)
	}
	if want := "before\nincluded\n<!-- awf:edit body: from  -->\nPREDEFAULTPOST\nafter\n"; out != want {
		t.Fatalf("assembled output = %q, want %q", out, want)
	}
	var defaultSpans []SourceSpan
	for _, span := range assembled.Spans {
		if span.Text == "DEFAULT" {
			defaultSpans = append(defaultSpans, span)
		}
	}
	if len(defaultSpans) != 1 || defaultSpans[0].Source != "guide.md.tmpl" {
		t.Fatalf("re-injected default spans = %#v", defaultSpans)
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

func TestParseSectionsInPlaceAttribute(t *testing.T) {
	inplace := ParseSections("<!-- awf:section a inplace -->\nbody\n<!-- awf:end -->\n")
	if len(inplace) < 1 || !inplace[0].IsSection {
		t.Fatalf("want a section segment first, got %#v", inplace)
	}
	if inplace[0].Name != "a" || !inplace[0].InPlace || inplace[0].Stub || inplace[0].Text != "body" {
		t.Errorf("inplace section = %#v", inplace[0])
	}
	// The attribute is exclusive: `stub` sets Stub only, `inplace` sets InPlace
	// only, and a bare marker sets neither.
	stub := ParseSections("<!-- awf:section a stub -->\nbody\n<!-- awf:end -->\n")
	if !stub[0].Stub || stub[0].InPlace {
		t.Errorf("stub section must parse InPlace=false: %#v", stub[0])
	}
	plain := ParseSections("<!-- awf:section a -->\nbody\n<!-- awf:end -->\n")
	if plain[0].Stub || plain[0].InPlace {
		t.Errorf("plain section must parse Stub=false, InPlace=false: %#v", plain[0])
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
		// invariant: rendering/render-engine:stub-part-verbatim (TestHasStubMarker)
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
		"other awf comment":     {"<!-- awf:edit x: default -->\n", false},
		"empty body":            {"", false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// invariant: rendering/render-engine:part-marker-advisory (TestHasMarkerLine)
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
	// invariant: rendering/render-engine:no-residual-section-marker (TestCheckResidualMarkersBareTokenLegal)
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
