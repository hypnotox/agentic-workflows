package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func writeMalformedPitfall(t *testing.T, root string) {
	t.Helper()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "docs", "pitfalls", "bad.md"), "malformed source\n")
}

func TestRunSyncSyncError(t *testing.T) {
	ctx := testContext(t)
	// A directory squatting on a rendered output path makes p.SyncReport() fail.
	root := scaffoldProject(t)
	out := filepath.Join(root, ".claude", "skills", "awf-maintenance", "SKILL.md")
	if err := os.RemoveAll(out); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(out, 0o755); err != nil { // SKILL.md is now a directory
		t.Fatal(err)
	}
	if err := runSync(ctx, root, io.Discard); err == nil {
		t.Error("expected Sync error when an output path is a directory")
	}
}

func TestSyncPreservesUnmanagedDecisionIndex(t *testing.T) {
	root := t.TempDir()
	awf := filepath.Join(root, ".awf")
	if err := os.MkdirAll(awf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(awf, "config.yaml"), []byte(minimalYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	adrDir := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(adrDir, "INDEX.md")
	if err := os.WriteFile(index, []byte("hand index\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (&manifest.Lock{AWFVersion: project.Version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{"prior": {}}}).Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := runFrom(root, []string{"awf", "render"}, &out, &errb); code != 0 {
		t.Fatalf("sync: %s", errb.String())
	}
	got, err := os.ReadFile(index)
	if err != nil || string(got) != "hand index\n" {
		t.Fatalf("unmanaged decision index changed: %q, %v", got, err)
	}
	if strings.Contains(out.String(), "docs/decisions/INDEX.md") {
		t.Fatalf("render reported unmanaged decision index activity:\n%s", out.String())
	}
	if _, err := os.Stat(index + ".awf-bak"); !os.IsNotExist(err) {
		t.Fatalf("render backed up an unmanaged decision index: %v", err)
	}
	for _, want := range []string{
		"added .claude/skills/awf-decisions/SKILL.md",
		"added .claude/skills/awf-effort/SKILL.md",
		"added .claude/skills/awf-maintenance/SKILL.md",
		"added .claude/skills/awf-topics/SKILL.md",
		"added .pi/skills/awf-decisions/SKILL.md",
		"added .pi/skills/awf-effort/SKILL.md",
		"added .pi/skills/awf-maintenance/SKILL.md",
		"added .pi/skills/awf-topics/SKILL.md",
		"added docs/architecture.md",
		"added docs/pitfalls.md",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("full-catalog output missing %q:\n%s", want, out.String())
		}
	}
}
