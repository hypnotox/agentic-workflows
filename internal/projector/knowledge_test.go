package projector

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReservedFilesNeverBecomeTopics(t *testing.T) {
	root := t.TempDir()
	writeTopicForTest(t, root, "real.md", []string{"**"})
	for _, name := range []string{"index.md", "log.md", "nested/INDEX.MD", "nested/log.md"} {
		writeTestFile(t, filepath.Join(root, "docs", "topics", filepath.FromSlash(name)), []byte("# Contents or empty history\n"), 0o644)
	}
	topics, err := LoadTopics(root)
	if err != nil || len(topics) != 1 || topics[0].ID != "real" {
		t.Fatalf("topics = %v, %v", topics, err)
	}
	coverage, err := ResolveCoverage(root, []string{"src/future.go"})
	if err != nil || len(coverage.Globals) != 1 || len(coverage.Paths[0].Matches) != 0 {
		t.Fatalf("coverage = %v, %v", coverage, err)
	}
	if _, err := Render(root); err != nil {
		t.Fatal(err)
	}
	if findings, err := Check(root); err != nil || len(findings) != 0 {
		t.Fatalf("reserved file check = %v, %v", findings, err)
	}
	// A malformed reserved file is a check finding, not a topic-loading gate.
	writeTestFile(t, filepath.Join(root, "docs/topics/index.md"), []byte("---\npaths: ['**']\n---\n# Index\n"), 0o644)
	if matches, err := Resolve(root, nil); err != nil || len(matches) != 1 {
		t.Fatalf("reserved file became routable: %v, %v", matches, err)
	}
	if _, err := Render(root); err != nil {
		t.Fatalf("render acquired a bundle gate: %v", err)
	}
	findings, err := Check(root)
	if err != nil || len(findings) != 1 || findings[0].Path != "docs/topics/index.md" {
		t.Fatalf("malformed reserved file check = %v, %v", findings, err)
	}
}

func TestManualAdopterMigrationPreservesAuthorityBodiesAndRouting(t *testing.T) {
	root := t.TempDir()
	legacy := map[string]string{
		"docs/topics/paths.md":          "---\npaths: ['src/**']\n---\n# Current paths\nAuthored topic.\n",
		"docs/decisions/predecessor.md": "---\nstatus: active\n---\n# Existing authority\nStill governs until replacement activation.\n",
		"docs/decisions/replacement.md": "---\nstatus: accepted\n---\n# Replacement\nAgreed, not yet in effect.\n",
		"docs/decisions/proposal.md":    "---\nstatus: pending\n---\n# Proposal\nNot yet agreed.\n",
		"docs/plans/route.md":           "# Route\nAuthored implementation plan.\n",
		"docs/manual/note.md":           "# Note\nManually authored reference.\n",
		"README.md":                     "Outside the bundle\xff\n",
	}
	for name, content := range legacy {
		writeTestFile(t, filepath.Join(root, filepath.FromSlash(name)), []byte(content), 0o644)
	}
	beforeRouting, err := ResolveCoverage(root, []string{"src/future.go", "other/file"})
	if err != nil {
		t.Fatal(err)
	}
	for _, generate := range []func(string) (RenderResult, error){Init, Render} {
		if _, err := generate(root); err != nil {
			t.Fatalf("legacy metadata blocked generation: %v", err)
		}
	}
	beforeCheck := snapshotTree(t, root)
	findings, err := Check(root)
	if err != nil || len(findings) == 0 {
		t.Fatalf("legacy metadata accepted: %v, %v", findings, err)
	}
	paths := make(map[string]bool)
	for _, finding := range findings {
		paths[finding.Path] = true
	}
	for name, content := range legacy {
		if strings.HasPrefix(name, "docs/") && !paths[name] {
			t.Errorf("missing migration diagnostic for %s", name)
		}
		got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || string(got) != content {
			t.Fatalf("init/render/check rewrote %s: %q, %v", name, got, err)
		}
	}
	if after := snapshotTree(t, root); !reflect.DeepEqual(beforeCheck, after) {
		t.Fatal("check mutated the repository")
	}

	// Explicit author edits, not a migration command: preserve body, selectors,
	// and each established decision state while supplying interoperable metadata.
	for name, content := range legacy {
		if !strings.HasPrefix(name, "docs/") {
			continue
		}
		var migrated string
		switch {
		case strings.HasPrefix(name, "docs/topics/"):
			migrated = strings.Replace(content, "---\n", "---\ntype: Project Topic\ndescription: Routes source changes to current knowledge.\n", 1)
		case strings.HasPrefix(name, "docs/decisions/"):
			migrated = strings.Replace(content, "status:", "decision_status:", 1)
			status := "draft"
			if strings.Contains(content, "status: active\n") {
				status = "stable"
			}
			migrated = strings.Replace(migrated, "---\n", "---\ntype: Architecture Decision\ndescription: Records this decision's scope and authority.\nstatus: "+status+"\n", 1)
		default:
			migrated = "---\ntype: Reference\ndescription: Describes the authored route or reference.\n---\n" + content
		}
		writeTestFile(t, filepath.Join(root, filepath.FromSlash(name)), []byte(migrated), 0o644)
	}
	if findings, err := Check(root); err != nil || len(findings) != 0 {
		t.Fatalf("migrated check = %v, %v", findings, err)
	}
	afterRouting, err := ResolveCoverage(root, []string{"src/future.go", "other/file"})
	if err != nil || !reflect.DeepEqual(beforeRouting, afterRouting) {
		t.Fatalf("migration changed routing: before=%v, after=%v, %v", beforeRouting, afterRouting, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); !os.IsNotExist(err) {
		t.Fatalf("test unexpectedly required Git: %v", err)
	}

	conflict := []byte("---\ntype: Architecture Decision\ndescription: Conflicting state\ndecision_status: active\nstatus: draft\n---\nRetain this body.\n")
	conflictPath := filepath.Join(root, "docs/decisions/conflict.md")
	writeTestFile(t, conflictPath, conflict, 0o644)
	findings, err = Check(root)
	if err != nil || len(findings) != 1 || !strings.Contains(findings[0].Message, "requires status") {
		t.Fatalf("conflict check = %v, %v", findings, err)
	}
	if _, err := Render(root); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(conflictPath)
	if err != nil || string(got) != string(conflict) {
		t.Fatalf("state conflict silently reconciled: %q, %v", got, err)
	}
}

func TestCheckReportsInvalidUTF8WithoutGatingRender(t *testing.T) {
	root := t.TempDir()
	invalid := []byte("ordinary documentation\x00\xff\n")
	filename := filepath.Join(root, "docs", "ordinary.md")
	writeTestFile(t, filename, invalid, 0o644)
	if _, err := Render(root); err != nil {
		t.Fatal(err)
	}
	before := snapshotTree(t, root)
	findings, err := Check(root)
	if err != nil || len(findings) != 1 || findings[0].Path != "docs/ordinary.md" || !strings.Contains(findings[0].Message, "UTF-8") {
		t.Fatalf("invalid UTF-8 check = %v, %v", findings, err)
	}
	if after := snapshotTree(t, root); !reflect.DeepEqual(before, after) {
		t.Fatal("check changed authored bytes")
	}
}
