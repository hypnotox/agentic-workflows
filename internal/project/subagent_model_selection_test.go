package project

import (
	"strings"
	"testing"
)

type governedDispatches struct {
	skill    string
	sections []string
}

var deliberateSelectionDispatches = []governedDispatches{
	{skill: "brainstorming", sections: []string{"grounding-check-output-format"}},
	{skill: "exploring", sections: []string{"dispatch"}},
	{skill: "executing-plans", sections: []string{"procedure-per-task"}},
	{skill: "subagent-driven-development", sections: []string{"dispatch-conventions", "procedure-status-handling"}},
	{skill: "reviewing-adr", sections: []string{"dispatch-subagent", "re-review-loop"}},
	{skill: "reviewing-plan", sections: []string{"dispatch-subagent", "re-review-loop"}},
	{skill: "reviewing-plan-resync", sections: []string{"dispatch-subagent-narrowed", "re-review-loop"}},
	{skill: "reviewing-impl", sections: []string{"dispatch-subagent", "re-review-loop"}},
}

var deliberateSelectionCommon = []string{
	"smallest model expected to complete reliably",
	"small` is for narrow, mechanical, low-ambiguity work",
	"standard` is for substantive but bounded work",
	"large` is for broad, intricate, cross-cutting, or high-consequence work",
	"Uncertainty, failed reasoning, or widened scope requires reconsideration and possible escalation.",
}

const (
	deliberateSelectionPiRule    = "Omit the `model` field entirely to use configured role routing; when the selected complexity warrants an override, pass the tier's exact `provider/model-id`. Never pass `default`, `auto`, or `inherit parent` as a model value."
	deliberateSelectionNonPiRule = "Select the smallest reliable target-native model explicitly; if this harness cannot select a model, use its default and note in the dispatch brief that explicit selection is unavailable."
)

func renderedEditSection(t *testing.T, body, section string) string {
	t.Helper()
	marker := "<!-- awf:edit " + section + ":"
	start := strings.Index(body, marker)
	if start < 0 {
		t.Fatalf("missing rendered section %q", section)
	}
	rest := body[start+len(marker):]
	if end := strings.Index(rest, "<!-- awf:edit "); end >= 0 {
		rest = rest[:end]
	}
	return rest
}

func assertDeliberateSelectionOccurrences(t *testing.T, body string, spec governedDispatches, rule string) {
	t.Helper()
	for _, section := range spec.sections {
		t.Run(section, func(t *testing.T) {
			occurrence := renderedEditSection(t, body, section)
			for _, want := range deliberateSelectionCommon {
				if !strings.Contains(occurrence, want) {
					t.Errorf("%s/%s missing deliberate-selection clause %q", spec.skill, section, want)
				}
			}
			if !strings.Contains(occurrence, rule) {
				t.Errorf("%s/%s missing target selection rule", spec.skill, section)
			}
			for _, bad := range []string{"model value, and", "model value. to confirm", "unavailable. to confirm"} {
				if strings.Contains(occurrence, bad) {
					t.Errorf("%s/%s contains incoherent dispatch fragment %q", spec.skill, section, bad)
				}
			}
		})
	}
}

func assertNoDeliberateSelectionLeakage(t *testing.T, body string) {
	t.Helper()
	for _, banned := range []string{"subagent_grounding", "subagent_explore", "subagent_review", "subagent_implement", "Luna", "Terra", "Sol", "price", "context limit", "registry catalog"} {
		if strings.Contains(body, banned) {
			t.Errorf("generic render leaks %q", banned)
		}
	}
}

// invariant: rendering/workflow-skill-templates:deliberate-subagent-model-selection
func TestDeliberateSubagentModelSelectionAcrossGovernedDispatches(t *testing.T) {
	dirs := map[string]string{
		"claude": ".claude/skills", "codex": ".agents/skills", "copilot": ".github/skills",
		"cursor": ".cursor/skills", "gemini": ".gemini/skills", "pi": ".pi/skills",
	}
	for _, target := range KnownTargets() {
		t.Run(target, func(t *testing.T) {
			files := explorationRenderedByPath(t, explorationFixtureConfig(target))
			for _, spec := range deliberateSelectionDispatches {
				path := dirs[target] + "/example-" + spec.skill + "/SKILL.md"
				body := files[path]
				if body == "" {
					t.Fatalf("missing %s", path)
				}
				if target == "pi" {
					assertDeliberateSelectionOccurrences(t, body, spec, deliberateSelectionPiRule)
					for _, sentinel := range []string{`model: "default"`, `model: "auto"`, `model: "inherit parent"`} {
						if strings.Contains(body, sentinel) {
							t.Errorf("Pi/%s passes sentinel %q", spec.skill, sentinel)
						}
					}
				} else {
					assertDeliberateSelectionOccurrences(t, body, spec, deliberateSelectionNonPiRule)
					assertNoDeliberateSelectionLeakage(t, body)
				}
			}
			agents := files["AGENTS.md"]
			if agents == "" {
				t.Fatal("missing AGENTS.md")
			}
			assertNoDeliberateSelectionLeakage(t, agents)
		})
	}

	empty := map[string]any{"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "skills": map[string]bool{}, "layout": testLayout()}
	for _, spec := range deliberateSelectionDispatches {
		body := renderSkillGolden(t, spec.skill, empty)
		assertDeliberateSelectionOccurrences(t, body, spec, deliberateSelectionNonPiRule)
		for _, bad := range []string{"<no value>", "{{", "}}", "``"} {
			if strings.Contains(body, bad) {
				t.Errorf("empty %s render leaks %q", spec.skill, bad)
			}
		}
		assertNoDeliberateSelectionLeakage(t, body)
	}
	assertNoDeliberateSelectionLeakage(t, renderGolden(t, "agents-doc/AGENTS.md.tmpl", empty))
}
