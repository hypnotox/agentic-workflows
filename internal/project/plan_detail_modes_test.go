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

// invariant: rendering/workflow-skill-templates:plan-task-detail-modes
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
		{"default writing skill", defaultWriter, "- **Tasks:**", "- **Commit subjects"},
		{"default plan reviewer", defaultReviewer, "1. **executability**", "1. **doc-currency"},
		{"default plans README", defaultReadme, "- Phases of tasks", "- A commit step"},
		{"default plan template", defaultPlanTemplate, "- [ ] **Task 1.1", "- [ ] **Task 1.2"},
	} {
		assertPlanTaskDetailContract(t, surface)
	}

	root := testsupport.RepoRoot(t)
	for _, surface := range []planPolicySurface{
		{name: ".pi/awf-workflows/writing-plans.md", start: "- **Tasks:**", end: "- **Commit subjects"},
		{name: ".pi/agents/plan-reviewer.md", start: "1. **executability**", end: "1. **doc-currency"},
		{name: "docs/plans/README.md", start: "- Phases of tasks", end: "- A commit step"},
		{name: "docs/plans/template.md", start: "- [ ] **Task 1.1", end: "- [ ] **Task 1.2"},
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
		"exact content/diffs",
		"implementation-ready pseudocode",
		"exact file paths",
		"relevant symbols",
		"expected terminal states",
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
		"no prior conversation context",
	} {
		if !strings.Contains(policy, clause) {
			t.Errorf("%s plan-policy section missing clause %q:\n%s", surface.name, clause, policy)
		}
	}
}
