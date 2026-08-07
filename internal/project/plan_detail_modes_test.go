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
// invariant: rendering/workflow-skill-templates:plan-review-before-first-commit (TestPlanTaskDetailModesStayAligned)
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
		{name: ".claude/skills/awf-writing-plans/SKILL.md", start: "- **Phases and tasks:**", end: "- **Self-contained"},
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
	"change-specific",
	"observable outcome",
	"authority link",
	"material boundar",
	"ordering dependen",
	"focused evidence",
	"confinement where ambiguity or helpers require",
	"commit-capable owner",
	"authority-determined local",
	"helper",
	"confined",
	"`latitude",
	"optional aid",
	"directly beneath the task declaration and before prose",
	"the vocabulary is `kind: spike`, `kind: batch`, `latitude: exact`, `question:`, `applying:`, `context:`, `paths:`, `representative:`, `edge:`, and `post-check:`",
	"`applying:` and `context:` carry nonempty json string arrays and are omitted rather than written as `[]`",
	"a `kind: spike` task is question-only, carries `question:`, has no implementation body, records its answer in notes, cannot own a phase, and sequences dependent work into a later phase",
	"`representative:` and `edge:`",
	"deterministic `post-check:`",
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

func TestPlanningVerificationGuidanceStayAligned(t *testing.T) {
	defaultWriter := renderSkillGolden(t, "writing-plans", map[string]any{
		"prefix": "example",
		"vars":   map[string]any{},
		"layout": map[string]any{"plansDir": "docs/plans", "plansTemplate": "docs/plans/template.md"},
		"data":   map[string]any{},
	})
	root := testsupport.RepoRoot(t)
	surfaces := []planPolicySurface{
		{name: "default writing skill", output: defaultWriter, start: "- **Phases and tasks:**", end: "- **Self-contained"},
		{name: ".claude/skills/awf-writing-plans/SKILL.md", output: readPlanPolicyFile(t, root, ".claude/skills/awf-writing-plans/SKILL.md"), start: "- **Phases and tasks:**", end: "- **Self-contained"},
		{name: ".pi/skills/awf-writing-plans/SKILL.md", output: readPlanPolicyFile(t, root, ".pi/skills/awf-writing-plans/SKILL.md"), start: "- **Phases and tasks:**", end: "- **Self-contained"},
	}
	clauses := []string{
		"each material `post-check:` names its input population, exclusions, lifecycle snapshot, and expected terminal set or lifecycle-authorized residual findings",
		"empty output proves absence only after a probe success sentinel or checked exit status establishes that the probe ran",
		"after a compound mutating command, read back every mutation target before trusting the result",
		"classify each material check as an authority, state, or choreography check",
		"preserve authority checks",
		"use the least restrictive state validation that proves the durable property",
		"omit a choreography-only constraint unless a named authority or state property requires it",
	}
	for _, surface := range surfaces {
		policy := strings.ToLower(planPolicySection(t, surface))
		for _, clause := range clauses {
			if count := strings.Count(policy, clause); count != 1 {
				t.Errorf("%s planning-verification clause %q occurs %d times, want exactly once:\n%s", surface.name, clause, count, policy)
			}
		}
	}

	convention := strings.ToLower(readPlanPolicyFile(t, root, ".awf/skills/parts/writing-plans/conventions-tasks.md"))
	if !strings.Contains(convention, "{{=awf:sectiondefault}}") {
		t.Errorf("writing-plans convention must preserve the generic section through sectionDefault")
	}
	if strings.Contains(convention, "each material `post-check:` names its input population") {
		t.Errorf("writing-plans convention duplicates the generic material-verification policy")
	}
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
	"state the change-specific observable outcome, relevant authority links, material boundary, ordering dependency, focused evidence, and any necessary confinement",
	"recognized fields are `kind`, `latitude`, `question`, `applying`, `context`, `paths`, `representative`, `edge`, and `post-check`",
	"`applying` and `context` require nonempty json string arrays and are omitted rather than written as `[]`",
	"`latitude`, `kind: batch`, `representative`, and `edge` are optional aids",
	"`kind: spike` requires `question`, no body, and an answer in notes",
	"any batch, glob, or pathspec scope requires deterministic `post-check`",
	"ambiguous scope requires `paths`",
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
		{"change-specific outcome removed", "State the change-specific observable outcome", "State an implementation summary"},
		{"field vocabulary narrowed", "recognized fields are `Kind`, `Latitude`, `Question`, `Applying`, `Context`, `Paths`, `Representative`, `Edge`, and `Post-check`", "recognized fields are `Kind` and `Latitude`"},
		{"empty decision arrays allowed", "`Applying` and `Context` require nonempty JSON string arrays and are omitted rather than written as `[]`", "`Applying` and `Context` may be empty JSON arrays"},
		{"optional aids made mandatory", "`Latitude`, `Kind: batch`, `Representative`, and `Edge` are optional aids", "`Latitude`, `Kind: batch`, `Representative`, and `Edge` are mandatory"},
		{"spike body allowed", "`Kind: spike` requires `Question`, no body, and an answer in Notes", "`Kind: spike` permits a body"},
		{"ambiguous paths optional", "ambiguous scope requires `Paths`", "ambiguous scope may omit `Paths`"},
		{"post-check optional", "any batch, glob, or pathspec scope requires deterministic `Post-check`", "batch scopes may omit `Post-check`"},
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

// invariant: rendering/workflow-skill-templates:plan-review-before-first-commit (TestPlanReviewBeforeFirstCommit)
func TestPlanReviewBeforeFirstCommit(t *testing.T) {
	variants := map[string]map[string]any{
		"configured": {
			"prefix": "example", "vars": map[string]any{"gateCmd": "./x gate"},
			"layout": testLayout(), "data": map[string]any{}, "skills": map[string]bool{},
		},
		"empty": {
			"prefix": "example", "vars": map[string]any{}, "layout": testLayout(),
			"data": map[string]any{}, "skills": map[string]bool{},
		},
	}
	for variant, data := range variants {
		writer := renderSkillGolden(t, "writing-plans", data)
		reviewer := renderSkillGolden(t, "reviewing-plan", data)
		for surface, body := range map[string]string{"writer": writer, "reviewer": reviewer} {
			if strings.Contains(body, "<no value>") {
				t.Errorf("%s/%s leaks empty render data", variant, surface)
			}
		}
		assertOrderedPhrases(t, writer,
			"Review the uncommitted draft",
			"explicit plan path and selected working-tree snapshot before its first commit",
			"mechanical corrections without a durable ledger",
			"substantive reasoned or user-decided findings and dispositions in Notes",
			"one settled initial",
			"Later substantive corrections are separate commits",
		)
		if strings.Contains(writer, "Commit the plan as soon as it is written") {
			t.Errorf("%s writer creates a pre-review plan commit", variant)
		}
		assertOrderedPhrases(t, reviewer,
			"explicit uncommitted plan path and selected working-tree snapshot",
			"review that snapshot rather than HEAD",
			"mechanical fixes directly without a durable ledger",
			"substantive reasoned or user-decided findings and dispositions in plan Notes",
			"one initial plan commit",
			"Later substantive corrections remain separate commits",
			"every actual commit uses staged checks and the full gate",
			"exactly one fresh `plan-reviewer` verify pass",
		)
		for _, reportOnly := range []string{"report-only judge", "Do not ask the reviewer to edit, commit, or re-review"} {
			if !strings.Contains(reviewer, reportOnly) {
				t.Errorf("%s reviewer missing report-only contract %q", variant, reportOnly)
			}
		}
		if strings.Contains(reviewer, "most recently-modified") || strings.Contains(reviewer, "review fixes land as new commits on top of the committed plan") {
			t.Errorf("%s reviewer retains committed-plan detection or correction choreography", variant)
		}
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
	root := testsupport.RepoRoot(t)
	for _, surface := range []struct {
		name string
		body string
	}{
		{"default writing skill", defaultWriter},
		{"default reviewing skill", defaultReviewer},
		{"rendered writing skill", readPlanPolicyFile(t, root, ".pi/skills/awf-writing-plans/SKILL.md")},
		{"rendered reviewing skill", readPlanPolicyFile(t, root, ".pi/skills/awf-reviewing-plan/SKILL.md")},
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

// invariant: rendering/workflow-skill-templates:authority-guided-implementation-autonomy (TestPlanDeviationReconciliationGuidanceStayAligned)
func TestPlanDeviationReconciliationGuidanceStayAligned(t *testing.T) {
	data := map[string]any{"prefix": "example", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{}, "skills": map[string]bool{}}
	root := testsupport.RepoRoot(t)
	surfaces := map[string]string{
		"default writing skill": renderSkillGolden(t, "writing-plans", data),
		"default plans README":  renderGolden(t, "plans-readme/README.md.tmpl", data),
		"default plan scaffold": renderGolden(t, "plans-template/template.md.tmpl", data),
		"Pi writing skill":      readPlanPolicyFile(t, root, ".pi/skills/awf-writing-plans/SKILL.md"),
		"Claude writing skill":  readPlanPolicyFile(t, root, ".claude/skills/awf-writing-plans/SKILL.md"),
		"root plans README":     readPlanPolicyFile(t, root, "docs/plans/README.md"),
		"root plan scaffold":    readPlanPolicyFile(t, root, "docs/plans/template.md"),
	}
	for name, body := range surfaces {
		normalized := strings.Join(strings.Fields(body), " ")
		assertOrderedPhrases(t, normalized,
			"Inline owners immediately",
			"reasoned deviations",
			"Delegated owners",
			"report rather than edit",
			"phase review",
			"focused",
			"settlement commit",
			"before checkpointing or later execution",
		)
		if strings.Contains(normalized, "<no value>") {
			t.Errorf("%s leaks missing template data", name)
		}
	}
}
