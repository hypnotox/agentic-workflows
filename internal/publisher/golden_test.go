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
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}

	maintenance, err := os.ReadFile(filepath.Join(root, ".claude", "skills", "awf-maintenance", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(maintenance), "# AWF maintenance") {
		t.Errorf("maintenance skill not rendered with its standard contract:\n%s", maintenance)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "agents")); !os.IsNotExist(err) {
		t.Fatalf("AWF retained a Claude agents directory: %v", err)
	}
	if drift, err := checkProject(p, testContext(t)); err != nil || len(drift) != 0 {
		t.Errorf("expected clean check, got drift=%#v err=%v", drift, err)
	}
}

// invariant: rendering/render-engine:include-in-templatehash (TestTemplateHashCoversExpandedSource)
func TestTemplateHashCoversExpandedSource(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := renderAll(p)
	if err != nil {
		t.Fatal(err)
	}
	const tid = "skills/awf-effort/SKILL.md.tmpl"
	var got string
	for _, f := range files {
		if f.TemplateID == tid {
			got = f.TemplateHash
		}
	}
	if got == "" {
		t.Fatal("awf-effort skill not rendered")
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
