package project

import (
	"strings"
	"testing"
)

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
		tmpl: "agents/grounding-checker.md.tmpl",
		want: []string{"Ground guide-first, in order", "Current-state documentation is what binds"},
	},
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
			"Exercise the selected evidence against the unfixed behaviour",
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
			"apply it directly under the durable-oracle rule in that case",
			"Apply the durable-oracle rule above directly.",
			"the project's gate",
			"apply the fix under the durable-oracle rule",
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
			"Run `./awf new pitfall \"<Title>\"`, then edit the reported authored source under `.awf/docs/pitfalls/`",
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
		want: []string{"the multi-state lifecycle", "Run `./awf render` to regenerate", "whose members may appear in any order", "settled review later appends only Implemented"},
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
		want: []string{"sole owner of the effort lifecycle", "Continue autonomously or through a target-native successor"},
	},
	{
		tmpl: "skills/executing-plans/SKILL.md.tmpl",
		want: []string{"the project's gate", "Auto-commit the phase only when green"},
	},
	{
		tmpl: "skills/proposing-adr/SKILL.md.tmpl",
		want: []string{"follow the ADR template's section order", "Run `./awf render` to regenerate"},
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
