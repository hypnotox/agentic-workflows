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
