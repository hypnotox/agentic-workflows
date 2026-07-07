package project

import (
	"strings"
	"testing"
)

// TestMemoryGitignoreAlwaysOn asserts RenderAll unconditionally emits the
// self-ignoring .awf/memory/.gitignore with a #-comment banner (ADR-0069) —
// no config gate, unlike bootstrap/hooks.
// invariant: memory-gitignore-always-on
func TestMemoryGitignoreAlwaysOn(t *testing.T) {
	root := scaffold(t, "prefix: example\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	out, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	var found *RenderedFile
	for i := range out {
		if out[i].Path == ".awf/memory/.gitignore" {
			found = &out[i]
		}
	}
	if found == nil {
		t.Fatal("expected .awf/memory/.gitignore in every RenderAll output")
	}
	want := "# " + bannerText + "\n*\n!.gitignore\n"
	if found.Content != want {
		t.Errorf("content = %q, want %q", found.Content, want)
	}
	if !strings.HasPrefix(found.Content, "# ") {
		t.Errorf("banner must be a #-comment, got %q", found.Content)
	}
}
