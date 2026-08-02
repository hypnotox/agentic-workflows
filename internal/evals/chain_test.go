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

// routineCheckpointSkills are the templates that carry the routine checkpoint
// protocol (ADR-0152): the non-terminal chain nodes outside the two mandatory
// approval boundaries, plus the bugfix and debugging task skills. The terminal
// retrospective instead carries the deletion step.
var routineCheckpointSkills = []string{
	"proposing-adr", "writing-plans", "reviewing-plan", "reviewing-plan-resync",
	"executing-plans", "subagent-driven-development", "executing-direct",
	"reviewing-impl", "bugfix", "debugging",
}

// finalApprovalCheckpointSkills are the two final approval boundaries: the end
// of brainstorming and the settled ADR review. Brainstorming also carries the
// distinct mandatory first-creation confirmation boundary.
var finalApprovalCheckpointSkills = []string{"brainstorming", "reviewing-adr"}

// phaseCheckpointSkills own a whole phase, so each renders exactly one routine
// checkpoint. Skills that checkpoint per resumable change render more.
var phaseCheckpointSkills = map[string]bool{"executing-plans": true, "subagent-driven-development": true}

// assertOrderedBody asserts each phrase appears in body after the previous one.
//
// The cursor only advances, so nothing before the FIRST phrase is ever scanned.
// Appending a phrase here therefore proves nothing about a site that renders
// ahead of that first anchor: a later copy of the same prose satisfies the
// match. Such a site needs its own index assertion, not another list entry.
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

// TestUnifiedEffortWorkflowCoverage derives the complete applicable skill set
// from the enabled catalog and validates both target fanouts rather than a
// hand-maintained corpus count.
// invariant: rendering/workflow-skill-templates:memory-checkpoint-chain-coverage (TestUnifiedEffortWorkflowCoverage)
// invariant: rendering/workflow-skill-templates:unified-effort-workflow-coverage (TestUnifiedEffortWorkflowCoverage)
// invariant: rendering/pi-workflows:pi-session-handoff-workflow (TestUnifiedEffortWorkflowCoverage)
func TestUnifiedEffortWorkflowCoverage(t *testing.T) {
	cat := loadCatalog(t)
	roles := map[string]string{
		"brainstorming": "creation", "proposing-adr": "carry", "adr-lifecycle": "carry",
		"writing-plans": "creation", "reviewing-plan": "review", "reviewing-plan-resync": "review",
		"reviewing-adr": "review", "executing-direct": "execution", "executing-plans": "execution",
		"subagent-driven-development": "execution", "reviewing-impl": "terminal-review",
		"retrospective": "finish", "debugging": "conditional-creation", "bugfix": "conditional-creation",
		"tdd": "conditional-creation", "refactor-coupling-audit": "report", "exploring": "report",
		"orienting": "report", "roadmap-graduation": "conditional-creation",
	}
	if len(roles) != len(cat.Skills) {
		t.Fatalf("unified-effort classification has %d skills, enabled catalog has %d", len(roles), len(cat.Skills))
	}
	for name := range cat.Skills {
		if _, ok := roles[name]; !ok {
			t.Errorf("enabled skill %q has no unified-effort classification", name)
		}
	}

	minimal := map[string]bool{"brainstorming": true, "executing-direct": true, "debugging": true, "bugfix": true, "tdd": true, "roadmap-graduation": true}
	reviewers := map[string]bool{"reviewing-plan": true, "reviewing-plan-resync": true, "reviewing-adr": true, "reviewing-impl": true, "refactor-coupling-audit": true, "exploring": true, "orienting": true}
	routineOrdered := []string{
		"**Routine checkpoint.**",
		"minimal simple fix uses no effort",
		"reaching a checkpoint never creates one",
		"already-confirmed immutable slugged effort",
		"always owns `.awf/efforts/<slug>/memory.md`",
		"primary-root-relative spelling",
		"Effort: <slug>",
		"managed worktree when one exists",
		"writer-owned tool batch",
		"any unrecorded settled decision and observation",
		"continuity notice",
	}
	for _, target := range []string{"pi", "claude"} {
		root := syncFullCatalogForTarget(t, cat, target)
		guide := read(t, filepath.Join(root, "AGENTS.md"))
		if target == "pi" {
			if !strings.Contains(guide, "discretionary eligible session-replacement choice") {
				t.Errorf("Pi guide omits discretionary session replacement")
			}
		} else {
			for _, banned := range []string{"session-replacement choice", "new session logs an actual replacement boundary"} {
				if strings.Contains(strings.ToLower(guide), banned) {
					t.Errorf("%s guide publishes unsupported replacement claim %q", target, banned)
				}
			}
		}
		for name, role := range roles {
			path := skillPath(root, name)
			if target == "pi" {
				path = filepath.Join(root, ".pi", "skills", evalPrefix+"-"+name, "SKILL.md")
			}
			body := read(t, path)
			lower := strings.ToLower(body)
			for _, want := range []string{".awf/efforts/<slug>/memory.md", "standalone memory is forbidden"} {
				if !strings.Contains(lower, strings.ToLower(want)) {
					t.Errorf("%s/%s (%s) missing %q", target, name, role, want)
				}
			}
			authoritative := strings.Contains(lower, "authority") || strings.Contains(lower, "authoritative") || strings.Contains(lower, "outrank")
			if !strings.Contains(lower, "repository") || !authoritative {
				t.Errorf("%s/%s does not subordinate checkpoint prose to repository authority", target, name)
			}
			if !strings.Contains(lower, "writer") {
				t.Errorf("%s/%s does not carry the one-writer contract", target, name)
			}
			if strings.Contains(body, ".awf/memory/") {
				t.Errorf("%s/%s retains standalone memory path", target, name)
			}
			if minimal[name] && !strings.Contains(lower, "minimal simple") {
				t.Errorf("%s/%s lost the minimal-simple effort exception", target, name)
			}
			if reviewers[name] && !strings.Contains(lower, "never edit") {
				t.Errorf("%s/%s does not keep report-only children from memory mutation", target, name)
			}
		}
		pathFor := func(name string) string {
			if target == "pi" {
				return filepath.Join(root, ".pi", "skills", evalPrefix+"-"+name, "SKILL.md")
			}
			return skillPath(root, name)
		}
		impl := read(t, pathFor("reviewing-impl"))
		assertOrderedBody(t, target+"/reviewing-impl docs-only", impl, []string{
			"Skipped (docs-only)", "continue at step 8", "Route settled terminal review",
		})
		assertOrderedBody(t, target+"/reviewing-impl integration", impl, []string{
			"Route settled terminal review", "If no managed", "continue to the deferred flip",
			"awf effort integrate <slug>",
			"Integration never implies review, removal, retrospective, or finish",
			"divergent merge", "awf check staged", "project gate", "merge commit",
			"terminal implementation review again", "deferred flip transaction",
			"If managed topology exists", "awf effort worktree remove <slug>", "retrospective",
		})
		retro := read(t, pathFor("retrospective"))
		assertOrderedBody(t, target+"/retrospective finish", retro, []string{
			"Repository sources and current-state documentation", "Promote recurring",
			"Verify managed topology is absent", "Finish last", "awf effort finish <slug>",
		})

		// The routine checkpoint carries the same unified-effort contract as the
		// approval boundaries, but continues instead of stopping. It must never
		// render an approval stop or claim the retrospective's finish step, and
		// only Pi may name the handoff tool.
		for _, name := range routineCheckpointSkills {
			body := read(t, pathFor(name))
			ordered := append([]string{}, routineOrdered...)
			if target == "pi" {
				ordered = append(ordered,
					"safe resumable point whose immediate successor can start independently",
					"judge retained-context relevance and successor work from the current `[session context]` model-window and active-branch-compaction evidence",
					"No fixed threshold controls this choice; continuing immediately in the current session is autonomous, not a check-in",
					"invoke `handoff_session` alone with kickoff prose directing the fresh session to read the effort checkpoint and append the actual boundary to `## Handoff log` as its first memory update before substantive work",
					"Cancellation or failure that leaves the old session active appends no handoff log",
				)
			} else {
				ordered = append(ordered, "Continue through the target-native successor without claiming session replacement")
			}
			ordered = append(ordered,
				"Mechanical corrections and authority-determined implementation details stay autonomous",
				"the workflow doc's working-memory section",
			)
			assertOrderedBody(t, target+"/"+name+" routine checkpoint", body, ordered)
			assertNoCheckpointTimeHandoffLog(t, target+"/"+name, body)
			// Only the two phase skills carry exactly one. executing-direct also
			// checkpoints per resumable change, so it legitimately renders more.
			if phaseCheckpointSkills[name] {
				if count := strings.Count(strings.ToLower(body), "routine checkpoint"); count != 1 {
					t.Errorf("%s/%s renders %d routine checkpoint instructions, want one", target, name, count)
				}
			}
			if strings.Contains(body, "explicitly request approval") {
				t.Errorf("%s/%s renders an approval stop in a routine skill", target, name)
			}
			if strings.Contains(body, "awf effort finish") {
				t.Errorf("%s/%s claims the retrospective's finish step", target, name)
			}
			if target != "pi" && strings.Contains(body, "handoff_session") {
				t.Errorf("%s/%s names handoff_session", target, name)
			}
		}

		// The claim asserts where a routine checkpoint may occur, so the doc
		// that states it is part of the proof. Without this the sentence could
		// be deleted, or a task/helper trigger added beside it, with the suite
		// still green.
		assertCheckpointBoundaryDoc(t, target+" catalog workflow", read(t, filepath.Join(root, "docs", "workflow.md")))
	}
	assertCheckpointBoundaryDoc(t, "project workflow", read(t, filepath.Join("..", "..", "docs", "workflow.md")))
}

// checkpointBoundarySentence is the authoritative statement of which boundaries
// are checkpoints. Its absence, or any sentence adding a task or helper
// trigger, breaks the claim.
const checkpointBoundarySentence = "heading-identified tasks, executable projections, and helper returns are not checkpoint boundaries"

func assertNoCheckpointTimeHandoffLog(t *testing.T, label, body string) {
	t.Helper()
	lower := strings.ToLower(body)
	for _, banned := range []string{
		"append one `## handoff log` line",
		"updates phase, next action, time, and handoff log",
		"invoke `handoff_session` alone with the exact",
	} {
		if strings.Contains(lower, banned) {
			t.Errorf("%s retains checkpoint-time or unconditional handoff contract %q", label, banned)
		}
	}
}

func assertCheckpointBoundaryDoc(t *testing.T, label, body string) {
	t.Helper()
	if !strings.Contains(body, checkpointBoundarySentence) {
		t.Errorf("%s lost the checkpoint-boundary sentence %q", label, checkpointBoundarySentence)
	}
	sentenceBoundary := regexp.MustCompile(`[.!?](?:\s+|$)`)
	for _, sentence := range sentenceBoundary.Split(strings.ToLower(body), -1) {
		if !strings.Contains(sentence, "checkpoint") {
			continue
		}
		mentionsTaskOrHelper := strings.Contains(sentence, "heading-identified task") || strings.Contains(sentence, "executable projection") || strings.Contains(sentence, "helper return")
		// "not commit, dispatch, review, or checkpoint boundaries" negates just
		// as well as the exact phrase, so the check accepts any negated form.
		negates := strings.Contains(sentence, "not ") && strings.Contains(sentence, "checkpoint boundaries")
		if mentionsTaskOrHelper && !negates {
			t.Errorf("%s adds a task/helper checkpoint trigger: %q", label, sentence)
		}
	}
}

// TestMandatoryApprovalBoundaries asserts first-creation confirmation plus the
// two final approval boundaries, and that final approval renders nowhere else.
// invariant: rendering/workflow-skill-templates:mandatory-approval-boundaries (TestMandatoryApprovalBoundaries)
// invariant: rendering/pi-workflows:pi-session-handoff-workflow (TestMandatoryApprovalBoundaries)
func TestMandatoryApprovalBoundaries(t *testing.T) {
	cat := loadCatalog(t)
	root := syncFullCatalogForTarget(t, cat, "pi")
	nonPiRoot := syncFullCatalogForTarget(t, cat, "claude")
	confirmationOrdered := []string{
		"**Mandatory first-creation confirmation.**",
		"Discovery creates no effort",
		"direct concrete non-minimal request follows the same boundary",
		"existing effort resumes under its fixed identity",
		"only while work remains within its confirmed outcome",
		"`Outcome: <concrete non-minimal outcome>`",
		"`Effort title: <proposed title>`",
		"Ask the user to confirm creation",
		"end the turn without creating an effort",
		"Only a clear response in a later turn confirms the pair",
		"`awf effort new \"<confirmed title>\"`",
		"requested change stays in discovery",
		"an ambiguous response receives",
		"creation fails while the pair and its later confirming response remain available",
		"retry without another confirmation",
		"context loss or session replacement makes that evidence unavailable",
		"present and confirm the pair",
	}
	for _, targetBody := range []struct {
		label string
		body  string
	}{
		{"pi/brainstorming", read(t, filepath.Join(root, ".pi", "skills", evalPrefix+"-brainstorming", "SKILL.md"))},
		{"claude/brainstorming", read(t, skillPath(nonPiRoot, "brainstorming"))},
	} {
		assertOrderedBody(t, targetBody.label+" first creation", targetBody.body, confirmationOrdered)
		if confirm, design := strings.Index(targetBody.body, "**Mandatory first-creation confirmation.**"), strings.Index(targetBody.body, "Present the design in sections"); confirm < 0 || design < 0 || confirm > design {
			t.Errorf("%s does not place first-creation confirmation before detailed design", targetBody.label)
		}
	}

	ordered := []string{
		"**Mandatory approval check-in.**",
		"already-confirmed non-minimal outcome",
		"exactly one immutable slugged effort",
		"always owns `.awf/efforts/<slug>/memory.md`",
		"primary-root-relative spelling",
		"Effort: <slug>",
		"managed worktree when one exists",
		"writer-owned tool batch",
		"any unrecorded settled decision and observation",
		"explicitly request approval",
		"end the turn",
		"Stop even when there is no concern to raise",
		"request approval again",
		"After explicit approval, persist the approval and next action before continuing",
	}
	for _, name := range finalApprovalCheckpointSkills {
		piBody := read(t, filepath.Join(root, ".pi", "skills", evalPrefix+"-"+name, "SKILL.md"))
		assertOrderedBody(t, "pi/"+name, piBody, append(append([]string{}, ordered...),
			"judge retained-context relevance and successor work from the current `[session context]` model-window and active-branch-compaction evidence",
			"No fixed threshold controls this choice; continuing immediately in the current session is autonomous, not a check-in",
			"invoke `handoff_session` alone with kickoff prose directing the fresh session to read the effort checkpoint and append the actual boundary to `## Handoff log` as its first memory update before substantive work",
			"Cancellation or failure that leaves the old session active appends no handoff log",
			"the workflow doc's working-memory section",
		))
		if handoff, approval := strings.Index(piBody, "handoff_session"), strings.Index(piBody, "explicitly request approval"); handoff >= 0 && handoff < approval {
			t.Errorf("pi/%s names handoff_session before the approval request", name)
		}
		assertNoCheckpointTimeHandoffLog(t, "pi/"+name, piBody)
		claudeBody := read(t, skillPath(nonPiRoot, name))
		assertOrderedBody(t, "claude/"+name, claudeBody, append(append([]string{}, ordered...),
			"Then continue through the target-native successor without claiming session replacement",
			"the workflow doc's working-memory section",
		))
		if strings.Contains(claudeBody, "handoff_session") {
			t.Errorf("non-Pi skill %q names handoff_session", name)
		}
	}

	rendered, err := os.ReadDir(filepath.Join(root, ".pi", "skills"))
	if err != nil {
		t.Fatalf("list rendered pi workflow bodies: %v", err)
	}
	for _, entry := range rendered {
		name := strings.TrimPrefix(entry.Name(), evalPrefix+"-")
		if slices.Contains(finalApprovalCheckpointSkills, name) {
			continue
		}
		if strings.Contains(read(t, filepath.Join(root, ".pi", "skills", entry.Name(), "SKILL.md")), "explicitly request approval") {
			t.Errorf("skill %q renders a final approval stop outside the two final approval boundaries", name)
		}
	}
}
