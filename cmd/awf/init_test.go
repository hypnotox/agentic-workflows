package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/initspec"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// readConfig returns the scaffolded .awf/config.yaml under root.
func readInitConfig(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	return string(b)
}

// TestInitDescribeReadOnly asserts `awf init --describe` prints the descriptor
// schema as JSON and writes nothing (no .awf/ created).
// invariant: tooling/cli:init-describe-read-only (TestInitDescribeReadOnly)
// invariant: tooling/cli:init-describe-read-only (TestInitDescribeReadOnly)
func TestWriteInitDescriptorProtocolBytesAndErrors(t *testing.T) {
	const payload = `{"descriptors":["x"]}`
	var out bytes.Buffer
	if err := writeInitDescriptorProtocol(&out, []byte(payload+"\n")); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), payload+"\n"; got != want {
		t.Fatalf("descriptor protocol = %q, want exact %q", got, want)
	}
	if err := writeInitDescriptorProtocol(errorWriter{}, []byte(payload)); err == nil {
		t.Fatal("descriptor write error was not propagated")
	}
}

func TestInitDescribeReadOnly(t *testing.T) {
	sourceRoot := testsupport.RepoRoot(t)
	root := t.TempDir()
	var out, errb bytes.Buffer
	if code := runFrom(root, []string{"awf", "init", "--describe"}, &out, &errb); code != 0 {
		t.Fatalf("init --describe: exit %d (%s)", code, errb.String())
	}
	want, err := os.ReadFile(filepath.Join(sourceRoot, "cmd", "awf", "testdata", "init-describe.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), want) || errb.Len() != 0 {
		t.Fatalf("init --describe streams stdout=%q stderr=%q, want exact checked-in protocol", out.String(), errb.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".awf")); !os.IsNotExist(err) {
		t.Errorf(".awf/ should not exist after --describe (err=%v)", err)
	}
}

// TestInitExplicitAnswersWin asserts a --set value lands in the scaffolded config.
// invariant: tooling/init-and-enablement:explicit-answers-win (TestInitExplicitAnswersWin)
func TestInitExplicitAnswersWin(t *testing.T) {
	root := t.TempDir()
	var out, errb bytes.Buffer
	if code := runFrom(root, []string{"awf", "init", "--set", "gateCmd=make gate"}, &out, &errb); code != 0 {
		t.Fatalf("init --set: exit %d (%s)", code, errb.String())
	}
	if cfg := readInitConfig(t, root); !strings.Contains(cfg, "gateCmd: make gate") {
		t.Errorf("config missing gateCmd override:\n%s", cfg)
	}
}

// TestInitNonInteractiveDefault asserts the silent (non-TTY, no-answers) path
// seeds every var empty and writes no invariants config - byte-identical to the
// pre-feature seed-empty output.
// invariant: tooling/init-and-enablement:init-noninteractive-default (TestInitNonInteractiveDefault)
// invariant: tooling/init-and-enablement:init-profile-default-core (TestInitNonInteractiveDefault)
func TestInitNonInteractiveDefault(t *testing.T) {
	root := t.TempDir()
	var out, errb bytes.Buffer
	if code := runFrom(root, []string{"awf", "init"}, &out, &errb); code != 0 {
		t.Fatalf("init: exit %d (%s)", code, errb.String())
	}
	cfg := readInitConfig(t, root)
	if !strings.Contains(cfg, `gateCmd: ""`) {
		t.Errorf("expected gateCmd seeded empty:\n%s", cfg)
	}
	if strings.Contains(cfg, "invariants:") {
		t.Errorf("silent init should not write an invariants config:\n%s", cfg)
	}
	if !strings.Contains(cfg, "profile: core") {
		t.Errorf("silent init did not select Core by default:\n%s", cfg)
	}
}

// TestInitAnswersFile asserts values come from a JSON answers file.
func TestInitAnswersFile(t *testing.T) {
	root := t.TempDir()
	ans := filepath.Join(t.TempDir(), "answers.json")
	if err := os.WriteFile(ans, []byte(`{"testCmd":"go test ./..."}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := runFrom(root, []string{"awf", "init", "--answers", ans}, &out, &errb); code != 0 {
		t.Fatalf("init --answers: exit %d (%s)", code, errb.String())
	}
	if cfg := readInitConfig(t, root); !strings.Contains(cfg, "testCmd: go test ./...") {
		t.Errorf("config missing testCmd from answers file:\n%s", cfg)
	}
}

func TestInitErrorPaths(t *testing.T) {
	cases := []struct {
		name string
		args []string
		pre  func(root string) []string // optional: returns extra args after creating files
	}{
		{name: "bad --set", args: []string{"awf", "init", "--set", "noequals"}},
		{name: "missing --answers file", args: []string{"awf", "init", "--answers", "/nonexistent/answers.json"}},
		{name: "retired selection answer", args: []string{"awf", "init", "--set", "skills=nonexistent-skill"}},
		{name: "non-map answers", pre: func(root string) []string {
			f := filepath.Join(root, "bad.yaml")
			_ = os.WriteFile(f, []byte("- a\n- b\n"), 0o644)
			return []string{"awf", "init", "--answers", f}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			args := tc.args
			if tc.pre != nil {
				args = tc.pre(root)
			}
			var out, errb bytes.Buffer
			if code := runFrom(root, args, &out, &errb); code == 0 {
				t.Fatalf("expected non-zero exit for %s, got 0", tc.name)
			}
		})
	}
}

// TestInitInteractivePromptWiring exercises the interactive path end to end: with
// isInteractive forced true and the stdin seam stubbed, awf init reads a prompted
// value and writes it to the scaffolded config. This is the only test that drives
// the run -> runInit -> initspec.Resolve prompt wiring through the stdin package
// var; every other writing test forces non-interactive.
func TestInitInteractivePromptWiring(t *testing.T) {
	root := t.TempDir()
	// gateCmd reads the first value; every later prompt hits EOF and takes its
	// empty default, so the invariants marker/globs stay unset.
	var out, errb bytes.Buffer
	if code := newRunner(func() (string, error) { return root, nil }, strings.NewReader("core\nmake gate\n"), func() bool { return true }).run([]string{"awf", "init"}, &out, &errb); code != 0 {
		t.Fatalf("interactive init: exit %d (%s)", code, errb.String())
	}
	cfg := readInitConfig(t, root)
	if !strings.Contains(cfg, "gateCmd: make gate") {
		t.Errorf("config missing prompted gateCmd:\n%s", cfg)
	}
	if strings.Contains(cfg, "invariants:") {
		t.Errorf("empty marker/globs prompts should write no invariants config:\n%s", cfg)
	}
}

// awf init over an existing config must not prompt for descriptor answers it
// then discards - the config is kept, init says so, and only the sync runs.
func TestInitExistingConfigSkipsPrompts(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: ex\nintegrationBranch: main\nvars:\n  gateCmd: make gate\n")
	if err := initializeProject(testContext(t), root, io.Discard); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	r := newRunner(func() (string, error) { return root, nil }, strings.NewReader("answered-one\nanswered-two\nanswered-three\n"), func() bool { return true })
	if code := r.run([]string{"awf", "init"}, &out, &errb); code != 0 {
		t.Fatalf("init over existing config: exit %d (%s)", code, errb.String())
	}
	if strings.Contains(out.String(), "gateCmd:") {
		t.Errorf("init prompted for descriptors it cannot apply:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "config action: kept and re-rendered") {
		t.Errorf("init did not say the existing config is kept:\n%s", out.String())
	}
	if cfg := readInitConfig(t, root); strings.Contains(cfg, "answered") {
		t.Errorf("existing config was modified:\n%s", cfg)
	}
	// Explicit answers against an existing config are surfaced as ignored.
	var out2 bytes.Buffer
	if code := r.run([]string{"awf", "init", "--set", "gateCmd=make gate"}, &out2, &errb); code != 0 {
		t.Fatalf("init --set over existing config: exit %d (%s)", code, errb.String())
	}
	if !strings.Contains(out2.String(), "ignored") {
		t.Errorf("init did not note that --set answers were ignored:\n%s", out2.String())
	}
}

// TestStdinIsInteractive exercises the concrete process predicate (the result
// depends on whether the test's stdin is a terminal; it must not panic).
func TestStdinIsInteractive(t *testing.T) {
	t.Logf("stdinIsInteractive() = %v", stdinIsInteractive(os.Stdin)())
}

// A commitScopes answer lands in audit.allowedScopes, never in vars; the
// silent default writes no audit block (ADR-0051).
func TestInitCommitScopesAnswer(t *testing.T) {
	root := t.TempDir()
	var out, errb bytes.Buffer
	if code := runFrom(root, []string{"awf", "init", "--set", "commitScopes=adr, awf"}, &out, &errb); code != 0 {
		t.Fatalf("init --set commitScopes: exit %d (%s)", code, errb.String())
	}
	cfg := readInitConfig(t, root)
	for _, want := range []string{"audit:", "allowedScopes:", "- adr", "- awf"} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config missing %q:\n%s", want, cfg)
		}
	}
	if strings.Contains(cfg, "commitScopes:") {
		t.Errorf("commitScopes must not be seeded as a var:\n%s", cfg)
	}
}

// After the chained sync succeeds, init presents render-completeness notes
// and each fixed next action as its own ordered step.
type nthInitErrorWriter struct {
	writes int
	failAt int
}

func (w *nthInitErrorWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == w.failAt {
		return 0, errors.New("write failed")
	}
	return len(p), nil
}

func TestInitProjectLoaderPropagatesFailure(t *testing.T) {
	want := errors.New("loader failed")
	err := runInitWithProjectLoader(testContext(t), scaffoldProject(t), true, false, nil, "", strings.NewReader(""), false, io.Discard, func(string) (*project.Loader, error) {
		return nil, want
	}, func(context.Context, string) error { return nil })
	if !errors.Is(err, want) {
		t.Fatalf("loader error = %v, want %v", err, want)
	}
}

func TestInitSyncFailureKeepsExistingAuthorityAndPresentsPartialOutcome(t *testing.T) {
	root := scaffoldProject(t)
	configPath := config.ConfigPath(root)
	lockPath := config.LockPath(root)
	beforeConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeLock, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	preparePublicSyncLaterFailure(t, root)

	var out bytes.Buffer
	err = runInit(testContext(t), root, true, false, nil, "", &out)
	if err == nil {
		t.Fatal("init accepted a later sync failure")
	}
	if got := out.String(); !strings.Contains(got, "status: initialization partially committed") || !strings.Contains(got, "output-replaced AGENTS.md") || !strings.Contains(got, "recovery:") {
		t.Fatalf("init stdout = %q, want complete partial outcome", got)
	}
	if got, readErr := os.ReadFile(configPath); readErr != nil || !bytes.Equal(got, beforeConfig) {
		t.Fatalf("config after sync failure = %q, read error = %v", got, readErr)
	}
	if got, readErr := os.ReadFile(lockPath); readErr != nil || !bytes.Equal(got, beforeLock) {
		t.Fatalf("lock after sync failure = %q, read error = %v", got, readErr)
	}
	info, statErr := os.Stat(filepath.Join(root, "AGENTS.md"))
	if statErr != nil {
		t.Fatal(statErr)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o644); got != want {
		t.Fatalf("AGENTS.md mode = %o, want earlier correction committed as %o", got, want)
	}
}

func TestRenderInitOutcomePropagatesFailures(t *testing.T) {
	if err := renderInitOutcome(initspec.Outcome{ConfigPath: "bad\npath"}, io.Discard); err == nil {
		t.Fatal("invalid outcome accepted")
	}
	if err := renderInitOutcome(initspec.Outcome{ConfigPath: "config"}, errorWriter{}); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("writer error = %v", err)
	}
}

func TestInitPropagatesOrdinaryPresentationWriteFailures(t *testing.T) {
	for _, test := range []struct {
		name   string
		root   func(*testing.T) string
		sets   []string
		failAt int
	}{
		{name: "existing config outcome", root: scaffoldProject, failAt: 1},
		{name: "ignored answers outcome", root: scaffoldProject, sets: []string{"gateCmd=go test ./..."}, failAt: 1},
		{name: "scaffold outcome", root: (*testing.T).TempDir, failAt: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			writer := &nthInitErrorWriter{failAt: test.failAt}
			err := runInit(testContext(t), test.root(t), true, false, test.sets, "", writer)
			if err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("write error = %v after %d writes", err, writer.writes)
			}
		})
	}
}

func TestInitPrintsNotesAndNextSteps(t *testing.T) {
	root := t.TempDir()
	var out, errb bytes.Buffer
	if code := runFrom(root, []string{"awf", "init"}, &out, &errb); code != 0 {
		t.Fatalf("init: exit %d (%s)", code, errb.String())
	}
	for _, want := range []string{
		"status: initialization completed",
		"references unset vars",
		"next actions:",
		"step 1: continue with the rendered project state",
		"step 2: fill the Identity section at .awf/parts/agents-doc/identity.md, then run awf render",
		"step 3: set still-empty vars in .awf/config.yaml (the notes above list what each artifact misses), then run awf render",
		"step 4: wire rendered hook payloads under .awf/hooks/ into git hooks you own (see the workflow doc's local-hooks section); awf never activates hooks itself",
		"step 5: commit .awf/ and the rendered files together",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("init output missing %q:\n%s", want, out.String())
		}
	}
}

func TestInitRejectsAmbiguousBrownfieldAuthority(t *testing.T) {
	ctx := testContext(t)
	for _, tc := range []struct {
		name  string
		files map[string]string
	}{
		{"malformed", map[string]string{"docs/decisions/0001-bad.md": "---\nstatus: [bad\n---\n"}},
		{"duplicate", map[string]string{
			"docs/decisions/0001-one.md": testsupport.ADR("Accepted", testsupport.WithDate("2026-07-21"), testsupport.WithTitle("0001: One")),
			"docs/decisions/0001-two.md": testsupport.ADR("Accepted", testsupport.WithDate("2026-07-21"), testsupport.WithTitle("0001: Two")),
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for path, body := range tc.files {
				testsupport.WriteFile(t, filepath.Join(root, path), body)
			}
			before := snapshotTree(t, root)
			var out bytes.Buffer
			if err := runInit(ctx, root, false, false, nil, "", &out); err == nil {
				t.Fatal("expected refusal")
			}
			if after := snapshotTree(t, root); after != before {
				t.Fatal("ambiguous first adoption mutated the repository tree")
			}
			if out.Len() != 0 {
				t.Fatalf("ambiguous first adoption wrote output: %q", out.String())
			}
		})
	}
}

func TestInitCollisionProbeOpenError(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: [bad\n")
	if err := runInit(testContext(t), root, false, false, nil, "", io.Discard); err == nil {
		t.Fatal("expected init probe error")
	}
}

func TestRunInitOnExistingConfigSkipsScaffold(t *testing.T) {
	ctx := testContext(t)
	// Pre-existing config -> scaffold branch is skipped; init still syncs.
	root := scaffoldProject(t)
	if err := runInit(ctx, root, false, false, nil, "", io.Discard); err != nil {
		t.Fatalf("runInit on existing config: %v", err)
	}
}

// invariant: tooling/init-and-enablement:init-collision-guard (TestInitGuardBlocksAndForceOverrides)
func TestInitGuardBlocksAndForceOverrides(t *testing.T) {
	root := t.TempDir()
	// A pre-existing, non-awf CLAUDE.md is a collision.
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := runFrom(root, []string{"awf", "init"}, &out, &errb); code == 0 {
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
	if code := runFrom(root, []string{"awf", "init", "--force"}, &out, &errb); code != 0 {
		t.Fatalf("init --force failed: %s", errb.String())
	}
	// The original is preserved at <path>.awf-bak.
	// invariant: tooling/init-and-enablement:init-force-backs-up (TestInitGuardBlocksAndForceOverrides)
	if b, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md.awf-bak")); string(b) != "mine\n" {
		t.Fatalf("CLAUDE.md.awf-bak = %q, want original %q", b, "mine\n")
	}
	// And the live file was overwritten with managed content.
	if b, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md")); string(b) == "mine\n" {
		t.Fatalf("CLAUDE.md should have been overwritten, still %q", b)
	}
	const initForcePrefix = "status: initialization completed\n\nmutation:\n  identity:\n    config: "
	if !strings.HasPrefix(out.String(), initForcePrefix) {
		t.Errorf("init --force lost its scaffold identity or backup report:\n%s", out.String())
	}
	for _, want := range []string{
		"skill tdd references unset vars",
		"docs/architecture.md has unauthored stub content",
		"step 5: commit .awf/ and the rendered files together",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("init --force Core report missing %q:\n%s", want, out.String())
		}
	}
	// Regression: init delegates its backup to the chained sync (one BackupFile path,
	// ADR-0035), so the colliding file is backed up exactly once - no double-backup.
	if _, err := os.Stat(filepath.Join(root, "CLAUDE.md.awf-bak.1")); !os.IsNotExist(err) {
		t.Error("expected exactly one backup; CLAUDE.md.awf-bak.1 should not exist")
	}
}

func TestInitRollbackPreservesExistingAwf(t *testing.T) {
	ctx := testContext(t)
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
	if err := runInit(ctx, root, false, false, nil, "", io.Discard); err == nil {
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
	var out, errb bytes.Buffer
	if code := runFrom(root, []string{"awf", "init", "--force"}, &out, &errb); code != 0 {
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
	var out, errb bytes.Buffer
	if code := runFrom(root, []string{"awf", "init"}, &out, &errb); code != 0 {
		t.Fatalf("first init failed: %s", errb.String())
	}
	// Re-init over the now-managed tree: every planned path is in the prior lock,
	// so p.InitCollisions skips them all and init proceeds without --force.
	out.Reset()
	errb.Reset()
	if code := runFrom(root, []string{"awf", "init"}, &out, &errb); code != 0 {
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
	lock := &manifest.Lock{AWFVersion: project.Version, SchemaVersion: 14, Files: map[string]manifest.Entry{}}
	if err := lock.Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := runFrom(root, []string{"awf", "init"}, &out, &errb); code == 0 {
		t.Fatal("expected init to fail when project.Open errors")
	}
	// --force skips the probe, so the same malformed config now fails at
	// runInit's own post-scaffold project.Open - keeping that branch covered.
	if code := runFrom(root, []string{"awf", "init", "--force"}, &out, &errb); code == 0 {
		t.Fatal("expected init --force to fail when project.Open errors")
	}
}

func TestInitAbortsWhenInitCollisionsFails(t *testing.T) {
	root := scaffoldProject(t)
	configPath := config.ConfigPath(root)
	lockPath := config.LockPath(root)
	beforeConfig := mustReadCLIFile(t, configPath)
	beforeLock := mustReadCLIFile(t, lockPath)
	writeMalformedPitfall(t, root)

	var out bytes.Buffer
	err := runInit(testContext(t), root, true, false, nil, "", &out)
	if err == nil || !strings.Contains(err.Error(), "bad.md") || !strings.Contains(err.Error(), "missing frontmatter") {
		t.Fatalf("init collision error = %v, want malformed pitfall", err)
	}
	if out.Len() != 0 {
		t.Fatalf("init stdout = %q, want empty", out.String())
	}
	if got := mustReadCLIFile(t, configPath); got != beforeConfig {
		t.Fatalf("config changed after collision error")
	}
	if got := mustReadCLIFile(t, lockPath); got != beforeLock {
		t.Fatalf("lock changed after collision error")
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
	var out, errb bytes.Buffer
	if code := newRunner(func() (string, error) { return root, nil }, strings.NewReader("SHOULD-NOT-BE-CONSUMED\n"), func() bool { return true }).run([]string{"awf", "init"}, &out, &errb); code == 0 {
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
// refuses and rolls the scaffolded config back. (The leaves-only trim derives
// zero agents under ADR-0081, so the selection is closure-valid.)
func TestInitPostAnswerCollisionAfterProbePasses(t *testing.T) {
	root := t.TempDir()
	skillPath := filepath.Join(root, ".claude", "skills", filepath.Base(root)+"-tdd", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skillPath, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := runFrom(root, []string{"awf", "init", "--set", "skills=tdd"}, &out, &errb); code == 0 {
		t.Fatal("expected init to refuse on the post-answer collision")
	}
	if !strings.Contains(errb.String(), "refusing to overwrite") {
		t.Fatalf("stderr = %q", errb.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "config.yaml")); !os.IsNotExist(err) {
		t.Error("scaffolded config should have been rolled back")
	}
}
