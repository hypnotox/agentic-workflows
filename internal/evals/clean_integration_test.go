package evals

import (
	"path/filepath"
	"strings"
	"testing"
)

// invariant: rendering/workflow-skill-templates:clean-integration (TestCleanIntegrationScenarios)
func TestCleanIntegrationScenarios(t *testing.T) {
	// Each scenario is an outcome reading, not a demand for checklist-shaped output.
	scenarios := map[string][]string{
		"duplicated-policy":  {"duplicated policy", "bounded enabling refactor"},
		"adapter-leakage":    {"representation leakage", "narrowest clean integration point"},
		"clean-owner":        {"no refactor", "few sentences rather than a fixed checklist"},
		"unrelated-cleanup":  {"unrelated cleanup", "YAGNI"},
		"obsolete-path":      {"remove or migrate", "residual debt"},
		"test-shaped-design": {"test-shaped production design", "existing real seam"},
		"material-boundary":  {"creates a durable choice", "separate material decision"},
	}
	for _, profile := range []string{"core", "full"} {
		t.Run(profile, func(t *testing.T) {
			root := syncPlanFlexibilityProfile(t, profile)
			for _, target := range []string{"pi", "claude"} {
				t.Run(target, func(t *testing.T) {
					bodies := cleanIntegrationConsumerBodies(t, root, profile, target)
					for name, body := range bodies {
						if !strings.Contains(body, "current and target owner") {
							t.Fatalf("%s/%s lacks the clean-integration shared partial/contract", target, name)
						}
					}
					for name, body := range bodies {
						for scenario, wants := range scenarios {
							for _, want := range wants {
								if !strings.Contains(body, want) {
									t.Errorf("%s/%s does not answer %s scenario with %q", target, name, scenario, want)
								}
							}
						}
						if strings.Contains(body, "mandatory long checklist") {
							t.Errorf("%s/%s turns proportional clean integration into checklist output", target, name)
						}
					}
				})
			}
		})
	}
}

func cleanIntegrationConsumerBodies(t *testing.T, root, profile, target string) map[string]string {
	t.Helper()
	skills := []string{"brainstorming", "executing-direct", "bugfix", "tdd", "reviewing-impl"}
	agents := []string{"implementer", "code-reviewer"}
	if profile == "full" {
		skills = append(skills, "writing-plans", "executing-plans", "subagent-driven-development", "reviewing-plan")
		agents = append(agents, "plan-reviewer")
	}
	bodies := make(map[string]string, len(skills)+len(agents))
	for _, skill := range skills {
		path := planSkillPath(root, target, skill)
		bodies[skill] = read(t, path)
	}
	for _, agent := range agents {
		bodies[agent] = read(t, filepath.Join(root, "."+target, "agents", agent+".md"))
	}
	return bodies
}
