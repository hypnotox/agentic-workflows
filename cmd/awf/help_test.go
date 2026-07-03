package main

import (
	"bytes"
	"strings"
	"testing"
)

// Every command answers --help and -h with exit 0 and its own usage line —
// guaranteeing no command is missing structured help.
func TestPerCommandHelp(t *testing.T) {
	for name := range argSpecs {
		for _, flag := range []string{"--help", "-h"} {
			t.Run(name+" "+flag, func(t *testing.T) {
				var out, errb bytes.Buffer
				if code := run([]string{"awf", name, flag}, &out, &errb); code != 0 {
					t.Fatalf("`awf %s %s` should exit 0, got %d (err=%s)", name, flag, code, errb.String())
				}
				if !strings.Contains(out.String(), "Usage: awf "+name) {
					t.Errorf("`awf %s %s` help missing usage line; got:\n%s", name, flag, out.String())
				}
			})
		}
	}
}

// The global overview lists every command's summary in the declared order.
func TestGlobalHelpListsAllCommands(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "help"}, &out, &errb); code != 0 {
		t.Fatalf("`awf help` should exit 0, got %d", code)
	}
	got := out.String()
	if !strings.Contains(got, "Commands:") {
		t.Errorf("global help missing Commands header:\n%s", got)
	}
	for name, spec := range argSpecs {
		// Line-anchored: "  <name> " at a line start, so a command name that is a
		// substring of another's summary cannot mask a real omission.
		if !strings.Contains(got, "\n  "+name+" ") {
			t.Errorf("global help omits command line for %q:\n%s", name, got)
		}
		if spec.summary == "" || spec.help == "" {
			t.Errorf("command %q has empty summary/help", name)
		}
	}
}

// commandOrder and argSpecs must stay in exact set parity: the global overview
// iterates commandOrder, so a command in one but not the other would silently
// drop from `awf help` (or print an empty summary line).
func TestCommandOrderMatchesArgSpecs(t *testing.T) {
	inOrder := map[string]bool{}
	for _, name := range commandOrder {
		if inOrder[name] {
			t.Errorf("commandOrder lists %q twice", name)
		}
		inOrder[name] = true
		if _, ok := argSpecs[name]; !ok {
			t.Errorf("commandOrder lists %q, absent from argSpecs", name)
		}
	}
	for name := range argSpecs {
		if !inOrder[name] {
			t.Errorf("argSpecs has %q, missing from commandOrder", name)
		}
	}
}

// `awf help <cmd>` prints that command's --help text; an unknown command
// falls back to the top-level overview and exits 0, like bare `awf help`.
func TestHelpSubcommandDispatch(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "help", "sync"}, &out, &errb); code != 0 {
		t.Fatalf("help sync: exit %d (%s)", code, errb.String())
	}
	if out.String() != argSpecs["sync"].help {
		t.Errorf("awf help sync = %q, want the sync --help text", out.String())
	}
	out.Reset()
	if code := run([]string{"awf", "help", "bogus"}, &out, &errb); code != 0 {
		t.Fatalf("help bogus: exit %d (%s)", code, errb.String())
	}
	if !strings.Contains(out.String(), "Commands:") {
		t.Errorf("unknown command should fall back to the overview:\n%s", out.String())
	}
}
