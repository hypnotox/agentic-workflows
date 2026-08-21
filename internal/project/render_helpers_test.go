package project

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/render"
)

// renderedByPath returns the content of the RenderAll output at path, failing if absent.
func parseSections(src string, markdown ...bool) []render.Segment {
	return render.ParseSourceSections(render.SourceText{Spans: []render.SourceSpan{{Text: src}}}, markdown...)
}

func assemble(segs []render.Segment, plan map[string]render.SectionPlan, style render.CommentStyle) (string, map[string]string) {
	assembled, parts := render.AssembleSourceWithTemplateSource(segs, plan, style, render.TemplateSource{})
	return assembled.AuthoredText(), parts
}

func renderedByPath(t *testing.T, files []RenderedFile, path string) string {
	t.Helper()
	for _, f := range files {
		if f.Path == path {
			return f.Content
		}
	}
	t.Fatalf("no rendered file at %s", path)
	return ""
}

// syncClean opens+syncs root and fails on any residual drift.
func syncClean(t *testing.T, root string) {
	t.Helper()
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
}
