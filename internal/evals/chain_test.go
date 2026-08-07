package evals

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

// read reads path or fails the test.
func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// invocationVerb matches a workflow-chain invocation instruction - the verb that
// makes a line a handoff/dispatch rather than an incidental mention (ADR-0054).
// Case-insensitive so "invoke"/"Call"/"Dispatch"/"chains through" all anchor.
var invocationVerb = regexp.MustCompile(`(?i)(invoke|call|dispatch|hands off|chains through)`)

// namesOnInvocationLine reports whether body has a line carrying both an
// invocation verb and the token as a whole skill/agent name - i.e. the token is
// named in an actual instruction, not merely present somewhere in the prose
// (ADR-0053 owns mere presence) and not just as an existing target (ADR-0046
// owns that). The trailing boundary ([^-\w] or line end) stops
// "example-reviewing-plan" from matching an "example-reviewing-plan-resync" line.
func namesOnInvocationLine(body, token string) bool {
	tokenRe := regexp.MustCompile(regexp.QuoteMeta(token) + `([^-\w]|$)`)
	for _, line := range strings.Split(body, "\n") {
		if tokenRe.MatchString(line) && invocationVerb.MatchString(line) {
			return true
		}
	}
	return false
}

// assertDispatch asserts a skill->agent->partial seam: the rendered reviewing
// `skill` names the reviewer `agent` on an invocation-verb line, and that agent
// carries the shared review-spine partial (ADR-0052) identified by spineToken.
func assertDispatch(t *testing.T, root, skill, agent, spineToken string) {
	t.Helper()
	if body := read(t, skillPath(root, skill)); !namesOnInvocationLine(body, agent) {
		t.Errorf("skill %q does not dispatch agent %q on an invocation line", skill, agent)
	}
	if agentBody := read(t, agentPath(root, agent)); !strings.Contains(agentBody, spineToken) {
		t.Errorf("agent %q missing spine partial token %q", agent, spineToken)
	}
}

// TestReviewerDispatchCarriesSpine asserts each reviewing skill dispatches its
// reviewer agent (on an invocation line) and that agent carries the spine partial.
func TestExplorationConsumerToPiToolSeam(t *testing.T) {
	cat := loadCatalog(t)
	root := syncFullCatalogForTarget(t, cat, "pi")
	// Orienting replaced brainstorming as the exploring consumer: brainstorming
	// now reaches exploration only by invoking orienting.
	for _, consumer := range []string{"orienting", "debugging", "refactor-coupling-audit"} {
		body := read(t, filepath.Join(root, ".pi", "skills", evalPrefix+"-"+consumer, "SKILL.md"))
		if !strings.Contains(body, "exploring") {
			t.Errorf("Pi consumer %q does not route through exploring", consumer)
		}
	}
	if body := read(t, filepath.Join(root, ".pi", "skills", evalPrefix+"-brainstorming", "SKILL.md")); !strings.Contains(body, "orienting") {
		t.Error("Pi brainstorming skill does not route through orienting")
	}
	exploring := read(t, filepath.Join(root, ".pi", "skills", evalPrefix+"-exploring", "SKILL.md"))
	if !namesOnInvocationLine(exploring, "subagent_explore") {
		t.Error("Pi exploring skill does not invoke subagent_explore")
	}
	extension := read(t, filepath.Join(root, ".pi", "extensions", "awf-subagents", "index.ts"))
	if !strings.Contains(extension, `name: "subagent_explore"`) {
		t.Error("Pi extension does not register subagent_explore")
	}
}

func TestPiReviewerDispatchNamesToolAndRenderedReviewer(t *testing.T) {
	cat := loadCatalog(t)
	root := syncFullCatalogForTarget(t, cat, "pi")
	extension := read(t, filepath.Join(root, ".pi", "extensions", "awf-subagents", "index.ts"))
	for _, tc := range []struct{ skill, agent string }{
		{"reviewing-impl", "code-reviewer"},
		{"reviewing-adr", "adr-reviewer"},
		{"reviewing-plan", "plan-reviewer"},
		{"reviewing-plan-resync", "plan-reviewer"},
	} {
		body := read(t, filepath.Join(root, ".pi", "skills", evalPrefix+"-"+tc.skill, "SKILL.md"))
		if !namesOnInvocationLine(body, "subagent_review") || !strings.Contains(extension, tc.agent+".md") {
			t.Errorf("Pi skill %q does not connect subagent_review to %q", tc.skill, tc.agent)
		}
		if got := strings.Count(body, "omit the `model` field to use configured role routing"); got != 2 {
			t.Errorf("Pi skill %q has %d deliberate selection rules, want primary and verify rules", tc.skill, got)
		}
		if reviewer := read(t, filepath.Join(root, ".pi", "agents", tc.agent+".md")); !strings.Contains(reviewer, "## Classification rules") {
			t.Errorf("Pi reviewer %q missing shared spine", tc.agent)
		}
	}
}

func TestReviewerDispatchCarriesSpine(t *testing.T) {
	cat := loadCatalog(t)
	root := syncFullCatalog(t, cat)
	for _, tc := range []struct{ skill, agent string }{
		{"reviewing-impl", "code-reviewer"},
		{"reviewing-adr", "adr-reviewer"},
		{"reviewing-plan", "plan-reviewer"},
	} {
		t.Run(tc.skill, func(t *testing.T) {
			assertDispatch(t, root, tc.skill, tc.agent, "## Classification rules")
		})
	}
}

func TestSemanticRenderingReviewReachesEnabledTargets(t *testing.T) {
	const (
		planningInstruction   = "- **Semantic rendering review:** plans name concrete examples and expected readings only when load-bearing. The implementation phase owner performs focused generated-prose meaning review, records inspected output boundaries and result in completion evidence, and checks contradictory fragments, concept-preserving paraphrase, and intentional literal placeholders without a universal language validator. Plan and code reviewers inspect the requirement and evidence."
		planReviewInstruction = "1. **semantic-rendering-review**: inspect the change-specific requirement and implementation completion evidence for focused generated-prose meaning review at affected output boundaries, including contradictory fragments, concept-preserving paraphrase, and intentional literal placeholders such as `<literal-placeholder>`. Concrete examples and expected readings are required only when load-bearing; this is not a general output validator."
		codeReviewInstruction = "1. **semantic-rendering-review**: for generated prose changes, inspect the requirement and phase completion evidence naming produced-output boundaries and result, including contradictory fragments, concept-preserving paraphrase, and literal-placeholder intent. Keep this as human meaning review, not a general output validator or new deterministic inference."
	)
	cat := loadCatalog(t)
	for _, target := range []string{"claude", "pi"} {
		t.Run(target, func(t *testing.T) {
			root := syncFullCatalogForTarget(t, cat, target)
			base := filepath.Join(root, "."+target)
			for _, tc := range []struct {
				path        string
				instruction string
			}{
				{filepath.Join(base, "skills", evalPrefix+"-writing-plans", "SKILL.md"), planningInstruction},
				{filepath.Join(base, "agents", "plan-reviewer.md"), planReviewInstruction},
				{filepath.Join(base, "agents", "code-reviewer.md"), codeReviewInstruction},
			} {
				out := read(t, tc.path)
				if !strings.Contains(out, tc.instruction) {
					t.Errorf("%s missing exact semantic rendering instruction %q:\n%s", tc.path, tc.instruction, out)
				}
				if strings.Contains(out, "<no value>") {
					t.Errorf("%s contains unresolved no-value token:\n%s", tc.path, out)
				}
			}
		})
	}
}

// chainNodes is the pinned forward-chain progression node set (ADR-0054 item 3).
// chainTerminal is the sole terminal (exempt from the outgoing-edge requirement).
// Task skills bugfix/debugging are deliberately NOT nodes - their handoffs are
// covered by the per-edge positional check above.
var chainNodes = []string{
	"brainstorming", "executing-direct", "proposing-adr", "reviewing-adr", "writing-plans",
	"reviewing-plan", "reviewing-plan-resync", "executing-plans",
	"subagent-driven-development", "reviewing-impl", "retrospective",
}

// The catalog's Chain flags and this suite's pinned node set are the same
// classification - a new chain skill must land in both, or the guide's
// task-skills derivation and the connectivity graph disagree.
func TestChainFlagsMatchPinnedNodes(t *testing.T) {
	var flagged []string
	for name, sp := range catalog.Standard.Skills {
		if sp.Profile.Kind == catalog.WorkflowChain {
			flagged = append(flagged, name)
		}
	}
	slices.Sort(flagged)
	pinned := slices.Clone(chainNodes)
	slices.Sort(pinned)
	if !slices.Equal(flagged, pinned) {
		t.Errorf("catalog Chain flags %v != pinned chain nodes %v", flagged, pinned)
	}
}

// chainEdges returns, for each chain node, the set of other chain nodes it names
// on an invocation-verb line in the full-catalog render.
// TestChainConnectivity asserts the forward-chain handoff graph has no orphaned
// node (every non-terminal node emits >=1 outgoing invocation edge) and every
// node is reachable from the root brainstorming (ADR-0054 item 3). This catches a
// skill that loses all its handoff instructions - a whole-node failure the
// per-edge positional check cannot see.
func TestStagedAuthorityExecutionOrder(t *testing.T) {
	cat := loadCatalog(t)
	root := syncFullCatalog(t, cat)
	paths := map[string]string{
		"adr-lifecycle":               skillPath(root, "adr-lifecycle"),
		"executing-plans":             skillPath(root, "executing-plans"),
		"subagent-driven-development": skillPath(root, "subagent-driven-development"),
		"AGENTS":                      filepath.Join(root, "AGENTS.md"),
	}
	for name, path := range paths {
		t.Run(name, func(t *testing.T) {
			body := read(t, path)
			position := 0
			for _, phrase := range []string{"the complete transaction", "`awf check staged`", "the project's gate", "wired pre-commit hook enforces both", "only in a clone without wired hooks"} {
				next := strings.Index(body[position:], phrase)
				if next < 0 {
					t.Fatalf("%s missing ordered authority step %q after byte %d", name, phrase, position)
				}
				position += next + len(phrase)
			}
		})
	}
}
