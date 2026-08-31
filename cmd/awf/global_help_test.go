package main

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func TestGlobalHelpRejectsInvalidCommandSpec(t *testing.T) {
	original := clispec.Commands
	t.Cleanup(func() { clispec.Commands = original })
	clispec.Commands = []clispec.Command{{Name: "bad\nname", Summary: "summary"}}
	if _, err := globalHelp(); err == nil {
		t.Fatal("invalid command name accepted")
	}
	clispec.Commands = []clispec.Command{{Name: "valid", Summary: " "}}
	if _, err := globalHelp(); err == nil {
		t.Fatal("invalid summary accepted")
	}
}

func TestRunNoArgs(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"awf"}, &out, &errb); code != 2 {
		t.Fatalf("expected exit 2 for no args, got %d", code)
	}
	want := "condition: awf: usage: " + clispec.UsageLine() + " [args]; run `awf help` for command details\n"
	if out.Len() != 0 || errb.String() != want {
		t.Errorf("streams stdout=%q stderr=%q, want stderr=%q", out.String(), errb.String(), want)
	}
}

func TestRunHelp(t *testing.T) {
	for _, arg := range []string{"help", "--help", "-h"} {
		var out, errb bytes.Buffer
		if code := run([]string{"awf", arg}, &out, &errb); code != 0 {
			t.Fatalf("%s: expected exit 0, got %d", arg, code)
		}
		if !strings.Contains(out.String(), "commands:") || !strings.Contains(out.String(), "uninstall") {
			t.Errorf("%s: help text missing content:\n%s", arg, out.String())
		}
	}
}

// invariant: tooling/cli:readable-text-output (TestInitHelpUsageAndOperationalFailure)
func TestInitHelpUsageAndOperationalFailure(t *testing.T) {
	var command clispec.Command
	for _, candidate := range clispec.Commands {
		if candidate.Name == "init" {
			command = candidate
			break
		}
	}
	if command.Name == "" {
		t.Fatal("init command is not available")
	}
	document, err := command.Help.Document("awf init", command.Summary)
	if err != nil {
		t.Fatal(err)
	}
	var want bytes.Buffer
	if err := presentation.Render(&want, document); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"awf", "init", "--help"}, &stdout, &stderr); code != 0 || stdout.String() != want.String() || stderr.Len() != 0 {
		t.Fatalf("help exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"awf", "init", "--invalid"}, &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.String() != "condition: awf: awf init: unknown flag \"--invalid\"\n" {
		t.Fatalf("usage exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := newRunner(func() (string, error) { return "", errors.New("working directory unavailable") }, os.Stdin, func() bool { return false }).run([]string{"awf", "init"}, &stdout, &stderr); code != 1 || stdout.Len() != 0 || stderr.String() != "condition: awf: working directory unavailable\n" {
		t.Fatalf("operational exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
