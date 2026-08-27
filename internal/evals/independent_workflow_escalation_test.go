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
// invariant: rendering/pi-runtime:pi-session-handoff-workflow (TestIndependentWorkflowEscalation)
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
				"repository truth is needed", "Fresh work", "Effort resume", "Handoff takeover", "Mid-chain re-orientation",
				"Orientation does not authorize mutation", "change's size, including minimal, is never an exception")
			assertContainsAll(t, target+" direct routing and transaction", bodies["executing-direct"],
				"Load this skill before directly mutating files", "Complete one coherent transaction with verification and a commit")
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
				"Evaluate brainstorming, continuity/effort, grounding, ADR, plan, and implementation-review triggers independently at intake", "activate newly warranted support before further mutation", "each establish the approved boundary", "load-bearing", "sequencing, coordination, or resumability", "Each written artifact gets a fresh-context review", "line count", "artifact type")
			assertContainsAll(t, target+" workflow ownership", workflow,
				"No line count, artifact type, or another mechanism firing selects a trigger", "parent owns inline integration", "one independently green coherent implementation transaction", "A checkpoint never creates an effort", "routine implementation checkpoint occurs only after a phase's closing commit has received report-only review")
			for name, body := range bodies {
				for _, piOnly := range []string{"handoff_session", "[session context]", "Continue with effort <slug>."} {
					if strings.Contains(body, piOnly) {
						t.Errorf("%s %s leaks Pi-only session protocol %q", target, name, piOnly)
					}
				}
			}
			// docs/workflow.md is a shared fixed-docs artifact, not a target output.
			// Target-specific workflow assertions above inspect only the requested
			// target's rendered skills and agents.
		})
	}

	// This is the comprehensive cross-profile, cross-runtime proof for the
	// continuity claims marked above. The focused boundary oracle below retains
	// its narrower diagnostic role without carrying those proof markers.
	for _, profile := range []string{"core", "full"} {
		t.Run("continuity-"+profile, func(t *testing.T) {
			root := syncPlanFlexibilityProfile(t, profile)
			for _, target := range []string{"pi", "claude"} {
				t.Run(target, func(t *testing.T) {
					brainstorming := read(t, planSkillPath(root, target, "brainstorming"))
					effort := read(t, planSkillPath(root, target, "effort-workflow"))
					workflow := read(t, filepath.Join(root, "docs", "workflow.md"))

					assertContainsAll(t, target+" "+profile+" comprehensive brainstorming continuity", brainstorming,
						"does not create an effort or require one merely to begin brainstorming",
						"Evaluate it when brainstorming begins and whenever a continuity-relevant fact changes",
						"When independent entry continuity evaluation fires, immediately invoke", "may begin effort-free",
						"first settled material decision", "before proceeding further",
						"single-decision brainstorm may remain effort-free only when no independent continuity need fires")
					assertOrderedPhrases(t, brainstorming,
						"Evaluate it when brainstorming begins", "When independent entry continuity evaluation fires, immediately invoke",
						"first settled material decision", "create or resume ownership before proceeding further")
					for clause, mutation := range map[string]string{
						"entry timing":                strings.Replace(brainstorming, "When independent entry continuity evaluation fires, immediately invoke", "After entry continuity evaluation fires, invoke later", 1),
						"single-decision eligibility": strings.Replace(brainstorming, "only when no independent continuity need fires", "even when an independent continuity need fires", 1),
						"first-decision backstop":     strings.Replace(brainstorming, "create or resume ownership before proceeding further", "proceed further before creating or resuming ownership", 1),
					} {
						if strings.Contains(mutation, map[string]string{
							"entry timing":                "When independent entry continuity evaluation fires, immediately invoke",
							"single-decision eligibility": "only when no independent continuity need fires",
							"first-decision backstop":     "create or resume ownership before proceeding further",
						}[clause]) {
							t.Errorf("%s %s continuity mutation did not remove %s", target, profile, clause)
						}
					}

					assertContainsAll(t, target+" "+profile+" comprehensive late memory", effort,
						"initialize the owned memory from retained evidence before any handoff", "current outcome in `## Brief`",
						"required user-provenance and `Record:` evidence", "relevant observations in `## Observations`",
						"current phase and next action", "reconfirmed, not reconstructed", "Do not hand off until this initialization is complete")
					assertOrderedPhrases(t, effort, "initialize the owned memory from retained evidence before any handoff", "Do not hand off until this initialization is complete")
					for clause, mutation := range map[string]string{
						"Brief":                strings.Replace(effort, "current outcome in `## Brief`", "current outcome omitted", 1),
						"decision evidence":    strings.Replace(effort, "required user-provenance and `Record:` evidence", "decision evidence omitted", 1),
						"observation recovery": strings.Replace(effort, "relevant observations in `## Observations`", "observations omitted", 1),
						"phase recovery":       strings.Replace(effort, "current phase and next action", "phase omitted", 1),
						"reconfirmation":       strings.Replace(effort, "reconfirmed, not reconstructed", "reconstructed", 1),
					} {
						if strings.Contains(mutation, map[string]string{"Brief": "current outcome in `## Brief`", "decision evidence": "required user-provenance and `Record:` evidence", "observation recovery": "relevant observations in `## Observations`", "phase recovery": "current phase and next action", "reconfirmation": "reconfirmed, not reconstructed"}[clause]) {
							t.Errorf("%s %s late-memory mutation did not remove %s", target, profile, clause)
						}
					}

					assertContainsAll(t, target+" "+profile+" comprehensive fixed identity", effort,
						"Effort title and slug are fixed", "Refinements inside the owned outcome remain in that effort",
						"material outcome drift requires a different outcome", "fixed-identity successor", "Transfer necessary still-valid context",
						"verify the successor is resumable", "close the obsolete effort through the existing topology-safety and finish/archive lifecycle")
					assertOrderedPhrases(t, effort, "fixed-identity successor", "Transfer necessary still-valid context", "verify the successor is resumable", "close the obsolete effort")
					wrongArchiveOrder := strings.Replace(effort, "Transfer necessary still-valid context, verify the successor is resumable, then close the obsolete effort", "close the obsolete effort, then transfer necessary still-valid context and verify the successor is resumable", 1)
					if hasOrderedPhrases(wrongArchiveOrder, "Transfer necessary still-valid context", "verify the successor is resumable", "close the obsolete effort") {
						t.Errorf("%s %s fixed-identity mutation accepted archive before successor resumability", target, profile)
					}
					assertContainsAll(t, target+" "+profile+" workflow continuity", workflow,
						"Handoff is prohibited while required late-creation memory initialization remains incomplete", "Missing exact required user evidence must be reconfirmed, not reconstructed")
				})
			}
		})
	}
}

func TestBrainstormContinuityBoundary(t *testing.T) {
	for _, profile := range []string{"core", "full"} {
		t.Run(profile, func(t *testing.T) {
			root := syncPlanFlexibilityProfile(t, profile)
			for _, target := range []string{"pi", "claude"} {
				t.Run(target, func(t *testing.T) {
					brainstorming := read(t, planSkillPath(root, target, "brainstorming"))
					effort := read(t, planSkillPath(root, target, "effort-workflow"))
					workflow := read(t, filepath.Join(root, "docs", "workflow.md"))

					assertContainsAll(t, target+" "+profile+" brainstorming continuity", brainstorming,
						"Evaluate it when brainstorming begins and whenever a continuity-relevant fact changes",
						"may begin effort-free", "first settled material decision", "before proceeding further", "single-decision brainstorm may remain effort-free only when no independent continuity need fires")
					assertOrderedPhrases(t, brainstorming,
						"**Evaluate continuity independently.**", "**Orient in the topic.**",
						"**Clarify one question at a time.**", "**Present proportionate approaches.**")
					assertOrderedPhrases(t, brainstorming, "first settled material decision", "create or resume ownership before proceeding further")
					lateOwnership := strings.Replace(brainstorming, "create or resume ownership before proceeding further", "proceed further before creating or resuming ownership", 1)
					if hasOrderedPhrases(lateOwnership, "first settled material decision", "create or resume ownership before proceeding further") {
						t.Errorf("%s %s accepted continuation before ownership", target, profile)
					}

					assertContainsAll(t, target+" "+profile+" effort continuity", effort,
						"whenever continuity-relevant facts change", "continuing after its first settled material decision requires creation or resumption before proceeding further",
						"initialize the owned memory from retained evidence before any handoff", "current outcome in `## Brief`", "required user-provenance and `Record:` evidence", "relevant observations in `## Observations`", "current phase and next action", "reconfirmed, not reconstructed", "Do not hand off until this initialization is complete")
					assertOrderedPhrases(t, effort, "initialize the owned memory from retained evidence before any handoff", "Do not hand off until this initialization is complete")
					missingEvidenceRecovery := strings.Replace(effort, "Missing exact required user evidence must be reconfirmed, not reconstructed.", "Missing exact required user evidence may be reconstructed.", 1)
					if strings.Contains(missingEvidenceRecovery, "reconfirmed, not reconstructed") {
						t.Errorf("%s %s accepted reconstructed user evidence", target, profile)
					}

					assertContainsAll(t, target+" "+profile+" fixed identity", effort,
						"Effort title and slug are fixed", "add no retitle operation, schema change, or history-deleting lifecycle", "Refinements inside the owned outcome remain in that effort", "material outcome drift requires a different outcome", "fixed-identity successor", "Transfer necessary still-valid context", "verify the successor is resumable", "close the obsolete effort through the existing topology-safety and finish/archive lifecycle")
					assertOrderedPhrases(t, effort, "fixed-identity successor", "Transfer necessary still-valid context", "verify the successor is resumable", "close the obsolete effort")
					wrongArchiveOrder := strings.Replace(effort, "Transfer necessary still-valid context, verify the successor is resumable, then close the obsolete effort", "close the obsolete effort, then transfer necessary still-valid context and verify the successor is resumable", 1)
					if hasOrderedPhrases(wrongArchiveOrder, "Transfer necessary still-valid context", "verify the successor is resumable", "close the obsolete effort") {
						t.Errorf("%s %s accepted archive before successor resumability", target, profile)
					}

					assertContainsAll(t, target+" "+profile+" workflow continuity", workflow,
						"Handoff is prohibited while required late-creation memory initialization remains incomplete", "Missing exact required user evidence must be reconfirmed, not reconstructed")
				})
			}
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
		assertContainsAll(t, target+" brainstorming approval", brainstorming, "material choice or clarification is unresolved", "Present the complete design", "explicitly request approval", "stop", "clear later approval response")
		// The conditional single-decision exception is part of the approval
		// boundary, so prove it on both governance footprints and runtimes.
		for _, profile := range []string{"core", "full"} {
			profileRoot := syncPlanFlexibilityProfile(t, profile)
			profileBrainstorming := read(t, planSkillPath(profileRoot, target, "brainstorming"))
			const eligibility = "A single-decision brainstorm may remain effort-free only when no independent continuity need fires."
			if !strings.Contains(profileBrainstorming, eligibility) {
				t.Errorf("%s %s brainstorming lacks conditional single-decision eligibility", target, profile)
			}
			ineligible := strings.Replace(profileBrainstorming, eligibility, "A single-decision brainstorm may remain effort-free whenever it wishes.", 1)
			if strings.Contains(ineligible, eligibility) {
				t.Errorf("%s %s approval-boundary mutation accepted unconditional single-decision eligibility", target, profile)
			}
		}
		assertOrderedPhrases(t, brainstorming, "Present the complete design", "explicitly request approval", "stop", "clear later approval response")
		adrReview := read(t, path("reviewing-adr"))
		assertContainsAll(t, target+" autonomous ADR hand-off", adrReview,
			"After the review settles", "run `./awf context --show references", "ordinary `"+evalPrefix+"-reviewing-plan`", "If no linked Proposed plan exists yet", "proceed directly to implementation")
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
	piAdapter := read(t, filepath.Join(pi, ".pi", "extensions", "awf-subagents", "index.ts"))
	piGrounding := read(t, filepath.Join(pi, ".pi", "skills", evalPrefix+"-grounding", "SKILL.md"))
	piExploring := read(t, filepath.Join(pi, ".pi", "skills", evalPrefix+"-exploring", "SKILL.md"))
	piCouplingAudit := read(t, filepath.Join(pi, ".pi", "skills", evalPrefix+"-refactor-coupling-audit", "SKILL.md"))
	claudeGrounding := read(t, skillPath(claude, "grounding"))

	assertContainsAll(t, "Pi grounding profile", piAdapter,
		`toolName: "subagent_grounding"`,
		`parameters: Type.Object({ task: Type.String({ minLength: 1 }), model: Type.Optional(MODEL_REFERENCE_SCHEMA) }, { additionalProperties: false })`,
		"Use subagent_grounding whenever the rendered grounding support contract calls for repository-premise checks",
		"include the complete grounding brief in task")
	assertContainsAll(t, "Pi grounding skill", piGrounding,
		"guide-first", "AWF_CONTEXT_SPILL_V1", "Call `subagent_grounding` exactly once", "brief in `task`", "omit the `model` field")
	assertContainsAll(t, "Pi exploration skill", piExploring,
		"Call `subagent_explore` for each child", "required task, breadth, and detail", "fan them out as sibling calls")
	assertContainsAll(t, "Pi coupling audit exploration routing", piCouplingAudit,
		"invoke `"+evalPrefix+"-exploring` once per information need", "breadth and detail", "Keep an exact-known-file or genuinely trivial category check inline")

	for _, name := range sortedKeys(cat.Skills) {
		body := read(t, skillPath(claude, name))
		for _, piTool := range []string{"subagent_grounding", "subagent_explore"} {
			if strings.Contains(body, piTool) {
				t.Errorf("Claude skill %s leaks Pi tool name %q", name, piTool)
			}
		}
	}
	for _, name := range sortedKeys(cat.Agents) {
		body := read(t, agentPath(claude, name))
		for _, piTool := range []string{"subagent_grounding", "subagent_explore"} {
			if strings.Contains(body, piTool) {
				t.Errorf("Claude agent %s leaks Pi tool name %q", name, piTool)
			}
		}
	}
	assertContainsAll(t, "Claude grounding", claudeGrounding, "grounding-checker", "guide-first", "AWF_CONTEXT_SPILL_V1")
}

type durableDesignFacts struct {
	viableApproaches             int
	meaningfullyDifferentDurable bool
}

type durableDesignTransition struct {
	intake       string
	presentation string
	approval     string
	authority    string
}

func TestTwoDurableDesignsRequireApproval(t *testing.T) {
	facts := durableDesignFacts{viableApproaches: 2, meaningfullyDifferentDurable: true}
	want := durableDesignTransition{
		intake:       "route-to-brainstorming",
		presentation: "compare-consequences",
		approval:     "request-user-decision-and-stop",
		authority:    "approved-design-becomes-boundary",
	}
	for _, profile := range []string{"core", "full"} {
		t.Run(profile, func(t *testing.T) {
			root := syncPlanFlexibilityProfile(t, profile)
			for _, target := range []string{"pi", "claude"} {
				t.Run(target, func(t *testing.T) {
					execution := read(t, planSkillPath(root, target, "executing-direct"))
					brainstorming := read(t, planSkillPath(root, target, "brainstorming"))
					if got := twoDurableDesignsTransition(execution, brainstorming, facts); got != want {
						t.Fatalf("two durable designs: got %#v, want %#v", got, want)
					}

					withoutDifferentConsequences := facts
					withoutDifferentConsequences.meaningfullyDifferentDurable = false
					if got := twoDurableDesignsTransition(execution, brainstorming, withoutDifferentConsequences); got.intake != "routine-route-detail" || got.approval != "" {
						t.Errorf("equivalent implementation routes created a material-decision stop: %#v", got)
					}

					mutations := map[string][2]string{
						"intake trigger":     {"viable approaches carry meaningfully different durable consequences", "viable approaches never require a decision"},
						"alternatives":       {"Offer alternatives with trade-offs and a recommendation", "Choose one approach silently"},
						"approval stop":      {"explicitly request approval, and stop", "continue without approval"},
						"approved authority": {"The final approved design becomes the implementation boundary", "The unapproved design becomes the implementation boundary"},
					}
					for name, mutation := range mutations {
						mutatedExecution := strings.ReplaceAll(execution, mutation[0], mutation[1])
						mutatedBrainstorming := strings.ReplaceAll(brainstorming, mutation[0], mutation[1])
						if got := twoDurableDesignsTransition(mutatedExecution, mutatedBrainstorming, facts); got == want {
							t.Errorf("two durable designs accepted contradictory %s guidance %q", name, mutation[1])
						}
					}

					reordered := strings.Replace(brainstorming,
						"Offer alternatives with trade-offs and a recommendation",
						"swap-alternatives-and-approval", 1)
					reordered = strings.Replace(reordered,
						"explicitly request approval, and stop",
						"Offer alternatives with trade-offs and a recommendation", 1)
					reordered = strings.Replace(reordered,
						"swap-alternatives-and-approval",
						"explicitly request approval, and stop", 1)
					if got := twoDurableDesignsTransition(execution, reordered, facts); got == want {
						t.Error("two durable designs accepted approval before alternatives were presented")
					}
				})
			}
		})
	}
}

func twoDurableDesignsTransition(execution, brainstorming string, facts durableDesignFacts) durableDesignTransition {
	if facts.viableApproaches < 2 || !facts.meaningfullyDifferentDurable {
		if strings.Contains(execution, "Routine implementation detail creates no approval boundary") {
			return durableDesignTransition{intake: "routine-route-detail"}
		}
		return durableDesignTransition{}
	}
	if !strings.Contains(execution, "viable approaches carry meaningfully different durable consequences") {
		return durableDesignTransition{}
	}
	got := durableDesignTransition{intake: "route-to-brainstorming"}
	if !hasOrderedPhrases(brainstorming,
		"Offer alternatives with trade-offs and a recommendation",
		"explicitly request approval, and stop") {
		return got
	}
	got.presentation = "compare-consequences"
	got.approval = "request-user-decision-and-stop"
	if strings.Contains(brainstorming, "The final approved design becomes the implementation boundary") {
		got.authority = "approved-design-becomes-boundary"
	}
	return got
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
				"description: Use when a material decision is unresolved", "concise implementation outline", "fuller material-choice design", "explicit approval")

			for _, name := range []string{"executing-direct", "tdd", "bugfix", "executing-plans", "subagent-driven-development"} {
				assertContainsAll(t, target+" "+name+" outline intake", read(t, path(name)),
					"unresolved material decision, never by the act of mutating production code",
					"Routine implementation detail creates no approval boundary, whatever kind of file it touches")
			}
			for _, name := range []string{"writing-plans", "proposing-adr"} {
				assertContainsAll(t, target+" "+name+" artifact intake", read(t, path(name)),
					"unresolved material decision", "Architecture summary")
			}
			implementer := read(t, filepath.Join(root, "."+target, "agents", "implementer.md"))
			assertContainsAll(t, target+" delegated intake", implementer,
				"parent-supplied protected contract", "never recreate the approval interaction", "stops without mutation to report to its parent when that contract is absent or must change")
			assertContainsAll(t, target+" phase-owner approved-boundary dispatch", read(t, path("subagent-driven-development")),
				"provide the complete phase, explicitly identify the parent-supplied approved boundary")
			assertContainsAll(t, target+" helper approved-boundary dispatch", read(t, path("executing-plans")),
				"Each brief explicitly identifies the parent-supplied approved boundary")
			for _, name := range []string{"executing-direct", "tdd", "bugfix", "executing-plans", "subagent-driven-development", "proposing-adr"} {
				body := read(t, path(name))
				assertContainsAll(t, target+" "+name+" evidence", body,
					"retained conversation", "Decision-log evidence", "explicit request to execute a named plan", "Architecture summary")
			}
			// The general rule already leaves documentation, test, and generated-output
			// work autonomous, so the retired per-artifact carve-out list must be gone
			// rather than merely unenforced.
			for _, retired := range []string{"mechanical production refactors", "tests that prepare a production change", "generated-output-only work, and non-code mechanical work remain autonomous"} {
				if strings.Contains(read(t, path("executing-direct")), retired) {
					t.Errorf("%s direct intake retains retired universal-approval wording %q", target, retired)
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
