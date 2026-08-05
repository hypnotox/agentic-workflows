package catalog

import (
	"strings"
	"testing"
)

// TestPlanReviewerStepExactnessSanctionsBatch pins the ADR-0095 refinement: the
// plan-reviewer's step-exactness focus item must accept the batch task form, not
// only exact diffs.
func TestPlanReviewerStepExactnessSanctionsBatch(t *testing.T) {
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
		if m["name"] == "step-exactness" {
			desc, _ = m["description"].(string)
		}
	}
	for _, clause := range []string{"batch task", "inline or subagent-driven", "green transaction", "path-disjoint", "parent-owned shared files", "command-confined"} {
		if !strings.Contains(desc, clause) {
			t.Errorf("step-exactness missing %q: %q", clause, desc)
		}
	}
	for _, forbidden := range []string{"coupled phase", "coupled-phase", "one commit per task"} {
		if strings.Contains(desc, forbidden) {
			t.Errorf("step-exactness retains %q: %q", forbidden, desc)
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
				"verification-instrument-can-fail": {"added or changed mechanical check", "negative case", "temporary falsification", "mutation landed", "verdict counts"},
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
