package project

import (
	"maps"
	"os"
	"path/filepath"
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
				"fresh context per task", "continue Task X", "recorded plan phases are immutable",
			} {
				if strings.Contains(output, forbidden) {
					t.Errorf("%s/%s retains forbidden %q", variant, name, forbidden)
				}
			}
			if !strings.Contains(output, "merge, split, reorder, add, remove, or replace recorded route detail") {
				t.Errorf("%s/%s does not permit pre-execution route regrouping", variant, name)
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
			"**Rule.**", "**Flexible helper detail.**", "**Runtime detail.**", "**Required helper evidence.**", "**Required phase evidence.**", "**Freshness stop.**",
			"mixed plan can hand subagent-driven phases", "Iterate phases, not tasks",
			"continue the plan loop without returning control to the user", "select the next unfinished phase", "A phase-complete report is not a plan-complete stopping point",
			"awf read plan <plan> <P[.T]>", "generated task scope notice", "phase-owner context only", "Advances and Completes outcomes",
			"never gives a task helper commit, review, checkpoint, handoff, or outcome authority",
			"projection changes neither phase ownership nor checkpoint boundaries",
			"parent owns every ordered task", "sequential commit-disabled helpers",
			"assigned scope, canonical checkout", "starting and ending HEAD", "`git status --short`", "changed paths",
			"exact focused command with cwd, argv, exit status, and actual result", "completed and remaining work", "deviations",
			"separately routed blockers", "applicable generated-output or fixture evidence",
			"checkout identity, relevant authority, and overlapping paths remain unchanged", "lost or unverifiable evidence", "changed relevant authority", "later overlapping mutation invalidates the affected receipt",
			"reject out-of-subset writes", "retain shared-file ownership", "report-only phase review",
			"Review settles before checkpointing", "**Routine checkpoint.**", "completed and remaining work",
			"complete inline", "restore the known green baseline", "stop for required user input",
			"complete revised phase", "recovery verification", "blind successor instruction")
		assertAll("subagent",
			"plan may mix modes", "hand inline phases", "known clean and green baseline",
			"continue the plan loop without returning control to the user", "select the next unfinished phase", "A phase-complete report is not a plan-complete stopping point",
			"awf read plan <plan> <P>", "generated scope notice, Phase close, and Advances/Completes outcomes are phase-owner context only",
			"never transfer commit, review, checkpoint, handoff, helper, or outcome authority", "projection changes neither ownership nor checkpoint boundaries",
			"commit-disabled implementation child alone", "parent-supplied approved boundary", "complete phase",
			"assigned scope, canonical checkout", "starting and ending HEAD", "`git status --short`", "changed paths", "exact focused command with cwd, argv, exit status, and actual result",
			"completed and remaining work", "deviations", "separately routed blockers", "applicable generated-output or fixture evidence",
			"parent independently inventories the checkout", "relevant authority, and overlapping paths remain unchanged", "lost or unverifiable evidence", "changed relevant authority", "later overlapping mutation invalidates the affected receipt",
			"declared phase-closing commit", "awf check staged", gatePhrase,
			"exact phase-closing commit", "complete phase scope", "verification results", "verbatim deviation report",
			"correctness, plan/authority, documentation, and maintainability lenses", "structured coverage summary",
			"reviewed scope and range", "freshness against the current branch tip", "any unreviewed settlement",
			"parent owns this transient evidence", "Evidence loss after context loss, session replacement, or effort-free continuation", "unverifiable freshness", "falls back to ordinary terminal review",
			"Divergence, changed authority, reasoned post-review fixes, or any material mutation invalidates affected coverage",
			"When deviations or findings exist", "focused post-review settlement commit", "plan Notes reconciliation",
			"before checkpointing or later execution", "never rewrites the parent-owned phase-closing commit",
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
			"inventory the completed child receipt",
			"Build the phase-review brief",
			"Dispatch one report-only phase review",
			"structured coverage summary",
			"focused post-review settlement commit",
			"plan Notes reconciliation",
			"before checkpointing or later execution",
			"**Routine checkpoint.**",
		)
		if got := strings.Count(subagent, "declared phase-closing commit"); got != 1 {
			t.Errorf("%s/subagent renders %d parent-owned phase-closing commit declarations, want one", variant, got)
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
		for _, want := range []string{
			"Effort-backed pre-integration implementation dispatches supply",
			"managed-worktree path as `verificationCheckout`",
			"effort-free or root-owned work intentionally omits it",
			"child CWD and before-and-after HEAD identity",
			"main Pi session remains at the project root",
			"parent mutations name managed-worktree paths explicitly",
			"Child CWD alignment is not filesystem confinement",
			"Governed primary-checkout lifecycle transitions remain at the primary checkout",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("Pi %s guidance missing semantic clause %q", name, want)
			}
		}
		for _, forbidden := range []string{
			"parent and child Pi CWD", "shared session CWD",
			"Parent and child Pi CWD do not become",
		} {
			if strings.Contains(body, forbidden) {
				t.Errorf("Pi %s guidance retains malformed or contradictory wording %q", name, forbidden)
			}
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

// invariant: rendering/workflow-skill-templates:phase-transaction-ownership (TestSelfHostedAuditLocalOverrides)
func TestSelfHostedAuditLocalOverrides(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	for _, path := range []string{
		".awf/skills/parts/executing-plans/terminal-step.md",
		".awf/skills/parts/reviewing-impl/run-audit.md",
		".awf/skills/parts/subagent-driven-development/terminal-step.md",
		".claude/skills/awf-executing-plans/SKILL.md",
		".claude/skills/awf-reviewing-impl/SKILL.md",
		".claude/skills/awf-subagent-driven-development/SKILL.md",
		".pi/skills/awf-executing-plans/SKILL.md",
		".pi/skills/awf-reviewing-impl/SKILL.md",
		".pi/skills/awf-subagent-driven-development/SKILL.md",
	} {
		body, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "./x audit-local") {
			t.Errorf("self-hosted audit surface %s lost the repository-local audit", path)
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
			"For generated-prose changes, perform the focused meaning review",
			"parent stages the complete transaction",
			"Before review, build the phase-review brief",
			"exact phase-closing commit and range",
			"verbatim deviation report built from the parent's inventory",
			"Then dispatch report-only phase review",
		)
		if !strings.Contains(surfaces["implementer"], "focused generated-prose meaning-review") {
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
		for _, want := range []string{"**Build one self-contained evidence brief.**", "**Required evidence.**", "**Flexible reuse.**", "**Freshness stop.**", "**Effort evidence.**", "**Authority context.**", "**Run at most one verify pass.**", "**Stop when:**"} {
			if !strings.Contains(surfaces["terminal assurance"], want) {
				t.Errorf("%s/terminal assurance missing execution-clarity marker %q", variant, want)
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
		if !strings.Contains(surfaces["implementer"], "assigned scope performed and every changed path") {
			t.Errorf("%s implementer lacks structured completed scope", variant)
		}
		if strings.Contains(surfaces["code reviewer"], "apply fixes") {
			t.Errorf("%s code reviewer ceased to be report-only", variant)
		}
	}
}
