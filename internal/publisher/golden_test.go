package publisher

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// TestEndToEndGolden keeps the render/sync/check integration oracle focused on
// the standard catalog and its retained target artifacts.
func TestEndToEndGolden(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}

	reviewer, err := os.ReadFile(filepath.Join(root, ".claude/agents/reviewer.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(reviewer), "Fresh report-only reviewer") {
		t.Errorf("reviewer not rendered with its standard contract:\n%s", reviewer)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "example-implementing", "SKILL.md")); err != nil {
		t.Fatalf("implementing skill not rendered: %v", err)
	}
	if drift, err := checkProject(p, testContext(t)); err != nil || len(drift) != 0 {
		t.Errorf("expected clean check, got drift=%#v err=%v", drift, err)
	}
}

// invariant: rendering/render-engine:include-in-templatehash (TestTemplateHashCoversExpandedSource)
func TestTemplateHashCoversExpandedSource(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := renderAll(p)
	if err != nil {
		t.Fatal(err)
	}
	const tid = "agents/reviewer.md.tmpl"
	var got string
	for _, f := range files {
		if f.TemplateID == tid {
			got = f.TemplateHash
		}
	}
	if got == "" {
		t.Fatal("reviewer not rendered")
	}
	raw, err := fs.ReadFile(templates.FS, tid)
	if err != nil {
		t.Fatal(err)
	}
	expanded, err := render.ExpandIncludesSource(string(raw), tid, templates.FS)
	if err != nil {
		t.Fatal(err)
	}
	if got != manifest.Hash([]byte(expanded.AuthoredText())) {
		t.Errorf("TemplateHash = %q, want authored expanded projection", got)
	}
}
