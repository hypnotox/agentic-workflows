package project

import (
	"maps"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

// invariant: rendering/workflow-skill-templates:phase-transaction-ownership (TestPhaseTransactionOwnershipAcrossWorkflowSurfaces)
// invariant: rendering/workflow-skill-templates:authority-guided-implementation-autonomy (TestPhaseTransactionOwnershipAcrossWorkflowSurfaces)
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
			"change-specific starting dependency", "generic clean and green baseline protocol",
			"one independently green coherent implementation transaction", "exhaustively assigns every affected site to the parent or exactly one helper",
			"path-disjoint", "shared files parent-owned", "mutating commands confined", "dead-code escape")
		assertAll("reviewer",
			"independent `inline` or `subagent-driven` ownership", "task-level transaction boundaries",
			"incomplete/overlapping partitions", "helper-owned shared files", "unconfined commands",
			"clean green baseline", "one coherent green transaction", "shared files parent-owned")
		assertAll("inline",
			"mixed plan can hand subagent-driven phases", "Iterate phases, not tasks",
			"continue the plan loop without returning control to the user", "select the next unfinished phase", "A phase-complete report is not a plan-complete stopping point",
			"awf read plan <plan> <P[.T]>", "generated task scope notice", "phase-owner context only", "Advances and Completes outcomes",
			"never gives a task helper commit, review, checkpoint, handoff, or outcome authority",
			"projection changes neither phase ownership nor checkpoint boundaries",
			"parent owns every ordered task", "sequential commit-disabled helpers",
			"reject out-of-subset writes", "retain shared-file ownership", "report-only phase review",
			"Review settles before checkpointing", "**Routine checkpoint.**", "completed and remaining work",
			"complete inline", "restore the known green baseline", "stop for required user input",
			"complete revised phase", "recovery verification", "blind successor instruction")
		assertAll("subagent",
			"plan may mix modes", "hand inline phases", "known clean and green baseline",
			"continue the plan loop without returning control to the user", "select the next unfinished phase", "A phase-complete report is not a plan-complete stopping point",
			"awf read plan <plan> <P>", "generated scope notice, Phase close, and Advances/Completes outcomes are phase-owner context only",
			"never transfer commit, review, checkpoint, handoff, helper, or outcome authority", "projection changes neither ownership nor checkpoint boundaries",
			"one implementation child alone", "state commit-capable phase-owner mode in the brief", "complete phase",
			"stages the complete transaction", "declared phase-closing commit", "awf check staged", gatePhrase,
			"exact phase-closing commit", "complete phase scope", "verification results", "verbatim deviation report",
			"correctness, plan/authority, documentation, and maintainability lenses", "structured coverage summary",
			"reviewed scope and range", "freshness against the current branch tip", "any unreviewed settlement",
			"parent owns this transient evidence", "Evidence loss after context loss, session replacement, or effort-free continuation", "unverifiable freshness", "falls back to ordinary terminal review",
			"Divergence, changed authority, reasoned post-review fixes, or any material mutation invalidates affected coverage",
			"When deviations or findings exist", "focused post-review settlement commit", "plan Notes reconciliation",
			"before checkpointing or later execution", "never rewrites the child phase-closing commit",
			"no-deviation, no-finding phase needs no empty settlement commit", "focused settlement commits",
			"**Routine checkpoint.**", "parent completion",
			"redispatch the complete revised phase", "stop for user input", "dirty-state inventory",
			"recovery verification", "blind successor instruction",
			"single phase reviews unreviewed settlement or integration", "multiple phases focus cross-phase composition, settlements, and integration",
			"awf audit", "complete final range", "including settlement commits")
		assertAll("readme",
			"exactly one execution mode: `inline` or `subagent-driven`", "one independently green coherent implementation transaction", "ordered steps",
			"change-specific dependencies", "generic baseline protocol",
			"parent or exactly one helper", "path-disjoint", "shared files parent-owned",
			"mutating commands confined to the assigned subset")
		assertAll("template",
			"format: plan-v2", "**Execution mode: inline.**", "Completes: [\"plan-outcome\"]", "### Task 1.1:",
			"### Phase close", "Name the one closing commit", "Generic staging, gate, clean-tree, checkpoint, routing, and reviewer protocol belongs to workflow owners",
			"```commit", "## Definition of done")

		for _, name := range []string{"inline", "subagent"} {
			body := surfaces[name]
			review := strings.Index(body, "report-only phase review")
			checkpoint := strings.Index(body, "**Routine checkpoint.**")
			continuation := strings.Index(body, "continue the plan loop without returning control to the user")
			terminal := strings.Index(body, "After all settled phases")
			if review < 0 || checkpoint < review {
				t.Errorf("%s/%s checkpoint is not after phase review settlement", variant, name)
			}
			if continuation < checkpoint {
				t.Errorf("%s/%s autonomous next-phase transition is not after the checkpoint", variant, name)
			}
			if terminal < continuation {
				t.Errorf("%s/%s terminal assurance is not after autonomous phase continuation", variant, name)
			}
			if strings.Count(body, "**Routine checkpoint.**") != 1 {
				t.Errorf("%s/%s has %d routine checkpoints, want one settled-phase checkpoint", variant, name, strings.Count(body, "**Routine checkpoint.**"))
			}
		}
		subagent := surfaces["subagent"]
		assertOrderedPhrases(t, subagent,
			"inventory the completed child report",
			"build the phase-review brief",
			"Dispatch one report-only phase review",
			"structured coverage summary",
			"focused post-review settlement commit",
			"plan Notes reconciliation",
			"before checkpointing or later execution",
			"**Routine checkpoint.**",
		)
		if got := strings.Count(subagent, "declared phase-closing commit"); got != 1 {
			t.Errorf("%s/subagent renders %d child phase-closing commit declarations, want one", variant, got)
		}
		phase := surfaces["template"]
		assertOrderedPhrases(t, phase, "Task 1.1", "Phase close", "Name the one closing commit", "Generic staging", "```commit", "Definition of done")
		for _, duplicated := range []string{"awf check staged", gatePhrase, "Stage the complete transaction"} {
			if strings.Contains(phase, duplicated) {
				t.Errorf("%s representative phase duplicates generic protocol %q", variant, duplicated)
			}
		}
		if got := strings.Count(phase, "```commit"); got != 1 {
			t.Errorf("%s representative multi-task phase has %d closing commit blocks, want one", variant, got)
		}
	}

	assertContract("configured", renderSurfaces(configured), "./x gate")
	assertContract("empty", renderSurfaces(empty), "the project's gate")
}

// invariant: rendering/workflow-skill-templates:phase-transaction-ownership (TestPiManagedWorktreeVerificationGuidance)
func TestPiManagedWorktreeVerificationGuidance(t *testing.T) {
	data := map[string]any{
		"prefix": "example", "vars": map[string]any{"gateCmd": "./x gate"},
		"layout": testLayout(), "data": map[string]any{}, "skills": map[string]bool{},
		"targetSubagentTools": true,
	}
	for name, body := range map[string]string{
		"inline":    renderSkillGolden(t, "executing-plans", data),
		"delegated": renderSkillGolden(t, "subagent-driven-development", data),
	} {
		for _, want := range []string{"verificationCheckout", "managed-worktree path", "actual mutation paths", "parent and child Pi CWD"} {
			if !strings.Contains(body, want) {
				t.Errorf("Pi %s guidance missing %q", name, want)
			}
		}
		if !strings.Contains(strings.ToLower(body), "omit `verificationcheckout` for root work") {
			t.Errorf("Pi %s guidance missing the complete root-work omission clause", name)
		}
	}
	claudeData := maps.Clone(data)
	claudeData["targetSubagentTools"] = false
	for name, body := range map[string]string{
		"inline":    renderSkillGolden(t, "executing-plans", claudeData),
		"delegated": renderSkillGolden(t, "subagent-driven-development", claudeData),
	} {
		if strings.Contains(body, "verificationCheckout") || strings.Contains(body, "parent and child Pi CWD") {
			t.Errorf("non-Pi %s guidance leaked Pi verification identity", name)
		}
	}
}

func TestFreshPhaseAssuranceReuseContract(t *testing.T) {
	variants := map[string]map[string]any{
		"configured": {
			"prefix": "example", "vars": map[string]any{"gateCmd": "./x gate"},
			"layout": testLayout(), "data": catalog.Standard.Agents["code-reviewer"].Data,
			"skills": map[string]bool{}, "targetSubagentTools": true,
		},
		"empty": {
			"prefix": "example", "vars": map[string]any{}, "layout": testLayout(),
			"data": map[string]any{}, "skills": map[string]bool{},
		},
	}
	for variant, data := range variants {
		surfaces := map[string]string{
			"inline executor":    renderSkillGolden(t, "executing-plans", data),
			"delegated executor": renderSkillGolden(t, "subagent-driven-development", data),
			"terminal assurance": renderSkillGolden(t, "reviewing-impl", data),
			"implementer":        renderAgentGolden(t, "implementer", data),
			"code reviewer":      renderAgentGolden(t, "code-reviewer", data),
		}
		for name, body := range surfaces {
			if strings.TrimSpace(body) == "" || strings.Contains(body, "<no value>") {
				t.Errorf("%s/%s is empty or leaks missing data", variant, name)
			}
		}
		assertOrderedPhrases(t, surfaces["inline executor"],
			"perform the focused meaning review",
			"Stage the complete transaction",
			"Before review, build the phase-review brief",
			"exact phase-closing commit and range",
			"verbatim deviation report built from the parent's inventory",
			"Then dispatch report-only phase review",
		)
		if !strings.Contains(surfaces["implementer"], "retain the inspected boundaries and result as completion evidence for your report") {
			t.Errorf("%s implementer lacks semantic-review completion evidence", variant)
		}
		for _, name := range []string{"inline executor", "delegated executor", "code reviewer"} {
			body := surfaces[name]
			for _, want := range []string{
				"exact phase-closing commit", "complete", "phase scope", "reviewed", "range",
				"verification results", "verbatim deviation report", "freshness", "branch tip", "unreviewed settlement",
			} {
				if !strings.Contains(strings.ToLower(body), strings.ToLower(want)) {
					t.Errorf("%s/%s missing structured phase evidence %q", variant, name, want)
				}
			}
		}
		for _, name := range []string{"inline executor", "delegated executor", "terminal assurance"} {
			body := surfaces[name]
			for _, want := range []string{"Evidence loss", "unverifiable freshness", "ordinary terminal review", "Divergence", "changed authority", "reasoned post-review fixes", "material mutation", "invalidates affected coverage"} {
				if !strings.Contains(body, want) {
					t.Errorf("%s/%s missing fallback or invalidation clause %q", variant, name, want)
				}
			}
		}
		for _, name := range []string{"inline executor", "delegated executor", "terminal assurance"} {
			body := surfaces[name]
			for _, want := range []string{"single phase", "unreviewed settlement", "multiple phases", "cross-phase", "settlement", "integration", "awf audit", "complete final", "including settlement commits"} {
				if !strings.Contains(body, want) {
					t.Errorf("%s/%s missing reuse or audit clause %q", variant, name, want)
				}
			}
			if strings.Contains(body, "audit-local") || strings.Contains(body, "cmd/repoaudit") {
				t.Errorf("%s/%s leaks awf repository-local audit instructions", variant, name)
			}
		}
		if !strings.Contains(surfaces["implementer"], "complete phase scope performed and every changed path") {
			t.Errorf("%s implementer lacks structured completed scope", variant)
		}
		if strings.Contains(surfaces["code reviewer"], "apply fixes") {
			t.Errorf("%s code reviewer ceased to be report-only", variant)
		}
	}
}
