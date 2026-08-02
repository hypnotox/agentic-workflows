package clispec

import (
	"strings"
	"testing"
)

func TestCheckCommitSpecIncludesStaleMergeAuthorization(t *testing.T) {
	check, ok := Lookup("check")
	if !ok {
		t.Fatal("missing check")
	}
	staged, ok := check.Child("staged")
	if !ok {
		t.Fatal("missing check staged")
	}
	commit, ok := staged.Child("commit")
	if !ok {
		t.Fatal("missing check staged commit")
	}
	if !strings.Contains(commit.Summary, "stale-ADR merge authorization") {
		t.Fatalf("summary = %q", commit.Summary)
	}
	for _, text := range []string{"MERGE_HEAD", "AWF-Allow-Version", "AWF-Allow-Reason", "unchanged", "git commit"} {
		if !strings.Contains(commit.HelpBody, text) {
			t.Errorf("help missing %q", text)
		}
	}
}

func TestContextHumanOnlyFacetSpec(t *testing.T) {
	context, ok := Lookup("context")
	if !ok {
		t.Fatal("missing context")
	}
	if strings.Contains(strings.Join(context.BoolFlags, " "), "--json") || !strings.Contains(strings.Join(context.ValueFlags, " "), "--show") || !strings.Contains(strings.Join(context.Repeatable, " "), "--show") {
		t.Fatalf("context spec=%#v", context)
	}
	for _, text := range []string{"tier 0", "tier-1", "relationships", "invariants", "all-rules", "all eight facets", "Only artifacts", "8,192", "caller", "JSON is not supported"} {
		if !strings.Contains(context.HelpBody, text) {
			t.Errorf("help missing %q", text)
		}
	}
}

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
		var walk func(parent string, children []Command)
		walk = func(parent string, children []Command) {
			for _, ch := range children {
				if ch.Name == "" || ch.Summary == "" || ch.HelpBody == "" {
					t.Errorf("child %s/%s has empty metadata", parent, ch.Name)
				}
				walk(parent+"/"+ch.Name, ch.Children)
			}
		}
		walk(c.Name, c.Children)
	}
}

func TestLookup(t *testing.T) {
	if _, ok := Lookup("render"); !ok {
		t.Error("Lookup(render) missing")
	}
	if _, ok := Lookup("nope"); ok {
		t.Error("Lookup(nope) should miss")
	}
	newCmd, ok := Lookup("new")
	if !ok {
		t.Fatal("Lookup(new) missing")
	}
	if len(newCmd.Children) != 6 {
		t.Errorf("new has %d children, want 6", len(newCmd.Children))
	}
	if _, ok := newCmd.Child("adr"); !ok {
		t.Error("new.Child(adr) missing")
	}
	if _, ok := newCmd.Child("plan"); !ok {
		t.Error("new.Child(plan) missing")
	}
	if topic, ok := newCmd.Child("topic"); !ok {
		t.Error("new.Child(topic) missing")
	} else if topic.MinPos != 2 || topic.MaxPos != -1 || !strings.Contains(topic.HelpBody, "without syncing") {
		t.Errorf("new topic spec = %#v", topic)
	}
	if _, ok := newCmd.Child("nope"); ok {
		t.Error("new.Child(nope) should miss")
	}
	topic, ok := Lookup("topic")
	if !ok || topic.MinPos != 1 || topic.MaxPos != 1 || topic.Gating != GatedInHandler {
		t.Fatalf("topic spec = %#v, found %v", topic, ok)
	}
	if got := strings.Join(topic.BoolFlags, ","); got != "--history,--references,--coverage,--json" {
		t.Errorf("topic flags = %q", got)
	}
	effort, ok := Lookup("effort")
	if !ok || len(effort.Children) != 6 {
		t.Fatalf("effort spec = %#v, found %v", effort, ok)
	}
	if newEffort, found := effort.Child("new"); !found || strings.Join(newEffort.BoolFlags, ",") != "--json,--no-worktree" || strings.Join(newEffort.ValueFlags, ",") != "--base" {
		t.Fatalf("effort new spec = %#v, found %v", newEffort, found)
	}
	wantEffortChildren := []string{"new", "list", "show", "finish", "worktree", "integrate"}
	for index, name := range wantEffortChildren {
		if effort.Children[index].Name != name {
			t.Errorf("effort child %d = %q, want %q", index, effort.Children[index].Name, name)
		}
	}
	for _, removed := range []string{"rename", "memory", "complete", "abandon", "reopen", "repair", "integrated"} {
		if _, found := effort.Child(removed); found {
			t.Errorf("removed effort child %q remains", removed)
		}
	}
}

// GatedCommandNames is the exact published gated set, in table order - the
// non-Ungated commands, a group contributing only its own token.
func TestGatedCommandNames(t *testing.T) {
	want := []string{"render", "check", "audit", "effort", "adr", "list", "config", "context", "topic", "new", "enable", "disable"}
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
	// invariant: tooling/cli:cli-command-spec-single-source (TestNamesAndUsageLine)
	if len(names) != len(Commands) || names[0] != "init" {
		t.Errorf("Names() = %v", names)
	}
	if got := UsageLine(); got != "awf <"+strings.Join(names, "|")+">" {
		t.Errorf("UsageLine() = %q", got)
	}
}
