package evals

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// invariant: rendering/workflow-skill-templates:independent-workflow-escalation (TestIndependentWorkflowEscalation)
// invariant: rendering/workflow-skill-templates:memory-checkpoint-chain-coverage (TestIndependentWorkflowEscalation)
// invariant: rendering/workflow-skill-templates:unified-effort-workflow-coverage (TestIndependentWorkflowEscalation)
// invariant: rendering/workflow-skill-templates:effort-workflow (TestIndependentWorkflowEscalation)
// invariant: rendering/workflow-skill-templates:workflow-transitions-advisory (TestIndependentWorkflowEscalation)
// invariant: rendering/workflow-skill-templates:phase-transaction-ownership (TestIndependentWorkflowEscalation)
// invariant: rendering/pi-workflows:pi-session-handoff-workflow (TestIndependentWorkflowEscalation)
func TestIndependentWorkflowEscalation(t *testing.T) {
	cat := loadCatalog(t)
	allNames := make([]string, 0, len(cat.Skills))
	for name := range cat.Skills {
		allNames = append(allNames, name)
	}
	sort.Strings(allNames)

	for _, target := range []string{"pi", "claude"} {
		t.Run(target, func(t *testing.T) {
			root := syncFullCatalogForTarget(t, cat, target)
			bodies := map[string]string{}
			for _, name := range allNames {
				path := skillPath(root, name)
				if target == "pi" {
					path = filepath.Join(root, ".pi", "skills", evalPrefix+"-"+name, "SKILL.md")
				}
				bodies[name] = read(t, path)
			}

			assertContainsAll(t, target+" brainstorming", bodies["brainstorming"],
				"material choice or clarification", "does not create an effort", "invoke `"+evalPrefix+"-grounding`", "explicitly request approval", "stop")
			if strings.Contains(bodies["brainstorming"], "grounding-checker") || strings.Contains(bodies["brainstorming"], "awf effort new") {
				t.Errorf("%s brainstorming owns grounding dispatch or effort creation", target)
			}

			assertContainsAll(t, target+" grounding", bodies["grounding"],
				"broad or uncertain repository premises", "advisory, report-only, single-pass, effort-noncreating", "never a workflow-chain prerequisite", "mechanical, reasoned, or user-decision")
			if strings.Contains(bodies["grounding"], "awf effort new") {
				t.Errorf("%s grounding creates an effort", target)
			}

			effort := bodies["effort-workflow"]
			normalizedEffort := strings.Join(strings.Fields(effort), " ")
			assertContainsAll(t, target+" effort-workflow", effort,
				"sole owner of the effort lifecycle", "durable continuity materially helps", "choose a faithful outcome, title, and canonical short slug", "awf effort new --slug", "report the allocated immutable identity", "managed worktree", "resumable checkpoint", "transfer necessary context", "repository identity and worktree state", "awf effort integrate <slug>", "awf effort worktree remove <slug>", "awf effort finish <slug>", "divergent result", "before any topology removal")
			const dispositionDecision = "When a different unfinished effort is active, reason why the new outcome needs separate ownership and whether the old effort remains resumable or is intentionally discontinued."
			if !strings.Contains(normalizedEffort, dispositionDecision) {
				t.Errorf("%s effort-workflow does not require deliberate disposition of the old effort", target)
			}
			withoutDispositionDecision := strings.Replace(normalizedEffort,
				"whether the old effort remains resumable or is intentionally discontinued",
				"skip deciding the old effort's disposition", 1)
			if strings.Contains(withoutDispositionDecision, dispositionDecision) {
				t.Errorf("%s effort-workflow disposition oracle accepted a missing decision", target)
			}
			assertOrderedPhrases(t, normalizedEffort,
				"When a different unfinished effort is active", "reason why the new outcome needs separate ownership",
				"whether the old effort remains resumable or is intentionally discontinued", "For a kept effort",
				"resumable checkpoint before switching")
			assertOrderedPhrases(t, normalizedEffort,
				"For a discontinued effort", "transfer necessary context", "inspect the repository identity and worktree state",
				"obsolete dirty or unmerged topology", "existing native Git safety primitives explicitly",
				"In either case, complete ordinary topology removal", "finish it through the ordinary archive lifecycle")
			cleanOnlyFinish := strings.Replace(normalizedEffort,
				"In either case, complete ordinary topology removal",
				"For clean topology only, complete ordinary topology removal", 1)
			if hasOrderedPhrases(cleanOnlyFinish,
				"For a discontinued effort", "transfer necessary context", "inspect the repository identity and worktree state",
				"obsolete dirty or unmerged topology", "existing native Git safety primitives explicitly",
				"In either case, complete ordinary topology removal", "finish it through the ordinary archive lifecycle") {
				t.Errorf("%s effort-workflow dirty-topology oracle accepted a clean-only finish", target)
			}
			if got := strings.Count(effort, "**Autonomous effort creation.**"); got != 1 {
				t.Errorf("%s effort-workflow autonomous creation contract count = %d, want 1", target, got)
			}
			if got := strings.Count(effort, "## Deliberate switching"); got != 1 {
				t.Errorf("%s effort-workflow switching section count = %d, want 1", target, got)
			}
			for _, name := range allNames {
				if name == "effort-workflow" {
					continue
				}
				for _, forbidden := range []string{"awf effort new --slug", "awf effort integrate <slug>", "awf effort worktree remove <slug>", "awf effort finish <slug>"} {
					if strings.Contains(bodies[name], forbidden) {
						t.Errorf("%s %s duplicates effort lifecycle command %q", target, name, forbidden)
					}
				}
				if strings.Contains(bodies[name], "**Autonomous effort creation.**") {
					t.Errorf("%s %s duplicates the effort-workflow creation contract", target, name)
				}
				if strings.Contains(bodies[name], "## Deliberate switching") {
					t.Errorf("%s %s duplicates the effort-workflow switching section", target, name)
				}
			}

			for _, name := range []string{"proposing-adr", "adr-lifecycle", "writing-plans", "reviewing-adr", "reviewing-plan", "refactor-coupling-audit", "executing-plans", "subagent-driven-development"} {
				assertContainsAll(t, target+" "+name, bodies[name], "may run without an effort", "otherwise omit effort and memory fields")
			}

			for _, name := range []string{"executing-direct", "executing-plans", "subagent-driven-development"} {
				body := bodies[name]
				assertContainsAll(t, target+" "+name,
					body, "locally obvious, low-risk, directly verified", "effort-free", "effort-backed", "review")
			}
			direct := bodies["executing-direct"]
			assertContainsAll(t, target+" direct independent triggers", direct,
				"no independent need for brainstorming", "material choice or clarification", "sequencing, coordination, or resumability", "only when that independent need fires")
			for _, obsolete := range []string{"only after brainstorming has settled", "becomes multi-step or interdependent"} {
				if strings.Contains(direct, obsolete) {
					t.Errorf("%s direct execution retains bundled trigger %q", target, obsolete)
				}
			}
			for _, name := range []string{"executing-plans", "subagent-driven-development"} {
				body := bodies[name]
				assertContainsAll(t, target+" "+name+" autonomous phase progression", body,
					"continue the plan loop without returning control to the user", "select the next unfinished phase", "A phase-complete report is not a plan-complete stopping point")
				assertOrderedPhrases(t, body, "**Routine checkpoint.**", "continue the plan loop without returning control to the user", "After all settled phases")
				assertContainsAll(t, target+" "+name+" closure", body,
					"assurance settles or is explicitly skipped", "effort-backed work returns", "effort-free work, the parent performs", "deferred ADR/plan terminal transaction", "adr-lifecycle", "gate and audit")
				if strings.Contains(body, "Terminal review owns") {
					t.Errorf("%s %s assigns artifact closure to review", target, name)
				}
			}
			assertContainsAll(t, target+" orienting", bodies["orienting"],
				"repository truth is needed", "Fresh work", "Effort resume", "Handoff takeover", "Mid-chain re-orientation")
			if strings.Contains(bodies["orienting"], "non-trivial") {
				t.Errorf("%s orienting retains non-trivial classifier", target)
			}
			review := bodies["reviewing-impl"]
			assertContainsAll(t, target+" review", review,
				"locally obvious, low-risk, directly verified", "meaningful breadth", "contract or compatibility effects", "migrations", "security", "concurrency", "data-loss risk", "Uncertainty resolves toward review", "Effort-free review creates no effort", "returns to `"+evalPrefix+"-effort-workflow`")
			for _, forbidden := range []string{"awf effort integrate", "awf adr number", "awf effort worktree remove", "awf effort finish", "Skipped (docs-only)"} {
				if strings.Contains(review, forbidden) {
					t.Errorf("%s reviewing-impl owns lifecycle or docs-only shortcut %q", target, forbidden)
				}
			}

			workflow := read(t, filepath.Join(root, "docs", "workflow.md"))
			assertContainsAll(t, target+" workflow", workflow,
				"evaluate brainstorming, continuity/effort, grounding, ADR, plan, and implementation-review need independently", "load-bearing", "sequencing, coordination, or resumability", "Each written artifact gets a fresh-context review", "line count", "artifact type")
			assertContainsAll(t, target+" workflow ownership", workflow,
				"No line count, artifact type, or another mechanism firing selects a trigger", "parent owns inline integration", "one independently green coherent implementation transaction", "A checkpoint never creates an effort", "routine implementation checkpoint occurs only after a phase's closing commit has received report-only review")
			// docs/workflow.md is a shared fixed-docs artifact, not a target output.
			// Target-specific workflow assertions above inspect only the requested
			// target's rendered skills and agents.
		})
	}
}

// invariant: rendering/workflow-skill-templates:mandatory-approval-boundaries (TestMandatoryApprovalBoundaries)
func TestMandatoryApprovalBoundaries(t *testing.T) {
	cat := loadCatalog(t)
	for _, target := range []string{"pi", "claude"} {
		root := syncFullCatalogForTarget(t, cat, target)
		path := func(name string) string {
			if target == "pi" {
				return filepath.Join(root, ".pi", "skills", evalPrefix+"-"+name, "SKILL.md")
			}
			return skillPath(root, name)
		}
		brainstorming := read(t, path("brainstorming"))
		assertContainsAll(t, target+" brainstorming approval", brainstorming, "before a hand-authored production-code change", "Present the complete design", "explicitly request approval", "stop", "clear later approval response")
		assertOrderedPhrases(t, brainstorming, "Present the complete design", "explicitly request approval", "stop", "clear later approval response")
		adrReview := read(t, path("reviewing-adr"))
		assertContainsAll(t, target+" autonomous ADR hand-off", adrReview,
			"After the review settles", "run `awf context --show references", "ordinary `"+evalPrefix+"-reviewing-plan`", "If no linked Proposed plan exists yet", "proceed directly to implementation")
		if strings.Contains(adrReview, "Stop for approval") || strings.Contains(adrReview, "settled ADR is the mandatory approval check-in") {
			t.Errorf("%s ADR review retains a settled-ADR approval stop", target)
		}
		for _, name := range []string{"proposing-adr", "writing-plans", "reviewing-plan", "executing-plans", "reviewing-impl"} {
			body := read(t, path(name))
			if strings.Contains(body, "settled ADR is the mandatory approval check-in") || strings.Contains(body, "Stop for approval, then") {
				t.Errorf("%s %s retains a downstream routine approval stop", target, name)
			}
		}
		writingPlans := read(t, path("writing-plans"))
		assertContainsAll(t, target+" plan-scope routing", writingPlans,
			"Use repository authority to settle local file structure and phase shape inside the approved boundary",
			"new material decision or would change the approved boundary", "invoke `"+evalPrefix+"-brainstorming`",
			"do not ask the user directly from plan writing")
		checkpoint := read(t, path("executing-plans"))
		assertContainsAll(t, target+" checkpoint routing", checkpoint,
			"Route a new material decision or changed approved boundary through the active workflow to brainstorming",
			"Separately, report a correctness or safety concern, blocker, or failed required verification through the active workflow",
			"remains unresolved after that workflow's required diagnosis and authority-guided remediation")
		bugfix := read(t, path("bugfix"))
		assertContainsAll(t, target+" bugfix larger-work routing", bugfix,
			"For materially larger work, route the disposition through `"+evalPrefix+"-brainstorming`")
		tdd := read(t, path("tdd"))
		assertContainsAll(t, target+" TDD larger-work routing", tdd,
			"Route materially larger work through `"+evalPrefix+"-brainstorming`")
		workflow := read(t, filepath.Join(root, "docs", "workflow.md"))
		assertContainsAll(t, target+" workflow checkpoint routing", workflow,
			"routes a new material decision or changed approved boundary through the active workflow to brainstorming",
			"separately reporting a correctness or safety concern, blocker, or failed required verification through the active workflow",
			"only when it remains unresolved after required diagnosis and authority-guided remediation")
		directRoutes := map[string]string{
			"writing-plans":  writingPlans,
			"checkpoint":     checkpoint,
			"bugfix":         bugfix,
			"tdd":            tdd,
			"workflow-guide": workflow,
		}
		for _, direct := range []string{
			"Confirm scope with the user",
			"Decide whether user attention is required",
			"material authority drift, a materially different choice, significant scope expansion",
			"For materially larger work, ask the user whether to",
			"Escalate materially larger work by asking the user whether to",
			"authority drift, materially changed choices, scope expansion",
		} {
			for surface, body := range directRoutes {
				if strings.Contains(body, direct) {
					t.Errorf("%s %s retains direct user route %q", target, surface, direct)
				}
			}
		}
		effort := read(t, path("effort-workflow"))
		normalizedEffort := strings.Join(strings.Fields(effort), " ")
		assertContainsAll(t, target+" autonomous effort creation", effort, "durable continuity materially helps", "choose a faithful outcome, title, and canonical short slug", "awf effort new --slug <slug>", "report the allocated immutable identity", "continue there")
		const noCreationAuthorization = "No user confirmation, later response, turn-ending authorization, or repeated authorization precedes creation."
		if !strings.Contains(normalizedEffort, noCreationAuthorization) {
			t.Errorf("%s effort-workflow does not prohibit creation authorization as one relation", target)
		}
		laterAuthorizationPrecedesCreation := strings.Replace(normalizedEffort, "No user confirmation,", "No user confirmation.", 1)
		if strings.Contains(laterAuthorizationPrecedesCreation, noCreationAuthorization) {
			t.Errorf("%s effort-workflow authorization oracle accepted later authorization before creation", target)
		}
		assertOrderedPhrases(t, effort, "durable continuity materially helps", "choose a faithful outcome, title, and canonical short slug", "awf effort new --slug <slug>", "report the allocated immutable identity", "continue there")
		for _, obsolete := range []string{
			"clear response in a " +
				"later turn",
			"reconfirm after " +
				"context loss",
			"Mandatory first-creation confirmation",
		} {
			if strings.Contains(effort, obsolete) {
				t.Errorf("%s effort-workflow retains obsolete creation policy %q", target, obsolete)
			}
		}
	}
}

// invariant: rendering/workflow-skill-templates:explorer-and-grounding-role-contracts (TestGroundingSupportOwnership)
// invariant: rendering/pi-workflows:pi-dedicated-grounding-dispatch (TestGroundingSupportOwnership)
func TestGroundingSupportOwnership(t *testing.T) {
	cat := loadCatalog(t)
	pi := syncFullCatalogForTarget(t, cat, "pi")
	claude := syncFullCatalogForTarget(t, cat, "claude")
	piGrounding := read(t, filepath.Join(pi, ".pi", "skills", evalPrefix+"-grounding", "SKILL.md"))
	claudeGrounding := read(t, skillPath(claude, "grounding"))
	if !strings.Contains(piGrounding, "subagent_grounding") {
		t.Error("Pi grounding does not use the dedicated grounding tool")
	}
	if strings.Contains(claudeGrounding, "subagent_grounding") {
		t.Error("Claude grounding leaks Pi tool name")
	}
	assertContainsAll(t, "Claude grounding", claudeGrounding, "grounding-checker", "guide-first", "AWF_CONTEXT_SPILL_V1")
}

// invariant: rendering/workflow-skill-templates:independent-workflow-escalation (TestProductionCodeOutlineApproval)
func TestProductionCodeOutlineApproval(t *testing.T) {
	cat := loadCatalog(t)
	for _, target := range []string{"pi", "claude"} {
		t.Run(target, func(t *testing.T) {
			root := syncFullCatalogForTarget(t, cat, target)
			path := func(name string) string {
				if target == "pi" {
					return filepath.Join(root, ".pi", "skills", evalPrefix+"-"+name, "SKILL.md")
				}
				return skillPath(root, name)
			}
			brainstorming := read(t, path("brainstorming"))
			assertContainsAll(t, target+" outline owner", brainstorming,
				"description: Use before hand-authored production-code changes", "concise implementation outline", "fuller material-choice design", "explicit outline approval")

			for _, name := range []string{"executing-direct", "tdd", "bugfix", "executing-plans", "subagent-driven-development"} {
				assertContainsAll(t, target+" "+name+" outline intake", read(t, path(name)),
					"hand-authored production-code", "mechanical production refactors", "tests that prepare a production change", "explicit outline approval")
			}
			for _, name := range []string{"writing-plans", "proposing-adr"} {
				assertContainsAll(t, target+" "+name+" artifact intake", read(t, path(name)),
					"explicit outline approval", "Architecture summary")
			}
			implementer := read(t, filepath.Join(root, "."+target, "agents", "implementer.md"))
			assertContainsAll(t, target+" delegated intake", implementer,
				"parent-supplied approved boundary", "never recreate the approval interaction", "stops without mutation to report missing evidence to its parent")
			assertContainsAll(t, target+" phase-owner approved-boundary dispatch", read(t, path("subagent-driven-development")),
				"provide the complete phase, explicitly identify the parent-supplied approved boundary")
			assertContainsAll(t, target+" helper approved-boundary dispatch", read(t, path("executing-plans")),
				"Each brief explicitly identifies the parent-supplied approved boundary")
			for _, name := range []string{"executing-direct", "tdd", "bugfix", "executing-plans", "subagent-driven-development", "proposing-adr"} {
				body := read(t, path(name))
				assertContainsAll(t, target+" "+name+" evidence", body,
					"retained conversation", "Decision-log evidence", "explicit request to execute a named plan", "Architecture summary")
			}
			for _, exclusion := range []string{"Documentation-only", "test-only maintenance", "generated-output-only", "non-code mechanical"} {
				if !strings.Contains(read(t, path("executing-direct")), exclusion) {
					t.Errorf("%s direct intake omits autonomous exclusion %q", target, exclusion)
				}
			}
		})
	}
}

func assertOrderedPhrases(t *testing.T, body string, wants ...string) {
	t.Helper()
	position := 0
	for _, want := range wants {
		next := strings.Index(body[position:], want)
		if next < 0 {
			t.Fatalf("expected %q after byte %d:\n%s", want, position, body)
		}
		position += next + len(want)
	}
}

func hasOrderedPhrases(body string, wants ...string) bool {
	position := 0
	for _, want := range wants {
		next := strings.Index(body[position:], want)
		if next < 0 {
			return false
		}
		position += next + len(want)
	}
	return true
}

func assertContainsAll(t *testing.T, label, body string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("%s missing %q", label, want)
		}
	}
	if strings.Contains(body, "<no value>") || strings.Contains(body, "%!") {
		t.Errorf("%s leaked template token", label)
	}
}
