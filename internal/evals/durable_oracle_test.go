package evals

import (
	"path/filepath"
	"strings"
	"testing"
)

type durableOracleFacts struct {
	deterministicBug        bool
	environmentSpecific     bool
	nondeterministicRace    bool
	destructiveMigration    bool
	automationUnavailable   bool
	merelyInconvenient      bool
	mechanicalEvidenceOrder bool
	weakenExpectedOutput    bool
	reduceVerification      bool
}

type durableOracleScenario struct {
	name      string
	facts     durableOracleFacts
	want      string
	governing []string
}

// invariant: rendering/workflow-skill-templates:strongest-practical-durable-oracle (TestStrongestPracticalDurableOracleScenarios)
func TestStrongestPracticalDurableOracleScenarios(t *testing.T) {
	scenarios := []durableOracleScenario{
		{
			name:  "normal-deterministic-bug",
			facts: durableOracleFacts{deterministicBug: true},
			want:  "red-then-green-regression",
			governing: []string{
				"normal and preferred path", "automated regression test",
				"observed failing for the right reason and then passing",
			},
		},
		{
			name:  "inconvenience-does-not-justify-alternative",
			facts: durableOracleFacts{merelyInconvenient: true},
			want:  "red-then-green-regression",
			governing: []string{
				"normal and preferred path", "automated regression test observed red then green",
				"preferred path is impractical, not merely inconvenient",
			},
		},
		{
			name:  "evidence-order-is-guidance",
			facts: durableOracleFacts{mechanicalEvidenceOrder: true},
			want:  "select-strongest-applicable-evidence",
			governing: []string{
				"evidence order as guidance", "not a requirement to mechanically attempt every earlier option",
				"strongest practical durable oracle",
			},
		},
		{
			name:  "environment-specific-integration",
			facts: durableOracleFacts{environmentSpecific: true},
			want:  "durable-reproduction",
			governing: []string{
				"concrete reason", "preferred path is impractical, not merely inconvenient",
				"preserve or improve verification strength", "deterministic integration or reproduction harness",
				"scripted, reproducible manual verification", "recorded inputs and expected result",
			},
		},
		{
			name:      "nondeterministic-race",
			facts:     durableOracleFacts{nondeterministicRace: true},
			want:      "stress-or-invariant-evidence",
			governing: []string{"concrete reason", "preferred path is impractical, not merely inconvenient", "stress or invariant evidence", "preserve or improve verification strength"},
		},
		{
			name:      "destructive-migration",
			facts:     durableOracleFacts{destructiveMigration: true},
			want:      "safe-fixture-or-dry-run",
			governing: []string{"concrete reason", "preferred path is impractical, not merely inconvenient", "safe fixture or dry-run evidence", "preserve or improve verification strength"},
		},
		{
			name:      "durable-automation-unavailable",
			facts:     durableOracleFacts{automationUnavailable: true},
			want:      "strongest-retained-safe-evidence",
			governing: []string{"durable automation is unavailable", "strongest safe evidence that can be retained", "concrete reason", "preserve or improve verification strength"},
		},
		{
			name:      "reduce-verification-strength",
			facts:     durableOracleFacts{reduceVerification: true},
			want:      "refuse-weakened-oracle",
			governing: []string{"preserve or improve verification strength", "Never weaken verification strength", "strongest safe, reproducible alternative"},
		},
		{
			name:      "weaken-expected-output",
			facts:     durableOracleFacts{weakenExpectedOutput: true},
			want:      "refuse-weakened-oracle",
			governing: []string{"Never weaken expected behaviour", "Never weaken verification strength", "root cause rather than the symptom"},
		},
	}

	for _, profile := range []string{"core", "full"} {
		t.Run(profile, func(t *testing.T) {
			root := syncStandardFootprint(t, profile)
			for _, target := range []string{"pi", "claude"} {
				t.Run(target, func(t *testing.T) {
					for consumer, body := range durableOracleConsumerBodies(t, root, profile, target) {
						for _, scenario := range scenarios {
							if got := durableOracleDisposition(body, scenario.facts, scenario.governing...); got != scenario.want {
								t.Errorf("%s/%s %s: got %q, want %q", target, consumer, scenario.name, got, scenario.want)
							}
							for _, clause := range scenario.governing {
								mutated := strings.ReplaceAll(body, clause, "missing-governing-clause")
								if got := durableOracleDisposition(mutated, scenario.facts, scenario.governing...); got == scenario.want {
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

func durableOracleDisposition(body string, facts durableOracleFacts, clauses ...string) string {
	for _, clause := range clauses {
		if !strings.Contains(body, clause) {
			return "missing"
		}
	}
	switch {
	case facts.weakenExpectedOutput || facts.reduceVerification:
		return "refuse-weakened-oracle"
	case facts.deterministicBug || facts.merelyInconvenient:
		return "red-then-green-regression"
	case facts.mechanicalEvidenceOrder:
		return "select-strongest-applicable-evidence"
	case facts.environmentSpecific:
		return "durable-reproduction"
	case facts.nondeterministicRace:
		return "stress-or-invariant-evidence"
	case facts.destructiveMigration:
		return "safe-fixture-or-dry-run"
	case facts.automationUnavailable:
		return "strongest-retained-safe-evidence"
	default:
		return "missing"
	}
}

func durableOracleConsumerBodies(t *testing.T, root, _, target string) map[string]string {
	t.Helper()
	bodies := map[string]string{
		"bugfix":        read(t, targetSkillPath(root, target, "bugfix")),
		"tdd":           read(t, targetSkillPath(root, target, "tdd")),
		"debugging":     read(t, targetSkillPath(root, target, "debugging")),
		"code-reviewer": read(t, filepath.Join(root, "."+target, "agents", "code-reviewer.md")),
		"testing":       read(t, filepath.Join(root, "docs", "testing.md")),
	}
	return bodies
}
