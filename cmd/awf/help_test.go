package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
)

func TestPerCommandHelp(t *testing.T) {
	for _, c := range clispec.Commands {
		for _, flag := range []string{"--help", "-h"} {
			t.Run(c.Name+" "+flag, func(t *testing.T) {
				var out, errb bytes.Buffer
				if code := run([]string{"awf", c.Name, flag}, &out, &errb); code != 0 {
					t.Fatalf("exit %d: %s", code, errb.String())
				}
				if !strings.Contains(out.String(), "command: awf "+c.Name) || !strings.Contains(out.String(), "usage:") {
					t.Errorf("structured help missing command or usage: %s", out.String())
				}
			})
		}
	}
}

func TestGlobalHelpListsAllCommands(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "help"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d", code)
	}
	got := out.String()
	if !strings.Contains(got, "commands:") {
		t.Errorf("global help missing commands: %s", got)
	}
	last := -1
	for _, c := range clispec.Commands {
		needle := c.Name + " | " + c.Summary
		index := strings.Index(got, needle)
		if index < 0 || index < last {
			t.Errorf("global help command %q missing or unordered: %s", c.Name, got)
		}
		last = index
		if c.Summary == "" || len(c.Help.Usage) == 0 {
			t.Errorf("command %q has empty structured metadata", c.Name)
		}
	}
}

// invariant: tooling/cli:help-lists-group-children (TestHelpListsGroupChildren)
func TestHelpListsGroupChildren(t *testing.T) {
	var out, errb bytes.Buffer
	run([]string{"awf", "help"}, &out, &errb)
	got := out.String()
	for _, parent := range clispec.Commands {
		for _, child := range parent.Children {
			if !strings.Contains(got, parent.Name+" "+child.Name+" | "+child.Summary) {
				t.Errorf("global help omits %s %s", parent.Name, child.Name)
			}
		}
	}
}

func TestHelpSubcommandDispatch(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "help", "render"}, &out, &errb); code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out.String(), "command: awf render") {
		t.Errorf("render help: %s", out.String())
	}
	out.Reset()
	if code := run([]string{"awf", "help", "bogus"}, &out, &errb); code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out.String(), "commands:") {
		t.Errorf("unknown help: %s", out.String())
	}
	out.Reset()
	if code := run([]string{"awf", "help", "new", "adr"}, &out, &errb); code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out.String(), "command: awf adr") {
		t.Errorf("child help: %s", out.String())
	}
}
