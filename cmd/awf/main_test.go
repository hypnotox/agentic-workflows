package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
)

func TestRunInitScaffoldsAndSyncs(t *testing.T) {
	root := t.TempDir()
	// Rename tempdir base via a child dir so prefix is predictable.
	proj := filepath.Join(root, "acme")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runInit(proj, false, false, nil, "", io.Discard); err != nil {
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
// handler without a command. Group children (new/adr...) are not separate keys.
func TestHandlerRegistryParity(t *testing.T) {
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
// alongside a resolved child, so run() gates and keys the handler off the
// top-level node rather than the child (whose Gating is an unset Ungated zero).
func TestResolveReturnsTopLevel(t *testing.T) {
	cmd, top, sub, rest, ok := resolve([]string{"new", "adr", "A Title"})
	if !ok || cmd.Name != "adr" || top.Name != "new" || sub != "adr" {
		t.Fatalf("resolve(new adr) = cmd=%q top=%q sub=%q ok=%v", cmd.Name, top.Name, sub, ok)
	}
	if len(rest) != 1 || rest[0] != "A Title" {
		t.Errorf("resolve(new adr) rest = %v", rest)
	}
	if cmd, top, _, _, ok := resolve([]string{"sync"}); !ok || cmd.Name != "sync" || top.Name != "sync" {
		t.Errorf("resolve(sync) = cmd=%q top=%q ok=%v; leaf should return itself as top", cmd.Name, top.Name, ok)
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseArgs(cmd, tc.rest); err == nil {
				t.Errorf("parseArgs(%v) = nil, want usage error", tc.rest)
			}
		})
	}
}
