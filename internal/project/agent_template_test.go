package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestAdrReviewerAgent(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"invariantTestPath": "internal/adrtools/invariants_test.go",
			"activeMdRegenCmd":  "go test ./internal/adrtools/",
		},
		"layout": map[string]any{"adrDir": "docs/decisions", "indexMd": "docs/decisions/INDEX.md"},
		"data": map[string]any{
			"focusItems": []map[string]any{
				{
					"name":        "context-grounding",
					"description": "Verify factual claims in the Context section against named files, ADRs, and state docs; flag stale claims and drift since brainstorm.",
				},
			},
		},
	}

	out := renderAgentGolden(t, "adr-reviewer", data)

	// Assert frontmatter name line (agents are unprefixed)
	if !strings.Contains(out, "name: adr-reviewer") {
		t.Errorf("expected 'name: adr-reviewer' in output:\n%s", out)
	}

	// Assert shared review-discipline spine phrases
	loadBearing := []string{
		"mechanical",
		"reasoned",
		"user-decision",
		"suggested_fix",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
	for _, phrase := range []string{
		"post-implementation",
		"counterfactual",
		"consent evidence establishes that the mechanism itself is load-bearing and the record explains why it is load-bearing",
		"reasoned finding",
		"structural-design",
		"docs/maintainable-code-design.md",
		"semantic model, representation, module/package boundary, dependency direction, ownership boundary, or comparable structural contract",
		"only when a Decision changes",
		"cohesion, representation isolation, dependency direction, enabling-refactor disposition, testable seams, and justification for indirection",
		"skip this lens rather than manufacturing structural requirements",
		"decision-adherence",
		"ADR-scope",
		"consent evidence",
		"semantic strengthening",
		"unnecessary constraint",
	} {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected structural-design phrase %q in output:\n%s", phrase, out)
		}
	}
	for _, banned := range []string{"doc-currency (ADR-level)", "## Doc-currency checklist", "same-commit update of the listed artifact"} {
		if strings.Contains(out, banned) {
			t.Errorf("ADR reviewer retains implementation-inventory requirement %q:\n%s", banned, out)
		}
	}
}

func TestPlanReviewerAgent(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars":   map[string]any{},
		"layout": map[string]any{"plansDir": "docs/plans"},
		"data": map[string]any{
			"focusItems": []map[string]any{
				{
					"name":        "convention-alignment-extra",
					"description": "Verify commit subjects follow Conventional Commits; flag subjects over 72 chars or missing scope.",
				},
			},
			"docCurrencyItems": []map[string]any{
				{"check": ".awf/topics/parts/<domain>/<topic>/current-state.md - update when plan shifts current authority"},
				{"check": "docs/workflow.md - update when plan changes a workflow rule"},
				{"check": "AGENTS.md - update when plan changes chain, principles, or invariants"},
				{"check": "docs/decisions/INDEX.md - regenerate when plan flips an ADR status"},
			},
		},
	}

	out := renderAgentGolden(t, "plan-reviewer", data)

	// Assert frontmatter name line (agents are unprefixed)
	if !strings.Contains(out, "name: plan-reviewer") {
		t.Errorf("expected 'name: plan-reviewer' in output:\n%s", out)
	}

	// Assert shared review-discipline spine phrases
	sharedPhrases := []string{
		"mechanical",
		"reasoned",
		"user-decision",
		"suggested_fix",
	}
	for _, phrase := range sharedPhrases {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected shared spine phrase %q in output:\n%s", phrase, out)
		}
	}

	// Assert plan-specific lens phrases
	planPhrases := []string{
		"scope-completeness",
		"executability",
		"one final `### Phase close` with a single commit fence and can pass focused verification on its own",
		"maintainable-design",
		"docs/maintainable-code-design.md",
		"model, ownership, representations, translation boundaries, dependency direction, and test seams",
		"where a protected property requires dependency ordering",
		"necessary enabling refactors precede dependent behavior",
		"bounded to the failure they prevent",
		"deterministically verifiable",
		"approved, deferred, or declined disposition",
		"needless indirection, pattern mandates, and unapproved or unjustified abstraction, indirection, validation, test machinery, tooling, cleanup, or process",
		"Do not demand additions merely because more structure, testing, cleanup, or validation is imaginable",
	}
	for _, phrase := range planPhrases {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected plan-lens phrase %q in output:\n%s", phrase, out)
		}
	}
}

func TestCodeReviewerAgent(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars":   map[string]any{},
		"data": map[string]any{
			"correctnessTraps": []map[string]any{
				{"description": "Check that error return paths use %w wrapping so callers can inspect the error chain."},
				{"description": "Flag nil pointer dereferences in struct methods where the receiver may be nil."},
			},
			"docCurrencyItems": []map[string]any{
				{"check": ".awf/topics/parts/<domain>/<topic>/current-state.md - update when the implementation shifts current authority"},
				{"check": "docs/decisions/INDEX.md - regenerate when ADR status flips to Implemented"},
			},
		},
	}

	out := renderAgentGolden(t, "code-reviewer", data)

	// Assert frontmatter name line (agents are unprefixed)
	if !strings.Contains(out, "name: code-reviewer") {
		t.Errorf("expected 'name: code-reviewer' in output:\n%s", out)
	}

	// Assert description contains Independent fresh-context reviewer for example (kept green by TestEndToEndGolden too)
	if !strings.Contains(out, "Independent fresh-context reviewer for example") {
		t.Errorf("expected 'Independent fresh-context reviewer for example' in description:\n%s", out)
	}

	// Assert shared review-discipline spine phrases (verbatim from siblings)
	sharedPhrases := []string{
		"mechanical",
		"reasoned",
		"user-decision",
		"suggested_fix",
		"~80 words",
	}
	for _, phrase := range sharedPhrases {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected shared spine phrase %q in output:\n%s", phrase, out)
		}
	}

	// Assert impl-lens phrases (correctness and contract-adherence are code-reviewer-specific)
	implPhrases := []string{
		"correctness",
		"contract-adherence",
		"maintainable-design",
		"docs/maintainable-code-design.md",
		"cohesion, coupling, dependency direction, representation leakage, duplicated policy, testability, needless indirection, protected-contract conformance, and coherent transactions rather than literal planned phase grouping",
		"behavior bolted onto an unsuitable abstraction",
		"refactoring scope silently broadened",
	}
	for _, phrase := range implPhrases {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected impl-lens phrase %q in output:\n%s", phrase, out)
		}
	}
}

func TestRenderedReviewerVerificationGuidance(t *testing.T) {
	root := testsupport.RepoRoot(t)
	contracts := map[string][]string{
		"plan-reviewer": {
			"material census and post-check commands",
			"exact intermediate snapshot",
			"terminal set or lifecycle-authorized residual findings",
			"reject a premature zero requirement",
			"authority, state, or choreography check",
			"preserve authority checks",
			"no stricter than the durable property",
			"no named authority or state obligation",
		},
		"code-reviewer": {
			"added or changed mechanical check",
			"negative case",
			"temporary falsification",
			"mutation landed",
			"verdict counts",
			"restore only the temporary mutation",
			"whole-file reset",
			"unrelated uncommitted work",
			"authority, state, or choreography check",
			"preserve authority checks",
			"no stricter than the durable property",
			"no named authority or state obligation",
		},
	}
	for agent, clauses := range contracts {
		surfaces := map[string]string{
			"generic " + agent: renderAgentGolden(t, agent, map[string]any{
				"prefix": "example",
				"vars":   map[string]any{},
				"layout": map[string]any{"plansDir": "docs/plans"},
				"data":   catalog.Standard.Agents[agent].Data,
			}),
		}
		for _, target := range []string{".claude", ".pi"} {
			path := filepath.Join(root, target, "agents", agent+".md")
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			surfaces[path] = string(body)
		}
		for surface, body := range surfaces {
			for _, clause := range clauses {
				if !strings.Contains(body, clause) {
					t.Errorf("%s missing reviewer verification clause %q", surface, clause)
				}
			}
		}
	}
}

// invariant: rendering/workflow-skill-templates:implementer-role-contract (TestImplementerAgent)
func TestImplementerAgent(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars":   map[string]any{"gateCmd": "make gate"},
		"data": map[string]any{
			"prohibitedShortcuts": []map[string]any{
				{"description": "adding an abstraction with no current call site"},
			},
		},
	}

	out := renderAgentGolden(t, "implementer", data)

	if !strings.Contains(out, "name: implementer") {
		t.Errorf("expected 'name: implementer' in output:\n%s", out)
	}
	// One phrase per contract clause, so a dropped clause fails loudly.
	for _, want := range []string{
		"Commit-disabled implementation child for example work",
		"commit-disabled implementation child",
		"commits are forbidden",
		"The parent alone owns integration",
		"Your brief is the whole job",
		"report an omitted necessary path to the parent",
		"implementation child reports the necessity",
		"adding an abstraction with no current call site",
		"workflow-chain routing do not bind you",
		"invoke no workflow skill, create or resume no effort",
		"Iterating on failures is the work",
		"Never weaken an assertion, golden, fixture, or check",
		"There is nobody to wait for",
		"stopped receipt for the parent",
		"fast gates",
		"terminal exhaustive verification",
		"assigned scope; canonical checkout",
		"starting and ending HEAD and",
		"`git status --short`",
		"changed paths",
		"exact focused command with cwd, argv, exit status",
		"actual result",
		"completed and remaining work",
		"separately routed blockers",
		"generated-output or fixture evidence",
		"named failing required check with actual output",
		"`deviations: none`",
		"changed detail, rationale, governing authority, and verification",
		"what was already tried",
		"There is no third outcome",
		"The invariants, conventions, and commands in the repository's agent guide bind you",
		"current and target owner",
		"residual debt",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected contract phrase %q in output:\n%s", want, out)
		}
	}

	returnStart := strings.Index(out, "## What to return")
	if returnStart < 0 {
		t.Fatalf("implementer output lacks the return-schema section:\n%s", out)
	}
	returnSchema := out[returnStart:]
	for _, want := range []string{
		"narrow authority",
		"conflict, required authority change",
		"material outcome or scope change",
		"unresolved design fork",
		"unsafe completion",
		"persistently unreachable verification boundary",
	} {
		if !strings.Contains(returnSchema, want) {
			t.Errorf("stopped return schema missing boundary %q:\n%s", want, returnSchema)
		}
	}

	// The claim's second sentence: both dispatching skills name the agent in
	// every dispatch branch, and their own imperatives carry a subject. Rendered
	// with and without subagent-tool capability, since each shape has its own
	// dispatch branch.
	for _, capable := range []bool{true, false} {
		for _, skill := range []string{"subagent-driven-development", "executing-plans"} {
			data := map[string]any{
				"prefix": "example", "vars": map[string]any{}, "data": map[string]any{},
				"skills": map[string]bool{}, "layout": testLayout(),
			}
			if capable {
				data["targetSubagentTools"] = true
			}
			rendered := renderSkillGolden(t, skill, data)
			if !strings.Contains(rendered, "`implementer` agent") {
				t.Errorf("%s (subagentTools=%v) does not name the implementer agent:\n%s", skill, capable, rendered)
			}
		}
	}

	sdd := renderSkillGolden(t, "subagent-driven-development", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "data": map[string]any{},
		"skills": map[string]bool{}, "layout": testLayout(),
	})
	if !strings.Contains(sdd, "You, the dispatching parent, resolve missing phase context") {
		t.Errorf("the raise-concerns imperative lost its explicit subject:\n%s", sdd)
	}
	inline := renderSkillGolden(t, "executing-plans", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "data": map[string]any{},
		"skills": map[string]bool{}, "layout": testLayout(),
	})
	if !strings.Contains(inline, "You, the parent executing this plan, resolve a missing or stale phase path") {
		t.Errorf("executing-plans' raise-concerns imperative lost its explicit subject:\n%s", inline)
	}
	for _, subject := range []string{"you preserve the plan's settled", "You run `./awf context"} {
		if !strings.Contains(inline, subject) {
			t.Errorf("executing-plans lost the explicit subject %q:\n%s", subject, inline)
		}
	}
	if !strings.Contains(inline, "You independently inventory each return") {
		t.Errorf("executing-plans' batch imperatives lost their explicit subject:\n%s", inline)
	}
}

// invariant: rendering/workflow-skill-templates:explorer-and-grounding-role-contracts (TestExplorerAgent)
func TestExplorerAgent(t *testing.T) {
	out := renderAgentGolden(t, "explorer", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "data": map[string]any{},
	})

	if !strings.Contains(out, "name: explorer") {
		t.Errorf("expected 'name: explorer' in output:\n%s", out)
	}
	// One literal per clause the claim enumerates, so a dropped section fails
	// loudly and no clause of the claim outruns its proof.
	for _, want := range []string{
		"This is report-only: do not edit files or commit",
		"Handle exactly one information need",
		"Do not bundle unrelated questions and do not recursively delegate",
		"refinement of an earlier result stays sequential",
		"Breadth is ordered targeted < bounded < broad",
		"paths < summary < analysis",
		"independent of breadth",
		"Ground every material claim with file:line evidence",
		"Return only the relevant final report, never the search narrative or intermediate activity",
		"Retain no search session or state",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected contract phrase %q in output:\n%s", want, out)
		}
	}

	// The rendered body carries no per-call or runtime-specific text: the
	// selected breadth and detail and Pi's limiter are per-call suffixes the
	// extension appends, not contract prose.
	for _, banned := range []string{"Selected breadth maximum", "at most ten active exploration children"} {
		if strings.Contains(out, banned) {
			t.Errorf("per-call or runtime-specific text %q leaked into the rendered body:\n%s", banned, out)
		}
	}
}

// invariant: rendering/workflow-skill-templates:explorer-and-grounding-role-contracts (TestGroundingCheckerAgent)
func TestGroundingCheckerAgent(t *testing.T) {
	out := renderAgentGolden(t, "grounding-checker", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "data": map[string]any{},
	})

	if !strings.Contains(out, "name: grounding-checker") {
		t.Errorf("expected 'name: grounding-checker' in output:\n%s", out)
	}
	for _, want := range []string{
		"do not edit files or commit",
		"Work only from the brief you were given",
		"never edit it",
		// The shared orientation ladder partial reaches this contract too,
		// including its current-state-first ordering and the conditional that
		// keeps history off every dispatch.
		"Ground guide-first, in order",
		"domain docs under `docs/domains`",
		"Current-state documentation is what binds",
		"only when current state leaves what you are seeing unexplained",
		"For managed context calls, provide one or more explicit paths",
		"omit `--show` and `--full` detail flags on the initial query",
		"do the named types, functions, and packages exist",
		"Surface unstated assumptions",
		"Assess whether the work needs a decision record",
		"Check convention fit",
		"advisory and single-pass",
		"open-question | possible-issue",
		"confidence: verified | interpreted | unverified",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected contract phrase %q in output:\n%s", want, out)
		}
	}

	// The claim's closing clause quantifies over BOTH bodies, so the negative
	// sweep has to hold here too, not only in TestExplorerAgent.
	for _, banned := range []string{"Selected breadth maximum", "at most ten active exploration children"} {
		if strings.Contains(out, banned) {
			t.Errorf("per-call or runtime-specific text %q leaked into the rendered body:\n%s", banned, out)
		}
	}
}
