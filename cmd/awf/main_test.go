package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
)

func TestRunInitScaffoldsAndSyncs(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := t.TempDir()
	// Rename tempdir base via a child dir so prefix is predictable.
	proj := filepath.Join(root, "acme")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runInit(ctx, proj, false, false, nil, "", io.Discard); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	cfg, err := os.ReadFile(filepath.Join(proj, ".awf", "config.yaml"))
	if err != nil {
		t.Fatalf("config not scaffolded: %v", err)
	}
	if !containsLine(string(cfg), "prefix: acme") {
		t.Errorf("scaffold prefix wrong:\n%s", cfg)
	}
	if _, err := os.Stat(filepath.Join(proj, ".awf", "awf.lock")); err != nil {
		t.Errorf("lock not written: %v", err)
	}
}

func containsLine(s, line string) bool {
	for _, l := range strings.Split(s, "\n") {
		if l == line {
			return true
		}
	}
	return false
}

// TestHandlerRegistryParity asserts the handler registry and the clispec table
// name exactly the same top-level commands - no command without a handler, no
// handler without a command. Group children are not separate keys.
// invariant: tooling/cli:cli-runner-instance-ownership (TestRunnerInstancesOwnProcessDependencies)
func TestRunnerInstancesOwnProcessDependencies(t *testing.T) {
	firstRoot, secondRoot := t.TempDir(), t.TempDir()
	first := newRunner(func() (string, error) { return firstRoot, nil }, strings.NewReader("must not be read\n"), func() bool { return false })
	second := newRunner(func() (string, error) { return secondRoot, nil }, strings.NewReader("core\nmake gate\n"), func() bool { return true })
	delete(first.handlers, "version")
	if _, ok := second.handlers["version"]; !ok {
		t.Fatal("runner handler maps share mutable state")
	}
	if root, _ := first.getwd(); root != firstRoot {
		t.Fatalf("first working directory = %q", root)
	}
	if root, _ := second.getwd(); root != secondRoot {
		t.Fatalf("second working directory = %q", root)
	}
	var firstOut, firstErr bytes.Buffer
	if code := first.run([]string{"awf", "init"}, &firstOut, &firstErr); code != 0 {
		t.Fatalf("non-interactive runner exit = %d, stderr=%q", code, firstErr.String())
	}
	if strings.Contains(firstOut.String(), "prompt:") {
		t.Fatalf("non-interactive runner prompted: %q", firstOut.String())
	}
	var secondOut, secondErr bytes.Buffer
	if code := second.run([]string{"awf", "init"}, &secondOut, &secondErr); code != 0 {
		t.Fatalf("interactive runner exit = %d, stderr=%q", code, secondErr.String())
	}
	if !strings.Contains(secondOut.String(), "prompt:") {
		t.Fatalf("interactive runner did not prompt: %q", secondOut.String())
	}
}

// invariant: tooling/cli:cli-runner-instance-ownership (TestHandlerRegistryParity)
func TestHandlerRegistryParity(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	handlers := newRunner(os.Getwd, os.Stdin, func() bool { return false }).handlers
	for _, c := range clispec.Commands {
		if _, ok := handlers[c.Name]; !ok {
			t.Errorf("clispec command %q has no handler", c.Name)
		}
	}
	for name := range handlers {
		if _, ok := clispec.Lookup(name); !ok {
			t.Errorf("handler %q has no clispec command", name)
		}
	}
}

// TestResolveReturnsTopLevel pins that resolve returns the top-level command
// alongside a resolved child, so run() keys the handler and inherited gating
// classification off the top-level node.
func TestResolveReturnsTopLevel(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	cmd, top, sub, rest, ok := resolve([]string{"new", "topic", "tooling", "A Title"})
	if !ok || cmd.Name != "topic" || top.Name != "new" || sub != "topic" {
		t.Fatalf("resolve(new topic) = cmd=%q top=%q sub=%q ok=%v", cmd.Name, top.Name, sub, ok)
	}
	if len(rest) != 2 || rest[0] != "tooling" || rest[1] != "A Title" {
		t.Errorf("resolve(new topic) rest = %v", rest)
	}
	if cmd, top, _, _, ok := resolve([]string{"render"}); !ok || cmd.Name != "render" || top.Name != "render" {
		t.Errorf("resolve(render) = cmd=%q top=%q ok=%v; leaf should return itself as top", cmd.Name, top.Name, ok)
	}
	if _, _, _, _, ok := resolve([]string{"nope"}); ok {
		t.Error("resolve(nope) should miss")
	}
}

// parseArgs folds flag/value/repeatable/positional parsing and arity validation
// into one pass: bool flags set bools, value flags consume their token, a
// repeatable flag collects into multi, non-flag tokens are positionals, and an
// unknown flag / missing value / out-of-range arity is a usage error.
func TestParseArgs(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	cmd := clispec.Command{
		Name: "x", BoolFlags: []string{"--flag"}, ValueFlags: []string{"--val", "--set"},
		Repeatable: []string{"--set"}, MinPos: 1, MaxPos: 2,
	}
	inv, err := parseArgs(cmd, []string{"--val", "v", "a", "--flag", "--set", "s1", "--set", "s2", "b"})
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if inv.values["--val"] != "v" || !inv.bools["--flag"] {
		t.Errorf("value/bool: %+v", inv)
	}
	if len(inv.multi["--set"]) != 2 || inv.multi["--set"][0] != "s1" || inv.multi["--set"][1] != "s2" {
		t.Errorf("repeatable: %v", inv.multi["--set"])
	}
	if len(inv.positionals) != 2 || inv.positionals[0] != "a" || inv.positionals[1] != "b" {
		t.Errorf("positionals: %v", inv.positionals)
	}
	for _, tc := range []struct {
		name string
		rest []string
	}{
		{"missing value", []string{"a", "--val"}},
		{"unknown flag", []string{"a", "--bogus"}},
		{"under min", nil},
		{"over max", []string{"a", "b", "c"}},
		{"duplicate value flag", []string{"a", "--val", "v1", "--val", "v2"}},
		{"duplicate bool flag", []string{"a", "--flag", "--flag"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseArgs(cmd, tc.rest); err == nil {
				t.Errorf("parseArgs(%v) = nil, want usage error", tc.rest)
			}
		})
	}
}

// invariant: tooling/cli:single-os-exit (TestNoOsExitOutsideMain)
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
