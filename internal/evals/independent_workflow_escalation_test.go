package evals

import (
	"path/filepath"
	"strings"
	"testing"
)

// invariant: rendering/workflow-skill-templates:independent-workflow-escalation (TestIndependentWorkflowEscalation)
// invariant: rendering/workflow-skill-templates:implementer-context-grounding (TestIndependentWorkflowEscalation)
// invariant: rendering/workflow-skill-templates:mandatory-approval-boundaries (TestIndependentWorkflowEscalation)
// invariant: rendering/workflow-skill-templates:memory-checkpoint-chain-coverage (TestIndependentWorkflowEscalation)
// invariant: rendering/workflow-skill-templates:unified-effort-workflow-coverage (TestIndependentWorkflowEscalation)
// invariant: rendering/workflow-skill-templates:effort-workflow (TestIndependentWorkflowEscalation)
// invariant: rendering/workflow-skill-templates:memory-log-consumer-coverage (TestIndependentWorkflowEscalation)
// invariant: rendering/workflow-skill-templates:workflow-transitions-advisory (TestIndependentWorkflowEscalation)
// invariant: rendering/workflow-skill-templates:phase-transaction-ownership (TestIndependentWorkflowEscalation)
// invariant: rendering/workflow-skill-templates:explorer-and-grounding-role-contracts (TestIndependentWorkflowEscalation)
// invariant: rendering/guide-and-doc-templates:working-memory-single-home (TestIndependentWorkflowEscalation)
// invariant: rendering/pi-workflows:pi-dedicated-grounding-dispatch (TestIndependentWorkflowEscalation)
// invariant: rendering/pi-workflows:pi-session-handoff-workflow (TestIndependentWorkflowEscalation)
func TestIndependentWorkflowEscalation(t *testing.T) {
	cat := loadCatalog(t)
	for _, target := range []string{"pi", "claude"} {
		root := syncFullCatalogForTarget(t, cat, target)
		pathFor := func(name string) string {
			if target == "pi" {
				return filepath.Join(root, ".pi", "skills", evalPrefix+"-"+name, "SKILL.md")
			}
			return skillPath(root, name)
		}
		brainstorming := read(t, pathFor("brainstorming"))
		for _, want := range []string{"material choice or clarification", "does not create an effort", "invoke `" + evalPrefix + "-grounding`"} {
			if !strings.Contains(brainstorming, want) {
				t.Errorf("%s brainstorming missing %q", target, want)
			}
		}
		grounding := read(t, pathFor("grounding"))
		for _, want := range []string{"broad or uncertain repository", "advisory, report-only, single-pass, effort-noncreating", "never a workflow-chain prerequisite", "mechanical, reasoned, or user-decision"} {
			if !strings.Contains(grounding, want) {
				t.Errorf("%s grounding missing %q", target, want)
			}
		}
		effort := read(t, pathFor("effort-workflow"))
		for _, want := range []string{"sole owner of the effort lifecycle", "labeled outcome, title, and canonical short slug", "integration, divergence handling, topology removal, retrospective routing, and finish"} {
			if !strings.Contains(effort, want) {
				t.Errorf("%s effort workflow missing %q", target, want)
			}
		}
		review := read(t, pathFor("reviewing-impl"))
		for _, want := range []string{"locally obvious, low-risk, directly verified", "Uncertainty resolves toward review", "Effort-free review creates no effort", "returns to `" + evalPrefix + "-effort-workflow`"} {
			if !strings.Contains(review, want) {
				t.Errorf("%s review missing %q", target, want)
			}
		}
	}
}
