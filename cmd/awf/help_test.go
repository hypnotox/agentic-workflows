package main

import (
	"bytes"
	"os"
	"path/filepath"
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

func TestPublicHelpGoldens(t *testing.T) {
	cases := []struct {
		name string
		args []string
		file string
	}{
		{"global", []string{"awf", "help"}, "global.txt"},
		{"group", []string{"awf", "help", "check"}, "group-check.txt"},
		{"leaf", []string{"awf", "help", "render"}, "leaf-render.txt"},
		{"deep leaf", []string{"awf", "help", "check", "repo", "drift"}, "deep-check-repo-drift.txt"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("testdata", "help", tc.file))
			if err != nil {
				t.Fatal(err)
			}
			var out, errb bytes.Buffer
			if code := run(tc.args, &out, &errb); code != 0 || errb.Len() != 0 {
				t.Fatalf("exit=%d stderr=%q", code, errb.String())
			}
			if out.String() != string(want) {
				t.Fatalf("help output differs from %s\n--- got ---\n%s--- want ---\n%s", tc.file, out.String(), want)
			}
		})
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
	if code := run([]string{"awf", "help"}, &out, &errb); code != 0 {
		t.Fatal(code)
	}
	got := out.String()
	var walk func([]clispec.Command, string)
	walk = func(commands []clispec.Command, parent string) {
		last := -1
		for _, command := range commands {
			needle := command.Name + " | " + command.Summary
			at := strings.Index(got, needle)
			if at < 0 {
				t.Errorf("%s omits direct child %q", parent, command.Name)
				continue
			}
			if at < last {
				t.Errorf("%s reorders child %q", parent, command.Name)
			}
			last = at
			if len(command.Children) > 0 {
				walk(command.Children, command.Name)
			}
		}
	}
	walk(clispec.Commands, "root")
	for _, want := range []string{"    check:\n", "      repo:\n", "        drift | Report stale or hand-edited rendered output"} {
		if !strings.Contains(got, want) {
			t.Errorf("hierarchy missing %q", want)
		}
	}
	if strings.Contains(got, "check repo drift |") {
		t.Error("deep child was flattened into a path record")
	}
}

func TestHelpPathsResolveRecursively(t *testing.T) {
	var walk func(path []string, commands []clispec.Command)
	walk = func(path []string, commands []clispec.Command) {
		for _, command := range commands {
			commandPath := append(append([]string{}, path...), command.Name)
			args := append([]string{"awf", "help"}, commandPath...)
			var out, errb bytes.Buffer
			if code := run(args, &out, &errb); code != 0 || errb.Len() != 0 {
				t.Fatalf("help %s: exit=%d stderr=%q", strings.Join(commandPath, " "), code, errb.String())
			}
			want := "command: awf " + strings.Join(commandPath, " ") + "\n"
			if !strings.HasPrefix(out.String(), want) {
				t.Errorf("help %s begins %q, want %q", strings.Join(commandPath, " "), out.String(), want)
			}
			walk(commandPath, command.Children)
		}
	}
	walk(nil, clispec.Commands)
}

func TestHelpSubcommandDispatch(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "help", "render"}, &out, &errb); code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out.String(), "command: awf render") {
		t.Errorf("render help: %s", out.String())
	}
}

func TestHelpRejectsUnknownChildrenRecursively(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"awf", "help", "bogus"}, &stdout, &stderr); code != 2 {
		t.Fatalf("top-level unknown help exit = %d, want 2", code)
	}
	if stdout.Len() != 0 || stderr.String() != "condition: awf: unknown command \"bogus\"\n" {
		t.Fatalf("top-level unknown help streams stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	var walk func([]string, []clispec.Command)
	walk = func(path []string, commands []clispec.Command) {
		for _, command := range commands {
			commandPath := append(append([]string{}, path...), command.Name)
			args := append(append([]string{"awf", "help"}, commandPath...), "bogus")
			var stdout, stderr bytes.Buffer
			if code := run(args, &stdout, &stderr); code != 2 {
				t.Errorf("help %s bogus exit = %d, want 2", strings.Join(commandPath, " "), code)
			}
			if stdout.Len() != 0 || stderr.String() != "condition: awf: unknown command \"bogus\"\n" {
				t.Errorf("help %s bogus streams stdout=%q stderr=%q", strings.Join(commandPath, " "), stdout.String(), stderr.String())
			}
			walk(commandPath, command.Children)
		}
	}
	walk(nil, clispec.Commands)
}

func TestHelpConstructionPropagatesInvalidMetadata(t *testing.T) {
	cases := []struct {
		name      string
		construct func() error
	}{
		{
			name: "top-level record",
			construct: func() error {
				_, err := commandSections([]clispec.Command{{Name: "bad\n", Summary: "summary"}})
				return err
			},
		},
		{
			name: "top-level child propagation",
			construct: func() error {
				_, err := commandSections([]clispec.Command{{Name: "group", Summary: "summary", Children: []clispec.Command{{Name: "bad\n", Summary: "summary"}}}})
				return err
			},
		},
		{
			name: "child record",
			construct: func() error {
				_, err := commandSection(clispec.Command{Name: "group", Children: []clispec.Command{{Name: "bad\n", Summary: "summary"}}})
				return err
			},
		},
		{
			name: "grandchild record",
			construct: func() error {
				_, err := commandSection(clispec.Command{Name: "group", Children: []clispec.Command{{Name: "child", Summary: "summary", Children: []clispec.Command{{Name: "bad\n", Summary: "summary"}}}}})
				return err
			},
		},
		{
			name: "nested group label",
			construct: func() error {
				_, err := commandSection(clispec.Command{Name: "group", Children: []clispec.Command{{Name: "BAD", Summary: "summary", Children: []clispec.Command{{Name: "leaf", Summary: "summary"}}}}})
				return err
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.construct(); err == nil {
				t.Fatal("invalid command metadata succeeded")
			}
		})
	}
}
