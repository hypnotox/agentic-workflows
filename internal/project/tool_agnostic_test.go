package project

import (
	"fmt"
	"io/fs"
	"regexp"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// TestSkillProseToolAgnostic backs inv: skill-prose-tool-agnostic (ADR-0038):
// every rendered skill and agent body is free of runtime tool-name tokens. The
// denylist is matched case-insensitively and word-anchored, so it does not fire
// on the neutral "subagent" / "subagent's prompt" replacement language.
// invariant: skill-prose-tool-agnostic
func TestSkillProseToolAgnostic(t *testing.T) {
	cat, err := catalog.Load(templates.FS)
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	forbidden := []*regexp.Regexp{
		regexp.MustCompile(`(?i)subagent_type`),
		regexp.MustCompile(`(?i)\bsubagent type\b`),
		regexp.MustCompile(`(?i)\bagent tool\b`),
		regexp.MustCompile(`(?i)\bskill tool\b`),
		regexp.MustCompile(`(?i)\bAskUserQuestion\b`),
		regexp.MustCompile("(?i)`agent` prompt"),
		// File-operation tool names (ADR-0038, broadened): backticked tool tokens and
		// the "via Edit" / "Edit calls" / "Read tool" phrase forms. The plain action
		// verbs ("Write the ADR file", "Read the file") and the shell `grep` are not
		// tool names and stay — hence the word-anchored phrase forms, not bare words.
		regexp.MustCompile("(?i)`(write|edit|read)`"),
		regexp.MustCompile(`(?i)\bvia (write|edit|read)\b`),
		regexp.MustCompile(`(?i)\b(write|edit|read) (tool|calls?)\b`),
	}
	scan := func(tid, requiresDoc string) {
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
		layout := map[string]any{
			"docsDir": "docs", "adrDir": "docs/decisions", "activeMd": "docs/decisions/ACTIVE.md",
			"adrReadme": "docs/decisions/README.md", "adrTemplate": "docs/decisions/template.md",
			"plansDir": "docs/plans", "domainsDir": "docs/domains",
			"docs": docs, "workflowRef": workflowRef,
		}
		vars := map[string]any{}
		for _, v := range render.ReferencedVars(string(src)) {
			vars[v] = ""
		}
		data := map[string]any{"prefix": "awf", "vars": vars, "layout": layout, "data": map[string]any{}}
		asm, parts := render.Assemble(render.ParseSections(string(src)), nil)
		out, err := render.Execute(asm, data, parts, "test")
		if err != nil {
			t.Fatalf("render %s: %v", tid, err)
		}
		for _, re := range forbidden {
			if tok := re.FindString(out); tok != "" {
				t.Errorf("%s: rendered body names runtime tool %q — use action-language (ADR-0038)", tid, tok)
			}
		}
	}
	for name, spec := range cat.Skills {
		scan(fmt.Sprintf("skills/%s/SKILL.md.tmpl", name), spec.RequiresDoc)
	}
	for name := range cat.Agents {
		scan(fmt.Sprintf("agents/%s.md.tmpl", name), "")
	}
}
