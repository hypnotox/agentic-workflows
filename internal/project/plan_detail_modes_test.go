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

var planTaskDetailContractClauses = []string{
	"qualifying implementation-ready instructions are the default",
	"require `latitude: exact` for machine-consumed configuration and manifests, contract-bearing declarations, fixtures, golden output, commands, mechanical replacements, required literal prose, and batch representative and edge transformations",
	"permit it voluntarily elsewhere",
	"directly beneath the task declaration and before prose",
	"the vocabulary is `kind: spike`, `kind: batch`, `latitude: exact`, `question:`, `paths:`, `representative:`, `edge:`, and `post-check:`",
	"a `kind: spike` task is question-only, carries `question:`, has no implementation body, records its answer in notes, cannot own a phase, and sequences dependent work into a later phase",
	"a `kind: batch` task carries `paths:`, `representative:`, `edge:`, and `post-check:`",
	"`paths:` is required whenever affected scope is ambiguous, including an ambiguous non-batch task; every batch is necessarily ambiguous",
	"`post-check:` is required for every batch and whenever `paths:` contains a `glob:` or `pathspec:` entry",
	"conditional and optional tasks are forbidden",
	"`tbd`",
	"`implement later`",
	"outcome-only summaries",
	"hidden design choices",
	"placeholders, never pseudocode",
	"no prior conversation context",
	"execution mode",
	"ordered steps",
	"one independently green coherent implementation transaction",
	"parent or exactly one helper",
	"path-disjoint",
	"shared files",
	"confined",
}

func assertPlanTaskDetailContract(t *testing.T, surface planPolicySurface) {
	t.Helper()
	policy := planPolicySection(t, surface)
	for _, clause := range missingPlanTaskDetailClauses(policy) {
		t.Errorf("%s plan-policy section missing contractual phrase %q:\n%s", surface.name, clause, policy)
	}
	if strings.Contains(policy, "<no value>") {
		t.Errorf("%s plan-policy section leaked missingkey output:\n%s", surface.name, policy)
	}
}

func planPolicySection(t *testing.T, surface planPolicySurface) string {
	t.Helper()
	start := strings.Index(surface.output, surface.start)
	if start < 0 {
		t.Fatalf("%s missing plan-policy start %q", surface.name, surface.start)
	}
	endOffset := strings.Index(surface.output[start+len(surface.start):], surface.end)
	if endOffset < 0 {
		t.Fatalf("%s missing plan-policy end %q", surface.name, surface.end)
	}
	return strings.Join(strings.Fields(surface.output[start:start+len(surface.start)+endOffset]), " ")
}

func missingPlanTaskDetailClauses(policy string) []string {
	policy = strings.ToLower(strings.Join(strings.Fields(policy), " "))
	var missing []string
	for _, clause := range planTaskDetailContractClauses {
		if !strings.Contains(policy, clause) {
			missing = append(missing, clause)
		}
	}
	return missing
}

func TestPlanTaskDetailContractRejectsInversions(t *testing.T) {
	output := renderGolden(t, "plans-template/template.md.tmpl", map[string]any{
		"vars": map[string]any{}, "layout": testLayout(),
	})
	policy := planPolicySection(t, planPolicySurface{
		name: "default plan template", output: output,
		start: "**Execution mode", end: "- [ ] **Phase-close",
	})
	for _, mutation := range []struct {
		name, from, to string
	}{
		{"default inverted", "Qualifying implementation-ready instructions are the default", "Exact instructions are the default"},
		{"exactness optional", "Require `Latitude: exact` for machine-consumed", "Permit `Latitude: exact` for machine-consumed"},
		{"spike answer optional", "records its answer in Notes", "may omit its answer from Notes"},
		{"post-check optional", "`Post-check:` is required for every batch", "`Post-check:` is optional for every batch"},
		{"conditional tasks allowed", "Conditional and optional tasks are forbidden", "Conditional and optional tasks are permitted"},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			if !strings.Contains(policy, mutation.from) {
				t.Fatalf("mutation source %q is absent", mutation.from)
			}
			mutated := strings.Replace(policy, mutation.from, mutation.to, 1)
			if missing := missingPlanTaskDetailClauses(mutated); len(missing) == 0 {
				t.Fatalf("contract accepted semantic inversion %q", mutation.to)
			}
		})
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
