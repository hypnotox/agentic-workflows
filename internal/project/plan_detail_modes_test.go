package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

type planPolicySurface struct {
	name, output, start, end string
}

// invariant: rendering/workflow-skill-templates:plan-task-detail-modes (TestPlanTaskDetailModesStayAligned)
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

	for _, surface := range []planPolicySurface{
		{"default writing skill", defaultWriter, "- **Phases and tasks:**", "- **Self-contained"},
		{"default plan reviewer", defaultReviewer, "1. **executability**", "1. **doc-currency"},
		{"default plans README", defaultReadme, "- Phases each", "- Each phase declares"},
		{"default plan template", defaultPlanTemplate, "**Execution mode", "- [ ] **Phase-close"},
	} {
		assertPlanTaskDetailContract(t, surface)
	}

	root := testsupport.RepoRoot(t)
	for _, surface := range []planPolicySurface{
		{name: ".pi/skills/awf-writing-plans/SKILL.md", start: "- **Phases and tasks:**", end: "- **Self-contained"},
		{name: ".pi/agents/plan-reviewer.md", start: "1. **executability**", end: "1. **doc-currency"},
		{name: "docs/plans/README.md", start: "- Phases each", end: "- Each phase declares"},
		{name: "docs/plans/template.md", start: "**Execution mode", end: "- [ ] **Phase-close"},
	} {
		body, err := os.ReadFile(filepath.Join(root, surface.name))
		if err != nil {
			t.Fatalf("read rendered policy surface %s: %v", surface.name, err)
		}
		surface.output = string(body)
		assertPlanTaskDetailContract(t, surface)
	}
}

func assertPlanTaskDetailContract(t *testing.T, surface planPolicySurface) {
	t.Helper()
	start := strings.Index(surface.output, surface.start)
	if start < 0 {
		t.Fatalf("%s missing plan-policy start %q", surface.name, surface.start)
	}
	endOffset := strings.Index(surface.output[start+len(surface.start):], surface.end)
	if endOffset < 0 {
		t.Fatalf("%s missing plan-policy end %q", surface.name, surface.end)
	}
	policy := surface.output[start : start+len(surface.start)+endOffset]
	policy = strings.Join(strings.Fields(policy), " ")

	for _, clause := range []string{
		"qualifying implementation-ready",
		"default",
		"`Latitude: exact`",
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
		"directly beneath",
		"`Kind: spike`",
		"`Question:`",
		"Notes",
		"later phase",
		"`Kind: batch`",
		"`Paths:`",
		"ambiguous",
		"every batch",
		"`Post-check:`",
		"glob",
		"pathspec",
		"conditional",
		"optional tasks",
		"`TBD`",
		"`implement later`",
		"outcome-only summaries",
		"hidden design choices",
		"placeholders, never pseudocode",
		"no prior conversation context",
		"Execution mode",
		"ordered steps",
		"one independently green coherent implementation transaction",
		"parent or exactly one helper",
		"path-disjoint",
		"shared files",
		"confined",
	} {
		if !strings.Contains(strings.ToLower(policy), strings.ToLower(clause)) {
			t.Errorf("%s plan-policy section missing clause %q:\n%s", surface.name, clause, policy)
		}
	}
	if strings.Contains(policy, "<no value>") {
		t.Errorf("%s plan-policy section leaked missingkey output:\n%s", surface.name, policy)
	}
}

func TestPlanContextPathsComeFromTasks(t *testing.T) {
	defaultWriter := renderSkillGolden(t, "writing-plans", map[string]any{
		"prefix": "example", "vars": map[string]any{},
		"layout": map[string]any{"plansDir": "docs/plans", "plansTemplate": "docs/plans/template.md", "maintainableCodeDesign": "docs/maintainable-code-design.md", "workflowRef": "docs/workflow.md"},
		"data":   map[string]any{},
	})
	defaultReviewer := renderSkillGolden(t, "reviewing-plan", map[string]any{
		"prefix": "example", "vars": map[string]any{},
		"layout": map[string]any{"plansDir": "docs/plans", "workflowRef": "docs/workflow.md"},
		"data":   map[string]any{}, "skills": map[string]bool{},
	})
	defaultResync := renderSkillGolden(t, "reviewing-plan-resync", map[string]any{
		"prefix": "example", "vars": map[string]any{},
		"layout": map[string]any{"plansDir": "docs/plans"},
		"data":   map[string]any{}, "skills": map[string]bool{},
	})

	root := testsupport.RepoRoot(t)
	for _, surface := range []struct {
		name string
		body string
	}{
		{"default writing skill", defaultWriter},
		{"default reviewing skill", defaultReviewer},
		{"default resync skill", defaultResync},
		{"rendered writing skill", readPlanPolicyFile(t, root, ".pi/skills/awf-writing-plans/SKILL.md")},
		{"rendered reviewing skill", readPlanPolicyFile(t, root, ".pi/skills/awf-reviewing-plan/SKILL.md")},
		{"rendered resync skill", readPlanPolicyFile(t, root, ".pi/skills/awf-reviewing-plan-resync/SKILL.md")},
	} {
		body := strings.Join(strings.Fields(surface.body), " ")
		for _, clause := range []string{"every task-level `Paths:` entry", "exact repository paths named in task titles and bodies", "deduplicate", "Do not infer a path from vague prose"} {
			if !strings.Contains(body, clause) {
				t.Errorf("%s missing context-path collection clause %q", surface.name, clause)
			}
		}
	}
}

func readPlanPolicyFile(t *testing.T, root, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatalf("read rendered policy surface %s: %v", name, err)
	}
	return string(body)
}
