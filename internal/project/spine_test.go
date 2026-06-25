package project

import (
	"io/fs"
	"strings"
	"testing"

	"agentic-workflows/internal/render"
	"agentic-workflows/templates"
)

func renderGolden(t *testing.T, tmplPath string, data map[string]any) string {
	t.Helper()
	src, err := fs.ReadFile(templates.FS, tmplPath)
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	out, err := render.Render(string(src), nil, func(string) (string, error) { return "", nil }, data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	assertNoLeaks(t, out)
	return out
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
	return renderGolden(t, "agents/"+name+".md.tmpl", data)
}

func TestAdrReviewerAgent(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"invariantTestPath": "internal/adrtools/invariants_test.go",
			"activeMdRegenCmd":  "go test ./internal/adrtools/",
		},
		"layout": map[string]any{"adrDir": "docs/decisions", "activeMd": "docs/decisions/ACTIVE.md"},
		"data": map[string]any{
			"focusItems": []map[string]any{
				{
					"name":        "context-grounding",
					"description": "Verify factual claims in the Context section against named files, ADRs, and state docs; flag stale claims and drift since brainstorm.",
				},
			},
			"docCurrencyItems": []map[string]any{
				{"check": "docs/decisions/state/<domain>.md — state-doc update or creation when ADR shifts a domain"},
				{"check": "Predecessor status flip when supersedes: is non-empty"},
				{"check": "docs/workflow.md — update when ADR changes a workflow rule"},
				{"check": "AGENTS.md — update when ADR changes chain, principles, or invariants"},
				{"check": "Frontmatter completeness: status, date, supersedes, superseded_by, tags, related"},
				{"check": "docs/decisions/ACTIVE.md — regenerate when status lands as Accepted or Implemented"},
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
		"3-round soft cap",
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
				{"check": "docs/decisions/state/<domain>.md — update when plan shifts a tracked domain"},
				{"check": "docs/workflow.md — update when plan changes a workflow rule"},
				{"check": "AGENTS.md — update when plan changes chain, principles, or invariants"},
				{"check": "docs/decisions/ACTIVE.md — regenerate when plan flips an ADR status"},
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
		"3-round soft cap",
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
				{"check": "docs/decisions/state/<domain>.md — update when the implementation shifts a tracked domain"},
				{"check": "docs/decisions/ACTIVE.md — regenerate when ADR status flips to Implemented"},
			},
		},
	}

	out := renderAgentGolden(t, "code-reviewer", data)

	// Assert frontmatter name line (agents are unprefixed)
	if !strings.Contains(out, "name: code-reviewer") {
		t.Errorf("expected 'name: code-reviewer' in output:\n%s", out)
	}

	// Assert description contains Specialised reviewer for example (kept green by TestEndToEndGolden too)
	if !strings.Contains(out, "Specialised reviewer for example") {
		t.Errorf("expected 'Specialised reviewer for example' in description:\n%s", out)
	}

	// Assert shared review-discipline spine phrases (verbatim from siblings)
	sharedPhrases := []string{
		"mechanical",
		"reasoned",
		"user-decision",
		"3-round soft cap",
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
			"workflowDoc":  "docs/workflow.md",
			"gateCmd":      "./x gate",
			"gateDuration": "~2 min",
		},
		"layout": map[string]any{"plansDir": "docs/plans"},
		"data":   map[string]any{},
	}

	out := renderSkillGolden(t, "writing-plans", data)

	// Assert frontmatter name line
	if !strings.Contains(out, "name: example-writing-plans") {
		t.Errorf("expected 'name: example-writing-plans' in output:\n%s", out)
	}

	// Assert load-bearing phrases unique to writing-plans
	loadBearing := []string{
		"bite-sized",
		"exact file paths",
		"No placeholders",
		"example-reviewing-plan",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
}

func TestExecutingPlansTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"workflowDoc":        "docs/workflow.md",
			"gateCmd":            "./x gate",
			"gateDuration":       "~2 min",
			"activeMdRegenCmd":   "go test ./internal/adrtools/",
			"hostGitAdrRef":      "docs/decisions/ADR-acme-001-host-git.md",
			"oracleStateDoc":     "docs/decisions/state/acme-oracle.md",
			"keyInvariantAdrRef": "",
		},
		"layout": map[string]any{"plansDir": "docs/plans", "activeMd": "docs/decisions/ACTIVE.md"},
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
		"Proposed → Accepted",
		"Implementation complete",
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
			"workflowDoc":         "docs/workflow.md",
			"gateCmd":             "./x gate",
			"gateCmdFull":         "./x gate full",
			"activeMdRegenCmd":    "go test ./internal/adrtools/",
			"perTaskReviewAdrRef": "",
			"keyInvariantAdrRef":  "",
			"hostGitAdrRef":       "",
			"oracleStateDoc":      "",
		},
		"layout": map[string]any{"plansDir": "docs/plans", "activeMd": "docs/decisions/ACTIVE.md"},
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
		"Sequential dispatch only — never parallel",
		"fresh context per task",
		"example-reviewing-impl",
		"example-executing-plans",
		"DONE_WITH_CONCERNS",
		"dispatch one review subagent",
		"Proposed → Accepted",
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
			"gateCmd":            "./x gate",
			"gateCmdFull":        "./x gate full",
			"workflowDoc":        "docs/workflow.md",
			"docCurrencyTargets": "docs/ and docs/decisions/",
			"pitfallsDoc":        "docs/pitfalls.md",
		},
		"data": map[string]any{},
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

func TestDebuggingTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"workflowDoc":  "docs/workflow.md",
			"debuggingDoc": "docs/debugging.md",
			"gateCmd":      "./x gate",
			"gateCmdFull":  "./x gate full",
		},
		"data": map[string]any{},
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
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
}

func TestProposingAdrTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"activeMdRegenCmd":    "go test ./internal/adrtools/",
			"gateCmd":             "./x gate",
			"workflowDoc":         "docs/workflow.md",
			"stateDocsPath":       "docs/decisions/state/",
			"adrProposeCommitFmt": "docs(adr): propose NNNN <short title>",
		},
		"layout": map[string]any{
			"adrDir": "docs/decisions", "adrTemplate": "docs/decisions/template.md",
			"activeMd": "docs/decisions/ACTIVE.md", "adrReadme": "docs/decisions/README.md",
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
			"workflowDoc":      "docs/workflow.md",
			"stateDocsPath":    "docs/decisions/state/",
			"gateCmd":          "./x gate",
		},
		"layout": map[string]any{
			"adrDir": "docs/decisions", "activeMd": "docs/decisions/ACTIVE.md",
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
					"name":       "Superseded by ADR-NNNN",
					"meaning":    "Replaced by a later ADR",
					"mutability": "Terminal; in-place status edit only at supersedence",
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
		"supersedes",
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
		"vars": map[string]any{
			"workflowDoc":      "docs/workflow.md",
			"stateDocsPath":    "docs/decisions/state/",
			"autonomousAdrRef": "",
			"noDivingAdrRef":   "",
		},
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
		"prefix": "example",
		"vars": map[string]any{
			"commitScope": "docs(plans)",
			"workflowDoc": "docs/workflow.md",
		},
		"layout": map[string]any{"plansDir": "docs/plans"},
		"data":   map[string]any{},
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
		"prefix": "example",
		"vars": map[string]any{
			"commitScope": "docs(plans)",
		},
		"layout": map[string]any{"plansDir": "docs/plans"},
		"data":   map[string]any{},
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
		"prefix": "example",
		"vars": map[string]any{
			"commitScope": "docs(adr)",
			"workflowDoc": "docs/workflow.md",
		},
		"layout": map[string]any{"adrDir": "docs/decisions", "plansDir": "docs/plans"},
		"data":   map[string]any{},
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
			"commitScope": "feat",
			"gateCmd":     "./x gate",
			"workflowDoc": "docs/workflow.md",
		},
		"data": map[string]any{},
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
		"vars": map[string]any{
			"modulePrefix": "github.com/acme/example",
			"workflowDoc":  "docs/workflow.md",
		},
		"data": map[string]any{},
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
		"layout": map[string]any{
			"adrDir": "docs/decisions", "activeMd": "docs/decisions/ACTIVE.md",
			"adrReadme": "docs/decisions/README.md", "plansDir": "docs/plans",
		},
		"data": map[string]any{},
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
		"make gate",
	} {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
	if strings.Contains(out, "reviewing-plan-resync") {
		t.Errorf("Workflow must not present reviewing-plan-resync as a primary step:\n%s", out)
	}
}

func TestAgentsDocTemplateConfigDriven(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"gateCmd": "",
		},
		"layout": map[string]any{
			"adrDir": "docs/decisions", "adrReadme": "docs/decisions/README.md",
			"activeMd": "docs/decisions/ACTIVE.md", "plansDir": "docs/plans",
		},
		"data": map[string]any{
			"identity":  "Example is a widget.",
			"ownership": "You own example.",
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
		"Example is a widget.",
		"You own example.",
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
		"vars": map[string]any{
			"roadmapDoc": "docs/roadmap.md",
		},
		"data": map[string]any{},
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
