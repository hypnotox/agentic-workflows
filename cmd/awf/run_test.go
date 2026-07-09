package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// minimalYAML is a valid tree-config for a scaffolded fixture project.
const minimalYAML = `prefix: example
vars: {testCmd: go test ./..., gateCmd: make gate}
skills: [tdd]
agents: []
`

// scaffoldProject writes a minimal tree config under root and syncs it, leaving a
// drift-clean project. It returns root.
func scaffoldProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, minimalYAML)
	if err := runSync(root, io.Discard); err != nil {
		t.Fatalf("scaffold sync: %v", err)
	}
	return root
}

func TestRunNoArgs(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"awf"}, &out, &errb); code != 2 {
		t.Fatalf("expected exit 2 for no args, got %d", code)
	}
	if !strings.Contains(errb.String(), "usage:") {
		t.Errorf("missing usage text: %q", errb.String())
	}
}

func TestRunHelp(t *testing.T) {
	for _, arg := range []string{"help", "--help", "-h"} {
		var out, errb bytes.Buffer
		if code := run([]string{"awf", arg}, &out, &errb); code != 0 {
			t.Fatalf("%s: expected exit 0, got %d", arg, code)
		}
		if !strings.Contains(out.String(), "Commands:") || !strings.Contains(out.String(), "uninstall") {
			t.Errorf("%s: help text missing content:\n%s", arg, out.String())
		}
	}
}

func TestRunGetwdError(t *testing.T) {
	testsupport.SwapVar(t, &getwd, func() (string, error) { return "", errors.New("boom") })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "sync"}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 on getwd error, got %d", code)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	testsupport.SwapVar(t, &getwd, func() (string, error) { return t.TempDir(), nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "bogus"}, &out, &errb); code != 2 {
		t.Fatalf("expected exit 2 for unknown command, got %d", code)
	}
	if !strings.Contains(errb.String(), "unknown command") {
		t.Errorf("missing unknown-command text: %q", errb.String())
	}
}

func TestRunAddMissingSkillArg(t *testing.T) {
	testsupport.SwapVar(t, &getwd, func() (string, error) { return t.TempDir(), nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "add"}, &out, &errb); code != 2 {
		t.Fatalf("expected exit 2 for add without skill, got %d", code)
	}
}

func TestRunArgValidation(t *testing.T) {
	testsupport.SwapVar(t, &getwd, func() (string, error) { return t.TempDir(), nil })
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"unknown flag", []string{"awf", "check", "--bogus"}, "unknown flag"},
		{"unexpected positional", []string{"awf", "sync", "extra"}, "unexpected arguments"},
		{"value flag without value", []string{"awf", "audit", "--base"}, "needs a value"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			if code := run(c.args, &out, &errb); code != 2 {
				t.Fatalf("expected exit 2, got %d", code)
			}
			if !strings.Contains(errb.String(), c.want) {
				t.Errorf("missing %q in stderr: %q", c.want, errb.String())
			}
		})
	}
}

func TestRunDispatchError(t *testing.T) {
	// sync in a bare dir: project.Open fails -> handler error -> exit 1.
	testsupport.SwapVar(t, &getwd, func() (string, error) { return t.TempDir(), nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "sync"}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 on dispatch error, got %d", code)
	}
	if !strings.HasPrefix(errb.String(), "awf:") {
		t.Errorf("expected awf-prefixed error, got %q", errb.String())
	}
}

// TestRunDispatchArms drives every switch arm through run() against a scaffolded
// project, covering the dispatch statements.
func TestRunDispatchArms(t *testing.T) {
	for _, cmd := range []string{"sync", "check", "invariants", "list", "upgrade"} {
		t.Run(cmd, func(t *testing.T) {
			root := scaffoldProject(t)
			testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
			var out, errb bytes.Buffer
			if code := run([]string{"awf", cmd}, &out, &errb); code != 0 {
				t.Fatalf("%s: expected exit 0, got %d (%s)", cmd, code, errb.String())
			}
		})
	}
	t.Run("add", func(t *testing.T) {
		root := t.TempDir()
		awf := filepath.Join(root, ".awf")
		if err := os.MkdirAll(awf, 0o755); err != nil {
			t.Fatal(err)
		}
		// skills: [] so a fresh skill can be added.
		cfg := strings.Replace(minimalYAML, "skills: [tdd]", "skills: []", 1)
		if err := os.WriteFile(filepath.Join(awf, "config.yaml"), []byte(cfg), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := runSync(root, io.Discard); err != nil {
			t.Fatal(err)
		}
		testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
		var out, errb bytes.Buffer
		if code := run([]string{"awf", "add", "skill", "tdd"}, &out, &errb); code != 0 {
			t.Fatalf("add: expected exit 0, got %d (%s)", code, errb.String())
		}
	})
	t.Run("init", func(t *testing.T) {
		root := t.TempDir()
		testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
		var out, errb bytes.Buffer
		if code := run([]string{"awf", "init"}, &out, &errb); code != 0 {
			t.Fatalf("init: expected exit 0, got %d (%s)", code, errb.String())
		}
	})
}

// TestHandlersOnBareDirError covers each handler's project.Open error return.
func TestHandlersOnBareDirError(t *testing.T) {
	bare := func(t *testing.T) string { return t.TempDir() }
	t.Run("check", func(t *testing.T) {
		if err := runCheck(bare(t), io.Discard); err == nil {
			t.Error("expected Open error")
		}
	})
	t.Run("invariants", func(t *testing.T) {
		if err := runInvariants(bare(t), io.Discard); err == nil {
			t.Error("expected Open error")
		}
	})
	t.Run("list", func(t *testing.T) {
		if err := runList(bare(t), "", io.Discard); err == nil {
			t.Error("expected Open error")
		}
	})
	t.Run("new", func(t *testing.T) {
		if err := runNew(bare(t), "adr", []string{"x"}, io.Discard); err == nil {
			t.Error("expected Open error")
		}
	})
	t.Run("add", func(t *testing.T) {
		if err := runAdd(bare(t), "skill", "tdd", false, io.Discard); err == nil {
			t.Error("expected Open error")
		}
	})
	t.Run("remove", func(t *testing.T) {
		if err := runRemove(bare(t), "skill", "tdd", false, false, io.Discard); err == nil {
			t.Error("expected Open error")
		}
	})
}

func TestRunInvariantsReportsFindings(t *testing.T) {
	root := scaffoldProject(t)
	// An Implemented ADR with an unbacked slug -> a finding.
	adrDir := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	adr := testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
		testsupport.WithTitle("0001: X"), testsupport.WithBody("## Invariants\n- `inv: unbacked-here` — x.\n## Consequences\nc\n"))
	testsupport.WriteFile(t, filepath.Join(adrDir, "0001-x.md"), adr)
	if err := runSync(root, io.Discard); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runInvariants(root, &out); err == nil {
		t.Error("expected invariants failure for an unbacked slug")
	}
	if !strings.Contains(out.String(), "unbacked-here") {
		t.Errorf("expected the slug in output, got %q", out.String())
	}
}

func TestRunInvariantsCheckError(t *testing.T) {
	// A malformed ADR makes invariants.Check (via ParseDir) error.
	root := scaffoldProject(t)
	adrDir := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adrDir, "0001-x.md"), []byte("---\n: : bad yaml : :\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runInvariants(root, io.Discard); err == nil {
		t.Error("expected CheckInvariants error on a malformed ADR")
	}
}

func TestRunCheckErrorPaths(t *testing.T) {
	t.Run("stale-schema", func(t *testing.T) {
		root := t.TempDir()
		claude := filepath.Join(root, ".claude")
		if err := os.MkdirAll(claude, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(claude, "awf.yaml"), []byte("prefix: x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := runCheck(root, io.Discard); err == nil {
			t.Error("expected gate error on stale schema")
		}
	})
	t.Run("check-error-malformed-adr", func(t *testing.T) {
		// A malformed ADR makes p.Check() (ACTIVE.md generation) error.
		root := scaffoldProject(t)
		adrDir := filepath.Join(root, "docs", "decisions")
		if err := os.MkdirAll(adrDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(adrDir, "0001-x.md"), []byte("---\n: : bad yaml : :\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := runCheck(root, io.Discard); err == nil {
			t.Error("expected check error on a malformed ADR")
		}
	})
}

func TestRunListSidecarError(t *testing.T) {
	// A malformed sidecar for the enabled skill makes Sidecar() error.
	root := scaffoldProject(t)
	skillsDir := filepath.Join(root, ".awf", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "tdd.yaml"), []byte("data: [not, a, map]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runList(root, "", io.Discard); err == nil {
		t.Error("expected Sidecar parse error")
	}
}

func TestRunSyncSyncError(t *testing.T) {
	// A directory squatting on a rendered output path makes p.SyncReport() fail.
	root := scaffoldProject(t)
	out := filepath.Join(root, ".claude", "skills", "example-tdd", "SKILL.md")
	if err := os.RemoveAll(out); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(out, 0o755); err != nil { // SKILL.md is now a directory
		t.Fatal(err)
	}
	if err := runSync(root, io.Discard); err == nil {
		t.Error("expected Sync error when an output path is a directory")
	}
}

func TestRunInitSyncError(t *testing.T) {
	// Config exists (skip scaffold); a squatting output dir makes the inner
	// runSync fail, covering runInit's runSync error return.
	root := scaffoldProject(t)
	out := filepath.Join(root, ".claude", "skills", "example-tdd", "SKILL.md")
	if err := os.RemoveAll(out); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runInit(root, false, false, nil, "", io.Discard); err == nil {
		t.Error("expected runInit to surface the sync error")
	}
}

func TestRunUpgradeAppliesLegacyMigration(t *testing.T) {
	// A legacy single-file project migrates to the tree layout, covering the
	// applied-migrations loop and the terminal sync.
	root := t.TempDir()
	claude := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := "prefix: example\nvars:\n  testCmd: go test ./...\n  gateCmd: make gate\nskills: {}\nagents: {}\n"
	if err := os.WriteFile(filepath.Join(claude, "awf.yaml"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runUpgrade(root, &out); err != nil {
		t.Fatalf("runUpgrade legacy: %v", err)
	}
	if !strings.Contains(out.String(), "applied") {
		t.Errorf("expected an applied migration, got %q", out.String())
	}
}

// A schema-7 config the ADR-0081 closure validation refuses is repaired by
// awf upgrade: close-enabled-set closes the enabled set, then the terminal
// sync opens it cleanly.
func TestRunUpgradeRepairsUnclosedConfig(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nvars: {}\nskills: [brainstorming]\nagents: []\n")
	lock := &manifest.Lock{SchemaVersion: 7, Files: map[string]manifest.Entry{}}
	if err := lock.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	if err := runCheck(root, io.Discard); err == nil {
		t.Fatal("pre-upgrade check should refuse (schema gate)")
	}
	var out bytes.Buffer
	if err := runUpgrade(root, &out); err != nil {
		t.Fatalf("runUpgrade: %v", err)
	}
	if !strings.Contains(out.String(), `close-enabled-set: enabled skill "proposing-adr" (required by "brainstorming")`) {
		t.Errorf("expected closure additions printed, got %q", out.String())
	}
	if err := runCheck(root, io.Discard); err != nil {
		t.Errorf("post-upgrade check should pass, got %v", err)
	}
}

func TestRunUpgradeMigrationError(t *testing.T) {
	// A legacy config that fails to parse makes the migration error.
	root := t.TempDir()
	claude := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claude, "awf.yaml"), []byte(": : not valid : :\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runUpgrade(root, io.Discard); err == nil {
		t.Error("expected migration error for a malformed legacy config")
	}
}

// invariant: single-os-exit
func TestNoOsExitOutsideMain(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if !strings.Contains(line, "os.Exit") {
				continue
			}
			// The sole permitted os.Exit is main's one-line wrapper.
			if f == "main.go" && strings.Contains(line, "func main()") {
				continue
			}
			t.Errorf("%s:%d: os.Exit outside main's wrapper: %s", f, i+1, strings.TrimSpace(line))
		}
	}
}

func TestGateRejectsStaleSchema(t *testing.T) {
	// A legacy single-file layout (.claude/awf.yaml, no tree config) reports
	// generation 0 -> GateState "gate".
	root := t.TempDir()
	claude := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claude, "awf.yaml"), []byte("prefix: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := gate(root); err == nil {
		t.Fatal("expected gate to reject stale schema")
	}
	// runSync surfaces the same gate error.
	if err := runSync(root, io.Discard); err == nil {
		t.Error("expected runSync to fail on stale schema")
	}
}

func TestRunInitOnExistingConfigSkipsScaffold(t *testing.T) {
	// Pre-existing config -> scaffold branch is skipped; init still syncs.
	root := scaffoldProject(t)
	if err := runInit(root, false, false, nil, "", io.Discard); err != nil {
		t.Fatalf("runInit on existing config: %v", err)
	}
}

func TestRunInvariantsClean(t *testing.T) {
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runInvariants(root, &out); err != nil {
		t.Fatalf("runInvariants: %v", err)
	}
	if !strings.Contains(out.String(), "clean") {
		t.Errorf("expected clean output, got %q", out.String())
	}
}

func TestRunListPrintsSkills(t *testing.T) {
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runList(root, "", &out); err != nil {
		t.Fatalf("runList: %v", err)
	}
	if !strings.Contains(out.String(), "tdd") {
		t.Errorf("expected tdd in listing, got %q", out.String())
	}
}

func TestRunUpgradeAlreadyCurrent(t *testing.T) {
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runUpgrade(root, &out); err != nil {
		t.Fatalf("runUpgrade: %v", err)
	}
	if !strings.Contains(out.String(), "already current") {
		t.Errorf("expected already-current, got %q", out.String())
	}
}

// invariant: init-collision-guard
func TestInitGuardBlocksAndForceOverrides(t *testing.T) {
	root := t.TempDir()
	// A pre-existing, non-awf CLAUDE.md is a collision.
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init"}, &out, &errb); code == 0 {
		t.Fatal("expected init to fail on collision")
	}
	if !strings.Contains(errb.String(), "refusing to overwrite") {
		t.Fatalf("stderr = %q", errb.String())
	}
	// Nothing written: the scaffolded config tree was rolled back.
	if _, err := os.Stat(filepath.Join(root, ".awf", "config.yaml")); !os.IsNotExist(err) {
		t.Fatal("expected .awf to be rolled back")
	}
	if b, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md")); string(b) != "mine\n" {
		t.Fatalf("CLAUDE.md clobbered: %q", b)
	}
	// --force backs up the colliding file, then overwrites and completes.
	out.Reset()
	errb.Reset()
	if code := run([]string{"awf", "init", "--force"}, &out, &errb); code != 0 {
		t.Fatalf("init --force failed: %s", errb.String())
	}
	// The original is preserved at <path>.awf-bak.
	if b, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md.awf-bak")); string(b) != "mine\n" {
		t.Fatalf("CLAUDE.md.awf-bak = %q, want original %q", b, "mine\n")
	}
	// And the live file was overwritten with managed content.
	if b, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md")); string(b) == "mine\n" {
		t.Fatalf("CLAUDE.md should have been overwritten, still %q", b)
	}
	if !strings.Contains(out.String(), "backed up CLAUDE.md") {
		t.Errorf("expected backup report on stdout, got %q", out.String())
	}
	// Regression: init delegates its backup to the chained sync (one BackupFile path,
	// ADR-0035), so the colliding file is backed up exactly once — no double-backup.
	if _, err := os.Stat(filepath.Join(root, "CLAUDE.md.awf-bak.1")); !os.IsNotExist(err) {
		t.Error("expected exactly one backup; CLAUDE.md.awf-bak.1 should not exist")
	}
}

func TestInitRollbackPreservesExistingAwf(t *testing.T) {
	root := t.TempDir()
	// Pre-existing authored .awf/ content but no config.yaml -> init scaffolds config,
	// then a collision (non-managed CLAUDE.md) forces a refusal + rollback.
	part := filepath.Join(root, ".awf", "skills", "parts", "foo", "extra.md")
	if err := os.MkdirAll(filepath.Dir(part), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(part, []byte("hand-authored\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runInit(root, false, false, nil, "", io.Discard); err == nil {
		t.Fatal("expected init to refuse on collision")
	}
	// The scaffolded config.yaml is rolled back...
	if _, err := os.Stat(filepath.Join(root, ".awf", "config.yaml")); !os.IsNotExist(err) {
		t.Error("config.yaml should have been removed on rollback")
	}
	// ...but the pre-existing authored content survives.
	if _, err := os.Stat(part); err != nil {
		t.Errorf("pre-existing .awf content must be preserved, got: %v", err)
	}
}

func TestInitForceBackupDoesNotClobberPriorBak(t *testing.T) {
	root := t.TempDir()
	// A colliding CLAUDE.md plus a pre-existing backup from an earlier --force.
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md.awf-bak"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init", "--force"}, &out, &errb); code != 0 {
		t.Fatalf("init --force: %s", errb.String())
	}
	if b, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md.awf-bak")); string(b) != "v1\n" {
		t.Errorf("prior .awf-bak clobbered: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md.awf-bak.1")); string(b) != "v2\n" {
		t.Errorf("CLAUDE.md.awf-bak.1 = %q, want v2", b)
	}
}

func TestInitIdempotentReinitNoCollision(t *testing.T) {
	root := scaffoldProject(t)
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init"}, &out, &errb); code != 0 {
		t.Fatalf("first init failed: %s", errb.String())
	}
	// Re-init over the now-managed tree: every planned path is in the prior lock,
	// so p.InitCollisions skips them all and init proceeds without --force.
	out.Reset()
	errb.Reset()
	if code := run([]string{"awf", "init"}, &out, &errb); code != 0 {
		t.Fatalf("re-init failed: %s", errb.String())
	}
}

func TestInitCollisionsOpenError(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".awf")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Unknown field → strict config.Load fails → project.Open errors inside runInit.
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("bogusField: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init"}, &out, &errb); code == 0 {
		t.Fatal("expected init to fail when project.Open errors")
	}
	// --force skips the probe, so the same malformed config now fails at
	// runInit's own post-scaffold project.Open — keeping that branch covered.
	if code := run([]string{"awf", "init", "--force"}, &out, &errb); code == 0 {
		t.Fatal("expected init --force to fail when project.Open errors")
	}
}

func TestInitAbortsWhenInitCollisionsFails(t *testing.T) {
	root := t.TempDir()
	// A malformed ADR makes PlannedOutputs (inside p.InitCollisions) fail after the
	// config is scaffolded, exercising runInit's collision-scan error forward.
	dd := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(dd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dd, "0099-bad.md"), []byte("---\nstatus: [unclosed\n---\n# Bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init"}, &out, &errb); code == 0 {
		t.Fatal("expected init to fail when p.InitCollisions errors")
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
	if err := os.WriteFile(filepath.Join(adrDir, "ACTIVE.md"), []byte("hand index\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "sync"}, &out, &errb); code != 0 {
		t.Fatalf("sync: %s", errb.String())
	}
	if !strings.Contains(out.String(), "backed up docs/decisions/ACTIVE.md") {
		t.Errorf("missing backup line: %q", out.String())
	}
	if !strings.Contains(out.String(), "note: awf now generates") {
		t.Errorf("missing ownership-takeover note: %q", out.String())
	}
}

// A collision refuses BEFORE any prompt: with a colliding AGENTS.md and an
// interactive stdin, init exits without emitting a single prompt line and
// without creating .awf/.
func TestInitCollisionProbeRefusesBeforePrompts(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	testsupport.SwapVar(t, &isInteractive, func() bool { return true })
	testsupport.SwapVar(t, &stdin, io.Reader(strings.NewReader("SHOULD-NOT-BE-CONSUMED\n")))
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init"}, &out, &errb); code == 0 {
		t.Fatal("expected init to refuse on collision")
	}
	if !strings.Contains(errb.String(), "refusing to overwrite") {
		t.Fatalf("stderr = %q", errb.String())
	}
	if out.String() != "" {
		t.Errorf("prompt text emitted before the collision refusal:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".awf")); !os.IsNotExist(err) {
		t.Errorf(".awf/ should not exist after a probe refusal (err=%v)", err)
	}
}

// A trim answer can enable a non-core artifact the curated-core probe set does
// not cover: the probe passes, and the accurate post-answer check still
// refuses and rolls the scaffolded config back. The trim carries the full core
// alongside tdd — until ADR-0081 Phase 4 derives init's agents from the trim,
// any selection excluding reviewing-plan-resync fails closure validation
// (the always-enabled plan-reviewer requires it).
func TestInitPostAnswerCollisionAfterProbePasses(t *testing.T) {
	root := t.TempDir()
	skillPath := filepath.Join(root, ".claude", "skills", filepath.Base(root)+"-tdd", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skillPath, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	forceNonInteractive(t)
	const coreAndTdd = "skills=adr-lifecycle,brainstorming,executing-plans,proposing-adr,retrospective,reviewing-adr,reviewing-impl,reviewing-plan,reviewing-plan-resync,subagent-driven-development,writing-plans,tdd"
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init", "--set", coreAndTdd}, &out, &errb); code == 0 {
		t.Fatal("expected init to refuse on the post-answer collision")
	}
	if !strings.Contains(errb.String(), "refusing to overwrite") {
		t.Fatalf("stderr = %q", errb.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "config.yaml")); !os.IsNotExist(err) {
		t.Error("scaffolded config should have been rolled back")
	}
}
