package project

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/templates"
)

func renderGolden(t *testing.T, tmplPath string, data map[string]any) string {
	t.Helper()
	src, err := fs.ReadFile(templates.FS, tmplPath)
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	withLayoutDefaults(data)
	expanded, err := render.ExpandIncludes(string(src), templates.FS)
	if err != nil {
		t.Fatalf("expand includes: %v", err)
	}
	asm, parts := assemble(parseSections(expanded), nil, render.HTMLComment)
	out, err := render.Execute(asm, data, parts, "test")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	assertNoLeaks(t, out)
	return out
}

// withLayoutDefaults seeds the always-present .layout members ADR-0013 added
// (docs/workflowRef/domainsDir) into a golden test's layout fixture when absent,
// so templates citing them render without a <no value> token. The docs map
// carries the docs the templates cite so guarded clauses render as before; a test
// that needs different values sets them explicitly and this leaves them untouched.
func withLayoutDefaults(data map[string]any) {
	if _, ok := data["skills"]; !ok {
		// The real render context always carries the effective-skills set
		// (ADR-0046); a typed nil map keeps chained .skills.X access safe.
		data["skills"] = map[string]bool{}
	}
	l, _ := data["layout"].(map[string]any)
	if l == nil {
		l = map[string]any{}
		data["layout"] = l
	}
	if _, ok := l["docs"]; !ok {
		l["docs"] = map[string]any{
			"debugging": "docs/debugging.md",
			"pitfalls":  "docs/pitfalls.md",
			"roadmap":   "docs/roadmap.md",
		}
	}
	if _, ok := l["workflowRef"]; !ok {
		l["workflowRef"] = "docs/workflow.md"
	}
	if _, ok := l["domainsDir"]; !ok {
		l["domainsDir"] = "docs/domains"
	}
	if _, ok := l["maintainableCodeDesign"]; !ok {
		l["maintainableCodeDesign"] = "docs/maintainable-code-design.md"
	}
}

func assertNoLeaks(t *testing.T, out string) {
	t.Helper()
	if strings.Contains(out, "awf:section") || strings.Contains(out, "awf:end") {
		t.Errorf("markers leaked:\n%s", out)
	}
	if strings.Contains(out, "<no value>") {
		t.Errorf("missing sample data (rendered <no value>):\n%s", out)
	}
	if strings.Contains(out, "{{") || strings.Contains(out, "}}") {
		t.Errorf("unrendered template action:\n%s", out)
	}
}

func renderAgentGolden(t *testing.T, name string, data map[string]any) string {
	t.Helper()
	body := renderGolden(t, "agents/"+name+".md.tmpl", data)
	description, err := render.Execute(catalog.Standard.Agents[name].Description, data, nil, "agent description")
	if err != nil {
		t.Fatalf("render agent description: %v", err)
	}
	out, err := encodeMarkdownAgent(agent{Name: catalog.Standard.Agents[name].Name, Description: description, Body: body})
	if err != nil {
		t.Fatalf("encode agent: %v", err)
	}
	return out
}

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
		"ordered before dependent behavior",
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

	// Assert impl-lens phrases (correctness and plan-adherence are code-reviewer-specific)
	implPhrases := []string{
		"correctness",
		"plan-adherence",
		"maintainable-design",
		"docs/maintainable-code-design.md",
		"cohesion, coupling, dependency direction, representation leakage, duplicated policy, testability, needless indirection, and conformance to the settled design",
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
		"Scoped implementation subagent for example",
		"Phase owner",
		"commits disabled",
		"act as a helper and say so in your report",
		"Your brief is the whole job",
		"In phase-owner mode, an omitted path alone is not a reason to stop",
		"report every added path as a deviation",
		"In helper mode, never modify an unassigned path",
		"necessity to the parent so it can preserve the ownership partition",
		"adding an abstraction with no current call site",
		"Its skill catalog and workflow-chain routing do not bind you",
		"no workflow skill, create or resume no effort, and write to no working-memory file",
		"Iterating on failures is the work",
		"Never make a check pass by weakening what it proves",
		"There is nobody to wait for",
		"That report is the escalation",
		"`awf check staged`",
		"`make gate`",
		// Each enumerated stopped field and owner step gets its own want, since
		// section parity catches only whole-section loss, not intra-section drift.
		"Stage the complete transaction explicitly, by path",
		"Create exactly one commit",
		"the exact output of `git status --short`",
		"what you completed",
		"what remains",
		"either the failing required check, named, with its actual output",
		"`deviations: none`",
		"changed detail, rationale, governing authority, and verification",
		"what you already tried, so the next attempt does not repeat it",
		"There is no third outcome",
		"The invariants, conventions, and commands in the repository's agent guide bind you",
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
		"narrow authority conflict",
		"required authority change",
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
	for _, subject := range []string{"you preserve the plan's settled", "You run `awf context"} {
		if !strings.Contains(inline, subject) {
			t.Errorf("executing-plans lost the explicit subject %q:\n%s", subject, inline)
		}
	}
	if !strings.Contains(inline, "You inventory each return") {
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

// invariant: rendering/workflow-skill-templates:maintainable-code-review-lenses (TestMaintainableCodeReviewLenses)
func TestMaintainableCodeReviewLenses(t *testing.T) {
	outputs := map[string]string{
		"plan": renderAgentGolden(t, "plan-reviewer", map[string]any{
			"prefix": "example", "vars": map[string]any{}, "data": map[string]any{},
			"layout": map[string]any{"plansDir": "docs/plans"},
		}),
		"code": renderAgentGolden(t, "code-reviewer", map[string]any{
			"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "layout": testLayout(),
		}),
		"adr": renderAgentGolden(t, "adr-reviewer", map[string]any{
			"prefix": "example", "vars": map[string]any{}, "data": map[string]any{},
			"layout": map[string]any{"adrDir": "docs/decisions", "indexMd": "docs/decisions/INDEX.md"},
		}),
	}
	contracts := map[string][]string{
		"plan": {
			"model", "ownership", "representations", "translation boundaries", "dependency direction", "test seams",
			"ordered before dependent behavior", "bounded to the failure they prevent", "deterministically verifiable",
			"approved, deferred, or declined disposition", "needless indirection", "pattern mandates", "unapproved or unjustified abstraction, indirection, validation, test machinery, tooling, cleanup, or process", "Do not demand additions merely because more structure, testing, cleanup, or validation is imaginable",
		},
		"code": {
			"cohesion", "coupling", "dependency direction", "representation leakage", "duplicated policy", "testability",
			"needless indirection", "conformance to the settled design", "behavior bolted onto an unsuitable abstraction",
			"refactoring scope silently broadened", "unapproved or unjustified abstraction, indirection, validation, test machinery, tooling, cleanup, or process", "Do not demand additions merely because more structure, testing, cleanup, or validation is imaginable",
		},
		"adr": {
			"semantic model", "representation", "module/package boundary", "dependency direction", "ownership boundary",
			"comparable structural contract", "only when a Decision changes", "cohesion", "representation isolation",
			"enabling-refactor disposition", "testable seams", "justification for indirection", "skip this lens",
		},
	}
	for name, out := range outputs {
		for _, want := range append([]string{"docs/maintainable-code-design.md", "Report-only"}, contracts[name]...) {
			if !strings.Contains(out, want) {
				t.Errorf("%s reviewer missing %q:\n%s", name, want, out)
			}
		}
		for _, line := range strings.Split(out, "\n") {
			directive := strings.TrimLeft(strings.TrimSpace(line), "-*#0123456789. )")
			lower := strings.ToLower(directive)
			for _, banned := range []string{"edit ", "fix ", "apply a fix", "commit ", "loop a re-review", "re-review "} {
				if strings.HasPrefix(lower, banned) {
					t.Errorf("%s reviewer must remain report-only, found directive %q:\n%s", name, directive, out)
				}
			}
		}
	}
}

func renderSkillGolden(t *testing.T, skill string, data map[string]any) string {
	t.Helper()
	return renderGolden(t, "skills/"+skill+"/SKILL.md.tmpl", data)
}

// invariant: rendering/workflow-skill-templates:authority-guided-implementation-autonomy (TestAuthorityGuidedImplementationAutonomy)
func TestAuthorityGuidedImplementationAutonomy(t *testing.T) {
	partial, err := fs.ReadFile(templates.FS, "partials/implementation-autonomy.md")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(partial), "{{") {
		t.Errorf("shared autonomy partial must remain variable-free:\n%s", partial)
	}
	for _, line := range strings.Split(string(partial), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			t.Errorf("shared autonomy partial must not interrupt consumer structure with heading %q", line)
		}
	}

	consumers := []string{"agents/implementer.md.tmpl", "skills/executing-direct/SKILL.md.tmpl", "skills/bugfix/SKILL.md.tmpl", "skills/tdd/SKILL.md.tmpl", "skills/executing-plans/SKILL.md.tmpl", "skills/subagent-driven-development/SKILL.md.tmpl", "skills/reviewing-impl/SKILL.md.tmpl"}
	variants := map[string]map[string]any{
		"configured": {
			"prefix": "example", "vars": map[string]any{"gateCmd": "./x gate"}, "layout": testLayout(),
			"data": catalog.Standard.Agents["plan-reviewer"].Data, "skills": map[string]bool{"reviewing-impl": true}, "targetSubagentTools": true,
		},
		"empty": {"prefix": "example", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{}, "skills": map[string]bool{}},
	}
	wants := []string{
		"Resolve implementation findings autonomously",
		"applicable ADRs, current-state claims, and repository authority",
		"approved outcome, material scope, settled durable boundaries, and required verification",
		"Diagnose a source contradiction, correctness or safety concern, review finding, blocker symptom, or failed check",
		"reasoned non-mechanical deviation records its changed detail, rationale, governing authority, and verification",
		"commit-capable phase owner may add an omitted path",
		"necessary to complete the approved outcome",
		"reports every added path as a reasoned deviation",
		"An omitted path alone is not a reason to stop",
		"Do not replan the approved outcome, broaden material scope, overturn settled structural choices, weaken an oracle, or perform unrelated cleanup",
		"authorities conflict or must change",
		"approved outcome or material scope must change",
		"genuine unresolved design fork remains",
		"safe or correct completion inside the boundary is impossible",
		"required verification remains unreachable after reasonable diagnosis and remediation",
	}
	obsolete := []string{
		"If a newly discovered need affects behavior, scope, structure, dependencies, patterns, checks, or testing strategy",
		"complete approval-requiring invalidating-source report",
	}
	for _, consumer := range consumers {
		raw, err := fs.ReadFile(templates.FS, consumer)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Count(string(raw), "<!-- awf:include implementation-autonomy -->"); got != 1 {
			t.Errorf("%s has %d autonomy includes, want 1", consumer, got)
		}
		for variant, data := range variants {
			var out string
			if strings.HasPrefix(consumer, "agents/") {
				out = renderAgentGolden(t, "implementer", data)
			} else {
				out = renderSkillGolden(t, strings.Split(consumer, "/")[1], data)
			}
			assertNoLeaks(t, out)
			for _, want := range wants {
				if !strings.Contains(out, want) {
					t.Errorf("%s/%s missing %q", variant, consumer, want)
				}
			}
			for _, reject := range obsolete {
				if strings.Contains(out, reject) {
					t.Errorf("%s/%s retains obsolete broad-stop rule %q", variant, consumer, reject)
				}
			}
			if consumer == "skills/executing-plans/SKILL.md.tmpl" {
				for _, want := range []string{"amend the mutable plan immediately", "record the reasoned deviation in Notes"} {
					if !strings.Contains(out, want) {
						t.Errorf("%s/%s missing inline reconciliation directive %q", variant, consumer, want)
					}
				}
			}
		}
	}
}

// invariant: rendering/workflow-skill-templates:independent-workflow-escalation (TestProductionCodeOutlineApprovalProjection)
func TestProductionCodeOutlineApprovalProjection(t *testing.T) {
	partial, err := fs.ReadFile(templates.FS, "partials/production-code-outline-approval.md")
	if err != nil {
		t.Fatal(err)
	}
	body := string(partial)
	if strings.Contains(body, "{{") {
		t.Errorf("outline approval partial must remain variable-free:\n%s", body)
	}
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			t.Errorf("outline approval partial must not interrupt consumer structure with heading %q", line)
		}
	}
	for _, want := range []string{
		"hand-authored production-code mutation", "mechanical production refactors", "tests that prepare a production change",
		"documentation-only", "test-only maintenance", "generated-output-only", "non-code mechanical",
		"retained conversation", "Decision-log evidence", "explicit request to execute a named plan", "Architecture summary",
		"brainstorming is the sole owner", "parent-supplied approved boundary", "never recreate the approval interaction",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("outline approval partial missing %q", want)
		}
	}
	for _, consumer := range []string{
		"agents/implementer.md.tmpl", "skills/executing-direct/SKILL.md.tmpl", "skills/tdd/SKILL.md.tmpl",
		"skills/bugfix/SKILL.md.tmpl", "skills/executing-plans/SKILL.md.tmpl", "skills/subagent-driven-development/SKILL.md.tmpl",
		"skills/writing-plans/SKILL.md.tmpl", "skills/proposing-adr/SKILL.md.tmpl",
	} {
		raw, err := fs.ReadFile(templates.FS, consumer)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Count(string(raw), "<!-- awf:include production-code-outline-approval -->"); got != 1 {
			t.Errorf("%s has %d outline approval includes, want 1", consumer, got)
		}
	}
}

func TestExecutingDirectTemplate(t *testing.T) {
	out := renderSkillGolden(t, "executing-direct", map[string]any{
		"prefix": "example", "vars": map[string]any{"gateCmd": "./x gate", "checkCmd": "./x check"},
		"data": map[string]any{}, "layout": map[string]any{"workflowRef": "docs/workflow.md"},
	})
	for _, want := range []string{"name: example-executing-direct", "direct implementation", "example-reviewing-impl"} {
		if !strings.Contains(out, want) {
			t.Errorf("executing-direct output missing %q:\n%s", want, out)
		}
	}
}

// invariant: rendering/workflow-skill-templates:mandatory-approval-boundaries (TestMaintainableCodeStageCoverage)
// invariant: rendering/workflow-skill-templates:maintainable-code-stage-coverage (TestMaintainableCodeStageCoverage)
func TestMaintainableCodeStageCoverage(t *testing.T) {
	allSkills := map[string]bool{
		"debugging": true, "reviewing-impl": true, "tdd": true,
	}
	type stageContract struct {
		wants   []string
		rejects []string
	}
	patternToolbox := []string{
		"Patterns are a non-exhaustive vocabulary",
		"Strategy can select among genuinely varying policies",
		"Adapter can isolate an incompatible representation",
		"Facade can present a focused entry point",
		"Value objects can protect a value",
		"Repositories can isolate storage concerns",
		"Ports-and-adapters can keep policy independent",
	}
	cases := map[string]stageContract{
		"brainstorming": {wants: []string{
			"docs/maintainable-code-design.md", "semantic model and ownership", "representation boundaries", "dependency direction", "test seams", "preparatory-refactor decision", "proportionate simplicity contract", "scope and exclusions", "structural approach and dependencies", "patterns or abstractions", "checks and testing strategy", "few sentences rather than a fixed checklist", "final approved design becomes the implementation boundary",
		}},
		"proposing-adr": {wants: []string{
			"docs/maintainable-code-design.md", "settled model and ownership, boundaries, dependency direction, constraints, and enabling work", "here and in Decision", "do not replace them with a pattern name",
		}},
		"refactor-coupling-audit": {wants: []string{
			"docs/maintainable-code-design.md", "duplication, coupling, representation leakage, or a workaround", "bounded enabling-refactor or larger-work result", "perform it first, include it in the current effort, defer it in a durable project-owned record, or decline it with the trade-off stated", "ADR scope", "Context section before", "Decision is drafted", "Scope shrink rule", "defer X",
		}},
		"writing-plans": {wants: []string{
			"docs/maintainable-code-design.md", "settled model and ownership, boundaries, dependency direction, representation translations, refactor decision, prohibited shortcuts, and validation", "ordered executable tasks", "self-contained", "no prior conversation context", "sequencing, coordination, or resumability materially helps", "records and operationalizes approved choices", "rather than inventing speculative structure, checks, or work",
		}},
		"tdd": {wants: []string{
			"docs/maintainable-code-design.md", "bounded enabling refactor", "duplication, coupling, representation leakage, or a workaround", "perform it first, include it in the current effort, defer it in a durable project-owned record, or decline it with the trade-off stated", "smallest behavior-proving, model-supporting seam", "force representation leakage or needless indirection", "confirm it fails for the right reason", "minimal change to pass", "Ground tests, checks, seams, and harness work only in changed behavior, a demonstrated regression, an existing documented contract, or a clearly applicable project invariant", "reject speculative test or policy machinery", "Resolve implementation findings autonomously", "applicable ADRs, current-state claims, and repository authority", "approved outcome, material scope, settled durable boundaries", "Stop and report through the active workflow only",
		}},
		"executing-plans": {wants: []string{
			"docs/maintainable-code-design.md", "preserve the plan's settled structural choices", "bounded enabling refactor", "reassess if grounded source contradicts them", "stop rather than bolt correctness onto the wrong abstraction", "do not drift from the plan", "Resolve implementation findings autonomously", "applicable ADRs, current-state claims, and repository authority", "approved outcome, material scope, settled durable boundaries", "Stop and report through the active workflow only",
		}},
		"executing-direct": {wants: []string{
			"docs/maintainable-code-design.md", "assess bounded enabling refactoring before editing", "preserve settled boundaries", "no independent need for brainstorming", "material choice or clarification", "Re-evaluate planning", "only when that independent need fires", "Resolve implementation findings autonomously", "applicable ADRs, current-state claims, and repository authority", "approved outcome, material scope, settled durable boundaries", "Stop and report through the active workflow only",
		}},
		"subagent-driven-development": {wants: []string{
			"docs/maintainable-code-design.md", "preserve the plan's settled structural choices", "bounded enabling refactor", "reassess them if grounded source contradicts them", "stop and escalate rather than accept a bolt-on workaround", "Sequential dispatch only, never parallel", "complete phase", "allowCommits: true", "Resolve implementation findings autonomously", "applicable ADRs, current-state claims, and repository authority", "approved outcome, material scope, settled durable boundaries", "Stop and report through the active workflow only",
		}},
		"bugfix": {wants: []string{
			"docs/maintainable-code-design.md", "unsuitable model or boundary", "bounded enabling work that prevents a workaround", "perform it first, include it in the current effort, defer it in a durable project-owned record, or decline it with the trade-off stated", "root-cause fix, not the symptom", "one concern per commit", "Resolve implementation findings autonomously", "applicable ADRs, current-state claims, and repository authority", "approved outcome, material scope, settled durable boundaries", "Stop and report through the active workflow only",
		}},
	}
	for skill, contract := range cases {
		t.Run(skill, func(t *testing.T) {
			out := renderSkillGolden(t, skill, map[string]any{
				"prefix": "example", "vars": map[string]any{}, "data": map[string]any{},
				"layout": testLayout(), "skills": allSkills, "targetSubagentTools": true,
			})
			assertNoLeaks(t, out)
			for _, want := range contract.wants {
				if !strings.Contains(out, want) {
					t.Errorf("%s output missing %q:\n%s", skill, want, out)
				}
			}
			for _, reject := range append(patternToolbox, contract.rejects...) {
				if strings.Contains(out, reject) {
					t.Errorf("%s output must not duplicate guide pattern-toolbox or later-stage handoff prose %q:\n%s", skill, reject, out)
				}
			}
		})
	}
}

// invariant: rendering/workflow-skill-templates:maintainable-code-subagent-contract (TestMaintainableCodeSubagentContract)
func TestMaintainableCodeSubagentContract(t *testing.T) {
	renderSection := func(t *testing.T, templateID, section string, data map[string]any) string {
		t.Helper()
		src, err := fs.ReadFile(templates.FS, templateID)
		if err != nil {
			t.Fatalf("read %s: %v", templateID, err)
		}
		expanded, err := render.ExpandIncludes(string(src), templates.FS)
		if err != nil {
			t.Fatalf("expand %s: %v", templateID, err)
		}
		for _, segment := range parseSections(expanded) {
			if segment.IsSection && segment.Name == section {
				out, err := render.Execute(segment.Text, data, nil, "test")
				if err != nil {
					t.Fatalf("render %s section %s: %v", templateID, section, err)
				}
				assertNoLeaks(t, out)
				return out
			}
		}
		t.Fatalf("%s has no %s section", templateID, section)
		return ""
	}
	const subagentTemplate = "skills/subagent-driven-development/SKILL.md.tmpl"
	const scopedContext = "semantic boundary and ownership, external/internal representations and their translation point, allowed dependency direction, preparatory-refactor decision, prohibited bolt-on shortcuts, validation expectations"
	data := map[string]any{"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "layout": testLayout()}

	context := renderSection(t, subagentTemplate, "procedure-extract-context", data)
	if !strings.Contains(context, scopedContext) {
		t.Errorf("scoped context is not closed to the six task-relevant categories:\n%s", context)
	}

	for _, tc := range []struct {
		name, dispatch, review, reportOnly string
		data                               map[string]any
		wants                              []string
	}{
		{
			name: "Pi", data: map[string]any{"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "layout": testLayout(), "targetSubagentTools": true},
			dispatch: "known clean and green baseline", review: "Review is report-only and phase-level", reportOnly: "parent-owned",
			wants: []string{"allowCommits: true", "complete phase", "stages the complete transaction"},
		},
		{
			name: "generic", data: data,
			dispatch: "known clean and green baseline", review: "Review is report-only and phase-level", reportOnly: "parent-owned",
			wants: []string{"complete phase", "stages the complete transaction"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dispatch := renderSection(t, subagentTemplate, "dispatch-conventions", tc.data)
			if !strings.Contains(dispatch, tc.dispatch) {
				t.Errorf("dispatch branch missing its instruction %q:\n%s", tc.dispatch, dispatch)
			}
			for _, want := range tc.wants {
				if !strings.Contains(dispatch, want) {
					t.Errorf("dispatch branch missing %q:\n%s", want, dispatch)
				}
			}
			review := renderSection(t, subagentTemplate, "per-task-review", tc.data)
			if !strings.Contains(review, tc.review) {
				t.Errorf("review branch missing its instruction %q:\n%s", tc.review, review)
			}
			if !strings.Contains(review, tc.reportOnly) {
				t.Errorf("review branch lost report-only clause %q:\n%s", tc.reportOnly, review)
			}
		})
	}

	implementer := renderAgentGolden(t, "implementer", data)
	for _, want := range []string{
		"Resolve implementation findings autonomously",
		"applicable ADRs, current-state claims, and repository authority",
		"reasoned non-mechanical deviation records its changed detail, rationale, governing authority, and verification",
		"commit-capable phase owner may add an omitted path",
		"reports every added path as a reasoned deviation",
		"An omitted path alone is not a reason to stop",
		"report every added path as a deviation",
		"In helper mode, never modify an unassigned path",
		"necessity to the parent so it can preserve the ownership partition",
		"Do not replan the approved outcome, broaden material scope",
		"or perform unrelated cleanup",
		"`deviations: none` or each deviation with changed detail, rationale, governing authority, and verification",
	} {
		if !strings.Contains(implementer, want) {
			t.Errorf("implementer contract missing authority-preserving deviation clause %q:\n%s", want, implementer)
		}
	}

	inline := renderSection(t, "skills/executing-plans/SKILL.md.tmpl", "procedure-per-task", data)
	const inlineContext = "Iterate phases, not tasks"
	if !strings.Contains(inline, inlineContext) {
		t.Errorf("inline current-task context is not closed to the six categories:\n%s", inline)
	}
}

// invariant: rendering/workflow-skill-templates:implementer-context-grounding (TestManagedContextCallersChooseProjection)
func TestManagedContextCallersChooseProjection(t *testing.T) {
	policies := map[string]string{
		"adr-lifecycle":               "--show pending",
		"brainstorming":               "",
		"bugfix":                      "",
		"debugging":                   "",
		"executing-plans":             "",
		"grounding":                   "",
		"orienting":                   "",
		"refactor-coupling-audit":     "",
		"reviewing-adr":               "--show references",
		"reviewing-impl":              "--show invariants --show all-rules --show evidence --show pending",
		"reviewing-plan":              "--show invariants --show all-rules --show evidence --show pending",
		"subagent-driven-development": "",
		"tdd":                         "",
		"writing-plans":               "",
	}
	clarifiedOrientationCallers := map[string]bool{
		"bugfix":                  true,
		"debugging":               true,
		"refactor-coupling-audit": true,
		"tdd":                     true,
		"writing-plans":           true,
	}
	const clarifiedOrientation = "Start by querying the explicit paths named above without `--show` or `--full` detail flags"
	spillBytes, err := fs.ReadFile(templates.FS, "partials/context-spill.md")
	if err != nil {
		t.Fatalf("read context spill partial: %v", err)
	}
	spillContract := strings.TrimSpace(string(spillBytes))
	seen := map[string]bool{}
	for name := range catalog.Standard.Skills {
		templateID := "skills/" + name + "/SKILL.md.tmpl"
		source, err := fs.ReadFile(templates.FS, templateID)
		if err != nil {
			t.Fatalf("read %s: %v", templateID, err)
		}
		expanded, err := render.ExpandIncludes(string(source), templates.FS)
		if err != nil {
			t.Fatalf("expand %s: %v", templateID, err)
		}
		if clarifiedOrientationCallers[name] && !strings.Contains(expanded, clarifiedOrientation) {
			t.Errorf("%s lacks clarified explicit-path orientation", templateID)
		}
		callCount := 0
		for lineNumber, line := range strings.Split(expanded, "\n") {
			if !strings.Contains(line, "awf context") && !strings.Contains(line, "./x context") {
				continue
			}
			callCount++
			seen[name] = true
			policy, ok := policies[name]
			if !ok {
				t.Errorf("%s:%d has an unclassified context invocation: %s", templateID, lineNumber+1, line)
				continue
			}
			if strings.Contains(line, "--full") || strings.Contains(line, "--json") {
				t.Errorf("%s:%d prescribes a retired context form: %s", templateID, lineNumber+1, line)
			}
			commandName := "awf context"
			if strings.Contains(line, "./x context") {
				commandName = "./x context"
			}
			commandTail := strings.SplitN(line, commandName, 2)[1]
			commandTail = strings.SplitN(commandTail, "`", 2)[0]
			if !strings.Contains(commandTail, "path>") && !strings.Contains(commandTail, "paths>") && !strings.Contains(commandTail, "$(") {
				t.Errorf("%s:%d context invocation must select explicit paths or Git-selected files: %s", templateID, lineNumber+1, line)
			}
			if policy == "" {
				if strings.Contains(line, "--show") {
					t.Errorf("%s:%d orientation invocation must omit detail facets: %s", templateID, lineNumber+1, line)
				}
			} else if !strings.Contains(line, "awf context "+policy) {
				t.Errorf("%s:%d invocation lacks policy %q: %s", templateID, lineNumber+1, policy, line)
			}
			if strings.Contains(name, "reviewing-") && strings.Contains(line, "paste the output of") {
				t.Errorf("%s:%d review dispatch must instruct the reviewer-run command, not paste output: %s", templateID, lineNumber+1, line)
			}
		}
		if got := strings.Count(expanded, spillContract); got != callCount {
			t.Errorf("%s expands the context spill contract %d times for %d call sites", templateID, got, callCount)
		}
	}
	for name := range policies {
		if !seen[name] {
			t.Errorf("managed context template %s has no context invocation", name)
		}
	}
	// The grounding-checker agent body carries the same pointer sentence the
	// skills do (ADR-0197 widened the claim to the agent body).
	agentSource, err := fs.ReadFile(templates.FS, "agents/grounding-checker.md.tmpl")
	if err != nil {
		t.Fatalf("read grounding-checker template: %v", err)
	}
	agentExpanded, err := render.ExpandIncludes(string(agentSource), templates.FS)
	if err != nil {
		t.Fatalf("expand grounding-checker template: %v", err)
	}
	if !strings.Contains(agentExpanded, spillContract) {
		t.Errorf("grounding-checker agent body lacks the spill pointer:\n%s", agentExpanded)
	}
	// The pointer's destination must exist: the working-with-awf doc template is
	// the contract's single rendered home, and deleting the subsection would
	// leave every pointer site dangling with the suite otherwise green.
	docSource, err := fs.ReadFile(templates.FS, "docs/working-with-awf.md.tmpl")
	if err != nil {
		t.Fatalf("read working-with-awf template: %v", err)
	}
	docExpanded, err := render.ExpandIncludes(string(docSource), templates.FS)
	if err != nil {
		t.Fatalf("expand working-with-awf template: %v", err)
	}
	for _, want := range []string{
		"### Context spill notices",
		"byte length equals",
		"`bytes=<decimal>` descriptor",
		"Best-effort delete the named file after packet use",
	} {
		if !strings.Contains(docExpanded, want) {
			t.Errorf("working-with-awf template lacks the spill contract clause %q", want)
		}
	}
	// This repository overrides the doc's commands part, so its own rendered
	// doc is a second home the template assertion cannot see; an override that
	// drops the contract would leave every pointer dangling here.
	repoDoc, err := os.ReadFile(filepath.Clean(filepath.Join("..", "..", "docs", "working-with-awf.md")))
	if err != nil {
		t.Fatalf("read repository working-with-awf doc: %v", err)
	}
	for _, want := range []string{
		"### Context spill notices",
		"byte length equals",
		"`bytes=<decimal>` descriptor",
		"Best-effort delete the named file after packet use",
	} {
		if !strings.Contains(string(repoDoc), want) {
			t.Errorf("repository working-with-awf doc lacks the spill contract clause %q", want)
		}
	}
}

// invariant: rendering/workflow-skill-templates:authority-guided-implementation-autonomy (TestConditionalVerifyPass)
// invariant: rendering/workflow-skill-templates:authority-guided-review-remediation (TestConditionalVerifyPass)
// TestConditionalVerifyPass pins ADR-0197 item 3: the reviewing skills
// dispatch the verify pass only for reasoned or user-decision fixes, a
// solely-mechanical round records the skip, and a fix-free round dispatches
// nothing.
func TestConditionalVerifyPass(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars":   map[string]any{},
		"layout": testLayout(),
		"data":   map[string]any{},
	}
	for _, name := range []string{"reviewing-adr", "reviewing-plan"} {
		t.Run(name, func(t *testing.T) {
			out := renderSkillGolden(t, name, data)
			assertOrderedPhrases(t, out,
				"**Verify pass.**",
				"applied no fixes dispatches no verify pass",
				"all classified `mechanical` skips it",
				"recording the skip and its ground",
				"classified `reasoned` or was applied under a `user-decision` ruling",
			)
		})
	}

	implementationReview := renderSkillGolden(t, "reviewing-impl", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{}, "skills": map[string]bool{"effort-workflow": true}, "targetSubagentTools": true,
	})
	for _, want := range []string{"locally obvious, low-risk, directly verified", "Uncertainty resolves toward review", "Effort-free review creates no effort", "returns to `example-effort-workflow`"} {
		if !strings.Contains(implementationReview, want) {
			t.Errorf("implementation review missing %q", want)
		}
	}
}

// TestCheckpointDigestShape pins the compression ADR-0197 item 2 delivers: the
// routine and approval checkpoint partials each stay a four-step digest, so a
// re-expanded fifth step cannot creep back in with the ordered-phrase proofs
// still green.
// invariant: rendering/workflow-skill-templates:memory-checkpoint-chain-coverage (TestCheckpointDigestShape)
// invariant: rendering/workflow-skill-templates:authority-guided-implementation-autonomy (TestCheckpointDigestShape)
func TestCheckpointDigestShape(t *testing.T) {
	for _, partial := range []string{"partials/checkpoint-routine.md", "partials/checkpoint-approval.md"} {
		raw, err := fs.ReadFile(templates.FS, partial)
		if err != nil {
			t.Fatalf("read %s: %v", partial, err)
		}
		body := string(raw)
		steps := 0
		for _, line := range strings.Split(body, "\n") {
			trimmed := strings.TrimSpace(line)
			if len(trimmed) > 1 && trimmed[0] >= '1' && trimmed[0] <= '9' && strings.HasPrefix(trimmed[1:], ". ") {
				steps++
			}
		}
		if steps != 4 {
			t.Errorf("%s renders %d numbered steps, want the four-step digest", partial, steps)
		}
		if strings.Contains(body, "awf effort new") {
			t.Errorf("%s creates missing effort ownership", partial)
		}
		if got := strings.Count(body, "./awf effort memory update"); got != 1 {
			t.Errorf("%s has %d structured memory updates, want exactly one", partial, got)
		}
		for _, phrase := range []string{
			"either legacy `Effort: <slug>` or canonical `effort: <slug>` identity",
			"canonical form is YAML",
			"legacy form is deprecated",
			"until active efforts finish",
			"sole writer of phase, next action, and time",
			"executable `awf read plan` projection never creates a checkpoint or handoff boundary",
			"## Handoff log",
		} {
			if !strings.Contains(body, phrase) {
				t.Errorf("%s missing checkpoint contract %q", partial, phrase)
			}
		}
		for _, direct := range []string{"set `Phase:`", "set `Next:`", "refresh `Updated:`"} {
			if strings.Contains(body, direct) {
				t.Errorf("%s directly edits checkpoint metadata with %q", partial, direct)
			}
		}
	}

	routine, err := fs.ReadFile(templates.FS, "partials/checkpoint-routine.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"correctness or safety concern, blocker, or failed required verification",
		"remains unresolved after the active workflow's required diagnosis and authority-guided remediation",
	} {
		if !strings.Contains(string(routine), want) {
			t.Errorf("routine checkpoint missing unresolved-only attention boundary %q", want)
		}
	}

	confirmation, err := fs.ReadFile(templates.FS, "partials/outcome-confirmation.md")
	if err != nil {
		t.Fatalf("read outcome confirmation partial: %v", err)
	}
	body := string(confirmation)
	if strings.Count(body, "**Mandatory first-creation confirmation.**") != 1 {
		t.Error("outcome confirmation partial must carry exactly one boundary header")
	}
	for _, want := range []string{"`Outcome: <confirmed outcome>`", "`Effort title: <proposed title>`", "`Effort slug: <proposed-short-slug>`", "clear response in a later turn", "awf effort new --slug <confirmed-slug> \"<confirmed-title>\""} {
		if !strings.Contains(body, want) {
			t.Errorf("outcome confirmation partial missing %q", want)
		}
	}

	executingPlans, err := fs.ReadFile(templates.FS, "skills/executing-plans/SKILL.md.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	executionBody := string(executingPlans)
	if got := strings.Count(executionBody, "Resolve the mutable plan"); got != 1 {
		t.Errorf("executing-plans has %d plan-resolution steps, want exactly one", got)
	}
	for _, phrase := range []string{
		"awf read plan <plan> <P[.T]>",
		"generated task scope notice",
		"phase-owned Advances and Completes outcomes",
		"projection changes neither phase ownership nor checkpoint boundaries",
		"either legacy `Effort: <slug>` or canonical `effort: <slug>` identity",
		"legacy form is deprecated",
		"until active efforts finish",
	} {
		if !strings.Contains(executionBody, phrase) {
			t.Errorf("executing-plans missing checkpoint contract %q", phrase)
		}
	}
}

func TestWritingPlansTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"gateCmd": "./x gate",
		},
		"layout": map[string]any{"plansDir": "docs/plans", "plansTemplate": "docs/plans/template.md"},
		"data":   map[string]any{},
	}

	out := renderSkillGolden(t, "writing-plans", data)

	// Assert frontmatter name line
	if !strings.Contains(out, "name: example-writing-plans") {
		t.Errorf("expected 'name: example-writing-plans' in output:\n%s", out)
	}

	// Assert load-bearing phrases unique to writing-plans
	loadBearing := []string{
		"exactly one execution mode: `inline` or `subagent-driven`",
		"one independently green coherent implementation transaction",
		"ordered steps",
		"change-specific observable outcomes",
		"relevant authority links",
		"focused evidence",
		"example-reviewing-plan",
		"selected working-tree snapshot before its first commit",
		"mechanical corrections without a durable ledger",
		"batch task",
		"path-disjoint",
		"dead-code escape",
		"nonempty JSON `Applying:` or `Context:` array",
		"stable `dod: <slug>` bullets",
		"frozen `#N` only for pre-V4 Decision prose",
		"generic staging, gate, clean-tree, checkpoint, routing, and reviewer protocol belong to their workflow owners",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
}

func assertOrderedPhrases(t *testing.T, out string, phrases ...string) {
	t.Helper()
	position := 0
	for _, phrase := range phrases {
		next := strings.Index(out[position:], phrase)
		if next < 0 {
			t.Fatalf("expected %q after byte %d:\n%s", phrase, position, out)
		}
		position += next + len(phrase)
	}
}

func TestStagedAuthorityWorkflowTemplates(t *testing.T) {
	configured := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"gateCmd":          "./x gate",
			"activeMdRegenCmd": "awf render",
		},
		"layout": testLayout(),
		"data": map[string]any{
			"adrStates": []map[string]any{{"name": "Proposed", "meaning": "Mutable", "mutability": "Mutable"}},
		},
	}
	for _, name := range []string{"adr-lifecycle", "executing-plans", "subagent-driven-development"} {
		t.Run(name, func(t *testing.T) {
			out := renderSkillGolden(t, name, configured)
			assertOrderedPhrases(t, out, "the complete transaction", "`awf check staged`", "`./x gate`", "wired pre-commit hook enforces both", "only in a clone without wired hooks")
		})
	}

	agents := renderGolden(t, "agents-doc/AGENTS.md.tmpl", configured)
	assertOrderedPhrases(t, agents, "the complete transaction", "`awf check staged`", "`./x gate`", "wired pre-commit hook enforces both", "only in a clone without wired hooks")

	fallback := map[string]any{"prefix": "example", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{}}
	for _, name := range []string{"adr-lifecycle", "executing-plans", "subagent-driven-development"} {
		t.Run(name+"-fallback", func(t *testing.T) {
			out := renderSkillGolden(t, name, fallback)
			assertOrderedPhrases(t, out, "the complete transaction", "`awf check staged`", "the project's gate", "wired pre-commit hook enforces both", "only in a clone without wired hooks")
		})
	}
	fallbackAgents := renderGolden(t, "agents-doc/AGENTS.md.tmpl", fallback)
	assertOrderedPhrases(t, fallbackAgents, "the complete transaction", "`awf check staged`", "the project's gate", "wired pre-commit hook enforces both", "only in a clone without wired hooks")
}

func TestExecutingPlansTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"gateCmd":          "./x gate",
			"gateCmdFull":      "./x gate full",
			"activeMdRegenCmd": "go test ./internal/adrtools/",
		},
		"layout": map[string]any{"plansDir": "docs/plans", "indexMd": "docs/decisions/INDEX.md"},
		"data": map[string]any{
			"e2eSuitePaths": []map[string]any{
				{"path": "tests/e2e/libraries/"},
				{"path": "tests/e2e/web/"},
				{"path": "cli_test.go"},
			},
		},
	}

	out := renderSkillGolden(t, "executing-plans", data)

	// Assert frontmatter name line
	if !strings.Contains(out, "name: example-executing-plans") {
		t.Errorf("expected 'name: example-executing-plans' in output:\n%s", out)
	}

	// Assert load-bearing phrases unique to executing-plans
	loadBearing := []string{
		"Iterate phases, not tasks",
		"parent owns",
		"commit-disabled helpers",
		"report-only phase review",
		"example-reviewing-impl",
		"generated task scope notice",
		"phase-owner context only",
		"never gives a task helper commit, review, checkpoint, handoff, or outcome authority",
		"every explicit batch of the ADR's declared State changes operations",
		"Do not flip terminal artifact status in a phase", "effort-free work, the parent performs", "deferred ADR/plan terminal transaction",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
}

func TestSubagentDrivenDevelopmentTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"gateCmd":          "./x gate",
			"gateCmdFull":      "./x gate full",
			"activeMdRegenCmd": "go test ./internal/adrtools/",
		},
		"layout": map[string]any{"plansDir": "docs/plans", "indexMd": "docs/decisions/INDEX.md"},
		"data":   map[string]any{},
	}

	out := renderSkillGolden(t, "subagent-driven-development", data)

	// Assert frontmatter name line
	if !strings.Contains(out, "name: example-subagent-driven-development") {
		t.Errorf("expected 'name: example-subagent-driven-development' in output:\n%s", out)
	}

	// Assert load-bearing phrases unique to subagent-driven-development
	loadBearing := []string{
		"complete phase",
		"state commit-capable phase-owner mode in the brief",
		"known clean and green baseline",
		"report-only phase review",
		"example-reviewing-impl",
		"example-executing-plans",
		"dirty-state inventory",
		"every explicit V2 batch, including the final batch",
		"Do not flip terminal artifact status in a phase", "effort-free work, the parent performs", "deferred ADR/plan terminal transaction",
		"generated scope notice, Phase close, and Advances/Completes outcomes are phase-owner context only",
		"never transfer commit, review, checkpoint, handoff, helper, or outcome authority",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
}

func TestBugfixTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"gateCmd":     "./x gate",
			"gateCmdFull": "./x gate full",
		},
		"data":   map[string]any{},
		"skills": map[string]bool{"tdd": true, "debugging": true, "reviewing-impl": true},
	}

	out := renderSkillGolden(t, "bugfix", data)

	// Assert frontmatter name line
	if !strings.Contains(out, "name: example-bugfix") {
		t.Errorf("expected 'name: example-bugfix' in output:\n%s", out)
	}

	// Assert load-bearing phrases unique to bugfix
	loadBearing := []string{
		"regression test",
		"root-cause fix",
		"example-reviewing-impl",
		"example-tdd",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
}

func TestTddTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"testCmd": "go test ./...",
			"gateCmd": "./x gate",
		},
		"data":   map[string]any{},
		"skills": map[string]bool{},
	}

	out := renderSkillGolden(t, "tdd", data)

	if !strings.Contains(out, "name: example-tdd") {
		t.Errorf("expected 'name: example-tdd' in output:\n%s", out)
	}

	loadBearing := []string{
		"confirm it fails for the right reason: `go test ./...`",
		"Run the gate: `./x gate`",
		"A test never observed failing proves nothing.",
		"Fix the code, not the oracle.",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
}

func TestDebuggingTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"gateCmd":     "./x gate",
			"gateCmdFull": "./x gate full",
		},
		"data":   map[string]any{},
		"skills": map[string]bool{"tdd": true, "bugfix": true, "brainstorming": true, "exploring": true},
	}

	out := renderSkillGolden(t, "debugging", data)

	// Assert frontmatter name line
	if !strings.Contains(out, "name: example-debugging") {
		t.Errorf("expected 'name: example-debugging' in output:\n%s", out)
	}

	// Assert load-bearing phrases unique to debugging
	loadBearing := []string{
		"falsifiable hypothesis",
		"reproduces the failure",
		"root cause",
		"example-bugfix",
		"example-brainstorming",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
	ordered := []string{
		"**Form one falsifiable hypothesis.**",
		"Invoke `example-exploring`",
		"Pick the cheapest oracle",
		"**Isolate with a failing test, written first.**",
	}
	position := -1
	for _, phrase := range ordered {
		next := strings.Index(out, phrase)
		if next <= position {
			t.Fatalf("debugging order violation at %q: positions must increase in %v", phrase, ordered)
		}
		position = next
	}
}

func TestExploringTemplate(t *testing.T) {
	pi := renderSkillGolden(t, "exploring", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "data": map[string]any{},
		"skills": map[string]bool{}, "targetSubagentTools": true,
	})
	fallback := renderSkillGolden(t, "exploring", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "skills": map[string]bool{},
	})
	for label, body := range map[string]string{"pi": pi, "fallback": fallback} {
		for _, want := range []string{
			"location is unknown and inline search would pollute the parent context",
			"exact-known-file", "genuinely trivial",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s exploring render missing %q:\n%s", label, want, body)
			}
		}
	}
	for _, want := range []string{"subagent_explore", "required task, breadth, and detail"} {
		if !strings.Contains(pi, want) {
			t.Errorf("Pi exploring render missing %q:\n%s", want, pi)
		}
	}
	if !strings.Contains(fallback, "target-native fresh-context exploration subagent") || strings.Contains(fallback, "subagent_explore") {
		t.Errorf("fallback exploring dispatch is not generic:\n%s", fallback)
	}
}

// invariant: rendering/workflow-skill-templates:using-awf-transaction-home (TestUsingAwfTemplate)
func TestUsingAwfTemplate(t *testing.T) {
	out := renderSkillGolden(t, "using-awf", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "data": map[string]any{},
	})
	want := strings.Join([]string{
		"---",
		"name: example-using-awf",
		"description: \"Use when maintaining awf's generated tree: edit `.awf/` sources, render outputs, resolve drift, or upgrade awf.\"",
		"---",
		"",
		"<!-- awf:edit procedure: default; create  to override -->",
		"# example-using-awf",
		"",
		"`.awf/` is the source. Never hand-edit rendered outputs.",
		"",
		"Edit source, render, check, then stage the source, rendered outputs, and `.awf/awf.lock` together; then run the gate. A drift finding carries its own repair hint: follow it rather than guessing at the generated output.",
		"",
		"For an upgrade, run the bootstrap script and then perform the residue sweep. `docs/working-with-awf.md` owns detailed commands and generated-tree guidance; `docs/config-reference.md` owns configuration keys and their meanings.",
		"",
	}, "\n")
	if out != want {
		t.Errorf("using-awf must remain the approved thin transaction body:\n%s", out)
	}
}

// invariant: rendering/workflow-skill-templates:writing-docs-delegation (TestWritingDocsTemplate)
func TestWritingDocsTemplate(t *testing.T) {
	out := renderSkillGolden(t, "writing-docs", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "data": map[string]any{},
	})
	want := strings.Join([]string{
		"---",
		"name: example-writing-docs",
		"description: \"Use when authoring project documentation: select the document that owns the fact and keep it current with the change.\"",
		"---",
		"",
		"<!-- awf:edit procedure: default; create  to override -->",
		"# example-writing-docs",
		"",
		"Select the single document that owns the fact. Read `docs/doc-standard.md` before writing; when another surface owns the detail, reference it rather than restating it. Let the document travel in the commit that makes the fact true.",
		"",
		"When authoring reaches a file edit, invoke `example-using-awf` for the generated-tree transaction. `docs/doc-standard.md` owns the documentation rules.",
		"",
	}, "\n")
	if out != want {
		t.Errorf("writing-docs must remain the approved thin delegation body:\n%s", out)
	}
}

func TestOrientingTemplate(t *testing.T) {
	out := renderSkillGolden(t, "orienting", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "skills": map[string]bool{},
	})
	if !strings.Contains(out, "name: example-orienting") {
		t.Errorf("expected 'name: example-orienting' in output:\n%s", out)
	}
	for _, want := range []string{"Four moments call for orientation", "Ground guide-first, in order", "`example-exploring`", "A discrepancy resolves in favor of the repository"} {
		if !strings.Contains(out, want) {
			t.Errorf("orienting render missing %q:\n%s", want, out)
		}
	}
}

// invariant: rendering/workflow-skill-templates:orienting-single-home (TestOrientingSkillContract)
func TestOrientingSkillContract(t *testing.T) {
	// The same render proves both the single home and the references that
	// replaced the three consumer skills' inline copies.
	config := func(target string) string {
		return "prefix: example\nintegrationBranch: main\n"
	}
	for _, target := range KnownTargets() {
		t.Run(target, func(t *testing.T) {
			files := explorationRenderedByPath(t, config(target))
			adapter := targetRegistry[target]
			body := files[adapter.SkillPath("example", "orienting")]
			if body == "" {
				t.Fatalf("missing rendered orienting skill for %s", target)
			}
			// One literal per property the skill contract promises: a heading
			// count alone would survive deleting the moments it counts.
			for _, want := range []string{
				"Four moments call for orientation",
				"**Fresh work:**", "**Effort resume:**", "**Handoff takeover:**", "**Mid-chain re-orientation:**",
				"Ground guide-first, in order", "domain docs under `docs/domains`",
				"Current-state documentation is what binds",
				"only when current state leaves what you are seeing unexplained",
				"one or more exploration subagents",
				"one information need", "every child is report-only",
				"location is unknown", "and inline search would pollute the parent context",
				"exact-known-file", "genuinely trivial", "`example-exploring`",
				"landed since the checkpoint", "git worktree list", "against the decision index",
				"its decision log including every `Record:` block present", "not yours to re-decide",
				"cited plan and file existence", "A discrepancy resolves in favor of the repository",
				"never creates an effort, never commits", "never prescribe `--full`",
				"single-pass and advisory, never a chain gate",
			} {
				if !strings.Contains(body, want) {
					t.Errorf("%s orienting skill missing %q", target, want)
				}
			}
			agent := files[adapter.AgentPath("grounding-checker")]
			for _, want := range []string{"Ground guide-first, in order", "For managed context calls, provide one or more explicit paths", "omit `--show` and `--full` detail flags on the initial query", "never prescribe `--full`", "AWF_CONTEXT_SPILL_V1"} {
				if !strings.Contains(agent, want) {
					t.Errorf("%s grounding-checker missing %q", target, want)
				}
			}
			// The single home is only single if the three sites that gave up
			// their inline copies reference it instead. Brainstorming's
			// reference is pinned to its first step, which is what the claim says.
			for _, consumer := range []string{"brainstorming", "proposing-adr", "writing-plans"} {
				if ref := files[adapter.SkillPath("example", consumer)]; !strings.Contains(ref, "`example-orienting`") {
					t.Errorf("%s %s does not reference the orienting skill", target, consumer)
				}
			}
			if b := files[adapter.SkillPath("example", "brainstorming")]; !strings.Contains(b, "1. **Orient in the topic.** Invoke `example-orienting`") {
				t.Errorf("%s brainstorming does not invoke orienting as its first step", target)
			}
		})
	}
}

func TestProposingAdrTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"activeMdRegenCmd": "go test ./internal/adrtools/",
			"gateCmd":          "./x gate",
			"checkCmd":         "./x check",
		},
		"layout": map[string]any{
			"adrDir": "docs/decisions", "adrTemplate": "docs/decisions/template.md",
			"indexMd": "docs/decisions/INDEX.md", "adrReadme": "docs/decisions/README.md",
		},
		"data": map[string]any{
			"adrTriggers": []string{
				"new package boundary or top-level directory",
				"auth or security behaviour change",
				"non-trivial new dependency",
				"workflow rule change",
			},
			"adrSections": []string{
				"Context",
				"Decision",
				"Invariants",
				"Consequences",
				"Alternatives Considered",
			},
		},
	}

	out := renderSkillGolden(t, "proposing-adr", data)

	// Assert frontmatter name line
	if !strings.Contains(out, "name: example-proposing-adr") {
		t.Errorf("expected 'name: example-proposing-adr' in output:\n%s", out)
	}

	// Assert the scaffold-first operations as one ordered procedure.
	procedure := "Run `awf new adr \"<Title>\"` before any ADR-file mutation. Capture the exact path it creates. Read the exact file it creates, then edit that scaffold in place."
	if !strings.Contains(out, procedure) {
		t.Errorf("expected ordered procedure %q in output:\n%s", procedure, out)
	}

	// Assert load-bearing phrases unique to proposing-adr.
	loadBearing := []string{
		"one decision per ADR",
		"Never create or replace an ADR by any other mechanism",
		"Context",
		"Consequences",
		"status: Proposed",
		"example-reviewing-adr",
		"remains meaningful after implementation",
		"post-implementation",
		"counterfactual",
		"consent evidence establishes that it is load-bearing and the ADR explains why it is load-bearing",
		"preserve exactly the frontmatter emitted by `awf new adr`",
		"Before any ADR-file mutation, identify the explicitly accepted decision set",
		"narrowest durable commitment",
		"outside the ADR until accepted",
		"effort-free",
		"approved design summary",
		"Decision log",
		"`Record:` evidence",
		"plan or direct execution",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Required frontmatter:") && strings.Contains(line, "current-state-v") {
			t.Errorf("proposing guidance chooses a literal current format in %q:\n%s", line, out)
		}
	}
}

func TestAdrLifecycleTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"activeMdRegenCmd": "go test ./internal/adrtools/",
			"gateCmd":          "./x gate",
		},
		"layout": map[string]any{
			"adrDir": "docs/decisions", "indexMd": "docs/decisions/INDEX.md",
			"adrReadme": "docs/decisions/README.md",
		},
		"data": map[string]any{
			"adrStates": []map[string]any{
				{
					"name":       "Proposed",
					"meaning":    "Under discussion; all sections mutable",
					"mutability": "Mutable; amendments encouraged",
				},
				{
					"name":       "Accepted",
					"meaning":    "Design final; implementation in progress",
					"mutability": "Append-only; only `status` editable in place",
				},
				{
					"name":       "Implemented",
					"meaning":    "Implementation complete; decision enacted",
					"mutability": "Append-only; only `status` editable in place",
				},
				{
					"name":       "Abandoned",
					"meaning":    "Will not be implemented; intended operations stay unapplied",
					"mutability": "Terminal; status and append-only Status history only",
				},
			},
		},
	}

	out := renderSkillGolden(t, "adr-lifecycle", data)

	// Assert frontmatter name line
	if !strings.Contains(out, "name: example-adr-lifecycle") {
		t.Errorf("expected 'name: example-adr-lifecycle' in output:\n%s", out)
	}

	// Assert load-bearing phrases unique to adr-lifecycle
	loadBearing := []string{
		"State changes",
		"status transition",
		"regenerate",
		"Append-only",
		"every explicit Applied batch, including the final batch",
		"direct implicit completion with its matching claim mutations",
		"status-only terminal transaction after explicit application",
		"V4 Decision items begin with a unique inline `decision: <lowercase-kebab-slug>` marker",
		"use canonical `#N` only after their authored-format lifecycle freezes the record",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}

	observable := []string{"distinct claim IDs", "separately observable authored transaction", "exact prefix", "legal ordered lifecycle"}
	scaffold := renderGolden(t, "adr-template/template.md.tmpl", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "skills": map[string]bool{}, "layout": testLayout(),
	})
	for name, body := range map[string]string{"generic ADR scaffold": scaffold, "generic lifecycle skill": out} {
		for _, phrase := range observable {
			if !strings.Contains(body, phrase) {
				t.Errorf("%s missing observable authored-transaction phrase %q:\n%s", name, phrase, body)
			}
		}
	}
	root := testsupport.RepoRoot(t)
	for _, rel := range []string{".claude/skills/awf-adr-lifecycle/SKILL.md", ".pi/skills/awf-adr-lifecycle/SKILL.md"} {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		for _, phrase := range observable {
			if !bytes.Contains(body, []byte(phrase)) {
				t.Errorf("%s missing observable authored-transaction phrase %q", rel, phrase)
			}
		}
	}
}

func TestIndependentWorkflowEscalation(t *testing.T) {
	body := renderGolden(t, "skills/grounding/SKILL.md.tmpl", map[string]any{
		"prefix": "example", "layout": map[string]any{},
	})
	for _, want := range []string{"broad or uncertain repository premises", "advisory, report-only, single-pass, effort-noncreating", "never a workflow-chain prerequisite"} {
		if !strings.Contains(body, want) {
			t.Errorf("grounding missing %q", want)
		}
	}
}

// invariant: rendering/workflow-skill-templates:linked-plan-review-freshness (TestLinkedPlanReviewFreshness)
func TestLinkedPlanReviewFreshness(t *testing.T) {
	data := map[string]any{
		"prefix": "example", "vars": map[string]any{}, "layout": testLayout(),
		"data": map[string]any{}, "skills": map[string]bool{},
	}
	plan := renderSkillGolden(t, "reviewing-plan", data)
	for _, want := range []string{
		"every ADR resolved from parsed plan-level `adrs:`",
		"modification time or session implication",
		"every linked plan whose parsed status remains `Proposed`",
		"inventory completed phases against the changed ADR",
		"renewed implementation assurance for affected landed work",
		"return first to ADR amendment and ordinary ADR review",
		"exactly one fresh `plan-reviewer` verify pass",
	} {
		if !strings.Contains(plan, want) {
			t.Errorf("ordinary plan review missing %q", want)
		}
	}
	adr := renderSkillGolden(t, "reviewing-adr", data)
	for _, want := range []string{
		"substantive amendment or correction while an ADR is `Proposed`, `Accepted`, or `Implementing`",
		"Preserve the ADR's entry status",
		"an `Accepted` or `Implementing` record uses `example-adr-lifecycle`",
		"Preserve the nonterminal status with which the ADR entered review",
		"each plan review inventories completed affected phases",
		"renews assurance where the amended decision can affect landed work",
	} {
		if !strings.Contains(adr, want) {
			t.Errorf("ordinary ADR review missing %q", want)
		}
	}
	assertOrderedPhrases(t, adr, "review converges", "After approval, run `awf context --show references <explicit-ADR-path>`", "Invoke ordinary `example-reviewing-plan` separately for every linked plan")
	if _, ok := catalog.Standard.Skills["reviewing-plan-"+"resync"]; ok {
		t.Fatal("retired plan review skill remains in the live catalog")
	}
}

func TestReviewingPlanTemplate(t *testing.T) {
	data := map[string]any{
		"prefix":       "example",
		"vars":         map[string]any{},
		"commitScopes": "`docs(plans)`",
		"layout":       map[string]any{"plansDir": "docs/plans"},
		"data":         map[string]any{},
	}

	out := renderSkillGolden(t, "reviewing-plan", data)

	// Assert frontmatter name line
	if !strings.Contains(out, "name: example-reviewing-plan") {
		t.Errorf("expected 'name: example-reviewing-plan' in output:\n%s", out)
	}

	// Assert thin-dispatcher load-bearing phrases
	loadBearing := []string{
		"plan-reviewer",
		"user-decision",
		"explicit uncommitted plan path",
		"selected working-tree snapshot",
		"review that snapshot rather than HEAD",
		"mechanical fixes directly without a durable ledger",
		"substantive reasoned or user-decided findings and dispositions in plan Notes",
		"Later substantive corrections remain separate commits",
		"all universal lenses",
		"per-phase ownership",
		"helper partitions",
		"V4 stable `decision:` selectors",
		"Proposed coverage notes are advisory",
		"historical Decision prose never replaces current-state authority",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
}

func TestReviewingAdrTemplate(t *testing.T) {
	data := map[string]any{
		"prefix":       "example",
		"vars":         map[string]any{},
		"commitScopes": "`docs(adr)`",
		"layout":       map[string]any{"adrDir": "docs/decisions", "plansDir": "docs/plans"},
		"data":         map[string]any{},
	}

	out := renderSkillGolden(t, "reviewing-adr", data)

	// Assert frontmatter name line
	if !strings.Contains(out, "name: example-reviewing-adr") {
		t.Errorf("expected 'name: example-reviewing-adr' in output:\n%s", out)
	}

	// Assert thin-dispatcher load-bearing phrases
	loadBearing := []string{
		"adr-reviewer",
		"user-decision",
		"example-reviewing-plan",
		"Proposed",
		"user-provenance decision-log",
		"explicitly approved design summary",
		"repository facts do not establish consent",
		"removed unauthorized surplus commitments",
		"semantics-preserving refinements",
		"writing `none` for an empty inventory",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
}

func TestRetrospectiveTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"skills": map[string]bool{"reviewing-impl": true, "proposing-adr": true},
		"vars": map[string]any{
			"gateCmd":           "./x gate",
			"invariantTestPath": "./internal/...",
		},
		"layout": map[string]any{
			"docs":        map[string]any{"pitfalls": "docs/pitfalls.md"},
			"workflowRef": "docs/workflow.md",
		},
		"data": map[string]any{},
	}

	out := renderSkillGolden(t, "retrospective", data)

	if !strings.Contains(out, "name: example-retrospective") {
		t.Errorf("expected 'name: example-retrospective' in output:\n%s", out)
	}

	// Load-bearing phrases unique to the retrospective ladder (ADR-0067).
	loadBearing := []string{
		"main thread",
		"promotion ladder",
		"Invariant",
		"example-proposing-adr",
		"docs/pitfalls.md",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
}

func TestRefactorCouplingAuditTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars":   map[string]any{},
		"data":   map[string]any{},
	}

	out := renderSkillGolden(t, "refactor-coupling-audit", data)

	// Assert frontmatter name line
	if !strings.Contains(out, "name: example-refactor-coupling-audit") {
		t.Errorf("expected 'name: example-refactor-coupling-audit' in output:\n%s", out)
	}

	// Assert load-bearing phrases unique to refactor-coupling-audit
	loadBearing := []string{
		"coupling audit",
		"Context section",
		"Sibling test files",
		"constructor",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
}

// invariant: rendering/guide-and-doc-templates:guide-entry-point-routing (TestAgentsDocNativeSkillRouter)
// invariant: rendering/guide-and-doc-templates:maintainable-code-design-guide (TestAgentsDocNativeSkillRouter)
func TestAgentsDocNativeSkillRouter(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"testCmd": "go test ./...",
			"gateCmd": "make gate",
		},
		"layout": testLayout(),
		"data":   map[string]any{},
		"skills": map[string]bool{"brainstorming": true, "adr-lifecycle": true, "tdd": true, "bugfix": true},
	}
	out := renderGolden(t, "agents-doc/AGENTS.md.tmpl", data)
	for _, phrase := range []string{
		"## You and this project",
		"## Identity",
		"## Invariants",
		"## Workflow",
		"## Commands",
		"## Document map",
		"Route settled content by authority lifetime",
		"Preserve the approved design boundary",
		"docs/maintainable-code-design.md",
		"make gate",
		"Use any native skill whose exposed description fits the current work.",
	} {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
	for _, banned := range []string{"Enabled skills:", "example-brainstorming", "purpose", "Trigger:", "Usually follows:", "Common follow-ups:", "fallback", "brainstorming → ADR", "warranted by", "A plan may use exact content/diffs", "V2 ADR", "pollute parent context", "Chain skills"} {
		if strings.Contains(out, banned) {
			t.Errorf("guide retains evicted prose or catalog residue %q:\n%s", banned, out)
		}
	}
	if got := strings.Count(out, "Stage the complete transaction"); got != 1 {
		t.Errorf("guide must carry exactly one concise gate rule, got %d:\n%s", got, out)
	}
}

type effortSignatureFinding struct {
	path     string
	line     int
	offset   int
	contract string
	lineText string
}

type effortSignaturePattern struct {
	contract string
	pattern  *regexp.Regexp
}

func effortSignaturePatterns() []effortSignaturePattern {
	return []effortSignaturePattern{
		{"title-only creation signature", regexp.MustCompile("awf effort " + `new[^<]*<(confirmed title|outcome|outcome-title)>`)},
		{"title-derived creation guidance", regexp.MustCompile("[Ee]ffort (creation )?" + `deriv(e|es|ed|ing)[^\r\n]{0,40}slug`)},
		{"title-derived creation guidance", regexp.MustCompile("[Dd]eriv" + `(e|es|ed|ing) an immutable slug`)},
		{"two-field confirmation", regexp.MustCompile("outcome/title " + `(pair|confirmation)`)},
		{"two-field confirmation", regexp.MustCompile("labeled outcome and " + `(proposed )?(effort )?title`)},
		{"two-field confirmation", regexp.MustCompile("confirms? the " + `pair`)},
		{"two-field confirmation", regexp.MustCompile("both " + `fields`)},
	}
}

func activeEffortSignatureFindings(t *testing.T, root string) []effortSignatureFinding {
	t.Helper()
	patterns := effortSignaturePatterns()
	var findings []effortSignatureFinding
	scan := func(relative string, raw []byte) {
		for _, candidate := range patterns {
			for _, match := range candidate.pattern.FindAllIndex(raw, -1) {
				lineStart := bytes.LastIndexByte(raw[:match[0]], '\n') + 1
				lineEnd := bytes.IndexByte(raw[match[0]:], '\n')
				if lineEnd < 0 {
					lineEnd = len(raw)
				} else {
					lineEnd += match[0]
				}
				findings = append(findings, effortSignatureFinding{
					path: relative, line: bytes.Count(raw[:match[0]], []byte("\n")) + 1,
					offset: match[0], contract: candidate.contract, lineText: string(raw[lineStart:lineEnd]),
				})
			}
		}
	}
	historical := func(relative string) bool {
		return relative == "docs/decisions" || strings.HasPrefix(relative, "docs/decisions/") ||
			relative == "docs/plans" || strings.HasPrefix(relative, "docs/plans/") ||
			relative == "changelog" || strings.HasPrefix(relative, "changelog/")
	}
	scanRoot := func(relativeRoot string) {
		start := filepath.Join(root, filepath.FromSlash(relativeRoot))
		info, err := os.Lstat(start)
		if os.IsNotExist(err) {
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsDir() {
			raw, err := os.ReadFile(start)
			if err != nil {
				t.Fatal(err)
			}
			scan(relativeRoot, raw)
			return
		}
		testsupport.WalkRepoFiles(t, start, func(relative string) bool {
			full := filepath.ToSlash(filepath.Join(relativeRoot, filepath.FromSlash(relative)))
			resident := full == ".awf/efforts" || strings.HasPrefix(full, ".awf/efforts/") || strings.Contains(full, "/.awf/efforts/") ||
				full == ".awf/worktrees" || strings.HasPrefix(full, ".awf/worktrees/") || strings.Contains(full, "/.awf/worktrees/")
			return !historical(full) && !resident
		}, func(relative string, raw []byte) {
			scan(filepath.ToSlash(filepath.Join(relativeRoot, filepath.FromSlash(relative))), raw)
		})
	}
	for _, relativeRoot := range []string{"cmd", "internal", ".awf/parts", ".awf/docs", ".awf/skills", ".awf/topics", "templates", "AGENTS.md", "README.md", "docs", ".pi", ".claude", "examples"} {
		scanRoot(relativeRoot)
	}
	if entries, err := os.ReadDir(filepath.Join(root, "examples")); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			for _, hidden := range []string{".awf", ".pi", ".claude"} {
				scanRoot(filepath.ToSlash(filepath.Join("examples", entry.Name(), hidden)))
			}
		}
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].path != findings[j].path {
			return findings[i].path < findings[j].path
		}
		if findings[i].offset != findings[j].offset {
			return findings[i].offset < findings[j].offset
		}
		return findings[i].contract < findings[j].contract
	})
	return findings
}

func formatEffortSignatureFindings(findings []effortSignatureFinding) string {
	var lines []string
	for _, finding := range findings {
		lines = append(lines, fmt.Sprintf("%s:%d:%d: %s", finding.path, finding.line, finding.offset, finding.contract))
	}
	return strings.Join(lines, "\n")
}

func explicitSlugADRStatus(t *testing.T, root string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(root, "docs", "decisions", "*require-explicit-short-effort-slugs.md"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("explicit-slug ADR matches = %v, err=%v", matches, err)
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`(?m)^status: ([^\r\n]+)$`).FindSubmatch(raw)
	if len(match) != 2 {
		t.Fatalf("explicit-slug ADR has no status: %s", matches[0])
	}
	return string(match[1])
}

func TestActiveEffortCreationSignaturesStaySynchronized(t *testing.T) {
	root := filepath.Join("..", "..")
	findings := activeEffortSignatureFindings(t, root)
	switch status := explicitSlugADRStatus(t, root); status {
	case "Implementing":
		expectedPaths := []string{
			".awf/topics/parts/rendering/workflow-skill-templates/current-state.md",
			"docs/topics/rendering/workflow-skill-templates.md",
		}
		if len(findings) != len(expectedPaths) {
			t.Fatalf("Implementing ADR requires exactly two active findings, got:\n%s", formatEffortSignatureFindings(findings))
		}
		for index, finding := range findings {
			if finding.path != expectedPaths[index] || finding.contract != "two-field confirmation" {
				t.Fatalf("unauthorized intermediate finding:\n%s", formatEffortSignatureFindings(findings))
			}
			if digest := fmt.Sprintf("%x", sha256.Sum256([]byte(finding.lineText))); digest != "5a3317a41dbd23aecdb54fdf4d2fc924a19b88e2f8600510b37d163540c0fa3e" {
				t.Fatalf("intermediate claim passage changed at %s:%d (digest %s)", finding.path, finding.line, digest)
			}
		}
	case "Implemented":
		if len(findings) != 0 {
			t.Fatalf("Implemented ADR requires zero active findings:\n%s", formatEffortSignatureFindings(findings))
		}
	default:
		t.Fatalf("explicit-slug signature test does not permit ADR status %q", status)
	}

	fixture := t.TempDir()
	writeFixture := func(path, body string) {
		t.Helper()
		full := filepath.Join(fixture, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		body     string
		contract string
	}{
		{"awf effort " + "new <outcome-title>", "title-only creation signature"},
		{"Effort creation " + "derives a slug", "title-derived creation guidance"},
		{"Deriving" + " an immutable slug", "title-derived creation guidance"},
		{"outcome/title " + "confirmation", "two-field confirmation"},
		{"labeled outcome and " + "proposed effort title", "two-field confirmation"},
		{"labeled outcome and " + "proposed title receive", "two-field confirmation"},
		{"confirms the " + "pair", "two-field confirmation"},
		{"both " + "fields", "two-field confirmation"},
	}
	var expected []string
	for index, test := range cases {
		path := fmt.Sprintf("cmd/stale-%d.md", index)
		writeFixture(path, test.body)
		expected = append(expected, fmt.Sprintf("%s:1:0: %s", path, test.contract))
	}
	multiplePath := "cmd/stale-multiple.md"
	multiple := cases[5].body + " / " + cases[5].body + "\n" + cases[5].body
	writeFixture(multiplePath, multiple)
	secondOffset := len(cases[5].body) + len(" / ")
	thirdOffset := secondOffset + len(cases[5].body) + 1
	expected = append(expected,
		fmt.Sprintf("%s:1:0: %s", multiplePath, cases[5].contract),
		fmt.Sprintf("%s:1:%d: %s", multiplePath, secondOffset, cases[5].contract),
		fmt.Sprintf("%s:2:%d: %s", multiplePath, thirdOffset, cases[5].contract),
	)
	for _, path := range []string{
		"cmd/active.md", "internal/active.md",
		".awf/parts/active.md", ".awf/docs/active.md", ".awf/skills/active.md", ".awf/topics/active.md",
		"templates/active.md", "AGENTS.md", "README.md", "docs/active.md",
		".pi/active.md", ".claude/active.md", "examples/demo/active.md",
		"examples/demo/.awf/active.md", "examples/demo/.pi/active.md", "examples/demo/.claude/active.md",
	} {
		writeFixture(path, cases[0].body)
		expected = append(expected, path+":1:0: "+cases[0].contract)
	}
	for _, path := range []string{
		"docs/decisions/historical.md", "docs/plans/historical.md", "changelog/historical.md",
		".awf/efforts/ignored.md", ".awf/worktrees/ignored.md",
		"examples/demo/.awf/efforts/ignored.md", "examples/demo/.awf/worktrees/ignored.md",
	} {
		writeFixture(path, cases[0].body)
	}
	sort.Strings(expected)
	if got := formatEffortSignatureFindings(activeEffortSignatureFindings(t, fixture)); got != strings.Join(expected, "\n") {
		t.Fatalf("closed active-path diagnostics =\n%s\nwant\n%s", got, strings.Join(expected, "\n"))
	}
}

// TestWorkingMemorySingleHomeSurfaces asserts the workflow doc remains the
// detailed protocol home while guides and skills carry executable routing.
// invariant: rendering/guide-and-doc-templates:working-memory-single-home (TestWorkingMemorySingleHomeSurfaces)
func TestWorkingMemorySingleHomeSurfaces(t *testing.T) {
	data := map[string]any{"prefix": "example", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{}, "skills": map[string]bool{"effort-workflow": true}}
	workflow := renderGolden(t, "docs/workflow.md.tmpl", data)
	assertOrderedPhrases(t, workflow,
		"## Working memory", "Session context is volatile", "`effort-workflow` alone proposes", "rendered orienting skill's resume-revalidation section is the procedural home", "One effort has one user-managed writer")
	effort := renderSkillGolden(t, "effort-workflow", data)
	assertContainsAll := func(body string, wants ...string) {
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Errorf("missing %q:\n%s", want, body)
			}
		}
	}
	assertContainsAll(effort, "sole owner of the effort lifecycle", "all effort checkpoints", "integration, divergence handling, topology removal, retrospective routing, and finish")
	orienting := renderSkillGolden(t, "orienting", data)
	assertContainsAll(orienting, "## Resume revalidation", "verify every load-bearing claim against repository truth", "A discrepancy resolves in favor of the repository", "a dispatched child never edits it")
	for _, other := range []string{"executing-direct", "reviewing-impl"} {
		body := renderSkillGolden(t, other, data)
		if strings.Contains(body, "awf effort finish <slug>") {
			t.Errorf("%s steals effort lifecycle closure", other)
		}
	}
}

// invariant: rendering/workflow-skill-templates:memory-log-consumer-coverage (TestMemoryLogConsumerCoverage)
func TestMemoryLogConsumerCoverage(t *testing.T) {
	data := map[string]any{
		"prefix": "example", "vars": map[string]any{"gateCmd": "./x gate"},
		"layout": testLayout(), "data": map[string]any{},
		"skills": map[string]bool{},
	}
	for _, agent := range []string{"adr-reviewer", "plan-reviewer", "code-reviewer"} {
		out := renderAgentGolden(t, agent, data)
		for _, want := range []string{
			"## Consensus adherence",
			"user-decision",
			"`location` cites the deviating",
			"`issue` names the deviation",
			"`suggested_fix` carries the escalation phrasing",
			"we decided X; during <phase> we found Z; recommend Y, approve?",
			"explicitly approved design summary",
			"Removing an unaccepted surplus commitment",
			"authority-preserving `reasoned` correction",
			"A brief without either form of consent evidence leaves this check idle",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("%s missing consensus-adherence phrase %q:\n%s", agent, want, out)
			}
		}
	}
	for _, skill := range []string{"reviewing-adr", "reviewing-plan", "reviewing-impl"} {
		out := renderSkillGolden(t, skill, data)
		for _, want := range []string{"verbatim", "including whatever `Record:` blocks exist"} {
			if !strings.Contains(out, want) {
				t.Errorf("%s missing decision-log paste phrase %q:\n%s", skill, want, out)
			}
		}
	}
	adrReview := renderSkillGolden(t, "reviewing-adr", data)
	for _, want := range []string{"For effort-backed work", "For effort-free work", "explicitly approved design summary", "repository facts do not establish consent", "reviewer must not infer it"} {
		if !strings.Contains(adrReview, want) {
			t.Errorf("reviewing-adr missing consent-evidence branch %q:\n%s", want, adrReview)
		}
	}
	effortFreeOmission := map[string]string{
		"reviewing-plan": "otherwise omit effort and memory fields",
		"reviewing-impl": "absence of an effort omits those fields",
	}
	for skill, want := range effortFreeOmission {
		out := renderSkillGolden(t, skill, data)
		if !strings.Contains(out, want) {
			t.Errorf("%s does not preserve effort-free memory omission %q:\n%s", skill, want, out)
		}
	}
	retrospective := renderSkillGolden(t, "retrospective", data)
	for _, want := range []string{"`## Observations`", "`## Decision log`", "as primary input", "across the effort's sessions"} {
		if !strings.Contains(retrospective, want) {
			t.Errorf("retrospective missing memory-log phrase %q:\n%s", want, retrospective)
		}
	}
}

func TestAgentsDocTemplateConfigDriven(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"gateCmd": "",
		},
		"layout": testLayout(),
		"skills": map[string]bool{"brainstorming": true, "adr-lifecycle": true},
		"data": map[string]any{
			"invariants": []map[string]any{
				{"text": "**Custom rule.**", "ref": "ADR-0009"},
			},
		},
		"docs": []map[string]any{
			{"title": "Architecture", "desc": "system shape", "path": "docs/architecture.md"},
		},
	}
	out := renderGolden(t, "agents-doc/AGENTS.md.tmpl", data)
	for _, phrase := range []string{
		"**Custom rule.** (ADR-0009)",
		"[docs/architecture.md](docs/architecture.md)",
	} {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
	if strings.Contains(out, "]()") {
		t.Errorf("empty-string vars must not render empty-target links:\n%s", out)
	}
}

// TestUnsetFallbackRenders pins the graceful-fallback branches the empty-init
// oracle never renders (ADR-0045/ADR-0046): the non-core skills are absent from
// a curated init, and the reviewer agents ship catalog default data there - so
// without these assertions a reverted guard in any of them passes the suite.
// Every template renders with empty vars, empty data, and an empty skills set;
// renderGolden's assertNoLeaks supplies the <no value> net.
// fallbackCase pins one template's hand-authored degraded output: want
// phrases must render under empty data, ban phrases must not; docs (when
// set) replaces the layout docs map - used by RequiresDoc-gated templates
// whose doc path must resolve. TestConditionalTemplatesHaveFallbackCases
// requires an entry per conditional catalog template (ADR-0080).
func TestV3ADRTemplateEmptyDataFallback(t *testing.T) {
	assertV3ADRTemplatePublicationSafe(t)
}

type fallbackCase struct {
	tmpl string
	docs map[string]any
	want []string // fallback prose that must render
	ban  []string // residue that must not render
}

var unsetFallbackCases = []fallbackCase{
	{
		tmpl: "agents/implementer.md.tmpl",
		want: []string{"the project's gate command"},
		ban:  []string{"Shortcuts that are never acceptable here", "``"},
	},
	{
		tmpl: "skills/executing-direct/SKILL.md.tmpl",
		want: []string{"Evaluate review independently", "locally obvious, low-risk, directly verified", "the effort lifecycle owner"},
		ban:  []string{"awf_workflow", "example-effort-workflow"},
	},
	{
		tmpl: "skills/tdd/SKILL.md.tmpl",
		want: []string{
			"Pick the smallest surface that can prove the behaviour",
			"confirm it fails for the right reason.",
			"Run the gate.",
		},
		ban: []string{"``"},
	},
	{
		tmpl: "skills/bugfix/SKILL.md.tmpl",
		want: []string{
			"confirm it with a falsifiable check before touching code",
			"Write the failing test first",
			"The project's gate is the default",
			"the project's docs",
			"Evaluate implementation review independently",
			"the project's review step",
			"the effort lifecycle owner",
		},
		ban: []string{"example-tdd", "example-debugging", "example-reviewing-impl", "``"},
	},
	{
		tmpl: "skills/debugging/SKILL.md.tmpl",
		want: []string{
			"fix it directly with a regression test in that case",
			"Write it test-first.",
			"the project's gate",
			"apply the fix with its regression test",
			"a design discussion before changing behaviour",
		},
		ban: []string{"example-bugfix", "example-tdd", "example-brainstorming", "``"},
	},
	{
		tmpl: "skills/exploring/SKILL.md.tmpl",
		want: []string{"target-native fresh-context exploration subagent"},
		ban:  []string{"subagent_explore"},
	},
	{
		tmpl: "skills/orienting/SKILL.md.tmpl",
		want: []string{"Ground guide-first, in order", "`example-exploring`"},
	},
	{
		tmpl: "skills/refactor-coupling-audit/SKILL.md.tmpl",
		want: []string{"<module-prefix>/", "the project's decision process"},
		ban:  []string{"example-proposing-adr"},
	},
	{
		// Every conditional rung/reference degrades to generic prose when its
		// skill/var/doc is absent - no empty inline code, no dangling reference
		// (ADR-0045/ADR-0020 publication-safety; ADR-0067 rung-4 pitfalls obligation).
		tmpl: "skills/retrospective/SKILL.md.tmpl",
		want: []string{
			"the effort lifecycle owner",
			"the project's pitfalls notes",
			"the project's decision process",
			"Run `awf new pitfall \"<Title>\"`, then edit the reported authored source under `.awf/docs/pitfalls/`",
		},
		ban: []string{"example-reviewing-impl", "example-proposing-adr", "``"},
	},
	// invariant: rendering/workflow-skill-templates:reviewers-report-only (agents/adr-reviewer.md.tmpl)
	{
		tmpl: "agents/adr-reviewer.md.tmpl",
		want: []string{"post-implementation", "counterfactual", "reasoned finding", "unordered membership within each batch"},
		ban:  []string{"For each item below", "Apply mechanical and reasoned fixes directly", "apply the fix directly", "3-round soft cap", "as new commits", "Edit the", "Apply a fix", "Commit the change", "Loop a re-review"},
	},
	{
		tmpl: "agents/plan-reviewer.md.tmpl",
		ban:  []string{"For each item below", "Apply mechanical and reasoned fixes directly", "apply the fix directly", "3-round soft cap", "as new commits", "Edit the", "Apply a fix", "Commit the change", "Loop a re-review"},
	},
	{
		tmpl: "agents/code-reviewer.md.tmpl",
		ban:  []string{"For each item below", "Apply mechanical and reasoned fixes directly", "apply the fix directly", "3-round soft cap", "as new commits", "Edit the", "Apply a fix", "Commit the change", "Loop a re-review"},
	},
	{
		tmpl: "agents-doc/AGENTS.md.tmpl",
		want: []string{"Conventional Commits; one concern per commit."},
		ban:  []string{"Chain skills", "Task skills", "example-brainstorming"},
	},
	{
		tmpl: "skills/adr-lifecycle/SKILL.md.tmpl",
		want: []string{"the multi-state lifecycle", "Run `awf render` to regenerate", "whose members may appear in any order", "settled review later appends only Implemented"},
	},
	{
		tmpl: "skills/brainstorming/SKILL.md.tmpl",
		want: []string{"material choice or clarification", "does not create an effort"},
	},
	{
		tmpl: "skills/grounding/SKILL.md.tmpl",
		want: []string{"broad or uncertain repository premises", "Dispatch the `grounding-checker` agent exactly once"},
	},
	{
		tmpl: "skills/effort-workflow/SKILL.md.tmpl",
		want: []string{"sole owner of the effort lifecycle", "Continue through the target-native successor"},
	},
	{
		tmpl: "skills/executing-plans/SKILL.md.tmpl",
		want: []string{"the project's gate", "Auto-commit the phase only when green"},
	},
	{
		tmpl: "skills/proposing-adr/SKILL.md.tmpl",
		want: []string{"follow the ADR template's section order", "Run `awf render` to regenerate"},
	},
	{
		tmpl: "skills/reviewing-adr/SKILL.md.tmpl",
		want: []string{
			"using the project's commit scope conventions",
			"exactly one fresh `adr-reviewer` verify pass",
		},
	},
	{
		tmpl: "skills/reviewing-impl/SKILL.md.tmpl",
		want: []string{"locally obvious, low-risk, directly verified", "Effort-free review creates no effort"},
	},
	{
		tmpl: "skills/reviewing-plan/SKILL.md.tmpl",
		want: []string{"explicit uncommitted plan path", "selected working-tree snapshot", "mechanical fixes directly without a durable ledger", "one initial plan commit"},
	},
	{
		tmpl: "skills/roadmap-graduation/SKILL.md.tmpl",
		docs: map[string]any{"roadmap": "docs/roadmap.md"},
		want: []string{
			"Write the ADR per the project's decision process.",
			"moving an item out of `docs/roadmap.md`",
		},
		ban: []string{"example-proposing-adr"},
	},
	{
		tmpl: "skills/subagent-driven-development/SKILL.md.tmpl",
		want: []string{"known clean and green baseline", "the project's gate", "wired pre-commit hook enforces both", "Sequential dispatch only, never parallel"},
	},
	{
		tmpl: "skills/writing-plans/SKILL.md.tmpl",
		want: []string{"per the example plan convention", "the project's gate runs before every commit"},
	},
	// Voluntary doc entry (ADR-0089): the ADR-0080 guard covers skills and
	// agents only, so the glossary's conditional is pinned by hand.
	{
		tmpl: "docs/glossary.md.tmpl",
		want: []string{"No terms recorded yet", "`data.terms` in `.awf/docs/glossary.yaml`"},
		ban:  []string{"| Term | Meaning |"},
	},
}

// The unset-data half of the implementer contract's third sentence: its
// fallback case above pins the degraded body.
// invariant: rendering/workflow-skill-templates:implementer-role-contract (TestUnsetFallbackRenders)
func TestUnsetFallbackRenders(t *testing.T) {
	for _, tc := range unsetFallbackCases {
		t.Run(tc.tmpl, func(t *testing.T) {
			layout := testLayout()
			if tc.docs != nil {
				layout["docs"] = tc.docs
			}
			data := map[string]any{
				"prefix": "example",
				"vars":   map[string]any{},
				"data":   map[string]any{},
				"skills": map[string]bool{},
				"layout": layout,
			}
			out := renderGolden(t, tc.tmpl, data)
			for _, phrase := range tc.want {
				if !strings.Contains(out, phrase) {
					t.Errorf("missing fallback phrase %q:\n%s", phrase, out)
				}
			}
			for _, phrase := range tc.ban {
				if strings.Contains(out, phrase) {
					t.Errorf("unset render must not contain %q:\n%s", phrase, out)
				}
			}
		})
	}
}

func TestBrainstormingTemplate(t *testing.T) {
	out := renderSkillGolden(t, "brainstorming", map[string]any{"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "layout": testLayout()})
	for _, want := range []string{"material choice or clarification", "approved decision set", "narrowest durable commitment", "outside the ADR until accepted"} {
		if !strings.Contains(out, want) {
			t.Fatalf("brainstorming contract missing %q", want)
		}
	}
}
func TestReviewingImplTemplate(t *testing.T) {
	out := renderSkillGolden(t, "reviewing-impl", map[string]any{"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "layout": testLayout()})
	if !strings.Contains(out, "Effort-free review creates no effort") {
		t.Fatal("review contract missing")
	}
}
func TestEffortWorkflowTemplate(t *testing.T) {
	out := renderSkillGolden(t, "effort-workflow", map[string]any{"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}})
	if !strings.Contains(out, "sole owner of the effort lifecycle") {
		t.Fatal("effort contract missing")
	}
}
func TestGroundingTemplate(t *testing.T) {
	out := renderSkillGolden(t, "grounding", map[string]any{"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}})
	if !strings.Contains(out, "broad or uncertain repository premises") {
		t.Fatal("grounding contract missing")
	}
}
func TestEffortWorkflowSkillContract(t *testing.T) { TestEffortWorkflowTemplate(t) }

func TestDocArchitectureTemplate(t *testing.T) {
	out := renderGolden(t, "docs/architecture.md.tmpl", map[string]any{
		"prefix": "example",
		"vars":   map[string]any{},
		"data":   map[string]any{},
	})
	if !strings.Contains(out, "# Architecture") {
		t.Errorf("expected '# Architecture' heading:\n%s", out)
	}
}

func TestTelemetryDocumentationTemplatesPublicationSafe(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars":   map[string]any{},
		"data":   map[string]any{},
		"skills": map[string]bool{},
		"layout": testLayout(),
	}
	cases := []struct {
		tmpl string
		want string
		ban  string
	}{
		{"docs/architecture.md.tmpl", "resident-data boundaries", ""},
		{"docs/workflow.md.tmpl", "# Workflow", "generated Pi telemetry tools"},
		{"docs/testing.md.tmpl", "minimum-runtime smoke tests", ""},
		{"docs/releasing.md.tmpl", "generated runtime smoke", ""},
	}
	for _, tc := range cases {
		t.Run(tc.tmpl, func(t *testing.T) {
			out := renderGolden(t, tc.tmpl, data)
			if !strings.Contains(out, tc.want) {
				t.Errorf("missing publication contract %q:\n%s", tc.want, out)
			}
			if strings.Contains(out, "<no value>") || tc.ban != "" && strings.Contains(out, tc.ban) {
				t.Errorf("empty-data render is not coherent:\n%s", out)
			}
		})
	}
}

// invariant: rendering/workflow-skill-templates:effort-workflow (TestEffortWorkflowSkillContract)
func TestRoadmapGraduationTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars":   map[string]any{},
		"data":   map[string]any{},
	}

	out := renderSkillGolden(t, "roadmap-graduation", data)

	// Assert frontmatter name line
	if !strings.Contains(out, "name: example-roadmap-graduation") {
		t.Errorf("expected 'name: example-roadmap-graduation' in output:\n%s", out)
	}

	// Assert load-bearing phrases unique to roadmap-graduation
	loadBearing := []string{
		"same commit",
		"roadmap",
		"benchmark",
		"docs(roadmap): drop",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
}

// invariant: rendering/guide-and-doc-templates:guide-entry-point-routing (TestGuideOmitsLocalAndStandardSkillMetadata)
func TestGuideOmitsLocalAndStandardSkillMetadata(t *testing.T) {
	const localDescription = "Route ultraviolet nebula work through its native procedure."
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\n", map[string]string{
		"skills/nebula-router.yaml": "data:\n  description: " + localDescription + "\n",
	})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}

	banned := []string{
		"Enabled skills:", "Trigger:", "Usually follows:", "Common follow-ups:", "fallback",
		"example-brainstorming", "example-bugfix", "example-nebula-router", localDescription,
		"(chain):", "(task):", "(support):",
	}
	for _, name := range []string{"brainstorming", "bugfix"} {
		profile := catalog.Standard.Skills[name].Profile
		banned = append(banned, profile.Purpose, profile.Trigger)
		for _, neighbor := range append(slices.Clone(profile.UsuallyFollows), profile.CommonFollowUps...) {
			banned = append(banned, "example-"+neighbor)
		}
	}
	for _, residue := range banned {
		if strings.Contains(string(body), residue) {
			t.Errorf("guide retains skill catalog residue %q:\n%s", residue, body)
		}
	}
}

// TestGlossaryTemplate is the glossary doc's golden (nonArtifactGoldens-listed:
// docs sit outside the ADR-0080 skills/agents completeness walk). The terms
// value arrives pre-transformed - renderGolden bypasses the project-layer
// transform, whose behavior glossary_test.go owns.
func TestGlossaryTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars":   map[string]any{},
		"data":   map[string]any{"terms": "| alpha | first |\n| beta | second |\n"},
		"skills": map[string]bool{},
		"layout": testLayout(),
	}
	out := renderGolden(t, "docs/glossary.md.tmpl", data)
	for _, want := range []string{"# Glossary", "| Term | Meaning |", "| alpha | first |", "| beta | second |"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "No terms recorded yet") {
		t.Errorf("placeholder must not render alongside a populated table:\n%s", out)
	}
}

// invariant: rendering/workflow-skill-templates:authority-guided-review-remediation (TestAuthorityGuidedReviewRemediation)
// TestAuthorityGuidedReviewRemediation pins the authority-guided review
// remediation boundary: the shared review spine stays the single semantic home
// of finding classification, one variable-free partial carries the
// dispatcher-side routing obligation into all reviewing skills, routes plan
// contradictions back through ADR review, and removes automatic residual escalation.
func TestAuthorityGuidedReviewRemediation(t *testing.T) {
	const (
		stopCriterion   = "every viable correct remediation would contradict or change a settled user-approved design or decision, or would require an unauthorized change to an active current-state claim"
		nonTriggers     = "competing clean options, severity, structural character, and the fact that a finding survived a prior correction"
		residualOpening = "Diagnose every residual finding under the authority-guided remediation boundary above"
	)

	partial, err := fs.ReadFile(templates.FS, "partials/review-remediation-autonomy.md")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(partial), "{{") {
		t.Errorf("shared review-remediation partial must remain variable-free:\n%s", partial)
	}
	for _, line := range strings.Split(string(partial), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			t.Errorf("shared review-remediation partial must not interrupt consumer structure with heading %q", line)
		}
	}
	// Exactly the three tokens ExpandIncludes rejects inside a partial.
	for _, token := range []string{"awf:section", "awf:end", "awf:include"} {
		if strings.Contains(string(partial), token) {
			t.Errorf("shared review-remediation partial carries the rejected token %q", token)
		}
	}

	retired := []string{
		"Escalate any residual structural findings as `user-decision` items",
		"the step-2 return edge applies to initial-dispatch findings only",
		// Pinned from the second character: lowercase in the plan and ADR
		// skills, capitalized in resync.
		"o not loop further without explicit user direction",
		"a genuine design fork or unresolved ambiguity that should not be decided unilaterally",
		"present a genuine unresolved `user-decision` fork or consensus deviation and stop",
		"(return edge, step 2)",
	}

	dispatcherWants := []string{
		"Apply mechanical corrections directly and reasoned corrections with a concise rationale, autonomously",
		"single semantic home",
		"routes it rather than redefining it",
		"route it through the existing grounded-design or ADR workflow",
		"pauses only at that workflow's mandatory approval boundary",
		"Exactly one fresh verify-pass dispatch is retained",
		"without dispatching another same-artifact review loop",
		"A consensus deviation remains a user decision.",
		"A review finding stops the workflow only when",
		// The reconciling clause that keeps this partial from reading as a
		// rival to an implementation stop list. Pinned with its full referent:
		// a positional "adjacent list" wording dangles in the six skill
		// outputs that never render implementation-autonomy beside it.
		"is not the unresolved design fork that an implementation stop list names",
		stopCriterion,
		nonTriggers,
		"A plan finding whose correction would contradict linked ADR authority returns to ADR amendment and independent review",
		// The stop criterion literal ends before the citation directive, so
		// pin the directive with enough of its left context to bind it to the
		// stop sentence rather than to any other authority mention.
		"active current-state claim; cite the affected authority",
	}

	// The pinned non-trigger enumeration deliberately starts at "competing
	// clean options" because the leading word's case differs between the two
	// prose homes, so assert the dropped word separately rather than leaving
	// the claim's ambiguity clause proven by nothing.
	assertNamesAmbiguity := func(t *testing.T, label, out string) {
		t.Helper()
		if !strings.Contains(strings.ToLower(out), "ambiguity") {
			t.Errorf("%s never names ambiguity as a non-trigger", label)
		}
	}

	// Every Latitude: exact routing replacement needs a positive pin. Absence
	// of the retired predecessor alone lets the replacement be degraded back
	// toward an unconditioned escalation without turning the suite red.
	routingWants := map[string]string{
		"reviewing-plan": "present to the user with the cited affected authority and wait",
		"reviewing-adr":  "present to the user with the cited affected authority and wait",
		"reviewing-impl": "present a `user-decision` finding with the cited affected authority, or a consensus deviation, and stop",
	}

	skillVariants := map[string]map[string]any{
		"configured": {
			"prefix": "example", "vars": map[string]any{"gateCmd": "./x gate"}, "layout": testLayout(),
			"commitScopes":        "`docs(plans)`",
			"skills":              map[string]bool{"effort-workflow": true, "adr-lifecycle": true, "reviewing-impl": true},
			"targetSubagentTools": true,
		},
		"empty": {
			"prefix": "example", "vars": map[string]any{}, "layout": testLayout(),
			"data": map[string]any{}, "skills": map[string]bool{},
		},
	}

	for _, skill := range []string{"reviewing-plan", "reviewing-adr", "reviewing-impl"} {
		raw, err := fs.ReadFile(templates.FS, "skills/"+skill+"/SKILL.md.tmpl")
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Count(string(raw), "<!-- awf:include review-remediation-autonomy -->"); got != 1 {
			t.Errorf("skills/%s has %d review-remediation includes, want 1", skill, got)
		}
		for _, reject := range retired {
			if strings.Contains(string(raw), reject) {
				t.Errorf("skills/%s source retains retired escalation phrase %q", skill, reject)
			}
		}
		for variant, data := range skillVariants {
			out := renderSkillGolden(t, skill, data)
			assertNoLeaks(t, out)
			for _, want := range dispatcherWants {
				if !strings.Contains(out, want) {
					t.Errorf("%s/%s missing dispatcher clause %q", variant, skill, want)
				}
			}
			for _, reject := range retired {
				if strings.Contains(out, reject) {
					t.Errorf("%s/%s retains retired escalation phrase %q", variant, skill, reject)
				}
			}
			assertNamesAmbiguity(t, variant+"/"+skill, out)
			// Every review skill carries the same residual-finding opening.
			if !strings.Contains(out, residualOpening) {
				t.Errorf("%s/%s missing residual re-review replacement %q", variant, skill, residualOpening)
			}
			if want := routingWants[skill]; !strings.Contains(out, want) {
				t.Errorf("%s/%s missing routing replacement %q", variant, skill, want)
			}
		}
	}

	reviewerData := map[string]map[string]any{
		"adr-reviewer": {
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
		},
		"plan-reviewer": {
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
		},
		"code-reviewer": {
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
		},
	}
	reviewerWants := []string{
		stopCriterion,
		nonTriggers,
		"cite the affected authority and name the deviation it would require",
		"is not an unauthorized deviation merely because its proposed future state differs from current state",
		"would make a new load-bearing choice material outside approved durable boundaries",
		"changing accepted semantics is a `user-decision` under the Consensus adherence rule below, while removing authority-free surplus is not",
		"Removing an unaccepted surplus commitment restores the accepted decision set and is an authority-preserving `reasoned` correction",
		// Pin the surviving classification bullets by their own text. The
		// bare tokens `mechanical` and `reasoned` also occur in the finding
		// schema, so asserting those alone lets a whole bullet be deleted
		// while the suite stays green.
		"- **mechanical**: the answer is unambiguous from existing rules, docs, or code",
		"- **reasoned**: a good answer can be reached by reading the relevant code or docs",
		"user-decision",
		"suggested_fix",
	}
	for _, name := range []string{"adr-reviewer", "plan-reviewer", "code-reviewer"} {
		outs := map[string]string{
			"populated": renderAgentGolden(t, name, reviewerData[name]),
			"empty": renderAgentGolden(t, name, map[string]any{
				"prefix": "example", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{},
			}),
		}
		for variant, out := range outs {
			assertNoLeaks(t, out)
			// The claim predicates the ambiguity non-trigger on the shared
			// spine, whose leading "Ambiguity," reaches only these agent
			// bodies. A bare token is not enough here: code-reviewer names
			// ambiguity elsewhere, so pin the spine sentence's own opening.
			if !strings.Contains(out, "Ambiguity, competing clean options") {
				t.Errorf("%s/%s spine paragraph no longer opens the non-trigger list with ambiguity", variant, name)
			}
			for _, want := range reviewerWants {
				if !strings.Contains(out, want) {
					t.Errorf("%s/%s missing spine clause %q", variant, name, want)
				}
			}
			for _, reject := range retired {
				if strings.Contains(out, reject) {
					t.Errorf("%s/%s retains retired escalation phrase %q", variant, name, reject)
				}
			}
		}
	}
}
