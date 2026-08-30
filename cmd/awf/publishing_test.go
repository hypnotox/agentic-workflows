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
	out := filepath.Join(root, ".claude", "skills", "example-brainstorming", "SKILL.md")
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

func TestSyncReportsIndexOwnershipTakeover(t *testing.T) {
	root := t.TempDir()
	awf := filepath.Join(root, ".awf")
	if err := os.MkdirAll(awf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(awf, "config.yaml"), []byte(minimalYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	// Foreign ADR index present before any sync (no lock yet).
	adrDir := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adrDir, "INDEX.md"), []byte("hand index\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (&manifest.Lock{AWFVersion: project.Version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{"prior": {}}}).Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := runFrom(root, []string{"awf", "render"}, &out, &errb); code != 0 {
		t.Fatalf("sync: %s", errb.String())
	}
	const indexTakeoverPrefix = "status: completed\n\nmutation:\n  changes:\n    backups:\n      docs/decisions/INDEX.md to docs/decisions/INDEX.md.awf-bak\n    outputs:\n"
	if !strings.HasPrefix(out.String(), indexTakeoverPrefix) {
		t.Errorf("index takeover lost its backup report:\n%s", out.String())
	}
	for _, want := range []string{
		"added .claude/agents/implementer.md",
		"added .claude/skills/example-brainstorming/SKILL.md",
		"added .pi/agents/implementer.md",
		"added .pi/skills/example-brainstorming/SKILL.md",
		"added docs/architecture.md",
		"added docs/pitfalls.md",
		"awf now generates docs/decisions/INDEX.md",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("index takeover full-catalog output missing %q:\n%s", want, out.String())
		}
	}
}
