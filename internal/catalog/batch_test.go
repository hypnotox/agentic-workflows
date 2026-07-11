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
	if !strings.Contains(desc, "batch task") {
		t.Errorf("step-exactness should sanction the batch task, got: %q", desc)
	}
}
