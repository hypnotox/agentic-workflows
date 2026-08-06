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
			assertContainsAll(t, target+" effort-workflow", effort,
				"sole owner of the effort lifecycle", "durable continuity materially helps", "clear response in a later turn", "awf effort new --slug", "awf effort integrate <slug>", "awf effort worktree remove <slug>", "awf effort finish <slug>", "divergent result", "before any topology removal")
			for _, name := range allNames {
				if name == "effort-workflow" {
					continue
				}
				for _, forbidden := range []string{"awf effort new --slug", "awf effort integrate <slug>", "awf effort worktree remove <slug>", "awf effort finish <slug>"} {
					if strings.Contains(bodies[name], forbidden) {
						t.Errorf("%s %s duplicates effort lifecycle command %q", target, name, forbidden)
					}
				}
			}

			for _, name := range []string{"proposing-adr", "adr-lifecycle", "writing-plans", "reviewing-adr", "reviewing-plan", "reviewing-plan-resync", "refactor-coupling-audit", "executing-plans", "subagent-driven-development"} {
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
			if target == "pi" {
				assertContainsAll(t, target+" session handoff", workflow,
					"After a persisted formal phase or approval checkpoint", "requests replacement", "## Handoff log", "continuation, cancellation, or failure that leaves the old session active appends none")
			} else if strings.Contains(workflow, "requests replacement") {
				t.Errorf("%s workflow leaks Pi session-replacement behavior", target)
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
		assertContainsAll(t, target+" brainstorming approval", brainstorming, "Present the complete design", "explicitly request approval", "stop", "clear later approval response")
		assertOrderedPhrases(t, brainstorming, "Present the complete design", "explicitly request approval", "stop", "clear later approval response")
		adrReview := read(t, path("reviewing-adr"))
		assertContainsAll(t, target+" ADR approval", adrReview, "approval", "stop")
		effort := read(t, path("effort-workflow"))
		assertContainsAll(t, target+" effort confirmation", effort, "`Outcome:", "`Effort title:", "`Effort slug:", "clear response in a later turn")
		assertOrderedPhrases(t, effort, "`Outcome: <confirmed outcome>`", "`Effort title: <proposed title>`", "`Effort slug: <proposed-short-slug>`", "clear response in a later turn", "awf effort new --slug <confirmed-slug>")
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
