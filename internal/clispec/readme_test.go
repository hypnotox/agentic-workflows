package clispec

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

const (
	readmeCommandBlockStart = "<!-- awf:clispec-commands:start -->"
	readmeCommandBlockEnd   = "<!-- awf:clispec-commands:end -->"
)

func formatREADMECommandBlock(commands []Command) string {
	var block strings.Builder
	block.WriteString(readmeCommandBlockStart)
	block.WriteString("\n| Command | Purpose |\n|---|---|\n")
	for _, command := range commands {
		fmt.Fprintf(&block, "| `%s` | %s |\n", escapeREADMECell(command.Help.Usage[0]), escapeREADMECell(command.Summary))
	}
	block.WriteString(readmeCommandBlockEnd)
	block.WriteString("\n")
	return block.String()
}

func escapeREADMECell(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}

func READMECommandBlock(readme string) (string, error) {
	if count := strings.Count(readme, readmeCommandBlockStart); count != 1 {
		return "", fmt.Errorf("README start marker count = %d, want 1", count)
	}
	if count := strings.Count(readme, readmeCommandBlockEnd); count != 1 {
		return "", fmt.Errorf("README end marker count = %d, want 1", count)
	}
	start := strings.Index(readme, readmeCommandBlockStart)
	end := strings.Index(readme[start:], readmeCommandBlockEnd)
	if end < 0 {
		return "", errors.New("README end marker precedes start marker")
	}
	end += start + len(readmeCommandBlockEnd)
	if end < len(readme) && readme[end] == '\n' {
		end++
	}
	return readme[start:end], nil
}

func TestFormatREADMECommandBlock(t *testing.T) {
	for _, test := range []struct {
		name     string
		commands []Command
		want     string
	}{
		{
			name:     "ordinary text",
			commands: []Command{{Summary: "Do the thing", Help: Help{Usage: []string{"awf thing"}}}},
			want: "<!-- awf:clispec-commands:start -->\n| Command | Purpose |\n|---|---|\n" +
				"| `awf thing` | Do the thing |\n<!-- awf:clispec-commands:end -->\n",
		},
		{
			name:     "usage pipe",
			commands: []Command{{Summary: "Choose", Help: Help{Usage: []string{"awf choose <a|b>"}}}},
			want: "<!-- awf:clispec-commands:start -->\n| Command | Purpose |\n|---|---|\n" +
				"| `awf choose <a\\|b>` | Choose |\n<!-- awf:clispec-commands:end -->\n",
		},
		{
			name: "source order",
			commands: []Command{
				{Summary: "First", Help: Help{Usage: []string{"awf first"}}},
				{Summary: "Second", Help: Help{Usage: []string{"awf second"}}},
			},
			want: "<!-- awf:clispec-commands:start -->\n| Command | Purpose |\n|---|---|\n" +
				"| `awf first` | First |\n| `awf second` | Second |\n<!-- awf:clispec-commands:end -->\n",
		},
		{
			name:     "summary pipe",
			commands: []Command{{Summary: "Choose a | b", Help: Help{Usage: []string{"awf choose"}}}},
			want: "<!-- awf:clispec-commands:start -->\n| Command | Purpose |\n|---|---|\n" +
				"| `awf choose` | Choose a \\| b |\n<!-- awf:clispec-commands:end -->\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := formatREADMECommandBlock(test.commands); got != test.want {
				t.Errorf("formatREADMECommandBlock() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestCompareREADMECommandBlock(t *testing.T) {
	want := formatREADMECommandBlock([]Command{{Summary: "One", Help: Help{Usage: []string{"awf one"}}}, {Summary: "Two", Help: Help{Usage: []string{"awf two"}}}})
	for _, test := range []struct {
		name   string
		readme string
		match  bool
	}{
		{"missing row", strings.Replace(want, "| `awf two` | Two |\n", "", 1), false},
		{"reordered row", strings.Replace(want, "| `awf one` | One |\n| `awf two` | Two |", "| `awf two` | Two |\n| `awf one` | One |", 1), false},
		{"reworded summary", strings.Replace(want, "| `awf two` | Two |", "| `awf two` | Changed |", 1), false},
		{"missing marker", strings.Replace(want, readmeCommandBlockStart+"\n", "", 1), false},
		{"exact match", want, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := READMECommandBlock(test.readme)
			matched := err == nil && got == want
			if matched != test.match {
				t.Errorf("README command block match = %v, want %v (block %q, error %v)", matched, test.match, got, err)
			}
		})
	}
}

func TestREADMECommandBlockRequiresUniqueMarkers(t *testing.T) {
	block := formatREADMECommandBlock([]Command{{Summary: "One", Help: Help{Usage: []string{"awf one"}}}})
	for _, test := range []struct {
		name   string
		readme string
		want   string
	}{
		{"duplicate start", block + readmeCommandBlockStart, "README start marker count = 2, want 1"},
		{"duplicate end", block + readmeCommandBlockEnd, "README end marker count = 2, want 1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := READMECommandBlock(test.readme)
			if err == nil || err.Error() != test.want {
				t.Fatalf("READMECommandBlock() error = %v, want %q", err, test.want)
			}
		})
	}
}

// TestREADMECommandBlock proves the root README's bounded top-level command projection.
// invariant: tooling/cli:cli-command-spec-single-source (TestREADMECommandBlock)
func TestREADMECommandBlock(t *testing.T) {
	contents, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	got, err := READMECommandBlock(string(contents))
	want := formatREADMECommandBlock(Commands)
	if err != nil || got != want {
		t.Errorf("README command block mismatch: %v\nreplace with:\n%s", err, want)
	}
}
