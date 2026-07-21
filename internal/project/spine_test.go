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
		"closing commit passes the project's gate on its own",
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
	}
	for _, phrase := range implPhrases {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected impl-lens phrase %q in output:\n%s", phrase, out)
		}
	}
}

func renderSkillGolden(t *testing.T, skill string, data map[string]any) string {
	t.Helper()
	return renderGolden(t, "skills/"+skill+"/SKILL.md.tmpl", data)
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
		"one reviewable, logically-coherent change",
		"exact file paths",
		"No placeholders",
		"whose first production use lands in a later phase",
		"cannot be sliced",
		"example-reviewing-plan",
		"batch task",
		"affected-site set",
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
			"activeMdRegenCmd": "awf sync",
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
		"one commit per task",
		"final Applied batch",
		"record implementation findings",
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
		"one subagent per task",
		"Sequential dispatch only, never parallel",
		"fresh context per task",
		"example-reviewing-impl",
		"example-executing-plans",
		"DONE_WITH_CONCERNS",
		"dispatch one review subagent",
		"final Applied batch",
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

func TestAgentsDocTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"testCmd": "go test ./...",
			"gateCmd": "make gate",
		},
		"layout": testLayout(),
		"data":   map[string]any{},
		"skills": map[string]bool{"brainstorming": true, "adr-lifecycle": true, "tdd": true},
	}
	out := renderGolden(t, "agents-doc/AGENTS.md.tmpl", data)
	for _, phrase := range []string{
		"## You and this project",
		"## Identity",
		"## Invariants",
		"## Workflow",
		"## Commands",
		"## Document map",
		"example-brainstorming",
		"example-reviewing-impl",
		"example-retrospective",
		"make gate",
	} {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
	// invariant: rendering/templates:workflow-chain-adr-before-plan
	if !strings.Contains(out, "ADR (if warranted) → plan (if warranted)") {
		t.Errorf("Workflow chain must present ADR before plan:\n%s", out)
	}
	// invariant: rendering/templates:workflow-chain-surfaces-resync
	if !strings.Contains(out, "resync (when both)") {
		t.Errorf("Workflow chain must surface the resync step:\n%s", out)
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
	// invariant: rendering/templates:reviewers-report-only
	{
		tmpl: "agents/adr-reviewer.md.tmpl",
		want: []string{"Regen command: `awf sync`."},
		ban:  []string{"For each item below", "Apply mechanical and reasoned fixes directly", "apply the fix directly", "3-round soft cap", "as new commits"},
	},
	{
		tmpl: "agents/plan-reviewer.md.tmpl",
		ban:  []string{"For each item below", "Apply mechanical and reasoned fixes directly", "apply the fix directly", "3-round soft cap", "as new commits"},
	},
	{
		tmpl: "agents/code-reviewer.md.tmpl",
		ban:  []string{"For each item below", "Apply mechanical and reasoned fixes directly", "apply the fix directly", "3-round soft cap", "as new commits"},
	},
	{
		tmpl: "agents-doc/AGENTS.md.tmpl",
		want: []string{"Conventional Commits; one concern per commit."},
		ban:  []string{"Chain skills", "Task skills", "example-brainstorming"},
	},
	{
		tmpl: "skills/adr-lifecycle/SKILL.md.tmpl",
		want: []string{"the multi-state lifecycle", "Run `awf sync` to regenerate"},
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
		want: []string{"the project's gate (fast tier)", "Auto-commit when green"},
	},
	{
		tmpl: "skills/proposing-adr/SKILL.md.tmpl",
		want: []string{"follow the ADR template's section order", "Run `awf sync` to regenerate"},
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
		want: []string{"an amendment-while-Proposed edit", "using the project's commit scope conventions"},
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
		want: []string{"**Validate before every commit.**", "run the project's gate", "defense in depth", "Sequential dispatch only, never parallel"},
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

// The AGENTS.md task-skills sentence derives from the catalog's enabled
// non-Chain skills - every catalog task skill appears iff enabled (a hand
// enumeration could never mention a newer one like refactor-coupling-audit),
// and disabled ones stay absent (ADR-0046 follow-up sweep).
func TestAgentsDocTaskSkillsGating(t *testing.T) {
	// brainstorming carries a local sidecar: the guide's chain sentence needs a
	// chain skill in the effective set, but a non-local one would demand its
	// ADR-0081 closure (including adr-lifecycle, banned below) at open.
	root := scaffoldFiles(t, "prefix: example\nskills:\n  - brainstorming\n  - bugfix\n  - exploring\n  - refactor-coupling-audit\nagents: []\n",
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
	if !strings.Contains(out, "**Task skills** (as needed): `example-bugfix`, `example-exploring`, `example-refactor-coupling-audit`.") {
		t.Errorf("expected a catalog-derived task-skills sentence:\n%s", out)
	}
	for _, banned := range []string{"example-tdd", "example-debugging", "example-adr-lifecycle"} {
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
