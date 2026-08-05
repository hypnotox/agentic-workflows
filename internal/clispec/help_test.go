package clispec

import "testing"

func TestStructuredHelpDocument(t *testing.T) {
	valid := Help{Usage: []string{"awf test <name>"}, Description: "describe", Details: []string{"detail"}, Positionals: []HelpItem{{"name", "a name"}}, Options: []HelpItem{{"--flag", "a flag"}}, Examples: []string{"awf test one"}, Related: []string{"awf other"}}
	if _, err := valid.Document("awf test", "summary"); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []Help{
		{Usage: []string{" "}},
		{Description: " "},
		{Usage: []string{"ok"}, Description: " "},
		{Usage: []string{"ok"}, Details: []string{" "}},
		{Usage: []string{"ok"}, Positionals: []HelpItem{{"name", ""}}},
		{Usage: []string{"ok"}, Options: []HelpItem{{"", "bad"}}},
		{Usage: []string{"ok"}, Examples: []string{" "}},
		{Usage: []string{"ok"}, Related: []string{" "}},
	} {
		if _, err := invalid.Document("awf test", "summary"); err == nil {
			t.Fatal("invalid help accepted")
		}
	}
	if _, err := valid.Document(" ", "summary"); err == nil {
		t.Fatal("invalid command accepted")
	}
	if _, err := valid.Document("awf test", " "); err == nil {
		t.Fatal("invalid summary accepted")
	}
}
