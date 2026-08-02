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
		{"default plans README", defaultReadme, "- Each phase independently declares", "A plan stays"},
	} {
		assertPlanTaskDetailContract(t, surface)
	}
	assertPlanScaffoldDetailContract(t, planPolicySurface{
		"default plan template", defaultPlanTemplate, "### Task 1.1:", "### Phase close",
	})

	root := testsupport.RepoRoot(t)
	for _, surface := range []planPolicySurface{
		{name: ".pi/skills/awf-writing-plans/SKILL.md", start: "- **Phases and tasks:**", end: "- **Self-contained"},
		{name: ".pi/agents/plan-reviewer.md", start: "1. **executability**", end: "1. **doc-currency"},
		{name: "docs/plans/README.md", start: "- Each phase independently declares", end: "A plan stays"},
	} {
		body, err := os.ReadFile(filepath.Join(root, surface.name))
		if err != nil {
			t.Fatalf("read rendered policy surface %s: %v", surface.name, err)
		}
		surface.output = string(body)
		assertPlanTaskDetailContract(t, surface)
	}
	assertPlanScaffoldDetailContract(t, planPolicySurface{
		name: "docs/plans/template.md", output: readPlanPolicyFile(t, root, "docs/plans/template.md"),
		start: "### Task 1.1:", end: "### Phase close",
	})
}

var planTaskDetailContractClauses = []string{
	"qualifying implementation-ready instructions are the default",
	"require `latitude: exact` for machine-consumed configuration and manifests, contract-bearing declarations, fixtures, golden output, commands, mechanical replacements, required literal prose, and batch representative and edge transformations",
	"permit it voluntarily elsewhere",
	"directly beneath the task declaration and before prose",
	"the vocabulary is `kind: spike`, `kind: batch`, `latitude: exact`, `question:`, `applying:`, `context:`, `paths:`, `representative:`, `edge:`, and `post-check:`",
	"`applying:` and `context:` carry nonempty json string arrays and are omitted rather than written as `[]`",
	"a `kind: spike` task is question-only, carries `question:`, has no implementation body, records its answer in notes, cannot own a phase, and sequences dependent work into a later phase",
	"a `kind: batch` task carries `paths:`, `representative:`, `edge:`, and `post-check:`",
	"`paths:` is required whenever affected scope is ambiguous, including an ambiguous non-batch task; every batch is necessarily ambiguous",
	"`post-check:` is required for every batch and whenever `paths:` contains a `glob:` or `pathspec:` entry",
	"conditional and optional tasks are forbidden",
	"`tbd`, `implement later`, outcome-only summaries, and hidden design choices are placeholders, never pseudocode",
	"no prior conversation context",
	"independently declares exactly one execution mode: `inline` or `subagent-driven`",
	"ordered steps",
	"one independently green coherent implementation transaction",
	"any helper partition exhaustively assigns every affected site to the parent or exactly one helper, keeps helper subsets path-disjoint, shared files parent-owned, and mutating commands confined to the assigned subset",
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

var planScaffoldDetailContractClauses = []string{
	"supply qualifying implementation-ready instructions",
	"recognized fields are `kind`, `latitude`, `question`, `applying`, `context`, `paths`, `representative`, `edge`, and `post-check`",
	"`applying` and `context` require nonempty json string arrays and are omitted rather than written as `[]`",
	"use `latitude: exact` for a contract-bearing task",
	"`kind: spike` requires `question`, no body, and an answer in notes",
	"`kind: batch` requires json-array `paths`, `representative`, `edge`, and `post-check`",
	"ambiguous scope requires `paths`",
	"any batch, glob, or pathspec scope requires `post-check`",
	"omit fields whose contracts do not apply",
}

func assertPlanScaffoldDetailContract(t *testing.T, surface planPolicySurface) {
	t.Helper()
	policy := strings.ToLower(planPolicySection(t, surface))
	for _, clause := range missingPlanScaffoldDetailClauses(policy) {
		t.Errorf("%s scaffold missing contractual phrase %q:\n%s", surface.name, clause, policy)
	}
	if strings.Contains(policy, "<no value>") {
		t.Errorf("%s scaffold leaked missingkey output:\n%s", surface.name, policy)
	}
}

func missingPlanScaffoldDetailClauses(policy string) []string {
	policy = strings.ToLower(strings.Join(strings.Fields(policy), " "))
	var missing []string
	for _, clause := range planScaffoldDetailContractClauses {
		if !strings.Contains(policy, clause) {
			missing = append(missing, clause)
		}
	}
	return missing
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
		start: "### Task 1.1:", end: "### Phase close",
	})
	for _, mutation := range []struct {
		name, from, to string
	}{
		{"qualifying instructions removed", "Supply qualifying implementation-ready instructions", "Supply exact instructions"},
		{"field vocabulary narrowed", "recognized fields are `Kind`, `Latitude`, `Question`, `Applying`, `Context`, `Paths`, `Representative`, `Edge`, and `Post-check`", "recognized fields are `Kind` and `Latitude`"},
		{"empty decision arrays allowed", "`Applying` and `Context` require nonempty JSON string arrays and are omitted rather than written as `[]`", "`Applying` and `Context` may be empty JSON arrays"},
		{"exactness optional", "Use `Latitude: exact` for a contract-bearing task", "Permit `Latitude: exact` for a contract-bearing task"},
		{"spike body allowed", "`Kind: spike` requires `Question`, no body, and an answer in Notes", "`Kind: spike` permits a body"},
		{"batch edge omitted", "`Kind: batch` requires JSON-array `Paths`, `Representative`, `Edge`, and `Post-check`", "`Kind: batch` requires JSON-array `Paths`"},
		{"ambiguous paths optional", "ambiguous scope requires `Paths`", "ambiguous scope may omit `Paths`"},
		{"post-check optional", "any batch, glob, or pathspec scope requires `Post-check`", "batch scopes may omit `Post-check`"},
		{"inapplicable fields retained", "Omit fields whose contracts do not apply", "Retain every field"},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			if !strings.Contains(policy, mutation.from) {
				t.Fatalf("mutation source %q is absent", mutation.from)
			}
			mutated := strings.Replace(policy, mutation.from, mutation.to, 1)
			if missing := missingPlanScaffoldDetailClauses(mutated); len(missing) == 0 {
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
