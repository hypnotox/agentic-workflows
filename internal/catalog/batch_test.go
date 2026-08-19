package catalog

import (
	"strings"
	"testing"
)

// TestPlanReviewerChangeSpecificExecutabilitySanctionsBatch pins the compact
// contract: batch metadata is optional while ambiguous populations retain
// deterministic scope and terminal-state evidence.
func TestPlanReviewerChangeSpecificExecutabilitySanctionsBatch(t *testing.T) {
	items, ok := Standard.Agents["plan-reviewer"].Data["focusItems"].([]any)
	if !ok {
		t.Fatalf("plan-reviewer focusItems missing or not []any")
	}
	var desc string
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		if m["name"] == "change-specific-executability" {
			desc, _ = m["description"].(string)
		}
	}
	for _, clause := range []string{
		"inline or subagent-driven", "green transaction", "change-specific outcomes",
		"ordering dependencies only where they protect a named authority, outcome, scope, safety, compatibility, lifecycle, or verification property",
		"focused evidence", "batch kind", "optional aids", "ambiguous populations",
		"exhaustive Paths", "deterministic Post-check", "commit-capable owners",
		"helpers remain path-confined and commit-disabled", "duplicated generic execution protocol",
	} {
		if !strings.Contains(desc, clause) {
			t.Errorf("change-specific-executability missing %q: %q", clause, desc)
		}
	}
	for _, forbidden := range []string{"coupled phase", "coupled-phase", "one commit per task", "exact paths, symbols, commands", "material boundaries, ordering dependencies, focused evidence"} {
		if strings.Contains(desc, forbidden) {
			t.Errorf("change-specific-executability retains %q: %q", forbidden, desc)
		}
	}
}

func TestADRReviewerDefaultsDoNotDuplicateUniversalLenses(t *testing.T) {
	items, ok := Standard.Agents["adr-reviewer"].Data["focusItems"].([]any)
	if !ok {
		t.Fatalf("adr-reviewer focusItems missing or not []any")
	}
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := entry["name"].(string)
		if name == "decision-clarity" || name == "decision-adherence" || name == "ADR-scope" {
			t.Errorf("ADR reviewer project focus duplicates universal lens %q", name)
		}
	}
}

func TestReviewerVerificationGuidanceDefaults(t *testing.T) {
	contracts := []struct {
		agent string
		items map[string][]string
	}{
		{
			agent: "plan-reviewer",
			items: map[string][]string{
				"snapshot-scoped-verification": {"material census and post-check commands", "exact intermediate snapshot", "terminal set", "lifecycle-authorized residual findings", "premature zero"},
				"check-authority-taxonomy":     {"authority, state, or choreography", "preserve authority checks", "no stricter than the durable property", "no named authority or state obligation"},
			},
		},
		{
			agent: "code-reviewer",
			items: map[string][]string{
				"verification-instrument-can-fail": {"added or changed mechanical check", "negative case", "temporary falsification", "mutation landed", "verdict counts", "restore only the temporary mutation", "whole-file reset", "unrelated uncommitted work"},
				"check-authority-taxonomy":         {"authority, state, or choreography", "preserve authority checks", "no stricter than the durable property", "no named authority or state obligation"},
			},
		},
	}

	for _, contract := range contracts {
		t.Run(contract.agent, func(t *testing.T) {
			items, ok := Standard.Agents[contract.agent].Data["focusItems"].([]any)
			if !ok {
				t.Fatalf("%s focusItems missing or not []any", contract.agent)
			}
			descriptions := map[string]string{}
			for _, item := range items {
				entry, ok := item.(map[string]any)
				if !ok {
					continue
				}
				name, _ := entry["name"].(string)
				descriptions[name], _ = entry["description"].(string)
			}
			for name, clauses := range contract.items {
				description := descriptions[name]
				for _, clause := range clauses {
					if !strings.Contains(description, clause) {
						t.Errorf("%s focus item %q missing %q: %q", contract.agent, name, clause, description)
					}
				}
			}
		})
	}
}
