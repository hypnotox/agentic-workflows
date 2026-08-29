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

type maintainabilityFinding struct {
	focus          string
	severity       string
	location       string
	issue          string
	suggestedFix   string
	classification string
}

// invariant: rendering/workflow-skill-templates:concrete-maintainability-review (TestConcreteMaintainabilityReviewScenarios)
func TestConcreteMaintainabilityReviewScenarios(t *testing.T) {
	scenarios := []struct {
		name    string
		facts   maintainabilityFacts
		want    string
		risk    string
		clauses []string
	}{
		{"dual-ownership", maintainabilityFacts{concreteRisk: true}, "actionable", "future divergence", []string{"semantic owner", "affected location", "concrete maintainability risk", "smallest clean remediation", "classification"}},
		{"duplicated-policy", maintainabilityFacts{concreteRisk: true}, "actionable", "hidden parallel policy", []string{"future divergence", "hidden parallel policy"}},
		{"dependency-inversion", maintainabilityFacts{concreteRisk: true}, "actionable", "inappropriate dependency", []string{"inappropriate dependency"}},
		{"representation-leakage", maintainabilityFacts{concreteRisk: true}, "actionable", "representation leakage", []string{"representation leakage"}},
		{"wrong-model-workaround", maintainabilityFacts{concreteRisk: true}, "actionable", "wrong model", []string{"wrong model"}},
		{"unbounded-debt", maintainabilityFacts{concreteRisk: true}, "actionable", "unbounded debt", []string{"unbounded debt"}},
		{"reduced-verification", maintainabilityFacts{concreteRisk: true}, "actionable", "reduced verification strength", []string{"reduced verification strength"}},
		{"aesthetic-preference", maintainabilityFacts{aestheticOnly: true}, "reject", "", []string{"pure aesthetic", "non-admissible"}},
		{"competing-clean-options", maintainabilityFacts{competingCleanOptions: true}, "autonomous", "", []string{"competing clean local remedies", "approved boundary"}},
		{"material-boundary", maintainabilityFacts{changedBoundary: true}, "brainstorm", "", []string{"changed approved boundary", "brainstorming", "independently of severity"}},
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
							if scenario.facts.concreteRisk {
								assertMaintainabilityFinding(t, target+"/"+consumer+"/"+scenario.name, maintainabilityFindingFromContract(body, scenario.risk), scenario.risk)
							}
							for _, clause := range scenario.clauses {
								mutated := strings.ReplaceAll(body, clause, "missing-governing-clause")
								if got := maintainabilityDisposition(mutated, scenario.facts, scenario.clauses...); got == scenario.want {
									t.Errorf("%s/%s %s still yields %q without %q", target, consumer, scenario.name, got, clause)
								}
							}
						}
						for _, mapping := range maintainabilityFieldMappings() {
							mutated := strings.ReplaceAll(body, mapping, "missing-field-mapping")
							if finding := maintainabilityFindingFromContract(mutated, "future divergence"); finding.location != "" || finding.issue != "" || finding.suggestedFix != "" || finding.classification != "" {
								t.Errorf("%s/%s still constructs mapped finding without %q: %#v", target, consumer, mapping, finding)
							}
						}
					}
				})
			}
		})
	}
}

// invariant: rendering/workflow-skill-templates:semantic-owner-assurance-decomposition (TestSemanticOwnerAssuranceScenarios)
func TestSemanticOwnerAssuranceScenarios(t *testing.T) {
	for _, profile := range []string{"core", "full"} {
		t.Run(profile, func(t *testing.T) {
			root := syncPlanFlexibilityProfile(t, profile)
			for _, target := range []string{"pi", "claude"} {
				t.Run(target, func(t *testing.T) {
					bodies := map[string]string{
						"implementer":    read(t, filepath.Join(root, "."+target, "agents", "implementer.md")),
						"code-reviewer":  read(t, filepath.Join(root, "."+target, "agents", "code-reviewer.md")),
						"reviewing-impl": read(t, planSkillPath(root, target, "reviewing-impl")),
						"workflow":       read(t, filepath.Join(root, "docs", "workflow.md")),
					}
					if profile == "full" {
						bodies["executing-plans"] = read(t, planSkillPath(root, target, "executing-plans"))
						bodies["subagent-driven-development"] = read(t, planSkillPath(root, target, "subagent-driven-development"))
					}
					for consumer, body := range bodies {
						for _, residue := range []string{"<no value>", "<nil>"} {
							if strings.Contains(body, residue) {
								t.Errorf("%s contains empty-data residue %q", consumer, residue)
							}
						}
						for _, want := range []string{
							"separates independently verifiable owners into distinct implementation, settlement, and assurance units",
							"cross-owner composition is itself one coherent transaction or protected contract",
							"same underlying semantic concern or violated contract across separable owners",
							"not severity, reviewer lens, or remediation classification",
							"partitions the finite remaining scope",
							"ordinary bounded review",
							"without another reviewer dispatch",
							"parent-owned focused evidence for each fresh unit",
							"terminal assurance covers composed integration effects and the complete range",
							"Unrelated blockers stay under implementation-autonomy routing and never widen the active outcome",
							"No file, line, commit, task, finding-count, or elapsed-time threshold",
						} {
							if !strings.Contains(body, want) {
								t.Errorf("%s missing semantic-owner scenario clause %q", consumer, want)
							}
						}
					}
				})
			}
		})
	}
}

func maintainabilityFieldMappings() []string {
	return []string{
		"`location` records the affected location",
		"`issue` names the semantic owner and concrete risk",
		"`suggested_fix` names the smallest clean remediation",
		"`classification` records remediation ownership",
	}
}

func maintainabilityFindingFromContract(body, risk string) maintainabilityFinding {
	for _, mapping := range maintainabilityFieldMappings() {
		if !strings.Contains(body, mapping) {
			return maintainabilityFinding{}
		}
	}
	if !strings.Contains(body, risk) || !strings.Contains(body, "six-field schema") || !strings.Contains(body, "severity remains informational") {
		return maintainabilityFinding{}
	}
	return maintainabilityFinding{
		focus:          "maintainable-design",
		severity:       "concern",
		location:       "owner/file.go:42",
		issue:          "policy owner risks " + risk,
		suggestedFix:   "consolidate at the existing owner",
		classification: "reasoned",
	}
}

func assertMaintainabilityFinding(t *testing.T, label string, got maintainabilityFinding, risk string) {
	t.Helper()
	if got.focus != "maintainable-design" || got.severity != "concern" || got.location != "owner/file.go:42" || !strings.Contains(got.issue, "policy owner") || !strings.Contains(got.issue, risk) || got.suggestedFix != "consolidate at the existing owner" || got.classification != "reasoned" {
		t.Errorf("%s produced incorrectly mapped six-field finding: %#v", label, got)
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
