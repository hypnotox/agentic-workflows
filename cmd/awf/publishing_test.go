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
	out := filepath.Join(root, ".claude", "skills", "example-tdd", "SKILL.md")
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
	if err := (&manifest.Lock{AWFVersion: project.Version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}).Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "render"}, &out, &errb); code != 0 {
		t.Fatalf("sync: %s", errb.String())
	}
	const indexTakeoverOutput = "status: completed\n\nmutation:\n  changes:\n    backups:\n      docs/decisions/INDEX.md to docs/decisions/INDEX.md.awf-bak\n    outputs:\n      added .awf/efforts/.gitignore\n      added .awf/hooks/commit-msg.sh\n      added .awf/hooks/pre-commit.sh\n      added .awf/hooks/pre-merge-commit.sh\n      added .awf/hooks/pre-push.sh\n      added .awf/hooks/reference-transaction.sh\n      added .awf/worktrees/.gitignore\n      added .claude/skills/example-tdd/SKILL.md\n      added AGENTS.md\n      added CLAUDE.md\n      added awf\n      added docs/agents-md-standard.md\n      added docs/config-reference.md\n      added docs/decisions/INDEX.md\n      added docs/decisions/README.md\n      added docs/decisions/template.md\n      added docs/doc-standard.md\n      added docs/maintainable-code-design.md\n      added docs/plans/README.md\n      added docs/plans/template.md\n      added docs/workflow.md\n      added docs/working-with-awf.md\n  notes:\n    awf now generates docs/decisions/INDEX.md; retire any external generator for it\n  next actions:\n    step 1: continue with the rendered project state\n"
	if !strings.HasPrefix(out.String(), strings.Split(indexTakeoverOutput, "    outputs:\n")[0]) {
		t.Errorf("index takeover lost its backup report:\n%s", out.String())
	}
	for _, want := range []string{
		"added .claude/agents/implementer.md",
		"added .claude/skills/example-tdd/SKILL.md",
		"added .pi/agents/implementer.md",
		"added .pi/skills/example-tdd/SKILL.md",
		"added docs/architecture.md",
		"added docs/pitfalls.md",
		"awf now generates docs/decisions/INDEX.md",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("index takeover full-catalog output missing %q:\n%s", want, out.String())
		}
	}
}
