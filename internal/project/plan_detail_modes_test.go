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
	defaultAgentGuide := renderGolden(t, "agents-doc/AGENTS.md.tmpl", map[string]any{
		"prefix": "example",
		"vars":   map[string]any{},
		"layout": testLayout(),
		"skills": map[string]bool{"brainstorming": true},
		"data":   map[string]any{},
	})
	if !strings.Contains(defaultAgentGuide, "implementation-ready pseudocode") {
		t.Errorf("default agent guide missing plan-detail summary:\n%s", defaultAgentGuide)
	}

	for name, output := range map[string]string{
		"default writing skill": defaultWriter,
		"default plan reviewer": defaultReviewer,
		"default plans README":  defaultReadme,
	} {
		for _, want := range []string{
			"implementation-ready pseudocode",
			"machine-consumed",
			"fixtures",
			"golden output",
			"mechanical replacements",
			"hidden design",
		} {
			if !strings.Contains(output, want) {
				t.Errorf("%s missing plan-detail contract %q:\n%s", name, want, output)
			}
		}
		if !strings.Contains(output, "batch task") {
			t.Errorf("%s lost the batch-task form:\n%s", name, output)
		}
	}

	root := testsupport.RepoRoot(t)
	for _, rel := range []string{
		".pi/skills/awf-writing-plans/SKILL.md",
		".pi/skills/plan-reviewer.md",
		"docs/plans/README.md",
		"AGENTS.md",
	} {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read rendered policy surface %s: %v", rel, err)
		}
		if !strings.Contains(string(body), "implementation-ready pseudocode") {
			t.Errorf("rendered policy surface %s does not sanction implementation-ready pseudocode", rel)
		}
	}
}
