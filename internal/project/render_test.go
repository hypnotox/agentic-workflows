package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// syncedWorkflowDoc scaffolds a minimal project whose commit-discipline part is
// body, syncs it, and returns the rendered docs/workflow.md content.
func syncedWorkflowDoc(t *testing.T, body string) string {
	t.Helper()
	root := scaffoldFiles(t, "prefix: example\nvars: {}\nskills: []\nagents: []\n",
		map[string]string{"parts/workflow/commit-discipline.md": body})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "docs", "workflow.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// A whole-line awf:comment directive in a convention part never reaches
// rendered output, while a mid-line occurrence and a fenced whole-line demo
// render verbatim (ADR-0121 Decisions 1-3; the template-source seam is proven
// by the render-layer unit tests plus the strip call in renderTarget).
// invariant: authoring-comment-stripped
func TestAuthoringCommentStrippedFromPart(t *testing.T) {
	out := syncedWorkflowDoc(t,
		"<!-- awf:comment touches-invariant: demo-slug - an internal tag -->\n"+
			"KEEP-TOP\n"+
			"mid-line <!-- awf:comment inline note --> kept\n"+
			"```\n<!-- awf:comment fenced demo -->\n```\n")
	if strings.Contains(out, "demo-slug") {
		t.Errorf("whole-line directive leaked into rendered output:\n%s", out)
	}
	for _, want := range []string{"KEEP-TOP", "mid-line <!-- awf:comment inline note --> kept", "<!-- awf:comment fenced demo -->"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

// A part whose only content is directive lines strips to an empty body: the
// section renders empty with its awf:edit pointer present, never falling back
// to the template default (ADR-0034 Decision 4 semantics preserved).
func TestCommentOnlyPartRendersEmptySection(t *testing.T) {
	out := syncedWorkflowDoc(t,
		"<!-- awf:comment first note -->\n<!-- awf:comment second note -->\n")
	if !strings.Contains(out, "from .awf/parts/workflow/commit-discipline.md") {
		t.Errorf("comment-only part must still be consumed (pointer present):\n%s", out)
	}
	if strings.Contains(out, "Use Conventional Commits, one concern per commit.") {
		t.Errorf("empty part must not fall back to the template default:\n%s", out)
	}
	if strings.Contains(out, "awf:comment") {
		t.Errorf("directive lines leaked:\n%s", out)
	}
}

// A malformed whole-line opener in a part fails the render naming the part path.
func TestMalformedAuthoringCommentFailsSync(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nvars: {}\nskills: []\nagents: []\n",
		map[string]string{"parts/workflow/commit-discipline.md": "<!-- awf:comment unclosed\n"})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	err = p.Sync()
	if err == nil {
		t.Fatal("a malformed opener must fail the sync")
	}
	if !strings.Contains(err.Error(), ".awf/parts/workflow/commit-discipline.md") ||
		!strings.Contains(err.Error(), "malformed awf:comment") {
		t.Errorf("error must name the part path and the directive, got %v", err)
	}
}

// An unknown {{=awf:key}} placeholder demonstrated inside an authoring comment
// must not hard-error: the strip runs before placeholder substitution
// (ADR-0121 Decision 2).
func TestUnknownPlaceholderInsideCommentRenders(t *testing.T) {
	out := syncedWorkflowDoc(t,
		"<!-- awf:comment mentions {{=awf:nonexistent}} -->\nBODY\n")
	if !strings.Contains(out, "BODY") || strings.Contains(out, "nonexistent") {
		t.Errorf("comment-wrapped unknown placeholder must strip cleanly:\n%s", out)
	}
}

// The template seam end-to-end: the embedded adr-readme template carries a real
// dogfooded touches-invariant authoring comment, so any regression in the
// renderTarget strip wiring (which the render-layer unit tests cannot see)
// leaks it into every scaffolded project's rendered README.
// touches-invariant: authoring-comment-stripped - the renderTarget wiring, proven end-to-end over the real embedded template
func TestEmbeddedTemplateAuthoringCommentStripped(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nvars: {}\nskills: []\nagents: []\n", nil)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "docs", "decisions", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "awf:comment") {
		t.Errorf("the embedded template's authoring comment leaked into rendered output:\n%s", b)
	}
}
