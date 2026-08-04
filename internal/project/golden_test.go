package project

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/templates"
)

func TestADRReadmeDecisionRouting(t *testing.T) {
	out := renderGolden(t, "adr-readme/README.md.tmpl", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "skills": map[string]bool{}, "layout": testLayout(),
	})
	for _, want := range []string{
		"remains meaningful after implementation",
		"post-implementation",
		"counterfactual",
		"mechanism itself is load-bearing",
		"Implementation plan",
		"rollout inventories",
		"proof transactions",
		"Historical ADRs remain unchanged",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("ADR README missing decision-routing contract %q:\n%s", want, out)
		}
	}
	for _, residue := range []string{"<no value>", "{{", "}}"} {
		if strings.Contains(out, residue) {
			t.Errorf("ADR README contains publication residue %q:\n%s", residue, out)
		}
	}
}

func TestEndToEndGolden(t *testing.T) {
	assertV3ADRTemplatePublicationSafe(t)
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}

	agent, err := os.ReadFile(filepath.Join(root, ".claude/agents/code-reviewer.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(agent), "Independent fresh-context reviewer for example") {
		t.Errorf("agent not interpolated:\n%s", agent)
	}

	proposingADR, err := os.ReadFile("../../.pi/skills/awf-proposing-adr/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(proposingADR), "preserve exactly the frontmatter emitted by `awf new adr`") {
		t.Errorf("project proposing skill lost scaffold authority:\n%s", proposingADR)
	}
	adrReviewer, err := os.ReadFile("../../.pi/agents/adr-reviewer.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"post-implementation", "counterfactual", "reasoned finding"} {
		if !strings.Contains(string(adrReviewer), want) {
			t.Errorf("project ADR reviewer missing semantic routing %q:\n%s", want, adrReviewer)
		}
	}
	if strings.Contains(string(adrReviewer), "## Doc-currency checklist") {
		t.Errorf("project ADR reviewer retains implementation-inventory checklist:\n%s", adrReviewer)
	}

	// The review-discipline spine is spliced in from templates/partials via awf:include
	// (ADR-0052); its content must appear in the fully rendered agent.
	for _, want := range []string{"## Classification rules", "## Dedup rule", "Impl review complete"} {
		if !strings.Contains(string(agent), want) {
			t.Errorf("spine partial not spliced: missing %q in:\n%s", want, agent)
		}
	}

	adrReadme, err := os.ReadFile(filepath.Join(root, "docs/decisions/README.md"))
	if err != nil {
		t.Fatalf("adr-readme not rendered: %v", err)
	}
	if !strings.Contains(string(adrReadme), "remains meaningful after implementation") {
		t.Errorf("adr-readme lost decision routing:\n%s", adrReadme)
	}

	agentsGuide, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("agent guide not rendered: %v", err)
	}
	if !strings.Contains(string(agentsGuide), "Route settled content by authority lifetime") {
		t.Errorf("agent guide lost decision routing:\n%s", agentsGuide)
	}

	plansReadme, err := os.ReadFile(filepath.Join(root, "docs/plans/README.md"))
	if err != nil {
		t.Fatalf("plans-readme not rendered: %v", err)
	}
	if !strings.Contains(string(plansReadme), "Implementation Plans") || !strings.Contains(string(plansReadme), "implementation directives") {
		t.Errorf("plans-readme not interpolated:\n%s", plansReadme)
	}

	// The plans-template singleton renders plan-v2: its intrinsic marker,
	// narrative spine, heading-identified phase and task, Phase close commit
	// fence, Definition of done, optional Notes, no retired sections or task
	// checkboxes, stripped section-assembly markers, and no unresolved value.
	// invariant: adr-system/plan-artifacts:plans-template-taxonomy (TestEndToEndGolden)
	plansTemplate, err := os.ReadFile(filepath.Join(root, "docs/plans/template.md"))
	if err != nil {
		t.Fatalf("plans-template not rendered: %v", err)
	}
	assertPlanTemplateTaxonomy(t, string(plansTemplate))
	parseDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(parseDir, "template.md"), plansTemplate, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := plan.NewFile(parseDir, "Scaffold"); err != nil {
		t.Fatalf("scaffold from rendered template: %v", err)
	}
	parsed, err := plan.ParseDir(parseDir)
	if err != nil || len(parsed) != 1 || strings.TrimSpace(parsed[0].Goal) == "" || strings.TrimSpace(parsed[0].ArchitectureSummary) == "" || !strings.Contains(parsed[0].DefinitionOfDone, "- ") {
		t.Fatalf("rendered plan-v2 scaffold is not substantively parseable: plans=%#v err=%v", parsed, err)
	}
	for _, bad := range []string{"awf:section", "awf:end", "{{", "}}"} {
		if strings.Contains(string(plansTemplate), bad) {
			t.Errorf("plans-template leaked marker/token %q:\n%s", bad, plansTemplate)
		}
	}

	// A fresh check on the synced tree is clean.
	drift, err := p.Check(testContext(t))
	if err != nil || len(drift) != 0 {
		t.Errorf("expected clean check, got drift=%#v err=%v", drift, err)
	}
}

func assertPlanTemplateTaxonomy(t *testing.T, text string) {
	t.Helper()
	for _, problem := range planTemplateTaxonomyProblems(text) {
		t.Errorf("plans-template taxonomy: %s:\n%s", problem, text)
	}
}

func planTemplateTaxonomyProblems(text string) []string {
	var problems []string
	previous := -1
	for _, token := range []string{
		"format: plan-v2", "date:", "adrs:", "status:", "# Plan:", "## Goal",
		"## Architecture summary", "## Phase 1:", "**Execution mode: inline.**",
		"### Task 1.1:", "### Phase close", "```commit", "## Definition of done", "## Notes",
	} {
		at := strings.Index(text, token)
		if at < 0 {
			problems = append(problems, "missing "+token)
			continue
		}
		if at <= previous {
			problems = append(problems, "out-of-order "+token)
		}
		previous = at
	}
	for _, retired := range []string{"## File structure", "## Verification", "- [ ] **Task"} {
		if strings.Contains(text, retired) {
			problems = append(problems, "retired plan-v1 declaration remains: "+retired)
		}
	}
	const vocabulary = "recognized fields are `Kind`, `Latitude`, `Question`, `Applying`, `Context`, `Paths`, `Representative`, `Edge`, and `Post-check`"
	if !strings.Contains(text, vocabulary) {
		problems = append(problems, "missing exact task field vocabulary")
	}
	if !strings.Contains(text, "`Applying` and `Context` require nonempty JSON string arrays and are omitted rather than written as `[]`") {
		problems = append(problems, "missing Decision-array omission contract")
	}
	for name, substance := range map[string]string{
		"Goal":                 "State the outcome and, in one line, its non-goals.",
		"Architecture summary": "State the execution structure and dependency direction without repeating ADR rationale.",
		"Definition of done":   "- `dod: plan-outcome` State at least one concrete observable whole-plan end condition.",
	} {
		if !strings.Contains(text, substance) {
			problems = append(problems, "missing nonempty "+name+" substance")
		}
	}
	return problems
}

func TestPlanTemplateTaxonomyRejectsInversions(t *testing.T) {
	text := renderGolden(t, "plans-template/template.md.tmpl", map[string]any{
		"vars": map[string]any{}, "layout": testLayout(),
	})
	for _, mutation := range []struct {
		name, from, to string
	}{
		{"frontmatter format", "format: plan-v2", "format: legacy"},
		{"frontmatter date", "date:", "written:"},
		{"frontmatter adrs", "adrs:", "decisions:"},
		{"frontmatter status", "status:", "state:"},
		{"title", "# Plan:", "# Procedure:"},
		{"goal", "## Goal", "## Outcome"},
		{"architecture", "## Architecture summary", "## Design"},
		{"phase", "## Phase 1:", "## Batch 1:"},
		{"execution mode", "**Execution mode: inline.**", "**Owner: inline.**"},
		{"task", "### Task 1.1:", "### Step 1.1:"},
		{"phase close", "### Phase close", "### Finish"},
		{"commit fence", "```commit", "```text"},
		{"definition", "## Definition of done", "## Verification"},
		{"field vocabulary", "recognized fields are `Kind`, `Latitude`, `Question`, `Applying`, `Context`, `Paths`, `Representative`, `Edge`, and `Post-check`", "recognized fields are `Kind` and `Latitude`"},
		{"decision array omission", "`Applying` and `Context` require nonempty JSON string arrays and are omitted rather than written as `[]`", "`Applying` and `Context` may be empty arrays"},
		{"goal substance", "State the outcome and, in one line, its non-goals.", ""},
		{"architecture substance", "State the execution structure and dependency direction without repeating ADR rationale.", ""},
		{"definition bullet", "- `dod: plan-outcome` State at least one concrete observable whole-plan end condition.", ""},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			if !strings.Contains(text, mutation.from) {
				t.Fatalf("mutation source %q is absent", mutation.from)
			}
			mutated := strings.Replace(text, mutation.from, mutation.to, 1)
			if problems := planTemplateTaxonomyProblems(mutated); len(problems) == 0 {
				t.Fatalf("taxonomy accepted semantic inversion %q", mutation.to)
			}
		})
	}
}

func assertV3ADRTemplatePublicationSafe(t *testing.T) {
	t.Helper()
	out := renderGolden(t, "adr-template/template.md.tmpl", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "skills": map[string]bool{}, "layout": testLayout(),
	})
	implementing := strings.Index(out, "Implementing; content-sha256")
	applied := strings.Index(out, "Applied; operations")
	history := strings.Index(out, "## Status history\n")
	if !strings.Contains(out, "format: current-state-v4") || implementing < 0 || applied < implementing || history < applied {
		t.Fatalf("V4 lifecycle example is not publication-safe:\n%s", out)
	}
	tail := out[history:]
	if strings.Count(tail, "- YYYY-MM-DD:") != 1 || !strings.Contains(tail, "- YYYY-MM-DD: Proposed") {
		t.Fatalf("fresh Proposed Status history contains non-Proposed events:\n%s", tail)
	}
	for _, want := range []string{"remains meaningful after implementation", "paths, commands, task order, rollout batches, and ordinary test transactions"} {
		if !strings.Contains(out, want) {
			t.Fatalf("ADR template missing decision-routing contract %q:\n%s", want, out)
		}
	}
	for _, residue := range []string{"<no value>", "{{", "format: current-state-v1"} {
		if strings.Contains(out, residue) {
			t.Fatalf("empty-data V2 template contains %q:\n%s", residue, out)
		}
	}
}

func TestTemplateHashCoversExpandedSource(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	const tid = "agents/code-reviewer.md.tmpl"
	var got string
	for _, f := range files {
		if f.TemplateID == tid {
			got = f.TemplateHash
		}
	}
	if got == "" {
		t.Fatal("code-reviewer not rendered")
	}
	raw, err := fs.ReadFile(templates.FS, tid)
	if err != nil {
		t.Fatal(err)
	}
	// code-reviewer.md.tmpl carries awf:include directives, so its expanded source differs
	// from its raw bytes; TemplateHash must be over the expanded source (ADR-0052). A
	// regression to manifest.Hash(src) would make these equal.
	// invariant: rendering/render-engine:include-in-templatehash (TestTemplateHashCoversExpandedSource)
	if got == manifest.Hash(raw) {
		t.Error("TemplateHash equals raw-source hash; expected expanded-source hash")
	}
}
