package project

import (
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// TestAllTemplatesProduceValidFrontmatter renders every catalog skill and agent
// template with a minimal-adopter data set (prefix + every referenced var seeded
// empty + full layout) and asserts the frontmatter parses with non-empty
// name/description and no leaked <no value> token.
// invariant: templates-valid-frontmatter
func TestAllTemplatesProduceValidFrontmatter(t *testing.T) {
	cat, err := catalog.Load(templates.FS)
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	// check renders one template with a docs map seeded only for the skill's own
	// required doc — mirroring the suppression guarantee (a doc-gated skill renders
	// only when its doc is enabled, so its unguarded .layout.docs.<doc> resolves;
	// non-gated skills must render cleanly with no docs enabled, guards omitting).
	check := func(tid, requiresDoc string) {
		t.Helper()
		src, err := fs.ReadFile(templates.FS, tid)
		if err != nil {
			t.Fatalf("read %s: %v", tid, err)
		}
		docs := map[string]any{}
		if requiresDoc != "" {
			docs[requiresDoc] = "docs/" + requiresDoc + ".md"
		}
		workflowRef := "AGENTS.md"
		if wp, ok := docs["workflow"]; ok {
			workflowRef = wp.(string)
		}
		layout := testLayout()
		layout["docs"] = docs
		layout["workflowRef"] = workflowRef
		vars := map[string]any{}
		for _, v := range render.ReferencedVars(string(src)) {
			vars[v] = ""
		}
		data := map[string]any{"prefix": "awf", "vars": vars, "layout": layout, "data": map[string]any{}, "skills": map[string]bool{}}
		asm, parts := render.Assemble(render.ParseSections(string(src)), nil)
		out, err := render.Execute(asm, data, parts, "test")
		if err != nil {
			t.Fatalf("render %s: %v", tid, err)
		}
		var fm skillFrontmatter
		_, found, err := frontmatter.Parse([]byte(out), &fm)
		if err != nil {
			t.Fatalf("%s: frontmatter parse: %v", tid, err)
		}
		if !found {
			t.Errorf("%s: no frontmatter", tid)
			return
		}
		if strings.TrimSpace(fm.Name) == "" {
			t.Errorf("%s: empty name", tid)
		}
		if strings.TrimSpace(fm.Description) == "" {
			t.Errorf("%s: empty description", tid)
		}
		if strings.Contains(fm.Name+fm.Description, "<no value>") {
			t.Errorf("%s: <no value> leaked into frontmatter", tid)
		}
	}
	for name, spec := range cat.Skills {
		check(fmt.Sprintf("skills/%s/SKILL.md.tmpl", name), spec.RequiresDoc)
	}
	for name := range cat.Agents {
		check(fmt.Sprintf("agents/%s.md.tmpl", name), "")
	}
}
