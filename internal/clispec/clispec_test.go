package clispec

import (
	"strings"
	"testing"
)

// Every command and child carries non-empty identifying metadata, and top-level
// names are unique.
func TestCommandsWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range Commands {
		if c.Name == "" || c.Summary == "" || c.HelpBody == "" {
			t.Errorf("command %q has an empty Name/Summary/HelpBody", c.Name)
		}
		if seen[c.Name] {
			t.Errorf("duplicate top-level command %q", c.Name)
		}
		seen[c.Name] = true
		if !strings.Contains(c.HelpBody, "Usage: awf "+c.Name) {
			t.Errorf("command %q help missing its usage line", c.Name)
		}
		for _, ch := range c.Children {
			if ch.Name == "" || ch.Summary == "" || ch.HelpBody == "" {
				t.Errorf("child %s/%s has empty metadata", c.Name, ch.Name)
			}
		}
	}
}

// Gating is a top-level property: a group command's children must leave Gating
// at the Ungated zero value, so the driver (which reads gating from the
// top-level node) is the single authority. A child that declared its own gating
// would be silently ignored — this guards that trap shut.
func TestGroupChildrenCarryNoGating(t *testing.T) {
	for _, c := range Commands {
		for _, ch := range c.Children {
			if ch.Gating != Ungated {
				t.Errorf("group %q child %q sets Gating=%d; children must stay Ungated (gating is top-level)", c.Name, ch.Name, ch.Gating)
			}
		}
	}
}

func TestLookup(t *testing.T) {
	if _, ok := Lookup("sync"); !ok {
		t.Error("Lookup(sync) missing")
	}
	if _, ok := Lookup("nope"); ok {
		t.Error("Lookup(nope) should miss")
	}
	newCmd, ok := Lookup("new")
	if !ok {
		t.Fatal("Lookup(new) missing")
	}
	if len(newCmd.Children) != 5 {
		t.Errorf("new has %d children, want 5", len(newCmd.Children))
	}
	if _, ok := newCmd.Child("adr"); !ok {
		t.Error("new.Child(adr) missing")
	}
	if _, ok := newCmd.Child("plan"); !ok {
		t.Error("new.Child(plan) missing")
	}
	if _, ok := newCmd.Child("nope"); ok {
		t.Error("new.Child(nope) should miss")
	}
}

// GatedCommandNames is the exact published gated set, in table order — the
// non-Ungated commands, a group contributing only its own token.
func TestGatedCommandNames(t *testing.T) {
	want := []string{"sync", "check", "invariants", "audit", "list", "config", "context", "new", "enable", "disable"}
	got := GatedCommandNames()
	if len(got) != len(want) {
		t.Fatalf("GatedCommandNames() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("GatedCommandNames()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNamesAndUsageLine(t *testing.T) {
	names := Names()
	if len(names) != len(Commands) || names[0] != "init" {
		t.Errorf("Names() = %v", names)
	}
	if got := UsageLine(); got != "awf <"+strings.Join(names, "|")+">" {
		t.Errorf("UsageLine() = %q", got)
	}
}
