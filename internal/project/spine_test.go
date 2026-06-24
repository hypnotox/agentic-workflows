// internal/project/spine_test.go
package project

import (
	"io/fs"
	"strings"
	"testing"

	"agentic-workflows/internal/render"
	"agentic-workflows/templates"
)

func renderSkillGolden(t *testing.T, skill string, data map[string]any) string {
	t.Helper()
	src, err := fs.ReadFile(templates.FS, "skills/"+skill+"/SKILL.md.tmpl")
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	out, err := render.Render(string(src), nil, func(string) (string, error) { return "", nil }, data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(out, "awf:section") || strings.Contains(out, "awf:end") {
		t.Errorf("markers leaked:\n%s", out)
	}
	if strings.Contains(out, "<no value>") {
		t.Errorf("missing sample data (rendered <no value>):\n%s", out)
	}
	if strings.Contains(out, "{{") || strings.Contains(out, "}}") {
		t.Errorf("unrendered template action:\n%s", out)
	}
	return out
}

func TestWritingPlansTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"workflowDoc":      "docs/workflow.md",
			"plansDir":         "docs/plans",
			"gateCmd":          "./x gate",
			"gateDuration":     "~2 min",
			"planTemplatePath": "docs/plans/template.md",
		},
		"data": map[string]any{},
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
			"workflowDoc":      "docs/workflow.md",
			"plansDir":         "docs/plans",
			"gateCmd":          "./x gate",
			"gateDuration":     "~2 min",
			"activeMdRegenCmd": "go test ./internal/adrtools/",
			"activeMdPath":     "docs/decisions/ACTIVE.md",
			"hostGitAdrRef":    "docs/decisions/ADR-acme-001-host-git.md",
			"oracleStateDoc":   "docs/decisions/state/acme-oracle.md",
			"keyInvariantAdrRef": "",
		},
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
			"workflowDoc":        "docs/workflow.md",
			"plansDir":           "docs/plans",
			"gateCmd":            "./x gate",
			"gateCmdFull":        "./x gate full",
			"activeMdRegenCmd":   "go test ./internal/adrtools/",
			"activeMdPath":       "docs/decisions/ACTIVE.md",
			"perTaskReviewAdrRef": "",
			"keyInvariantAdrRef": "",
			"hostGitAdrRef":      "",
			"oracleStateDoc":     "",
		},
		"data": map[string]any{},
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
			"workflowDoc":      "docs/workflow.md",
			"debuggingDoc":     "docs/debugging.md",
			"gateCmd":          "./x gate",
			"gateCmdFull":      "./x gate full",
			"bugfixSkill":      "example-bugfix",
			"brainstormingSkill": "example-brainstorming",
			"tddSkill":         "example-tdd",
			"pitfallsDoc":      "",
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
		"reproduce",
		"root cause",
		"example-bugfix",
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
			"workflowDoc":        "docs/workflow.md",
			"stateDocsPath":      "docs/decisions/state/",
			"adrReadme":          "docs/decisions/README.md",
			"autonomousAdrRef":   "",
			"noDivingAdrRef":     "",
			"groundingCheckAdrRef": "",
		},
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
