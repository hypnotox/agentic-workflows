package plan

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestParseSourcesParityAcrossFilesystemContract(t *testing.T) {
	dir := t.TempDir()
	v1 := []byte("---\nformat: plan-v1\ndate: 2026-08-02\nadrs: []\nstatus: Proposed\n---\n# Plan: Valid\n\n## Goal\n\nGoal.\n\n## Architecture summary\n\nSummary.\n\n## Phase 1: Work\n\n**Execution mode: inline.**\n\n### Task 1.1: Do it\n\nDo it.\n\n### Phase close\n\n```commit\nfeat(awf): work\n```\n\n## Definition of done\n\n- Done.\n")
	v2 := strings.Replace(string(v1), "format: plan-v1", "format: plan-v2", 1)
	v2 = strings.Replace(v2, "- Done.", "- `dod: complete` Done.", 1)
	v2 = strings.Replace(v2, "**Execution mode: inline.**", "**Execution mode: inline.**\n\nCompletes: [\"complete\"]", 1)
	files := []struct {
		name string
		body []byte
	}{
		{"2026-08-01-v1.md", v1},
		{"2026-08-02-v2.md", []byte(v2)},
		{"2026-08-03-legacy.md", []byte("# Historical plan without a format marker.\n")},
		{"2026-08-04-frontmatter.md", []byte("---\nformat: [\n---\n")},
		{"2026-08-05-structure.md", []byte(strings.Replace(string(v1), "## Goal", "## Missing", 1))},
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(dir, file.name), file.body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "fixture.md"), v1, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("fixture.md", filepath.Join(dir, "2026-08-06-inside.md")); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, v1, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "2026-08-07-outside.md")); err != nil {
		t.Fatal(err)
	}

	fromDir, dirErr := ParseDir(dir)
	sources, err := parseDirSources(dir)
	if err != nil {
		t.Fatal(err)
	}
	fromSources, sourceErr := ParseSources(sources)
	if !reflect.DeepEqual(fromDir, fromSources) || !reflect.DeepEqual(dirErr, sourceErr) {
		t.Fatalf("filesystem/source parity = %#v %v; %#v %v", fromDir, dirErr, fromSources, sourceErr)
	}
	if len(fromDir) != 4 || fromDir[0].Format != "plan-v1" || fromDir[1].Format != "plan-v2" || fromDir[2].Filename != "2026-08-03-legacy.md" || fromDir[3].Filename != "2026-08-06-inside.md" || !reflect.DeepEqual(fromDir[0].Source, v1) {
		t.Fatalf("parsed models or retained bytes = %#v", fromDir)
	}
	var diagnostics *DiagnosticsError
	if !errors.As(dirErr, &diagnostics) || len(diagnostics.Diagnostics) != 3 || diagnostics.Diagnostics[0].Path != "2026-08-04-frontmatter.md" || diagnostics.Diagnostics[1].Path != "2026-08-05-structure.md" || diagnostics.Diagnostics[2].Path != "2026-08-07-outside.md" || diagnostics.Diagnostics[2].Category != "path" {
		t.Fatalf("ordered diagnostics = %#v", diagnostics)
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

func TestParseDirSourcesAcceptsAbsentDirectory(t *testing.T) {
	sources, err := parseDirSources(filepath.Join(t.TempDir(), "absent"))
	if err != nil || sources != nil {
		t.Fatalf("absent plans directory = %#v, %v", sources, err)
	}
}
