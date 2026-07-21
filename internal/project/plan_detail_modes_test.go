package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// invariant: rendering/templates:plan-task-detail-modes
func TestPlanTaskDetailModesStayAligned(t *testing.T) {
	defaultWriter := renderSkillGolden(t, "writing-plans", map[string]any{
		"prefix": "example",
		"vars":   map[string]any{},
		"layout": map[string]any{"plansDir": "docs/plans", "plansTemplate": "docs/plans/template.md"},
		"data":   map[string]any{},
	})
	defaultReviewer := renderAgentGolden(t, "plan-reviewer", map[string]any{
		"prefix": "example",
		"layout": map[string]any{"plansDir": "docs/plans"},
		"data":   catalog.Standard.Agents["plan-reviewer"].Data,
	})
	defaultReadme := renderGolden(t, "plans-readme/README.md.tmpl", map[string]any{
		"layout": map[string]any{"plansDir": "docs/plans"},
	})
	defaultPlanTemplate := renderGolden(t, "plans-template/template.md.tmpl", map[string]any{
		"vars":   map[string]any{},
		"layout": testLayout(),
	})
	defaultAgentGuide := renderGolden(t, "agents-doc/AGENTS.md.tmpl", map[string]any{
		"prefix": "example",
		"vars":   map[string]any{},
		"layout": testLayout(),
		"skills": map[string]bool{"brainstorming": true},
		"data":   map[string]any{},
	})

	for name, output := range map[string]string{
		"default writing skill": defaultWriter,
		"default plan reviewer": defaultReviewer,
		"default plans README":  defaultReadme,
		"default plan template": defaultPlanTemplate,
		"default agent guide":   defaultAgentGuide,
	} {
		assertPlanTaskDetailContract(t, name, output)
	}

	root := testsupport.RepoRoot(t)
	for _, rel := range []string{
		".pi/skills/awf-writing-plans/SKILL.md",
		".pi/skills/plan-reviewer.md",
		"docs/plans/README.md",
		"docs/plans/template.md",
		"AGENTS.md",
	} {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read rendered policy surface %s: %v", rel, err)
		}
		assertPlanTaskDetailContract(t, rel, string(body))
	}
}

func assertPlanTaskDetailContract(t *testing.T, name, output string) {
	t.Helper()
	output = strings.Join(strings.Fields(output), " ")
	for _, clause := range []string{
		"exact content/diffs",
		"implementation-ready pseudocode",
		"exact file paths",
		"relevant symbols",
		"behavior, branches, ordering, failures",
		"constraints",
		"forbidden behavior",
		"tests",
		"acceptance assertions",
		"deterministic verification",
		"machine-consumed",
		"configuration",
		"manifests",
		"contract-bearing",
		"fixtures",
		"golden output",
		"commands",
		"mechanical replacements",
		"required literal prose",
		"representative and edge",
		"affected-site set",
		"post-check",
		"Non-contractual prose",
		"mixed task",
		"`TBD`",
		"`implement later`",
		"outcome-only summaries",
		"hidden design choices",
		"placeholders, never pseudocode",
	} {
		if !strings.Contains(output, clause) {
			t.Errorf("%s missing plan-detail clause %q:\n%s", name, clause, output)
		}
	}
}
