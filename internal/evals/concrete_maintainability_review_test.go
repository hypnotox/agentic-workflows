package evals

import (
	"path/filepath"
	"strings"
	"testing"
)

type maintainabilityFacts struct {
	concreteRisk          bool
	aestheticOnly         bool
	competingCleanOptions bool
	changedBoundary       bool
}

// invariant: rendering/workflow-skill-templates:concrete-maintainability-review (TestConcreteMaintainabilityReviewScenarios)
func TestConcreteMaintainabilityReviewScenarios(t *testing.T) {
	scenarios := []struct {
		name    string
		facts   maintainabilityFacts
		want    string
		clauses []string
	}{
		{"dual-ownership", maintainabilityFacts{concreteRisk: true}, "actionable", []string{"semantic owner", "affected location", "concrete maintainability risk", "smallest clean remediation", "classification"}},
		{"duplicated-policy", maintainabilityFacts{concreteRisk: true}, "actionable", []string{"future divergence", "hidden parallel policy"}},
		{"dependency-inversion", maintainabilityFacts{concreteRisk: true}, "actionable", []string{"inappropriate dependency"}},
		{"representation-leakage", maintainabilityFacts{concreteRisk: true}, "actionable", []string{"representation leakage"}},
		{"wrong-model-workaround", maintainabilityFacts{concreteRisk: true}, "actionable", []string{"wrong model"}},
		{"unbounded-debt", maintainabilityFacts{concreteRisk: true}, "actionable", []string{"unbounded debt"}},
		{"reduced-verification", maintainabilityFacts{concreteRisk: true}, "actionable", []string{"reduced verification strength"}},
		{"aesthetic-preference", maintainabilityFacts{aestheticOnly: true}, "reject", []string{"pure aesthetic", "non-admissible"}},
		{"competing-clean-options", maintainabilityFacts{competingCleanOptions: true}, "autonomous", []string{"competing clean local remedies", "approved boundary"}},
		{"material-boundary", maintainabilityFacts{changedBoundary: true}, "brainstorm", []string{"changed approved boundary", "brainstorming", "independently of severity"}},
	}
	for _, profile := range []string{"core", "full"} {
		t.Run(profile, func(t *testing.T) {
			root := syncPlanFlexibilityProfile(t, profile)
			for _, target := range []string{"pi", "claude"} {
				t.Run(target, func(t *testing.T) {
					for consumer, body := range maintainabilityConsumerBodies(t, root, profile, target) {
						for _, scenario := range scenarios {
							if got := maintainabilityDisposition(body, scenario.facts, scenario.clauses...); got != scenario.want {
								t.Errorf("%s/%s %s: got %q, want %q", target, consumer, scenario.name, got, scenario.want)
							}
							for _, clause := range scenario.clauses {
								mutated := strings.ReplaceAll(body, clause, "missing-governing-clause")
								if got := maintainabilityDisposition(mutated, scenario.facts, scenario.clauses...); got == scenario.want {
									t.Errorf("%s/%s %s still yields %q without %q", target, consumer, scenario.name, got, clause)
								}
							}
						}
					}
				})
			}
		})
	}
}

func maintainabilityDisposition(body string, facts maintainabilityFacts, scenarioClauses ...string) string {
	has := func(parts ...string) bool {
		for _, part := range parts {
			if !strings.Contains(body, part) {
				return false
			}
		}
		return true
	}
	switch {
	case facts.changedBoundary:
		if has(scenarioClauses...) {
			return "brainstorm"
		}
	case facts.aestheticOnly:
		if has(scenarioClauses...) {
			return "reject"
		}
	case facts.competingCleanOptions:
		if has(scenarioClauses...) {
			return "autonomous"
		}
	case facts.concreteRisk:
		if has(append([]string{"semantic owner", "affected location", "concrete maintainability risk", "smallest clean remediation", "classification"}, scenarioClauses...)...) {
			return "actionable"
		}
	}
	return "missing"
}

func maintainabilityConsumerBodies(t *testing.T, root, profile, target string) map[string]string {
	t.Helper()
	bodies := map[string]string{
		"reviewing-impl": read(t, planSkillPath(root, target, "reviewing-impl")),
		"code-reviewer":  read(t, filepath.Join(root, "."+target, "agents", "code-reviewer.md")),
	}
	if profile == "full" {
		bodies["reviewing-plan"] = read(t, planSkillPath(root, target, "reviewing-plan"))
		bodies["plan-reviewer"] = read(t, filepath.Join(root, "."+target, "agents", "plan-reviewer.md"))
	}
	return bodies
}
