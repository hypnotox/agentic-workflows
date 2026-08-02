package plan

import (
	"errors"
	"testing"
)

// ParseSources is the immutable-snapshot boundary: malformed siblings report
// their own paths without discarding valid documents or admitting escapes.
func TestParseSourcesAggregatesIndependentSnapshotDiagnostics(t *testing.T) {
	valid := []byte("---\nformat: plan-v1\ndate: 2026-08-02\nadrs: []\nstatus: Proposed\n---\n# Plan: Valid\n\n## Goal\n\nGoal.\n\n## Architecture summary\n\nSummary.\n\n## Phase 1: Work\n\n**Execution mode: inline.**\n\n### Task 1.1: Do it\n\nDo it.\n\n### Phase close\n\n```commit\nfeat(awf): work\n```\n\n## Definition of done\n\n- Done.\n")
	plans, err := ParseSources([]Source{
		{Filename: "2026-08-02-valid.md", Path: "snapshot/valid.md", Bytes: valid},
		{Filename: "2026-08-03-escape.md", Path: "\x00escape"},
		{Filename: "2026-08-04-bad.md", Path: "snapshot/bad.md", Bytes: []byte("---\nformat: [\n---\n")},
		{Filename: "not-a-plan.md", Path: "snapshot/ignored.md", Bytes: []byte("bad")},
	})
	var diagnostics *DiagnosticsError
	if !errors.As(err, &diagnostics) || len(plans) != 1 || plans[0].Filename != "2026-08-02-valid.md" {
		t.Fatalf("ParseSources = %#v, %v", plans, err)
	}
	if len(diagnostics.Diagnostics) != 2 || diagnostics.Diagnostics[0].Category != "path" || diagnostics.Diagnostics[0].Path != "2026-08-03-escape.md" || diagnostics.Diagnostics[1].Category != "frontmatter" || diagnostics.Diagnostics[1].Path != "2026-08-04-bad.md" {
		t.Fatalf("diagnostics = %#v", diagnostics.Diagnostics)
	}
}
