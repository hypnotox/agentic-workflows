package catalog

import (
	"strings"
	"testing"
)

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
