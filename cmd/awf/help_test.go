package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
)

// Every command answers --help and -h with exit 0 and its own usage line —
// guaranteeing no command is missing structured help.
func TestPerCommandHelp(t *testing.T) {
	for _, c := range clispec.Commands {
		for _, flag := range []string{"--help", "-h"} {
			t.Run(c.Name+" "+flag, func(t *testing.T) {
				var out, errb bytes.Buffer
				if code := run([]string{"awf", c.Name, flag}, &out, &errb); code != 0 {
					t.Fatalf("`awf %s %s` should exit 0, got %d (err=%s)", c.Name, flag, code, errb.String())
				}
				if !strings.Contains(out.String(), "Usage: awf "+c.Name) {
					t.Errorf("`awf %s %s` help missing usage line; got:\n%s", c.Name, flag, out.String())
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
	for _, c := range clispec.Commands {
		// Line-anchored: "  <name> " at a line start, so a command name that is a
		// substring of another's summary cannot mask a real omission.
		if !strings.Contains(got, "\n  "+c.Name+" ") {
			t.Errorf("global help omits command line for %q:\n%s", c.Name, got)
		}
		if c.Summary == "" || c.HelpBody == "" {
			t.Errorf("command %q has empty summary/help", c.Name)
		}
	}
}

// The top-level usage line, `awf help` overview order, and the command set all
// derive from clispec — no parallel enumeration in cmd/awf.
// (inv: cli-command-spec-single-source, backed in internal/clispec.)
func TestCliCommandSpecSingleSource(t *testing.T) {
	// globalHelp lists commands in clispec order; assert each name appears and in
	// the same relative order as clispec.Commands.
	var out, errb bytes.Buffer
	run([]string{"awf", "help"}, &out, &errb)
	got := out.String()
	last := -1
	for _, c := range clispec.Commands {
		idx := strings.Index(got, "\n  "+c.Name+" ")
		if idx < 0 {
			t.Fatalf("globalHelp omits %q", c.Name)
		}
		if idx < last {
			t.Errorf("globalHelp lists %q out of clispec order", c.Name)
		}
		last = idx
	}
	// The bare-usage line is the clispec usage token list.
	errb.Reset()
	run([]string{"awf"}, &out, &errb)
	if !strings.Contains(errb.String(), clispec.UsageLine()) {
		t.Errorf("bare usage line does not derive from clispec.UsageLine():\n%s", errb.String())
	}
}

// `awf help <cmd>` prints that command's --help text; an unknown command
// falls back to the top-level overview and exits 0, like bare `awf help`.
func TestHelpSubcommandDispatch(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "help", "sync"}, &out, &errb); code != 0 {
		t.Fatalf("help sync: exit %d (%s)", code, errb.String())
	}
	sync, _ := clispec.Lookup("sync")
	if out.String() != sync.HelpBody {
		t.Errorf("awf help sync = %q, want the sync --help text", out.String())
	}
	out.Reset()
	if code := run([]string{"awf", "help", "bogus"}, &out, &errb); code != 0 {
		t.Fatalf("help bogus: exit %d (%s)", code, errb.String())
	}
	if !strings.Contains(out.String(), "Commands:") {
		t.Errorf("unknown command should fall back to the overview:\n%s", out.String())
	}
	// `awf help <group> <child>` prints the child's body.
	out.Reset()
	if code := run([]string{"awf", "help", "new", "adr"}, &out, &errb); code != 0 {
		t.Fatalf("help new adr: exit %d (%s)", code, errb.String())
	}
	newCmd, _ := clispec.Lookup("new")
	adr, _ := newCmd.Child("adr")
	if out.String() != adr.HelpBody {
		t.Errorf("awf help new adr = %q, want the new-adr child help", out.String())
	}
	// An unknown child falls back to the group's own body.
	out.Reset()
	if code := run([]string{"awf", "help", "new", "bogus"}, &out, &errb); code != 0 {
		t.Fatalf("help new bogus: exit %d (%s)", code, errb.String())
	}
	if out.String() != newCmd.HelpBody {
		t.Errorf("awf help new bogus = %q, want the new group help", out.String())
	}
}
