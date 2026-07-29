package project

import (
	"strings"
	"testing"
)

// invariant: rendering/workflow-skill-templates:deliberate-subagent-model-selection
func TestDeliberateSubagentModelSelectionAcrossGovernedDispatches(t *testing.T) {
	skills := []string{
		"brainstorming", "exploring", "executing-plans", "subagent-driven-development",
		"reviewing-adr", "reviewing-plan", "reviewing-plan-resync", "reviewing-impl",
	}
	dirs := map[string]string{
		"claude": ".claude/skills", "codex": ".agents/skills", "copilot": ".github/skills",
		"cursor": ".cursor/skills", "gemini": ".gemini/skills", "pi": ".pi/skills",
	}
	common := []string{
		"smallest model expected to complete reliably",
		"small` is for narrow, mechanical, low-ambiguity work",
		"standard` is for substantive but bounded work",
		"large` is for broad, intricate, cross-cutting, or high-consequence work",
		"Uncertainty, failed reasoning, or widened scope requires reconsideration and possible escalation.",
	}
	piRule := "Omit the `model` field entirely to use configured role routing; when the selected complexity warrants an override, pass the tier's exact `provider/model-id`. Never pass `default`, `auto`, or `inherit parent` as a model value."
	nonPiRule := "Select the smallest reliable target-native model explicitly; if this harness cannot select a model, use its default and note in the dispatch brief that explicit selection is unavailable."
	for _, target := range KnownTargets() {
		t.Run(target, func(t *testing.T) {
			files := explorationRenderedByPath(t, explorationFixtureConfig(target))
			for _, skill := range skills {
				path := dirs[target] + "/example-" + skill + "/SKILL.md"
				body := files[path]
				if body == "" {
					t.Fatalf("missing %s", path)
				}
				for _, want := range common {
					if !strings.Contains(body, want) {
						t.Errorf("%s/%s missing deliberate-selection clause %q", target, skill, want)
					}
				}
				if target == "pi" {
					if !strings.Contains(body, piRule) {
						t.Errorf("Pi/%s missing deliberate Pi selection rule", skill)
					}
					for _, sentinel := range []string{`model: "default"`, `model: "auto"`, `model: "inherit parent"`} {
						if strings.Contains(body, sentinel) {
							t.Errorf("Pi/%s passes sentinel %q", skill, sentinel)
						}
					}
				} else if !strings.Contains(body, nonPiRule) {
					t.Errorf("%s/%s missing target-native selection rule", target, skill)
				}
			}
		})
	}

	empty := map[string]any{"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "skills": map[string]bool{}, "layout": testLayout()}
	for _, skill := range skills {
		body := renderSkillGolden(t, skill, empty)
		for _, bad := range []string{"<no value>", "{{", "}}", "``"} {
			if strings.Contains(body, bad) {
				t.Errorf("empty %s render leaks %q", skill, bad)
			}
		}
	}
	agents := renderGolden(t, "agents-doc/AGENTS.md.tmpl", empty)
	for _, body := range append([]string{agents}, func() []string {
		out := make([]string, 0, len(skills))
		for _, skill := range skills {
			out = append(out, renderSkillGolden(t, skill, empty))
		}
		return out
	}()...) {
		for _, banned := range []string{"subagent_grounding", "subagent_explore", "subagent_review", "subagent_implement", "Luna", "Terra", "Sol", "price", "context limit", "registry catalog"} {
			if strings.Contains(body, banned) {
				t.Errorf("generic render leaks %q", banned)
			}
		}
	}
}
