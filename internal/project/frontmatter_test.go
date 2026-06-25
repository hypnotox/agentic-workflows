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
	layout := map[string]any{
		"docsDir": "docs", "adrDir": "docs/decisions", "activeMd": "docs/decisions/ACTIVE.md",
		"adrReadme": "docs/decisions/README.md", "adrTemplate": "docs/decisions/template.md",
		"plansDir": "docs/plans",
	}
	check := func(tid string) {
		t.Helper()
		src, err := fs.ReadFile(templates.FS, tid)
		if err != nil {
			t.Fatalf("read %s: %v", tid, err)
		}
		vars := map[string]any{}
		for _, v := range render.ReferencedVars(string(src)) {
			vars[v] = ""
		}
		data := map[string]any{"prefix": "awf", "vars": vars, "layout": layout, "data": map[string]any{}}
		out, err := render.Render(string(src), nil, func(string) (string, error) { return "", nil }, data)
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
	for name := range cat.Skills {
		check(fmt.Sprintf("skills/%s/SKILL.md.tmpl", name))
	}
	for name := range cat.Agents {
		check(fmt.Sprintf("agents/%s.md.tmpl", name))
	}
}
