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

// assertHandoff asserts the rendered `from` skill names the prefixed `to` skill
// on an invocation-verb line - the successor sits in a real handoff instruction.
func assertHandoff(t *testing.T, root, from, to string) {
	t.Helper()
	body := read(t, skillPath(root, from))
	want := evalPrefix + "-" + to
	if !namesOnInvocationLine(body, want) {
		t.Errorf("skill %q does not hand off to %q on an invocation line", from, want)
	}
}

// nonHandoffRequires pins the catalog requiresSkills pairs that are
// deliberately not handoffs - the reference is real (ADR-0080's sweep demands
// the declaration) but sits in companion mentions, lifecycle citations, or
// arrival-context prose rather than on an invocation-verb line. Entries fail
// when stale: a pair that starts holding as a handoff must be removed so the
// derivation covers it.
var nonHandoffRequires = map[string]bool{
	"executing-plans->subagent-driven-development": true,
	"proposing-adr->adr-lifecycle":                 true,
	"reviewing-adr->adr-lifecycle":                 true,
	"reviewing-adr->executing-plans":               true,
	"reviewing-adr->subagent-driven-development":   true,
	"reviewing-impl->executing-plans":              true,
	"reviewing-impl->subagent-driven-development":  true,
	"reviewing-plan-resync->reviewing-plan":        true,
}

// conditionalHandoffs are handoffs present in the full-catalog render whose
// template reference is conditional, so requiresSkills cannot declare it
// (ADR-0080 declares unconditional references only) and the derivation below
// cannot see it.
var conditionalHandoffs = []struct{ from, to string }{
	{"bugfix", "reviewing-impl"},
}

// TestWorkflowChainHandoffs asserts each load-bearing chain handoff names its
// successor in an actual invocation instruction in the same full-catalog
// render. The pair set derives from the catalog's requiresSkills declarations
// (minus the pinned non-handoff references), never a hand list, so a new
// skill's declared couplings are handoff-checked automatically.
func TestWorkflowChainHandoffs(t *testing.T) {
	cat := loadCatalog(t)
	root := syncFullCatalog(t, cat)
	declared := map[string]bool{}
	for name, sp := range catalog.Standard.Skills {
		body := read(t, skillPath(root, name))
		for _, req := range sp.RequiresSkills {
			pair := name + "->" + req
			declared[pair] = true
			holds := namesOnInvocationLine(body, evalPrefix+"-"+req)
			switch {
			case nonHandoffRequires[pair] && holds:
				t.Errorf("stale nonHandoffRequires entry %q: the reference now sits on an invocation line - remove the entry", pair)
			case !nonHandoffRequires[pair] && !holds:
				t.Errorf("skill %q does not hand off to %q on an invocation line", name, evalPrefix+"-"+req)
			}
		}
	}
	for pair := range nonHandoffRequires {
		if !declared[pair] {
			t.Errorf("stale nonHandoffRequires entry %q: no such requiresSkills declaration", pair)
		}
	}
	for _, tc := range conditionalHandoffs {
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
func TestExplorationConsumerToPiToolSeam(t *testing.T) {
	cat := loadCatalog(t)
	root := syncFullCatalogForTarget(t, cat, "pi")
	for _, consumer := range []string{"brainstorming", "debugging", "refactor-coupling-audit"} {
		body := read(t, filepath.Join(root, ".pi", "awf-workflows", consumer+".md"))
		if !strings.Contains(body, "awf_workflow") || !strings.Contains(body, `skill: "exploring"`) {
			t.Errorf("Pi consumer %q does not route through exploring", consumer)
		}
	}
	exploring := read(t, filepath.Join(root, ".pi", "awf-workflows", "exploring.md"))
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
	} {
		body := read(t, filepath.Join(root, ".pi", "awf-workflows", tc.skill+".md"))
		if !namesOnInvocationLine(body, "subagent_review") || !strings.Contains(extension, tc.agent+".md") {
			t.Errorf("Pi skill %q does not connect subagent_review to %q", tc.skill, tc.agent)
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

// chainNodes is the pinned forward-chain progression node set (ADR-0054 item 3).
// chainTerminal is the sole terminal (exempt from the outgoing-edge requirement).
// Task skills bugfix/debugging are deliberately NOT nodes - their handoffs are
// covered by the per-edge positional check above.
var chainNodes = []string{
	"brainstorming", "executing-direct", "proposing-adr", "reviewing-adr", "writing-plans",
	"reviewing-plan", "reviewing-plan-resync", "executing-plans",
	"subagent-driven-development", "reviewing-impl", "retrospective",
}

const (
	chainRoot     = "brainstorming"
	chainTerminal = "retrospective"
)

// The catalog's Chain flags and this suite's pinned node set are the same
// classification - a new chain skill must land in both, or the guide's
// task-skills derivation and the connectivity graph disagree.
func TestChainFlagsMatchPinnedNodes(t *testing.T) {
	var flagged []string
	for name, sp := range catalog.Standard.Skills {
		if sp.Chain {
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
// skill that loses all its handoff instructions - a whole-node failure the
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
			for _, phrase := range []string{"Stage the complete transaction", "`awf check --staged`", "the project's gate", "Commit only after both commands pass", "defense in depth"} {
				next := strings.Index(body[position:], phrase)
				if next < 0 {
					t.Fatalf("%s missing ordered authority step %q after byte %d", name, phrase, position)
				}
				position += next + len(phrase)
			}
		})
	}
}

// routineCheckpointSkills are the templates that carry the routine checkpoint
// protocol (ADR-0152): the non-terminal chain nodes outside the two mandatory
// approval boundaries, plus the bugfix and debugging task skills. The terminal
// retrospective instead carries the deletion step.
var routineCheckpointSkills = []string{
	"proposing-adr", "writing-plans", "reviewing-plan", "reviewing-plan-resync",
	"executing-plans", "subagent-driven-development", "reviewing-impl",
	"bugfix", "debugging",
}

// approvalCheckpointSkills are the two mandatory approval boundaries: the end
// of brainstorming and the settled ADR review (ADR-0152).
var approvalCheckpointSkills = []string{"brainstorming", "reviewing-adr"}

// assertOrderedBody asserts each phrase appears in body after the previous one.
func assertOrderedBody(t *testing.T, label, body string, phrases []string) {
	t.Helper()
	position := 0
	for _, phrase := range phrases {
		next := strings.Index(body[position:], phrase)
		if next < 0 {
			t.Errorf("%s missing ordered checkpoint phrase %q", label, phrase)
			return
		}
		position += next + len(phrase)
	}
}

// TestMemoryCheckpointCoverage asserts every routine-boundary skill renders the
// complete routine protocol (persist, classify, then check-in or continue), the
// implementation skills embed it at their per-task sections, and the chain
// terminal instructs the deletion (ADR-0152).
// invariant: rendering/workflow-skill-templates:memory-checkpoint-chain-coverage
func TestMemoryCheckpointCoverage(t *testing.T) {
	cat := loadCatalog(t)
	root := syncFullCatalogForTarget(t, cat, "pi")
	ordered := []string{
		"**Routine checkpoint.**",
		"Working memory is optional",
		"do not create a file merely because this checkpoint was reached",
		"update it in its own tool batch",
		"Effort: <active-effort-id>",
		"Decide whether user attention is required",
		"material authority drift",
		"a blocker, or failed required verification",
		"raise a check-in that names the issue, the options, a recommendation, and the blocked next action",
		"then stop and wait",
		"continuity notice",
		"never a stop",
		"If a validated memory file exists",
		"invoke `handoff_session` alone",
		"continue automatically in the fresh session",
		"unless the user cancels during the five-second window",
		"A failed handoff leaves the checkpoint valid and becomes a check-in",
	}
	piSkillPath := func(name string) string {
		return filepath.Join(root, ".pi", "awf-workflows", name+".md")
	}
	for _, name := range routineCheckpointSkills {
		body := read(t, piSkillPath(name))
		assertOrderedBody(t, "pi/"+name, body, ordered)
		if strings.Contains(body, "explicitly request approval") {
			t.Errorf("routine skill %q renders an approval stop", name)
		}
		if strings.Contains(body, "Delete the effort's working-memory file") {
			t.Errorf("non-terminal skill %q claims the retrospective's memory deletion", name)
		}
	}
	perTask := map[string]string{
		"executing-plans":             "After each independently resumable committed and reviewed task",
		"subagent-driven-development": "After each implemented and reviewed task",
	}
	for name, sentence := range perTask {
		body := read(t, piSkillPath(name))
		start := strings.Index(body, sentence)
		if start < 0 {
			t.Errorf("%s lost its per-task checkpoint sentence", name)
			continue
		}
		end := strings.Index(body[start:], "Terminal step")
		if end < 0 {
			end = len(body) - start
		}
		if !strings.Contains(body[start:start+end], "**Routine checkpoint.**") {
			t.Errorf("%s per-task section does not embed the complete routine protocol", name)
		}
	}
	if body := read(t, piSkillPath("retrospective")); !strings.Contains(body, "Delete the effort's working-memory file") {
		t.Errorf("retrospective missing the working-memory deletion step")
	}

	nonPiRoot := syncFullCatalogForTarget(t, cat, "claude")
	for _, name := range routineCheckpointSkills {
		body := read(t, skillPath(nonPiRoot, name))
		if strings.Contains(body, "handoff_session") {
			t.Errorf("non-Pi skill %q names handoff_session", name)
		}
		notice := strings.Index(body, "continuity notice")
		continuation := strings.Index(body, "Continue through the target-native successor without claiming session replacement")
		if notice < 0 || continuation < notice {
			t.Errorf("non-Pi skill %q does not continue target-natively after the continuity notice", name)
		}
	}
}

// TestMandatoryApprovalBoundaries asserts the two approval-boundary skills stop
// for explicit approval, persist it, and only then continue target-natively,
// and that the approval stop renders nowhere else (ADR-0152).
// invariant: rendering/workflow-skill-templates:mandatory-approval-boundaries
func TestMandatoryApprovalBoundaries(t *testing.T) {
	cat := loadCatalog(t)
	root := syncFullCatalogForTarget(t, cat, "pi")
	nonPiRoot := syncFullCatalogForTarget(t, cat, "claude")
	ordered := []string{
		"**Mandatory approval check-in.**",
		"Complete the memory update in its own tool batch",
		"Effort: <active-effort-id>",
		"explicitly request approval",
		"end the turn",
		"Stop even when there is no concern to raise",
		"request approval again",
		"After explicit approval, persist the approval and next action before continuing",
	}
	for _, name := range approvalCheckpointSkills {
		piBody := read(t, filepath.Join(root, ".pi", "awf-workflows", name+".md"))
		assertOrderedBody(t, "pi/"+name, piBody, append(append([]string{}, ordered...),
			"invoke `handoff_session` alone",
			"unless the user cancels during the five-second window",
			"A failed handoff leaves the checkpoint valid and becomes a check-in",
		))
		if handoff, approval := strings.Index(piBody, "handoff_session"), strings.Index(piBody, "explicitly request approval"); handoff >= 0 && handoff < approval {
			t.Errorf("pi/%s names handoff_session before the approval request", name)
		}
		claudeBody := read(t, skillPath(nonPiRoot, name))
		assertOrderedBody(t, "claude/"+name, claudeBody, append(append([]string{}, ordered...),
			"Then continue through the target-native successor without claiming session replacement",
		))
		if strings.Contains(claudeBody, "handoff_session") {
			t.Errorf("non-Pi skill %q names handoff_session", name)
		}
	}
}
