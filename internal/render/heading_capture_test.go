package render

import "testing"

func TestExtractStructuralHeadingsRejectsIncompleteCapture(t *testing.T) {
	if _, err := ExtractStructuralHeadings("start", map[string][2]string{"body": {"start", "end"}}); err == nil {
		t.Fatal("incomplete capture must fail")
	}
	headings, err := ExtractStructuralHeadings("no tokens", map[string][2]string{"body": {"start", "end"}})
	if err != nil || len(headings) != 0 {
		t.Fatalf("absent conditional heading = %#v, %v", headings, err)
	}
}

func TestStructuralHeadingCapturePreservesSkeletonTemplateContext(t *testing.T) {
	segs := ParseSections("{{ $title := .title }}{{ if .enabled }}<!-- awf:section body inplace -->\n## {{ $title }}: {{ .nested.name }}\ndefault\n<!-- awf:end -->{{ end }}")
	skeleton, tokens := StructuralHeadingCapture(segs)
	output, err := Execute(skeleton, map[string]any{
		"title":   "Contextual",
		"enabled": true,
		"nested":  map[string]any{"name": "heading"},
	}, nil, "heading capture")
	if err != nil {
		t.Fatal(err)
	}
	headings, err := ExtractStructuralHeadings(output, tokens)
	if err != nil {
		t.Fatal(err)
	}
	if got := headings["body"]; got != "## Contextual: heading" {
		t.Fatalf("captured heading = %q", got)
	}
}

func TestStructuralHeadingCaptureRejectsTokenCollisionsWithoutTruncation(t *testing.T) {
	segs := ParseSections("surrounding={{ .surrounding }}\n<!-- awf:section body -->\n## {{ .heading }}\ndefault\n<!-- awf:end -->")
	skeleton, tokens := StructuralHeadingCapture(segs)
	oldStart := "\x00awf:heading:body:start\x00"
	oldEnd := "\x00awf:heading:body:end\x00"
	output, err := Execute(skeleton, map[string]any{
		"surrounding": oldStart,
		"heading":     "Safe " + oldEnd,
	}, nil, "heading capture")
	if err != nil {
		t.Fatal(err)
	}
	headings, err := ExtractStructuralHeadings(output, tokens)
	if err != nil {
		t.Fatal(err)
	}
	if got := headings["body"]; got != "## Safe "+oldEnd {
		t.Fatalf("noncolliding token-shaped data truncated heading to %q", got)
	}

	pair := tokens["body"]
	output, err = Execute(skeleton, map[string]any{
		"surrounding": pair[0],
		"heading":     "Safe " + pair[1],
	}, nil, "heading capture")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractStructuralHeadings(output, tokens); err == nil {
		t.Fatal("colliding token-shaped data must fail rather than misbind a capture")
	}
}

func TestExtractStructuralHeadingsRejectsOutOfOrderFraming(t *testing.T) {
	_, err := ExtractStructuralHeadings("end heading start", map[string][2]string{"body": {"start", "end"}})
	if err == nil {
		t.Fatal("out-of-order framing must fail")
	}
}
