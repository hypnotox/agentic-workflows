package evals

import (
	"path/filepath"
	"strings"
	"testing"
)

type cleanIntegrationScenario struct {
	name    string
	input   string
	outcome string
	wants   []string
	rejects []string
}

// invariant: rendering/workflow-skill-templates:clean-integration (TestCleanIntegrationScenarios)
func TestCleanIntegrationScenarios(t *testing.T) {
	scenarios := []cleanIntegrationScenario{
		{name: "duplicated-policy", input: "Two commands would each decide the same policy.", outcome: "Centralize the policy through bounded enabling work before adding behavior.", wants: []string{"duplicated policy", "bounded enabling work", "inside scope"}},
		{name: "adapter-leakage", input: "An adapter representation would enter policy logic.", outcome: "Translate at the narrow integration point rather than accept representation leakage.", wants: []string{"representation leakage", "narrowest clean integration point", "inside scope"}},
		{name: "clean-owner", input: "The existing owner already carries the behavior cleanly.", outcome: "Keep the direct owner and propose no ceremonial refactor.", wants: []string{"no refactor", "few sentences rather than a fixed checklist"}, rejects: []string{"always refactor"}},
		{name: "unrelated-cleanup", input: "A broad cleanup is attractive but unnecessary.", outcome: "Exclude the cleanup under YAGNI.", wants: []string{"YAGNI", "reject unrelated cleanup"}},
		{name: "obsolete-path", input: "The new mechanism replaces a parallel route.", outcome: "Remove or migrate the route when practical, otherwise state reasoned residual debt.", wants: []string{"remove or migrate", "residual debt"}},
		{name: "test-shaped-design", input: "A test would be easier with new production indirection despite a real seam.", outcome: "Use the real seam and reject test-shaped production design.", wants: []string{"test-shaped production design", "existing real seam"}},
		{name: "material-boundary", input: "The enabling work creates a durable choice or expands the requested outcome.", outcome: "Return to the material-decision boundary.", wants: []string{"creates a durable choice", "expands the requested outcome", "separate material decision"}},
	}
	for _, profile := range []string{"core", "full"} {
		t.Run(profile, func(t *testing.T) {
			root := syncPlanFlexibilityProfile(t, profile)
			for _, target := range []string{"pi", "claude"} {
				t.Run(target, func(t *testing.T) {
					for consumer, body := range cleanIntegrationConsumerBodies(t, root, profile, target) {
						for _, scenario := range scenarios {
							if !cleanIntegrationOutcomeMatches(body, scenario) {
								t.Errorf("%s/%s scenario %s (%s) does not yield %s", target, consumer, scenario.name, scenario.input, scenario.outcome)
							}
							for _, required := range scenario.wants {
								mutated := strings.ReplaceAll(body, required, "contradictory-outcome")
								if cleanIntegrationOutcomeMatches(mutated, scenario) {
									t.Errorf("%s/%s scenario %s survives removal of outcome clause %q", target, consumer, scenario.name, required)
								}
							}
						}
					}
				})
			}
		})
	}
}

func cleanIntegrationOutcomeMatches(body string, scenario cleanIntegrationScenario) bool {
	for _, want := range scenario.wants {
		if !strings.Contains(body, want) {
			return false
		}
	}
	for _, reject := range scenario.rejects {
		if strings.Contains(body, reject) {
			return false
		}
	}
	return true
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
		bodies[skill] = read(t, planSkillPath(root, target, skill))
	}
	for _, agent := range agents {
		bodies[agent] = read(t, filepath.Join(root, "."+target, "agents", agent+".md"))
	}
	return bodies
}
