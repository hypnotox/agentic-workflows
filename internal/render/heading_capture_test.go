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
