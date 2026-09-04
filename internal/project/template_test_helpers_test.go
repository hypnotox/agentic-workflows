package project

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

func expandIncludes(src string) (string, error) {
	expanded, err := render.ExpandIncludesSource(src, "", templates.FS)
	if err != nil {
		return "", err
	}
	return expanded.AuthoredText(), nil
}

func renderGolden(t *testing.T, tmplPath string, data map[string]any) string {
	t.Helper()
	src, err := fs.ReadFile(templates.FS, tmplPath)
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	withLayoutDefaults(data)
	expanded, err := expandIncludes(string(src))
	if err != nil {
		t.Fatalf("expand includes: %v", err)
	}
	asm, parts := assemble(parseSections(expanded))
	out, err := render.Execute(asm, data, parts, "test")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	assertNoLeaks(t, out)
	return out
}

// withLayoutDefaults seeds the always-present .layout members ADR-0013 added
// (docs/workflowRef/domainsDir) into a golden test's layout fixture when absent,
// so templates citing them render without a <no value> token. The docs map
// carries the docs the templates cite so guarded clauses render as before; a test
// that needs different values sets them explicitly and this leaves them untouched.
func withLayoutDefaults(data map[string]any) {
	l, _ := data["layout"].(map[string]any)
	if l == nil {
		l = map[string]any{}
		data["layout"] = l
	}
	if _, ok := l["docs"]; !ok {
		l["docs"] = map[string]any{
			"debugging": "docs/debugging.md",
			"pitfalls":  "docs/pitfalls.md",
			"roadmap":   "docs/roadmap.md",
		}
	}
	if _, ok := l["workflowRef"]; !ok {
		l["workflowRef"] = "docs/workflow.md"
	}
	if _, ok := l["domainsDir"]; !ok {
		l["domainsDir"] = "docs/domains"
	}
}

func assertNoLeaks(t *testing.T, out string) {
	t.Helper()
	if strings.Contains(out, "<!-- awf:section") || strings.Contains(out, "<!-- awf:end") {
		t.Errorf("markers leaked:\n%s", out)
	}
	if strings.Contains(out, "<no value>") {
		t.Errorf("missing sample data (rendered <no value>):\n%s", out)
	}
	if strings.Contains(out, "{{") || strings.Contains(out, "}}") {
		t.Errorf("unrendered template action:\n%s", out)
	}
}

func renderSkillGolden(t *testing.T, skill string, data map[string]any) string {
	t.Helper()
	return renderGolden(t, "skills/"+skill+"/SKILL.md.tmpl", data)
}

func assertOrderedPhrases(t *testing.T, out string, phrases ...string) {
	t.Helper()
	position := 0
	for _, phrase := range phrases {
		next := strings.Index(out[position:], phrase)
		if next < 0 {
			t.Fatalf("expected %q after byte %d:\n%s", phrase, position, out)
		}
		position += next + len(phrase)
	}
}
