package project

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

// invariant: rendering/workflow-skill-templates:phase-transaction-ownership (TestPhaseTransactionOwnershipAcrossWorkflowSurfaces)
func TestPhaseTransactionOwnershipAcrossWorkflowSurfaces(t *testing.T) {
	renderSurfaces := func(data map[string]any) map[string]string {
		t.Helper()
		normalize := func(body string) string { return strings.Join(strings.Fields(body), " ") }
		return map[string]string{
			"writer":   normalize(renderSkillGolden(t, "writing-plans", data)),
			"reviewer": normalize(renderAgentGolden(t, "plan-reviewer", data)),
			"inline":   normalize(renderSkillGolden(t, "executing-plans", data)),
			"subagent": normalize(renderSkillGolden(t, "subagent-driven-development", data)),
			"readme":   normalize(renderGolden(t, "plans-readme/README.md.tmpl", data)),
			"template": normalize(renderGolden(t, "plans-template/template.md.tmpl", data)),
		}
	}
	configured := map[string]any{
		"prefix": "example", "vars": map[string]any{"gateCmd": "./x gate"},
		"layout": testLayout(), "data": catalog.Standard.Agents["plan-reviewer"].Data,
	}
	empty := map[string]any{
		"prefix": "example", "vars": map[string]any{}, "layout": testLayout(),
		"data": map[string]any{}, "skills": map[string]bool{},
	}
	assertContract := func(variant string, surfaces map[string]string, gatePhrase string) {
		t.Helper()
		for name, output := range surfaces {
			if strings.TrimSpace(output) == "" {
				t.Errorf("%s/%s rendered empty", variant, name)
			}
			for _, forbidden := range []string{
				"<no value>", "{{", "one commit per task", "one subagent per task",
				"fresh context per task", "continue Task X",
			} {
				if strings.Contains(output, forbidden) {
					t.Errorf("%s/%s retains forbidden %q", variant, name, forbidden)
				}
			}
		}
		assertAll := func(name string, clauses ...string) {
			t.Helper()
			for _, clause := range clauses {
				if !strings.Contains(surfaces[name], clause) {
					t.Errorf("%s/%s missing %q", variant, name, clause)
				}
			}
		}
		for _, name := range []string{"writer", "readme", "template"} {
			for _, forbidden := range []string{"coupled-phase", "coupled phase"} {
				if strings.Contains(surfaces[name], forbidden) {
					t.Errorf("%s/%s retains removed exception %q", variant, name, forbidden)
				}
			}
		}
		assertAll("writer",
			"exactly one execution mode: `inline` or `subagent-driven`", "independently", "ordered steps",
			"clean and green starting baseline", "exact commands and expected terminal states",
			"one independently green coherent implementation transaction", "exhaustively assigns every affected site to the parent or exactly one helper",
			"path-disjoint", "shared files parent-owned", "mutating commands confined", "dead-code escape")
		assertAll("reviewer",
			"independent `inline` or `subagent-driven` ownership", "task-level transaction boundaries",
			"incomplete/overlapping partitions", "helper-owned shared files", "unconfined commands",
			"clean green baseline", "one coherent green transaction", "shared files parent-owned")
		assertAll("inline",
			"mixed plan can hand subagent-driven phases", "Iterate phases, not tasks",
			"awf read plan <plan> <P[.T]>", "projection changes neither phase ownership nor checkpoint boundaries",
			"parent owns every ordered task", "sequential commit-disabled helpers",
			"reject out-of-subset writes", "retain shared-file ownership", "report-only phase review",
			"Review settles before checkpointing", "**Routine checkpoint.**", "completed and remaining work",
			"complete inline", "restore the known green baseline", "stop for required user input",
			"complete revised phase", "recovery verification", "blind successor instruction")
		assertAll("subagent",
			"plan may mix modes", "hand inline phases", "known clean and green baseline",
			"awf read plan <plan> <P>", "projection changes neither ownership nor checkpoint boundaries",
			"one implementation child alone", "state commit-capable phase-owner mode in the brief", "complete phase",
			"stages the complete transaction", "declared phase-closing commit", "awf check --staged", gatePhrase,
			"report-only phase review", "focused settlement commits",
			"checkpoints only after findings resolve", "**Routine checkpoint.**", "parent completion",
			"redispatch the complete revised phase", "stop for user input", "dirty-state inventory",
			"recovery verification", "blind successor instruction")
		assertAll("readme",
			"exactly one execution mode: `inline` or `subagent-driven`", "one independently green coherent implementation transaction", "ordered steps",
			"clean and green starting baseline", "exact commands and expected terminal states",
			"parent or exactly one helper", "path-disjoint", "shared files parent-owned",
			"mutating commands confined to the assigned subset")
		assertAll("template",
			"format: plan-v1", "**Execution mode: inline.**", "### Task 1.1:",
			"### Phase close", "Stage the complete transaction", "one closing commit",
			"staged check and gate pass", "```commit", "## Definition of done")

		for _, name := range []string{"inline", "subagent"} {
			body := surfaces[name]
			review := strings.Index(body, "report-only phase review")
			checkpoint := strings.Index(body, "**Routine checkpoint.**")
			if review < 0 || checkpoint < review {
				t.Errorf("%s/%s checkpoint is not after phase review settlement", variant, name)
			}
			if strings.Count(body, "**Routine checkpoint.**") != 1 {
				t.Errorf("%s/%s has %d routine checkpoints, want one settled-phase checkpoint", variant, name, strings.Count(body, "**Routine checkpoint.**"))
			}
		}
		phase := surfaces["template"]
		assertOrderedPhrases(t, phase, "Task 1.1", "Phase close", "Stage the complete transaction", "```commit", "Definition of done")
		if got := strings.Count(phase, "staged check"); got != 1 {
			t.Errorf("%s representative phase has %d staged-check boundaries, want one", variant, got)
		}
		if got := strings.Count(phase, "```commit"); got != 1 {
			t.Errorf("%s representative multi-task phase has %d closing commit blocks, want one", variant, got)
		}
	}

	assertContract("configured", renderSurfaces(configured), "./x gate")
	assertContract("empty", renderSurfaces(empty), "the project's gate")
}
