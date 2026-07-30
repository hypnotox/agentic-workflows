package project

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/render"
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
	asm, parts := render.Assemble(render.ParseSections(expanded), nil, render.HTMLComment)
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
			"docCurrencyItems": []map[string]any{
				{"check": "each State changes claim is authored to match in the same Implemented commit"},
				{"check": "each operation's destination topic metadata exists before the ADR is Accepted"},
				{"check": "docs/workflow.md - update when ADR changes a workflow rule"},
				{"check": "AGENTS.md - update when ADR changes chain, principles, or invariants"},
				{"check": "Frontmatter completeness: format, status, date"},
				{"check": "docs/decisions/INDEX.md - regenerate when status lands as Accepted or Implemented"},
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
		"structural-design",
		"docs/maintainable-code-design.md",
		"semantic model, representation, module/package boundary, dependency direction, ownership boundary, or comparable structural contract",
		"only when a Decision changes",
		"cohesion, representation isolation, dependency direction, enabling-refactor disposition, testable seams, and justification for indirection",
		"skip this lens rather than manufacturing structural requirements",
	} {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected structural-design phrase %q in output:\n%s", phrase, out)
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
		"one declared closing commit and passes the gate on its own",
		"maintainable-design",
		"docs/maintainable-code-design.md",
		"model, ownership, representations, translation boundaries, dependency direction, and test seams",
		"ordered before dependent behavior",
		"bounded to the failure they prevent",
		"deterministically verifiable",
		"approved, deferred, or declined disposition",
		"needless indirection and pattern mandates",
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

// invariant: rendering/workflow-skill-templates:implementer-role-contract
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
		"adding an abstraction with no current call site",
		"Its skill catalog and workflow-chain routing do not bind you",
		"no workflow skill, create or resume no effort, and write to no working-memory file",
		"Iterating on failures is the work",
		"Never make a check pass by weakening what it proves",
		"There is nobody to wait for",
		"That report is the escalation",
		"`awf check --staged`",
		"`make gate`",
		// Each enumerated stopped field and owner step gets its own want, since
		// section parity catches only whole-section loss, not intra-section drift.
		"Stage the complete transaction explicitly, by path",
		"Create exactly one commit",
		"the exact output of `git status --short`",
		"what you completed",
		"what remains",
		"the failing check, named, with its actual output",
		"what you already tried, so the next attempt does not repeat it",
		"There is no third outcome",
		"The invariants, conventions, and commands in the repository's agent guide bind you",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected contract phrase %q in output:\n%s", want, out)
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
	if !strings.Contains(sdd, "You, the dispatching parent, stop before dispatch") {
		t.Errorf("the raise-concerns imperative lost its explicit subject:\n%s", sdd)
	}
	inline := renderSkillGolden(t, "executing-plans", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "data": map[string]any{},
		"skills": map[string]bool{}, "layout": testLayout(),
	})
	if !strings.Contains(inline, "You, the parent executing this plan, raise missing phase context") {
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

// invariant: rendering/workflow-skill-templates:explorer-and-grounding-role-contracts
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

// invariant: rendering/workflow-skill-templates:explorer-and-grounding-role-contracts
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
		"For managed context calls, start bare",
		"do the named types, functions, and packages exist",
		"Surface unstated assumptions",
		"Assess whether the effort needs a decision record",
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

// invariant: rendering/workflow-skill-templates:maintainable-code-review-lenses
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
			"approved, deferred, or declined disposition", "needless indirection", "pattern mandates",
		},
		"code": {
			"cohesion", "coupling", "dependency direction", "representation leakage", "duplicated policy", "testability",
			"needless indirection", "conformance to the settled design", "behavior bolted onto an unsuitable abstraction",
			"refactoring scope silently broadened",
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

// invariant: rendering/workflow-skill-templates:maintainable-code-stage-coverage
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
			"docs/maintainable-code-design.md", "semantic model and ownership", "representation boundaries", "dependency direction", "test seams", "preparatory-refactor decision", "before approving an approach",
		}},
		"proposing-adr": {wants: []string{
			"docs/maintainable-code-design.md", "settled model and ownership, boundaries, dependency direction, constraints, and enabling work", "here and in Decision", "do not replace them with a pattern name",
		}},
		"refactor-coupling-audit": {wants: []string{
			"docs/maintainable-code-design.md", "duplication, coupling, representation leakage, or a workaround", "bounded enabling-refactor or larger-work result", "perform it first, include it in the current effort, defer it in a durable project-owned record, or decline it with the trade-off stated", "ADR scope", "Context section before", "Decision is drafted", "Scope shrink rule", "defer X",
		}},
		"writing-plans": {wants: []string{
			"docs/maintainable-code-design.md", "settled model and ownership, boundaries, dependency direction, representation translations, refactor decision, prohibited shortcuts, and validation", "ordered executable tasks", "self-contained", "no prior conversation context",
		}},
		"tdd": {wants: []string{
			"docs/maintainable-code-design.md", "bounded enabling refactor", "duplication, coupling, representation leakage, or a workaround", "perform it first, include it in the current effort, defer it in a durable project-owned record, or decline it with the trade-off stated", "smallest behavior-proving, model-supporting seam", "force representation leakage or needless indirection", "confirm it fails for the right reason", "minimal change to pass",
		}},
		"executing-plans": {wants: []string{
			"docs/maintainable-code-design.md", "preserve the plan's settled structural choices", "bounded enabling refactor", "reassess if grounded source contradicts them", "stop rather than bolt correctness onto the wrong abstraction", "No drift from the plan",
		}},
		"executing-direct": {wants: []string{
			"docs/maintainable-code-design.md", "assess bounded enabling refactoring before editing", "preserve settled boundaries", "new load-bearing or materially larger choice", "return to brainstorming", "rather than silently expanding scope or accepting a workaround", "Invoke only after brainstorming has settled the design",
		}},
		"subagent-driven-development": {wants: []string{
			"docs/maintainable-code-design.md", "preserve the plan's settled structural choices", "bounded enabling refactor", "reassess them if grounded source contradicts them", "stop and escalate rather than accept a bolt-on workaround", "Sequential dispatch only, never parallel", "complete phase", "allowCommits: true",
		}},
		"bugfix": {wants: []string{
			"docs/maintainable-code-design.md", "unsuitable model or boundary", "bounded enabling work that prevents a workaround", "perform it first, include it in the current effort, defer it in a durable project-owned record, or decline it with the trade-off stated", "root-cause fix, not the symptom", "one concern per commit",
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

// invariant: rendering/workflow-skill-templates:maintainable-code-subagent-contract
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
		for _, segment := range render.ParseSections(expanded) {
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
			wants: []string{"allowCommits: true", "complete phase", "Stage the complete transaction"},
		},
		{
			name: "generic", data: data,
			dispatch: "known clean and green baseline", review: "Review is report-only and phase-level", reportOnly: "parent-owned",
			wants: []string{"complete phase", "Stage the complete transaction"},
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

	inline := renderSection(t, "skills/executing-plans/SKILL.md.tmpl", "procedure-per-task", data)
	const inlineContext = "Iterate phases, not tasks"
	if !strings.Contains(inline, inlineContext) {
		t.Errorf("inline current-task context is not closed to the six categories:\n%s", inline)
	}
}

// invariant: rendering/workflow-skill-templates:implementer-context-grounding
func TestManagedContextCallersChooseProjection(t *testing.T) {
	policies := map[string]string{
		"adr-lifecycle":               "--show pending",
		"brainstorming":               "",
		"bugfix":                      "",
		"debugging":                   "",
		"executing-plans":             "",
		"orienting":                   "",
		"refactor-coupling-audit":     "",
		"reviewing-impl":              "--show invariants --show all-rules --show evidence --show pending",
		"reviewing-plan":              "--show invariants --show all-rules --show evidence --show pending",
		"reviewing-plan-resync":       "--show invariants --show all-rules --show pending",
		"subagent-driven-development": "",
		"tdd":                         "",
		"writing-plans":               "",
	}
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
			if policy == "" {
				if strings.Contains(line, "--show") {
					t.Errorf("%s:%d orientation invocation must use bare context: %s", templateID, lineNumber+1, line)
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
		"Execution mode",
		"one independently green coherent implementation transaction",
		"ordered steps",
		"exact file paths",
		"example-reviewing-plan",
		"batch task",
		"path-disjoint",
		"dead-code escape",
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
			assertOrderedPhrases(t, out, "Stage the complete transaction", "`awf check --staged`", "`./x gate`", "Commit only after both commands pass", "defense in depth")
		})
	}

	agents := renderGolden(t, "agents-doc/AGENTS.md.tmpl", configured)
	assertOrderedPhrases(t, agents, "Stage the complete transaction", "`awf check --staged`", "`./x gate`", "Commit only after both commands pass", "defense in depth")

	fallback := map[string]any{"prefix": "example", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{}}
	for _, name := range []string{"adr-lifecycle", "executing-plans", "subagent-driven-development"} {
		t.Run(name+"-fallback", func(t *testing.T) {
			out := renderSkillGolden(t, name, fallback)
			assertOrderedPhrases(t, out, "Stage the complete transaction", "`awf check --staged`", "the project's gate", "Commit only after both commands pass", "defense in depth")
		})
	}
	fallbackAgents := renderGolden(t, "agents-doc/AGENTS.md.tmpl", fallback)
	assertOrderedPhrases(t, fallbackAgents, "Stage the complete transaction", "`awf check --staged`", "the project's gate", "Commit only after both commands pass", "defense in depth")
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
		"allowCommits: true",
		"known clean and green baseline",
		"report-only phase review",
		"example-reviewing-impl",
		"example-executing-plans",
		"dirty-state inventory",
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

func TestOrientingSkillContract(t *testing.T) {
	if !catalog.Standard.Skills["orienting"].Core {
		t.Fatal("orienting is not a core skill")
	}
	// The three consumer skills are enabled so the same render proves both the
	// single home and the references that replaced their inline copies.
	config := func(target string) string {
		return "prefix: example\nskills: [brainstorming, exploring, orienting, proposing-adr, writing-plans]\nagents: [explorer, grounding-checker]\ntargets: [" + target + "]\n"
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
				"its decision log including every `Record:` block", "not yours to re-decide",
				"cited plan and file existence", "A discrepancy resolves in favor of the repository",
				"never creates an effort, never commits", "never prescribe `--full`",
				"single-pass and advisory, never a chain gate",
			} {
				if !strings.Contains(body, want) {
					t.Errorf("%s orienting skill missing %q", target, want)
				}
			}
			agent := files[adapter.AgentPath("grounding-checker")]
			for _, want := range []string{"Ground guide-first, in order", "For managed context calls, start bare", "never prescribe `--full`", "AWF_CONTEXT_SPILL_V1"} {
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

	// Assert load-bearing phrases unique to proposing-adr
	loadBearing := []string{
		"one decision per ADR",
		"Context",
		"Consequences",
		"status: Proposed",
		"example-reviewing-adr",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
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
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
}

func TestBrainstormingTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars":   map[string]any{},
		"layout": map[string]any{"adrReadme": "docs/decisions/README.md"},
		"data": map[string]any{
			"errorBoundaries": []map[string]any{
				{"name": "HTTP input"},
				{"name": "session credentials"},
				{"name": "subprocess arguments"},
				{"name": "database"},
			},
			"loadBearingExamples": []map[string]any{
				{"item": "package boundary change"},
				{"item": "auth model change"},
				{"item": "non-trivial new dependency"},
				{"item": "workflow rule change"},
			},
		},
	}

	out := renderSkillGolden(t, "brainstorming", data)

	// Assert frontmatter name line
	if !strings.Contains(out, "name: example-brainstorming") {
		t.Errorf("expected 'name: example-brainstorming' in output:\n%s", out)
	}

	// Assert load-bearing phrases unique to brainstorming
	loadBearing := []string{
		"grounding-check subagent",
		"2-3 approaches",
		"Load-bearing",
		"Anti-patterns",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
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
		"example-reviewing-plan-resync",
		"scope-completeness",
		"per-phase ownership",
		"helper partitions",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
}

func TestReviewingPlanResyncTemplate(t *testing.T) {
	data := map[string]any{
		"prefix":       "example",
		"vars":         map[string]any{},
		"commitScopes": "`docs(plans)`",
		"layout":       map[string]any{"plansDir": "docs/plans"},
		"data":         map[string]any{},
	}

	out := renderSkillGolden(t, "reviewing-plan-resync", data)

	// Assert frontmatter name line
	if !strings.Contains(out, "name: example-reviewing-plan-resync") {
		t.Errorf("expected 'name: example-reviewing-plan-resync' in output:\n%s", out)
	}

	// Assert thin-dispatcher load-bearing phrases
	loadBearing := []string{
		"plan-reviewer",
		"scope-completeness",
		"doc-currency",
		"per-phase ownership",
		"helper partitions",
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
		"example-reviewing-plan-resync",
		"Proposed",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
}

func TestReviewingImplTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"gateCmd": "./x gate",
		},
		"commitScopes": "`feat`",
		"layout":       map[string]any{"adrDir": "docs/decisions", "plansDir": "docs/plans"},
		"data":         map[string]any{},
	}

	out := renderSkillGolden(t, "reviewing-impl", data)

	// Assert frontmatter name line
	if !strings.Contains(out, "name: example-reviewing-impl") {
		t.Errorf("expected 'name: example-reviewing-impl' in output:\n%s", out)
	}

	// Assert thin-dispatcher load-bearing phrases
	loadBearing := []string{
		"code-reviewer",
		"user-decision",
		"SHA range",
		"docs/decisions/",
		"example-retrospective",
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

// invariant: rendering/guide-and-doc-templates:guide-entry-point-routing
func TestAgentsDocGuide(t *testing.T) {
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

		"make gate",
	} {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
	for _, banned := range []string{
		"brainstorming → ADR",
		"warranted by",
		"A plan may use exact content/diffs",
		"V2 ADR",
		"pollute parent context",
		"Chain skills",
	} {
		if strings.Contains(out, banned) {
			t.Errorf("evicted workflow prose %q must not render in the guide:\n%s", banned, out)
		}
	}
	// Exactly the invariants-section copy: the workflow section must not
	// regrow the duplicated gate sentence (ADR-0157 evicted-prose class).
	if got := strings.Count(out, "Stage the complete transaction"); got != 1 {
		t.Errorf("guide must carry exactly one gate sentence (invariants section), got %d:\n%s", got, out)
	}
}

// TestWorkingMemorySingleHomeSurfaces asserts the workflow doc remains the
// detailed protocol home while guides and skills carry executable routing.
// invariant: rendering/guide-and-doc-templates:working-memory-single-home
func TestWorkingMemorySingleHomeSurfaces(t *testing.T) {
	data := map[string]any{
		"prefix": "example", "vars": map[string]any{"gateCmd": "./x gate"},
		"layout": testLayout(), "data": map[string]any{},
		"skills":               map[string]bool{"brainstorming": true, "reviewing-impl": true, "retrospective": true},
		"targetSessionHandoff": true,
	}
	workflow := renderGolden(t, "docs/workflow.md.tmpl", data)
	guide := renderGolden(t, "agents-doc/AGENTS.md.tmpl", data)
	routine := renderSkillGolden(t, "executing-plans", data)
	approval := renderSkillGolden(t, "brainstorming", data)
	for label, body := range map[string]string{"workflow": workflow, "guide": guide, "routine": routine, "approval": approval} {
		if !strings.Contains(body, ".awf/efforts/<slug>/memory.md") {
			t.Errorf("%s missing unified owned-memory path", label)
		}
		if strings.Contains(body, ".awf/memory/") {
			t.Errorf("%s retains standalone memory path", label)
		}
	}
	for _, detailed := range []string{"`Phase:`", "`Next:`", "`Updated:`", "`## Brief`", "`## Decision log`", "`## Observations`", "`## Handoff log`", "awf effort finish <slug>"} {
		if !strings.Contains(workflow, detailed) {
			t.Errorf("workflow protocol missing %q", detailed)
		}
	}
	if strings.Contains(guide, "The memory skeleton contains") || strings.Contains(routine, "The memory skeleton contains") {
		t.Error("guide or routine skill duplicated the workflow document's detailed skeleton")
	}
	// The workflow doc keeps the memory contract but routes resume verification
	// to the orienting skill, which owns the procedure it routes to.
	if !strings.Contains(workflow, "the rendered orienting skill's resume-revalidation section is the procedural home of that check") {
		t.Error("workflow doc does not route resume revalidation to the orienting skill")
	}
	orienting := renderSkillGolden(t, "orienting", data)
	for _, want := range []string{
		"landed since the checkpoint", "git worktree list", "against the decision index",
		"its decision log including every `Record:` block", "not yours to re-decide",
		"A discrepancy resolves in favor of the repository",
	} {
		if !strings.Contains(orienting, want) {
			t.Errorf("orienting resume-revalidation missing %q", want)
		}
	}
}

// invariant: rendering/workflow-skill-templates:memory-log-consumer-coverage
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
			"A brief without consensus entries leaves this check idle.",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("%s missing consensus-adherence phrase %q:\n%s", agent, want, out)
			}
		}
	}
	for _, skill := range []string{"reviewing-adr", "reviewing-plan", "reviewing-impl"} {
		out := renderSkillGolden(t, skill, data)
		for _, want := range []string{"pasted verbatim", "`Record:` blocks included"} {
			if !strings.Contains(out, want) {
				t.Errorf("%s missing decision-log paste phrase %q:\n%s", skill, want, out)
			}
		}
	}
	if out := renderSkillGolden(t, "reviewing-plan-resync", data); strings.Contains(out, "pasted verbatim") {
		t.Errorf("reviewing-plan-resync must keep its narrowed contract:\n%s", out)
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
func TestV2ADRTemplateEmptyDataFallback(t *testing.T) {
	assertV2ADRTemplatePublicationSafe(t)
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
		want: []string{"Invoke `example-reviewing-impl` as the terminal step."},
		ban:  []string{"awf_workflow"},
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
			"The project's gate (fast tier) is the default",
			"the project's docs",
			"Run the project's review step as the terminal step.",
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
			"the project's review step",
			"the project's pitfalls notes",
			"the project's decision process",
			"Record it in the project's pitfalls notes.",
		},
		ban: []string{"example-reviewing-impl", "example-proposing-adr", "``"},
	},
	// invariant: rendering/templates:local-base-publication-safe
	{
		tmpl: "skills/_base/SKILL.md.tmpl",
		want: []string{
			"example-local-skill",
			"A project-local example skill.",
			"Describe when to use this skill",
		},
		ban: []string{"<no value>", "``"},
	},
	{
		tmpl: "agents/_base.md.tmpl",
		want: []string{
			"# local-agent",
			"Describe this agent's role",
		},
		ban: []string{"<no value>"},
	},
	// invariant: rendering/templates:local-doc-base-publication-safe
	{
		tmpl: "docs/_base.md.tmpl",
		want: []string{"Project documentation", "Project-local documentation.", "Replace this with the document body"},
		ban:  []string{"<no value>"},
	},
	// invariant: rendering/workflow-skill-templates:reviewers-report-only
	{
		tmpl: "agents/adr-reviewer.md.tmpl",
		want: []string{"Regen command: `awf render`."},
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
		want: []string{"the multi-state lifecycle", "Run `awf render` to regenerate"},
	},
	{
		tmpl: "skills/brainstorming/SKILL.md.tmpl",
		want: []string{
			"hard prerequisite for any non-trivial change",
			"The design lands in the ADR (if load-bearing) or the plan (if not)",
		},
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
		want: []string{
			"(or this project's runner alias for it)",
			"using the project's commit scope conventions",
		},
	},
	{
		tmpl: "skills/reviewing-plan/SKILL.md.tmpl",
		want: []string{"Only the plan file is edited", "using the project's commit scope conventions"},
	},
	{
		tmpl: "skills/reviewing-plan-resync/SKILL.md.tmpl",
		want: []string{"an amendment-until-terminal edit", "using the project's commit scope conventions"},
		ban:  []string{"example-adr-lifecycle"},
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
		want: []string{"known clean and green baseline", "the project's gate", "defense in depth", "Sequential dispatch only, never parallel"},
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
// invariant: rendering/workflow-skill-templates:implementer-role-contract
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

// The AGENTS.md task-skill trigger table derives from the catalog's enabled
// non-Chain skills - every catalog task skill's trigger row appears iff enabled
// (a hand enumeration could never mention a newer one like
// refactor-coupling-audit), and disabled ones stay absent (ADR-0046 follow-up
// sweep; table shape per ADR-0157).
// invariant: rendering/guide-and-doc-templates:guide-entry-point-routing
func TestAgentsDocTaskSkillsGating(t *testing.T) {
	// brainstorming carries a local sidecar: the guide's chain sentence needs a
	// chain skill in the effective set, but a non-local one would demand its
	// ADR-0081 closure (including adr-lifecycle, banned below) at open.
	root := scaffoldFiles(t, "prefix: example\nskills:\n  - brainstorming\n  - bugfix\n  - exploring\n  - refactor-coupling-audit\nagents: [explorer]\n",
		map[string]string{"skills/brainstorming.yaml": "local: true\n"})
	localSkill := filepath.Join(root, ".claude", "skills", "example-brainstorming", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(localSkill), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(localSkill, []byte("---\nname: example-brainstorming\ndescription: local chain skill\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	guide, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	out := string(guide)
	for _, row := range []string{"example-bugfix", "example-exploring", "example-refactor-coupling-audit"} {
		if !strings.Contains(out, row) {
			t.Errorf("expected catalog-derived trigger row %q:\n%s", row, out)
		}
	}
	for _, banned := range []string{"example-tdd", "example-debugging", "example-adr-lifecycle", "example-roadmap-graduation"} {
		if strings.Contains(out, banned) {
			t.Errorf("disabled task skill %q must not render:\n%s", banned, out)
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
