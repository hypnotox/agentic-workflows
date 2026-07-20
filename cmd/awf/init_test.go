package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// forceNonInteractive pins the isInteractive seam to false for the test, so the
// silent resolution path runs deterministically regardless of the real stdin.
func forceNonInteractive(t *testing.T) {
	t.Helper()
	testsupport.SwapVar(t, &isInteractive, func() bool { return false })
}

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
// invariant: tooling/cli:describe-read-only
func TestInitDescribeReadOnly(t *testing.T) {
	root := t.TempDir()
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init", "--describe"}, &out, &errb); code != 0 {
		t.Fatalf("init --describe: exit %d (%s)", code, errb.String())
	}
	var parsed struct {
		Descriptors []map[string]any `json:"descriptors"`
	}
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("describe output is not valid JSON: %v\n%s", err, out.String())
	}
	if len(parsed.Descriptors) == 0 {
		t.Error("describe emitted no descriptors")
	}
	var hasTrimOptions bool
	for _, d := range parsed.Descriptors {
		if d["target"] == "catalog-skills" {
			if opts, ok := d["options"].([]any); ok && len(opts) > 0 {
				hasTrimOptions = true
			}
		}
	}
	if !hasTrimOptions {
		t.Errorf("describe missing computed catalog-skills options:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".awf")); !os.IsNotExist(err) {
		t.Errorf(".awf/ should not exist after --describe (err=%v)", err)
	}
}

// TestInitExplicitAnswersWin asserts a --set value lands in the scaffolded config.
// invariant: tooling/cli:explicit-answers-win
func TestInitExplicitAnswersWin(t *testing.T) {
	root := t.TempDir()
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	forceNonInteractive(t)
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init", "--set", "gateCmd=make gate"}, &out, &errb); code != 0 {
		t.Fatalf("init --set: exit %d (%s)", code, errb.String())
	}
	if cfg := readInitConfig(t, root); !strings.Contains(cfg, "gateCmd: make gate") {
		t.Errorf("config missing gateCmd override:\n%s", cfg)
	}
}

// TestInitNonInteractiveDefault asserts the silent (non-TTY, no-answers) path
// seeds every var empty and writes no invariants config - byte-identical to the
// pre-feature seed-empty output.
// invariant: tooling/cli:init-noninteractive-default
func TestInitNonInteractiveDefault(t *testing.T) {
	root := t.TempDir()
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	forceNonInteractive(t)
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init"}, &out, &errb); code != 0 {
		t.Fatalf("init: exit %d (%s)", code, errb.String())
	}
	cfg := readInitConfig(t, root)
	if !strings.Contains(cfg, `gateCmd: ""`) {
		t.Errorf("expected gateCmd seeded empty:\n%s", cfg)
	}
	if strings.Contains(cfg, "invariants:") {
		t.Errorf("silent init should not write an invariants config:\n%s", cfg)
	}
}

// TestInitAnswersFile asserts values come from a JSON answers file.
func TestInitAnswersFile(t *testing.T) {
	root := t.TempDir()
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	forceNonInteractive(t)
	ans := filepath.Join(t.TempDir(), "answers.json")
	if err := os.WriteFile(ans, []byte(`{"testCmd":"go test ./..."}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init", "--answers", ans}, &out, &errb); code != 0 {
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
		{name: "invalid multiselect answer", args: []string{"awf", "init", "--set", "skills=nonexistent-skill"}},
		{name: "non-map answers", pre: func(root string) []string {
			f := filepath.Join(root, "bad.yaml")
			_ = os.WriteFile(f, []byte("- a\n- b\n"), 0o644)
			return []string{"awf", "init", "--answers", f}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
			forceNonInteractive(t)
			args := tc.args
			if tc.pre != nil {
				args = tc.pre(root)
			}
			var out, errb bytes.Buffer
			if code := run(args, &out, &errb); code == 0 {
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
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	origInteractive := isInteractive
	isInteractive = func() bool { return true }
	t.Cleanup(func() { isInteractive = origInteractive })
	origStdin := stdin
	// Multiselects prompt first (ADR-0086): two empty lines keep the skills and
	// docs core defaults, then gateCmd reads its value; every later prompt hits
	// EOF and takes its empty default, so the invariants marker/globs stay unset.
	stdin = strings.NewReader("\n\nmake gate\n")
	t.Cleanup(func() { stdin = origStdin })

	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init"}, &out, &errb); code != 0 {
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
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	testsupport.SwapVar(t, &isInteractive, func() bool { return true })
	testsupport.WriteAwfConfig(t, root, "prefix: ex\nskills: []\nagents: []\n")
	origStdin := stdin
	stdin = strings.NewReader("answered-one\nanswered-two\nanswered-three\n")
	t.Cleanup(func() { stdin = origStdin })

	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init"}, &out, &errb); code != 0 {
		t.Fatalf("init over existing config: exit %d (%s)", code, errb.String())
	}
	if strings.Contains(out.String(), "gateCmd:") {
		t.Errorf("init prompted for descriptors it cannot apply:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "keeping it") {
		t.Errorf("init did not say the existing config is kept:\n%s", out.String())
	}
	if cfg := readInitConfig(t, root); strings.Contains(cfg, "answered") {
		t.Errorf("existing config was modified:\n%s", cfg)
	}
	// Explicit answers against an existing config are surfaced as ignored.
	var out2 bytes.Buffer
	if code := run([]string{"awf", "init", "--set", "gateCmd=make gate"}, &out2, &errb); code != 0 {
		t.Fatalf("init --set over existing config: exit %d (%s)", code, errb.String())
	}
	if !strings.Contains(out2.String(), "ignored") {
		t.Errorf("init did not note that --set answers were ignored:\n%s", out2.String())
	}
}

// TestInitCatalogTrim asserts --set skills=/docs= drive the scaffolded enable
// arrays verbatim (full-deselectable catalog trim, ADR-0029).
func TestInitCatalogTrim(t *testing.T) {
	// Leaf-only trim: init derives the agent set from the selection's
	// requirements (ADR-0081 Decision 9), so nothing pins the chain back in
	// and core skills are genuinely deselectable.
	root := t.TempDir()
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	forceNonInteractive(t)
	var out, errb bytes.Buffer
	code := run([]string{"awf", "init", "--set", "skills=tdd,bugfix", "--set", "docs=testing"}, &out, &errb)
	if code != 0 {
		t.Fatalf("init --set trim: exit %d (%s)", code, errb.String())
	}
	cfg := readInitConfig(t, root)
	for _, want := range []string{"skills:", "- bugfix", "- tdd", "docs:", "- testing"} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config missing %q:\n%s", want, cfg)
		}
	}
	// A core skill not in the selection must be absent (full-deselectable),
	// and a leaves-only selection derives zero agents.
	if strings.Contains(cfg, "- reviewing-impl") || strings.Contains(cfg, "- code-reviewer") {
		t.Errorf("trim should have deselected reviewing-impl and derived no agents:\n%s", cfg)
	}
}

// A trim naming a chain skill is closure-completed with a note per addition
// (ADR-0081 Decision 9).
func TestInitCatalogTrimClosesChainSelection(t *testing.T) {
	root := t.TempDir()
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	forceNonInteractive(t)
	var out, errb bytes.Buffer
	code := run([]string{"awf", "init", "--set", "skills=brainstorming"}, &out, &errb)
	if code != 0 {
		t.Fatalf("init --set trim: exit %d (%s)", code, errb.String())
	}
	if !strings.Contains(out.String(), "note: also enabled skill reviewing-plan-resync (required by your selection)") ||
		!strings.Contains(out.String(), "note: also enabled agent plan-reviewer (required by your selection)") {
		t.Errorf("expected closure-addition notes, got:\n%s", out.String())
	}
	cfg := readInitConfig(t, root)
	for _, want := range []string{"- brainstorming", "- retrospective", "- plan-reviewer"} {
		if !strings.Contains(cfg, want) {
			t.Errorf("closure-completed config missing %q:\n%s", want, cfg)
		}
	}
}

// TestIsInteractive exercises the real isInteractive seam (the result depends on
// whether the test's stdin is a terminal; we only assert it runs without panic).
func TestIsInteractive(t *testing.T) {
	t.Logf("isInteractive() = %v", isInteractive())
}

// A commitScopes answer lands in audit.allowedScopes, never in vars; the
// silent default writes no audit block (ADR-0051).
func TestInitCommitScopesAnswer(t *testing.T) {
	root := t.TempDir()
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	forceNonInteractive(t)
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init", "--set", "commitScopes=adr, awf"}, &out, &errb); code != 0 {
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

// After the chained sync succeeds, init prints the render-completeness notes
// (same rendering as awf check, ADR-0045) and a fixed next-steps block.
func TestInitPrintsNotesAndNextSteps(t *testing.T) {
	root := t.TempDir()
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	forceNonInteractive(t)
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init"}, &out, &errb); code != 0 {
		t.Fatalf("init: exit %d (%s)", code, errb.String())
	}
	for _, want := range []string{
		"references unset vars",
		"next steps:",
		".awf/parts/agents-doc/identity.md",
		".awf/hooks/",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("init output missing %q:\n%s", want, out.String())
		}
	}
}
