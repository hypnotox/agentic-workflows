package evals

import (
	"path/filepath"
	"strings"
	"testing"
)

type cleanIntegrationFacts struct {
	duplicatedPolicy       bool
	representationLeak     bool
	cleanOwner             bool
	unrelatedCleanup       bool
	obsoletePath           bool
	testConvenience        bool
	realSeam               bool
	durableChoice          bool
	riskIncrease           bool
	externalBehaviorChange bool
	outcomeExpansion       bool
}

type cleanIntegrationScenario struct {
	name      string
	input     cleanIntegrationFacts
	accepted  string
	rejected  string
	governing []string
}

// invariant: rendering/workflow-skill-templates:clean-integration (TestCleanIntegrationScenarios)
func TestCleanIntegrationScenarios(t *testing.T) {
	scenarios := []cleanIntegrationScenario{
		{name: "duplicated-policy", input: cleanIntegrationFacts{duplicatedPolicy: true}, accepted: "bounded-refactor", rejected: "duplicate-policy", governing: []string{"duplicated policy", "bounded enabling work", "inside scope"}},
		{name: "adapter-leakage", input: cleanIntegrationFacts{representationLeak: true}, accepted: "boundary-translation", rejected: "accept-representation-leakage", governing: []string{"representation leakage", "narrowest clean integration point", "inside scope"}},
		{name: "clean-owner", input: cleanIntegrationFacts{cleanOwner: true}, accepted: "retain-clean-owner", rejected: "ceremonial-refactor", governing: []string{"no refactor", "few sentences rather than a fixed checklist"}},
		{name: "unrelated-cleanup", input: cleanIntegrationFacts{unrelatedCleanup: true}, accepted: "exclude-unrelated-cleanup", rejected: "expand-cleanup", governing: []string{"YAGNI", "reject unrelated cleanup"}},
		{name: "obsolete-path", input: cleanIntegrationFacts{obsoletePath: true}, accepted: "retire-or-record-debt", rejected: "leave-parallel-route-silent", governing: []string{"remove or migrate", "obsolete paths when practical", "residual debt"}},
		{name: "test-shaped-design", input: cleanIntegrationFacts{testConvenience: true, realSeam: true}, accepted: "use-real-seam", rejected: "add-test-shaped-indirection", governing: []string{"test-shaped production design", "existing real seam"}},
		{name: "durable-choice", input: cleanIntegrationFacts{durableChoice: true}, accepted: "material-decision", rejected: "silent-scope-growth", governing: []string{"creates a durable choice", "separate material decision"}},
		{name: "risk-increase", input: cleanIntegrationFacts{riskIncrease: true}, accepted: "material-decision", rejected: "silent-risk-increase", governing: []string{"increases risk", "separate material decision"}},
		{name: "external-behavior-change", input: cleanIntegrationFacts{externalBehaviorChange: true}, accepted: "material-decision", rejected: "silent-behavior-change", governing: []string{"changes external behavior", "separate material decision"}},
		{name: "outcome-expansion", input: cleanIntegrationFacts{outcomeExpansion: true}, accepted: "material-decision", rejected: "silent-outcome-expansion", governing: []string{"expands the requested outcome", "separate material decision"}},
	}
	for _, profile := range []string{"core", "full"} {
		t.Run(profile, func(t *testing.T) {
			root := syncStandardFootprint(t, profile)
			for _, target := range []string{"pi", "claude"} {
				t.Run(target, func(t *testing.T) {
					for consumer, body := range cleanIntegrationConsumerBodies(t, root, profile, target) {
						if !strings.Contains(body, "current and target owner") {
							t.Fatalf("%s/%s does not expose the shared clean-integration rule", target, consumer)
						}
						for _, scenario := range scenarios {
							got, ok := cleanIntegrationDisposition(body, scenario.input)
							if !ok || got != scenario.accepted || got == scenario.rejected {
								t.Errorf("%s/%s scenario %s: got %q, want accepted %q and reject %q", target, consumer, scenario.name, got, scenario.accepted, scenario.rejected)
							}
							for _, clause := range scenario.governing {
								mutated := strings.ReplaceAll(body, clause, "contradictory-outcome")
								if mutatedGot, mutatedOK := cleanIntegrationDisposition(mutated, scenario.input); mutatedOK && mutatedGot == scenario.accepted {
									t.Errorf("%s/%s scenario %s still accepts %q after governing clause %q is removed", target, consumer, scenario.name, scenario.accepted, clause)
								}
							}
						}
					}
				})
			}
		})
	}
}

func cleanIntegrationDisposition(body string, facts cleanIntegrationFacts) (string, bool) {
	requires := func(parts ...string) bool {
		for _, part := range parts {
			if !strings.Contains(body, part) {
				return false
			}
		}
		return true
	}
	switch {
	case facts.durableChoice:
		return "material-decision", requires("creates a durable choice", "separate material decision")
	case facts.riskIncrease:
		return "material-decision", requires("increases risk", "separate material decision")
	case facts.externalBehaviorChange:
		return "material-decision", requires("changes external behavior", "separate material decision")
	case facts.outcomeExpansion:
		return "material-decision", requires("expands the requested outcome", "separate material decision")
	case facts.duplicatedPolicy:
		return "bounded-refactor", requires("duplicated policy", "bounded enabling work", "inside scope")
	case facts.representationLeak:
		return "boundary-translation", requires("representation leakage", "narrowest clean integration point", "inside scope")
	case facts.cleanOwner:
		return "retain-clean-owner", requires("no refactor", "few sentences rather than a fixed checklist")
	case facts.unrelatedCleanup:
		return "exclude-unrelated-cleanup", requires("YAGNI", "reject unrelated cleanup")
	case facts.obsoletePath:
		return "retire-or-record-debt", requires("remove or migrate", "obsolete paths when practical", "residual debt")
	case facts.testConvenience && facts.realSeam:
		return "use-real-seam", requires("test-shaped production design", "existing real seam")
	default:
		return "retain-clean-owner", requires("no refactor")
	}
}

func cleanIntegrationConsumerBodies(t *testing.T, root, _, target string) map[string]string {
	t.Helper()
	skills := []string{"brainstorming", "executing-direct", "bugfix", "tdd", "reviewing-impl"}
	agents := []string{"implementer", "code-reviewer"}
	bodies := make(map[string]string, len(skills)+len(agents))
	for _, skill := range skills {
		bodies[skill] = read(t, targetSkillPath(root, target, skill))
	}
	for _, agent := range agents {
		bodies[agent] = read(t, filepath.Join(root, "."+target, "agents", agent+".md"))
	}
	return bodies
}
