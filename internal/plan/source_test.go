package plan

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// ParseSources is the immutable-snapshot boundary: malformed siblings report
// their own paths without discarding valid documents or admitting escapes.
func TestParseSourcesMatchesConfinedDirectoryBytesAndDiagnostics(t *testing.T) {
	dir := t.TempDir()
	valid := []byte("---\nformat: plan-v1\ndate: 2026-08-02\nadrs: []\nstatus: Proposed\n---\n# Plan: Valid\n\n## Goal\n\nGoal.\n\n## Architecture summary\n\nSummary.\n\n## Phase 1: Work\n\n**Execution mode: inline.**\n\n### Task 1.1: Do it\n\nDo it.\n\n### Phase close\n\n```commit\nfeat(awf): work\n```\n\n## Definition of done\n\n- Done.\n")
	bad := []byte("---\nformat: [\n---\n")
	for name, data := range map[string][]byte{"2026-08-02-valid.md": valid, "2026-08-03-bad.md": bad} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fromDir, dirErr := ParseDir(dir)
	fromSources, sourceErr := ParseSources([]Source{
		{Filename: "2026-08-02-valid.md", Path: filepath.Join(dir, "2026-08-02-valid.md"), Bytes: valid},
		{Filename: "2026-08-03-bad.md", Path: filepath.Join(dir, "2026-08-03-bad.md"), Bytes: bad},
	})
	if !reflect.DeepEqual(fromDir, fromSources) || !reflect.DeepEqual(dirErr, sourceErr) {
		t.Fatalf("directory/source parity = %#v %v; %#v %v", fromDir, dirErr, fromSources, sourceErr)
	}
}

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
