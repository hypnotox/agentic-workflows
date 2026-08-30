package main

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
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

// TestTopLevelCommandFamiliesUseStructuredHelpAndUsageFailures is an explicit
// registry-keyed interface contract. A new top-level family must be added here;
// there is no count-based or missing-family allowance. Each entry names the
// separately executed test that pins a real result from that family; this unit
// additionally pins its exact help, usage failure, and operational failure.
func TestTopLevelCommandFamiliesUseStructuredHelpAndUsageFailures(t *testing.T) {
	families := map[string]string{
		"init":      "TestInitDescribeReadOnly",
		"render":    "TestEmptyInitChecksOnUnbornHead",
		"edit":      "TestPartAuthoringCLI",
		"reset":     "TestPartAuthoringCLI",
		"check":     "TestRunCheckCleanThenDirty",
		"read":      "TestReadTopicExposesOnlyReferencesAndCoverage",
		"resolve":   "TestRunResolveTopicUsage",
		"audit":     "TestRunAuditDispatch",
		"effort":    "TestEffortPublicTextProtocol",
		"list":      "TestRunListPrintsSkills",
		"config":    "TestRunConfigDispatch",
		"new":       "TestRunNewDispatch",
		"remove":    "TestRunNewDomainLifecycle",
		"upgrade":   "TestRunUpgradeRendersSuccessfulFinalJournalMutation",
		"uninstall": "TestRunUninstallDispatch",
		"changelog": "TestChangelogPublicPayloadContracts",
		"version":   "TestRunVersion",
	}
	contractTests := map[string]bool{}
	paths, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, declaration := range file.Decls {
			if function, ok := declaration.(*ast.FuncDecl); ok {
				contractTests[function.Name.Name] = true
			}
		}
	}
	commands := make(map[string]clispec.Command, len(clispec.Commands))
	for _, command := range clispec.Commands {
		commands[command.Name] = command
		if _, ok := families[command.Name]; !ok {
			t.Errorf("uncontracted top-level command family %q", command.Name)
		}
	}
	for name, contractTest := range families {
		command, ok := commands[name]
		if !ok {
			t.Errorf("contract names missing top-level command family %q", name)
			continue
		}
		if !contractTests[contractTest] {
			t.Errorf("%s result contract test %q is missing", name, contractTest)
		}
		t.Run(name, func(t *testing.T) {
			document, err := command.Help.Document("awf "+name, command.Summary)
			if err != nil {
				t.Fatal(err)
			}
			var want bytes.Buffer
			if err := presentation.Render(&want, document); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			if code := run([]string{"awf", name, "--help"}, &stdout, &stderr); code != 0 || stdout.String() != want.String() || stderr.Len() != 0 {
				t.Fatalf("help exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			stdout.Reset()
			stderr.Reset()
			if code := run([]string{"awf", name, "--presentation-contract-invalid"}, &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.String() != "condition: awf: awf "+name+": unknown flag \"--presentation-contract-invalid\"\n" {
				t.Fatalf("usage exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			stdout.Reset()
			stderr.Reset()
			if code := newRunner(func() (string, error) { return "", errors.New("working directory unavailable") }, os.Stdin, func() bool { return false }).run([]string{"awf", name}, &stdout, &stderr); code != 1 || stdout.Len() != 0 || stderr.String() != "condition: awf: working directory unavailable\n" {
				t.Fatalf("operational exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}
