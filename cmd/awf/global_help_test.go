package main

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
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
