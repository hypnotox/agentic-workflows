package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// minimalYAML is a valid tree-config for a scaffolded fixture project.
const minimalYAML = `prefix: example
vars: {testCmd: go test ./..., gateCmd: make gate}
skills: [tdd]
agents: []
hooks: []
`

// scaffoldProject writes a minimal tree config under root and syncs it, leaving a
// drift-clean project. It returns root.
func scaffoldProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	awf := filepath.Join(root, ".claude", "awf")
	if err := os.MkdirAll(awf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(awf, "config.yaml"), []byte(minimalYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runSync(root, io.Discard); err != nil {
		t.Fatalf("scaffold sync: %v", err)
	}
	return root
}

// swapGetwd overrides the package getwd seam for the duration of a test.
func swapGetwd(t *testing.T, fn func() (string, error)) {
	t.Helper()
	orig := getwd
	getwd = fn
	t.Cleanup(func() { getwd = orig })
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

func TestRunGetwdError(t *testing.T) {
	swapGetwd(t, func() (string, error) { return "", errors.New("boom") })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "sync"}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 on getwd error, got %d", code)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	swapGetwd(t, func() (string, error) { return t.TempDir(), nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "bogus"}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 for unknown command, got %d", code)
	}
	if !strings.Contains(errb.String(), "unknown command") {
		t.Errorf("missing unknown-command text: %q", errb.String())
	}
}

func TestRunAddMissingSkillArg(t *testing.T) {
	swapGetwd(t, func() (string, error) { return t.TempDir(), nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "add"}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 for add without skill, got %d", code)
	}
}

func TestRunDispatchError(t *testing.T) {
	// sync in a bare dir: project.Open fails -> handler error -> exit 1.
	swapGetwd(t, func() (string, error) { return t.TempDir(), nil })
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
			swapGetwd(t, func() (string, error) { return root, nil })
			var out, errb bytes.Buffer
			if code := run([]string{"awf", cmd}, &out, &errb); code != 0 {
				t.Fatalf("%s: expected exit 0, got %d (%s)", cmd, code, errb.String())
			}
		})
	}
	t.Run("add", func(t *testing.T) {
		root := t.TempDir()
		awf := filepath.Join(root, ".claude", "awf")
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
		swapGetwd(t, func() (string, error) { return root, nil })
		var out, errb bytes.Buffer
		if code := run([]string{"awf", "add", "tdd"}, &out, &errb); code != 0 {
			t.Fatalf("add: expected exit 0, got %d (%s)", code, errb.String())
		}
	})
	t.Run("init", func(t *testing.T) {
		root := t.TempDir()
		swapGetwd(t, func() (string, error) { return root, nil })
		var out, errb bytes.Buffer
		if code := run([]string{"awf", "init"}, &out, &errb); code != 0 {
			t.Fatalf("init: expected exit 0, got %d (%s)", code, errb.String())
		}
	})
	t.Run("setup", func(t *testing.T) {
		root := scaffoldProject(t)
		if err := os.MkdirAll(filepath.Join(root, ".githooks"), 0o755); err != nil {
			t.Fatal(err)
		}
		swapGetwd(t, func() (string, error) { return root, nil })
		var out, errb bytes.Buffer
		// Not a git repo -> setup warns and is a no-op -> exit 0.
		if code := run([]string{"awf", "setup"}, &out, &errb); code != 0 {
			t.Fatalf("setup: expected exit 0, got %d (%s)", code, errb.String())
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
		if err := runList(bare(t), io.Discard); err == nil {
			t.Error("expected Open error")
		}
	})
	t.Run("add", func(t *testing.T) {
		if err := runAdd(bare(t), "tdd", io.Discard); err == nil {
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
	adr := "---\nstatus: Implemented\ndate: 2026-06-25\ntags: [x]\n---\n# ADR-0001: X\n## Invariants\n- `inv: unbacked-here` — x.\n## Consequences\nc\n"
	if err := os.WriteFile(filepath.Join(adrDir, "0001-x.md"), []byte(adr), 0o644); err != nil {
		t.Fatal(err)
	}
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
	skillsDir := filepath.Join(root, ".claude", "awf", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "tdd.yaml"), []byte("data: [not, a, map]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runList(root, io.Discard); err == nil {
		t.Error("expected Sidecar parse error")
	}
}

func TestRunSyncSyncError(t *testing.T) {
	// A directory squatting on a rendered output path makes p.Sync() fail.
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
	if err := runInit(root, false, io.Discard, io.Discard); err == nil {
		t.Error("expected runInit to surface the sync error")
	}
}

func TestRunSetupGitConfigError(t *testing.T) {
	// .git is a file (not a dir) so it exists for the Stat guard but `git config`
	// fails — covering runSetup's cmd.Run error branch without permission tricks.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".githooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("not a repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runSetup(root, io.Discard, io.Discard); err == nil {
		t.Error("expected git config to fail when .git is not a repository")
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
	legacy := "prefix: example\nvars:\n  testCmd: go test ./...\n  gateCmd: make gate\nskills: {}\nagents: {}\nhooks: []\n"
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
	if err := runInit(root, false, io.Discard, io.Discard); err != nil {
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
	if err := runList(root, &out); err != nil {
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

func TestAppendSkill(t *testing.T) {
	cases := map[string]struct {
		src     string
		wantErr bool
		wantSub string
	}{
		"empty-array": {src: "prefix: x\nskills: []\n", wantSub: "  - tdd"},
		"bare-key":    {src: "prefix: x\nskills:\n", wantSub: "  - tdd"},
		"no-key":      {src: "prefix: x\nagents: []\n", wantErr: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := appendSkill(tc.src, "tdd")
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(got, tc.wantSub) {
				t.Errorf("missing %q in:\n%s", tc.wantSub, got)
			}
		})
	}
}

func TestRunAddAppendSkillError(t *testing.T) {
	// A config that parses but has no skills: key -> appendSkill error path.
	root := t.TempDir()
	awf := filepath.Join(root, ".claude", "awf")
	if err := os.MkdirAll(awf, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "prefix: example\nagents: []\nhooks: []\n"
	if err := os.WriteFile(filepath.Join(awf, "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runSync(root, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := runAdd(root, "tdd", io.Discard); err == nil {
		t.Error("expected appendSkill error for a config with no skills: key")
	}
}

// invariant: init-collision-guard
func TestInitGuardBlocksAndForceOverrides(t *testing.T) {
	root := t.TempDir()
	// A pre-existing, non-awf CLAUDE.md is a collision.
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	swapGetwd(t, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init"}, &out, &errb); code == 0 {
		t.Fatal("expected init to fail on collision")
	}
	if !strings.Contains(errb.String(), "refusing to overwrite") {
		t.Fatalf("stderr = %q", errb.String())
	}
	// Nothing written: the scaffolded config tree was rolled back.
	if _, err := os.Stat(filepath.Join(root, ".claude", "awf", "config.yaml")); !os.IsNotExist(err) {
		t.Fatal("expected .claude/awf to be rolled back")
	}
	if b, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md")); string(b) != "mine\n" {
		t.Fatalf("CLAUDE.md clobbered: %q", b)
	}
	// --force overwrites and completes.
	out.Reset()
	errb.Reset()
	if code := run([]string{"awf", "init", "--force"}, &out, &errb); code != 0 {
		t.Fatalf("init --force failed: %s", errb.String())
	}
}

func TestInitIdempotentReinitNoCollision(t *testing.T) {
	root := scaffoldProject(t)
	swapGetwd(t, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init"}, &out, &errb); code != 0 {
		t.Fatalf("first init failed: %s", errb.String())
	}
	// Re-init over the now-managed tree: every planned path is in the prior lock,
	// so initCollisions skips them all and init proceeds without --force.
	out.Reset()
	errb.Reset()
	if code := run([]string{"awf", "init"}, &out, &errb); code != 0 {
		t.Fatalf("re-init failed: %s", errb.String())
	}
}

func TestInitCollisionsOpenError(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".claude", "awf")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Unknown field → strict config.Load fails → project.Open errors.
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("bogusField: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := initCollisions(root); err == nil {
		t.Fatal("expected initCollisions to surface the Open error")
	}
}

func TestInitCollisionsPlannedOutputsError(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".claude", "awf")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"),
		[]byte("prefix: awf\nskills: []\nagents: []\nhooks: []\ndocs: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Open succeeds (ADRs are not parsed at Open); the malformed ADR fails
	// generateActiveMD inside PlannedOutputs.
	dd := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(dd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dd, "0099-bad.md"), []byte("---\nstatus: [unclosed\n---\n# Bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := initCollisions(root); err == nil {
		t.Fatal("expected initCollisions to surface the PlannedOutputs error")
	}
}

func TestInitAbortsWhenInitCollisionsFails(t *testing.T) {
	root := t.TempDir()
	// A malformed ADR makes PlannedOutputs (inside initCollisions) fail after the
	// config is scaffolded, exercising runInit's initCollisions error forward.
	dd := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(dd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dd, "0099-bad.md"), []byte("---\nstatus: [unclosed\n---\n# Bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	swapGetwd(t, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init"}, &out, &errb); code == 0 {
		t.Fatal("expected init to fail when initCollisions errors")
	}
}
