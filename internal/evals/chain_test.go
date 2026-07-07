package evals

import (
	"os"
	"regexp"
	"strings"
	"testing"
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

// invocationVerb matches a workflow-chain invocation instruction — the verb that
// makes a line a handoff/dispatch rather than an incidental mention (ADR-0054).
// Case-insensitive so "invoke"/"Invoke"/"Dispatch"/"chains through" all anchor.
var invocationVerb = regexp.MustCompile(`(?i)(invoke|dispatch|hands off|chains through)`)

// namesOnInvocationLine reports whether body has a line carrying both an
// invocation verb and the token as a whole skill/agent name — i.e. the token is
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

// assertHandoff asserts the rendered `from` skill names the prefixed `to` skill
// on an invocation-verb line — the successor sits in a real handoff instruction.
func assertHandoff(t *testing.T, root, from, to string) {
	t.Helper()
	body := read(t, skillPath(root, from))
	want := evalPrefix + "-" + to
	if !namesOnInvocationLine(body, want) {
		t.Errorf("skill %q does not hand off to %q on an invocation line", from, want)
	}
}

// TestWorkflowChainHandoffs asserts each load-bearing chain handoff names its
// successor in an actual invocation instruction in the same full-catalog render.
func TestWorkflowChainHandoffs(t *testing.T) {
	cat := loadCatalog(t)
	root := syncFullCatalog(t, cat)
	for _, tc := range []struct{ from, to string }{
		{"brainstorming", "proposing-adr"},
		{"brainstorming", "writing-plans"},
		{"proposing-adr", "reviewing-adr"},
		{"writing-plans", "reviewing-plan"},
		{"bugfix", "reviewing-impl"},
		{"reviewing-impl", "retrospective"},
	} {
		t.Run(tc.from+"_to_"+tc.to, func(t *testing.T) {
			assertHandoff(t, root, tc.from, tc.to)
		})
	}
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

// chainNodes is the pinned forward-chain progression node set (ADR-0054 item 3).
// chainTerminal is the sole terminal (exempt from the outgoing-edge requirement).
// Task skills bugfix/debugging are deliberately NOT nodes — their handoffs are
// covered by the per-edge positional check above.
var chainNodes = []string{
	"brainstorming", "proposing-adr", "reviewing-adr", "writing-plans",
	"reviewing-plan", "reviewing-plan-resync", "executing-plans",
	"subagent-driven-development", "reviewing-impl", "retrospective",
}

const (
	chainRoot     = "brainstorming"
	chainTerminal = "retrospective"
)

// chainEdges returns, for each chain node, the set of other chain nodes it names
// on an invocation-verb line in the full-catalog render.
func chainEdges(t *testing.T, root string) map[string][]string {
	t.Helper()
	edges := map[string][]string{}
	for _, from := range chainNodes {
		body := read(t, skillPath(root, from))
		for _, to := range chainNodes {
			if to == from {
				continue
			}
			if namesOnInvocationLine(body, evalPrefix+"-"+to) {
				edges[from] = append(edges[from], to)
			}
		}
	}
	return edges
}

// TestChainConnectivity asserts the forward-chain handoff graph has no orphaned
// node (every non-terminal node emits >=1 outgoing invocation edge) and every
// node is reachable from the root brainstorming (ADR-0054 item 3). This catches a
// skill that loses all its handoff instructions — a whole-node failure the
// per-edge positional check cannot see.
func TestChainConnectivity(t *testing.T) {
	cat := loadCatalog(t)
	root := syncFullCatalog(t, cat)
	edges := chainEdges(t, root)

	for _, n := range chainNodes {
		if n == chainTerminal {
			continue
		}
		if len(edges[n]) == 0 {
			t.Errorf("chain node %q is orphaned: no outgoing invocation edge", n)
		}
	}

	seen := map[string]bool{chainRoot: true}
	queue := []string{chainRoot}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, to := range edges[cur] {
			if !seen[to] {
				seen[to] = true
				queue = append(queue, to)
			}
		}
	}
	for _, n := range chainNodes {
		if !seen[n] {
			t.Errorf("chain node %q is unreachable from %q", n, chainRoot)
		}
	}
}

// memoryCheckpointSkills are the templates that must carry the working-memory
// checkpoint (ADR-0069): the nine non-terminal chain nodes plus the bugfix and
// debugging task skills. The terminal retrospective instead carries the
// deletion step.
var memoryCheckpointSkills = []string{
	"brainstorming", "proposing-adr", "reviewing-adr", "writing-plans",
	"reviewing-plan", "reviewing-plan-resync", "executing-plans",
	"subagent-driven-development", "reviewing-impl", "bugfix", "debugging",
}

// TestMemoryCheckpointCoverage asserts every non-terminal chain node and the
// multi-step task skills instruct the working-memory checkpoint in the rendered
// full-catalog output, and the chain terminal instructs the deletion (ADR-0069).
// invariant: memory-checkpoint-chain-coverage
func TestMemoryCheckpointCoverage(t *testing.T) {
	cat := loadCatalog(t)
	root := syncFullCatalog(t, cat)
	const token = "**Working-memory checkpoint.**"
	for _, name := range memoryCheckpointSkills {
		if body := read(t, skillPath(root, name)); !strings.Contains(body, token) {
			t.Errorf("skill %q missing the working-memory checkpoint", name)
		}
	}
	if body := read(t, skillPath(root, "retrospective")); !strings.Contains(body, "Delete the effort's working-memory file") {
		t.Errorf("retrospective missing the working-memory deletion step")
	}
}
