package project

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/templates"
)

// TestCheckpointDigestShape pins the compression ADR-0197 item 2 delivers: the
// routine and approval checkpoint partials each stay a four-step digest, so a
// re-expanded fifth step cannot creep back in with the ordered-phrase proofs
// still green.
// invariant: rendering/workflow-skill-templates:memory-checkpoint-chain-coverage (TestCheckpointDigestShape)
// invariant: rendering/workflow-skill-templates:authority-guided-implementation-autonomy (TestCheckpointDigestShape)
// invariant: rendering/pi-runtime:pi-session-handoff-workflow (TestCheckpointDigestShape)
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
			"executable `./awf read plan` projection never creates a checkpoint or fresh-boundary log",
			"judge retained-context relevance and successor work",
			"Continue autonomously or through a target-native successor",
			"Append a log only for an actual fresh boundary",
		} {
			if !strings.Contains(body, phrase) {
				t.Errorf("%s missing checkpoint contract %q", partial, phrase)
			}
		}
		for _, piOnly := range []string{"handoff_session", "[session context]", "Continue with effort <slug>."} {
			if strings.Contains(body, piOnly) {
				t.Errorf("%s leaks Pi-only protocol %q", partial, piOnly)
			}
		}
		for _, direct := range []string{
			"set `Phase:`",
			"set `Next:`",
			"refresh `Updated:`",
			"directing the replacement to read the effort checkpoint",
			"append the actual boundary to `## Handoff log`",
			"attach to it",
			"read its memory",
			"continue with phase",
			"do not start",
		} {
			if strings.Contains(strings.ToLower(body), strings.ToLower(direct)) {
				t.Errorf("%s directly carries handoff procedure or scope %q", partial, direct)
			}
		}
	}

	routine, err := fs.ReadFile(templates.FS, "partials/checkpoint-routine.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Route a new material decision or changed approved boundary through the active workflow to brainstorming",
		"which owns the approval interaction",
		"Separately, report a correctness or safety concern, blocker, or failed required verification through the active workflow",
		"remains unresolved after that workflow's required diagnosis and authority-guided remediation",
	} {
		if !strings.Contains(string(routine), want) {
			t.Errorf("routine checkpoint missing routed attention boundary %q", want)
		}
	}
	for _, direct := range []string{
		"Decide whether user attention is required",
		"material authority drift, a materially different choice, significant scope expansion",
	} {
		if strings.Contains(string(routine), direct) {
			t.Errorf("routine checkpoint retains direct user route %q", direct)
		}
	}

	creation, err := fs.ReadFile(templates.FS, "partials/effort-creation.md")
	if err != nil {
		t.Fatalf("read effort creation partial: %v", err)
	}
	body := string(creation)
	for _, want := range []string{"**Autonomous effort creation.**", "continuity materially helps", "faithful outcome, title, and canonical short slug", "awf effort new --slug <slug> \"<title>\"", "report the allocated immutable identity", "continue there"} {
		if !strings.Contains(body, want) {
			t.Errorf("effort creation partial missing %q", want)
		}
	}
	for _, obsolete := range []string{"clear response in a later turn", "reconfirm after context loss", "Mandatory first-creation confirmation"} {
		if strings.Contains(body, obsolete) {
			t.Errorf("effort creation partial retains obsolete policy %q", obsolete)
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
			assertOrderedPhrases(t, out, "the complete transaction", "`./awf check staged`", "`./x gate`", "wired pre-commit hook enforces both", "only in a clone without wired hooks")
			if name == "executing-plans" && !strings.Contains(out, "when in doubt, run both manually") {
				t.Errorf("executing-plans lost uncertain-hook verification fallback")
			}
		})
	}

	agents := renderGolden(t, "agents-doc/AGENTS.md.tmpl", configured)
	assertOrderedPhrases(t, agents, "the complete transaction", "`./awf check staged`", "`./x gate`", "wired pre-commit hook enforces both", "only in a clone without wired hooks")

	fallback := map[string]any{"prefix": "example", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{}}
	for _, name := range []string{"adr-lifecycle", "executing-plans", "subagent-driven-development"} {
		t.Run(name+"-fallback", func(t *testing.T) {
			out := renderSkillGolden(t, name, fallback)
			assertOrderedPhrases(t, out, "the complete transaction", "`./awf check staged`", "the project's gate", "wired pre-commit hook enforces both", "only in a clone without wired hooks")
			if name == "executing-plans" && !strings.Contains(out, "when in doubt, run both manually") {
				t.Errorf("executing-plans lost uncertain-hook verification fallback")
			}
		})
	}
	fallbackAgents := renderGolden(t, "agents-doc/AGENTS.md.tmpl", fallback)
	assertOrderedPhrases(t, fallbackAgents, "the complete transaction", "`./awf check staged`", "the project's gate", "wired pre-commit hook enforces both", "only in a clone without wired hooks")
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
