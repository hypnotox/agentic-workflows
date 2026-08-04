package clispec

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func helpText(c Command) string {
	parts := append(append([]string{}, c.Help.Usage...), c.Help.Description)
	parts = append(parts, c.Help.Details...)
	parts = append(parts, c.Help.Examples...)
	parts = append(parts, c.Help.Related...)
	for _, item := range append(append([]HelpItem{}, c.Help.Positionals...), c.Help.Options...) {
		parts = append(parts, item.Name, item.Description)
	}
	return strings.Join(parts, " ")
}

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
		if !strings.Contains(helpText(commit), text) {
			t.Errorf("help missing %q", text)
		}
	}
}

func TestReadPlanSpec(t *testing.T) {
	read, ok := Lookup("read")
	if !ok || read.Gating != Gated {
		t.Fatalf("read spec = %#v, found %v", read, ok)
	}
	plan, ok := read.Child("plan")
	if !ok || plan.MinPos != 2 || plan.MaxPos != 2 {
		t.Fatalf("read plan spec = %#v, found %v", plan, ok)
	}
	for _, text := range []string{"awf read plan <plan> <P[.T]>", "exact filename", "canonical positive", "available"} {
		if !strings.Contains(helpText(plan), text) {
			t.Errorf("read plan help missing %q", text)
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
		if !strings.Contains(helpText(context), text) {
			t.Errorf("help missing %q", text)
		}
	}
}

// Every command and child carries non-empty identifying metadata, and top-level
// names are unique.
func TestCommandsWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range Commands {
		if c.Name == "" || c.Summary == "" || len(c.Help.Usage) == 0 {
			t.Errorf("command %q has empty command metadata", c.Name)
		}
		if seen[c.Name] {
			t.Errorf("duplicate top-level command %q", c.Name)
		}
		seen[c.Name] = true
		if !strings.Contains(helpText(c), "awf "+c.Name) {
			t.Errorf("command %q help missing its usage line", c.Name)
		}
		var walk func(parent string, children []Command)
		walk = func(parent string, children []Command) {
			for _, ch := range children {
				if ch.Name == "" || ch.Summary == "" || len(ch.Help.Usage) == 0 {
					t.Errorf("child %s/%s has empty metadata", parent, ch.Name)
				}
				walk(parent+"/"+ch.Name, ch.Children)
			}
		}
		walk(c.Name, c.Children)
	}
}

// TestCommandHelpSemantics audits the sole registry recursively. It proves
// every structured Help field lowers through the common presentation boundary,
// and that parser-declared flags have complete, truthful option metadata.
// invariant: tooling/cli:cli-command-spec-single-source (TestCommandHelpSemantics)
func TestCommandHelpSemantics(t *testing.T) {
	var walk func(path string, commands []Command)
	walk = func(path string, commands []Command) {
		for _, command := range commands {
			fullPath := strings.TrimSpace(path + " " + command.Name)
			help := command.Help
			if help.Description == "" {
				t.Errorf("%s has no semantic help description", fullPath)
			}
			options := map[string]HelpItem{}
			for _, option := range help.Options {
				if _, duplicate := options[option.Name]; duplicate {
					t.Errorf("%s repeats option %s", fullPath, option.Name)
				}
				options[option.Name] = option
			}
			declaredFlags := append(append([]string{}, command.BoolFlags...), command.ValueFlags...)
			for _, flag := range declaredFlags {
				item, found := options[flag]
				if !found || item.Description == "" {
					t.Errorf("%s parser flag %s lacks an option description", fullPath, flag)
				}
			}
			for option := range options {
				if !contains(declaredFlags, option) {
					t.Errorf("%s documents undeclared option %s", fullPath, option)
				}
			}
			for _, flag := range command.Repeatable {
				if !contains(command.ValueFlags, flag) || options[flag].Description == "" {
					t.Errorf("%s repeatable flag %s is not completely described", fullPath, flag)
				}
			}
			for _, item := range append(append([]HelpItem{}, help.Positionals...), help.Options...) {
				if item.Name == "" || item.Description == "" {
					t.Errorf("%s has incomplete help item %#v", fullPath, item)
				}
				if strings.Contains(item.Description, "input required by this command") || strings.Contains(item.Description, "command positional argument") {
					t.Errorf("%s has placeholder help for %s", fullPath, item.Name)
				}
				if !strings.HasPrefix(item.Name, "--") && !usageNames(item.Name, help.Usage) {
					t.Errorf("%s documents positional %s outside its usage", fullPath, item.Name)
				}
			}
			if len(command.Children) == 0 {
				assertPositionalSemantics(t, fullPath, command)
			}
			for _, example := range help.Examples {
				if !strings.HasPrefix(example, "awf "+fullPath) {
					t.Errorf("%s example %q is not an invocation of that command", fullPath, example)
				}
			}
			for _, related := range help.Related {
				if !strings.HasPrefix(related, "awf ") {
					t.Errorf("%s related command %q is not public CLI syntax", fullPath, related)
				}
			}
			document, err := help.Document("awf "+fullPath, command.Summary)
			if err != nil {
				t.Errorf("%s help document: %v", fullPath, err)
			} else {
				var rendered bytes.Buffer
				if err := presentation.Render(&rendered, document); err != nil {
					t.Errorf("%s help render: %v", fullPath, err)
				}
				for _, text := range helpValues(help) {
					if !strings.Contains(compactHelp(rendered.String()), compactHelp(text)) {
						t.Errorf("%s lowered help omits %q", fullPath, text)
					}
				}
			}
			walk(fullPath, command.Children)
		}
	}
	walk("", Commands)
}

// assertPositionalSemantics compares the documented grammar with the parser's
// bounds. A usage token is one parser operand even when it contains alternatives
// (for example <base>|<a>..<b>), and a documented <add|remove> names one literal
// operand slot. Value-flag operands are skipped, so --owner <uuid> is not treated
// as a command positional.
func assertPositionalSemantics(t *testing.T, path string, command Command) {
	documented := make(map[string]bool, len(command.Help.Positionals))
	for _, positional := range command.Help.Positionals {
		if documented[positional.Name] {
			t.Errorf("%s repeats positional %s", path, positional.Name)
		}
		documented[positional.Name] = false
	}
	maxSlots := 0
	for _, usage := range command.Help.Usage {
		slots, found := usagePositionalSlots(usage, "awf "+path, command.ValueFlags, command.Help.Positionals)
		if slots > maxSlots {
			maxSlots = slots
		}
		for name := range found {
			documented[name] = true
		}
	}
	for name, found := range documented {
		if !found {
			t.Errorf("%s documents positional %s outside its parser grammar", path, name)
		}
	}
	if command.MaxPos >= 0 && maxSlots != command.MaxPos {
		t.Errorf("%s documents %d positional slots, parser maximum is %d", path, maxSlots, command.MaxPos)
	}
	if maxSlots < command.MinPos {
		t.Errorf("%s documents %d positional slots, parser minimum is %d", path, maxSlots, command.MinPos)
	}
}

func usagePositionalSlots(usage, prefix string, valueFlags []string, documented []HelpItem) (int, map[string]bool) {
	found := make(map[string]bool, len(documented))
	rest := strings.TrimSpace(strings.TrimPrefix(usage, prefix))
	tokens := strings.Fields(rest)
	slots := 0
	for i := 0; i < len(tokens); i++ {
		raw := tokens[i]
		token := strings.Trim(raw, "[]")
		if strings.HasPrefix(token, "--") {
			if contains(valueFlags, token) && i+1 < len(tokens) {
				i++
			}
			continue
		}
		matched := false
		for _, item := range documented {
			if positionalTokenMatches(item.Name, raw) {
				found[item.Name] = true
				matched = true
			}
		}
		if matched || strings.Contains(raw, "<") || hasLiteralAlternative(documented, raw) {
			slots++
		}
	}
	return slots, found
}

func positionalTokenMatches(name, token string) bool {
	if strings.Contains(token, name) {
		return true
	}
	if !strings.HasPrefix(name, "<") || !strings.HasSuffix(name, ">") || !strings.Contains(name, "|") {
		return false
	}
	for _, alternative := range strings.Split(name[1:len(name)-1], "|") {
		if token == alternative {
			return true
		}
	}
	return false
}

func hasLiteralAlternative(documented []HelpItem, token string) bool {
	for _, item := range documented {
		if strings.HasPrefix(item.Name, "<") && strings.HasSuffix(item.Name, ">") && strings.Contains(item.Name, "|") {
			for _, alternative := range strings.Split(item.Name[1:len(item.Name)-1], "|") {
				if token == alternative {
					return true
				}
			}
		}
	}
	return false
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// usageNames handles mutually exclusive usage alternatives such as
// <add|remove>: either literal token is valid, but they never occur together.
func usageNames(name string, usage []string) bool {
	if strings.Contains(strings.Join(usage, " "), name) {
		return true
	}
	if strings.HasPrefix(name, "<") && strings.HasSuffix(name, ">") && strings.Contains(name, "|") {
		for _, alternative := range strings.Split(name[1:len(name)-1], "|") {
			if !strings.Contains(strings.Join(usage, " "), alternative) {
				return false
			}
		}
		return true
	}
	return false
}

func compactHelp(text string) string {
	return strings.NewReplacer(" ", "", "\n", "", "\\", "").Replace(text)
}

func helpValues(help Help) []string {
	values := append(append([]string{}, help.Usage...), help.Description)
	values = append(values, help.Details...)
	values = append(values, help.Examples...)
	values = append(values, help.Related...)
	for _, item := range append(append([]HelpItem{}, help.Positionals...), help.Options...) {
		values = append(values, item.Name, item.Description)
	}
	return values
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
	} else if topic.MinPos != 2 || topic.MaxPos != -1 || !strings.Contains(helpText(topic), "without syncing") {
		t.Errorf("new topic spec = %#v", topic)
	}
	if _, ok := newCmd.Child("nope"); ok {
		t.Error("new.Child(nope) should miss")
	}
	topic, ok := Lookup("topic")
	if !ok || topic.MinPos != 1 || topic.MaxPos != 1 || topic.Gating != GatedInHandler {
		t.Fatalf("topic spec = %#v, found %v", topic, ok)
	}
	if got := strings.Join(topic.BoolFlags, ","); got != "--history,--references,--coverage" {
		t.Errorf("topic flags = %q", got)
	}
	effort, ok := Lookup("effort")
	if !ok || len(effort.Children) != 8 {
		t.Fatalf("effort spec = %#v, found %v", effort, ok)
	}
	if newEffort, found := effort.Child("new"); !found || strings.Join(newEffort.BoolFlags, ",") != "--json,--no-worktree" || strings.Join(newEffort.ValueFlags, ",") != "--slug,--base" || !strings.Contains(helpText(newEffort), "awf effort new --slug <slug> <outcome-title>") || !strings.Contains(helpText(newEffort), "1 through 32 bytes") {
		t.Fatalf("effort new spec = %#v, found %v", newEffort, found)
	}
	wantEffortChildren := []string{"new", "list", "show", "finish", "worktree", "integrate", "memory", "activity"}
	for index, name := range wantEffortChildren {
		if effort.Children[index].Name != name {
			t.Errorf("effort child %d = %q, want %q", index, effort.Children[index].Name, name)
		}
	}
	memory, found := effort.Child("memory")
	if !found || len(memory.Children) != 1 || memory.Children[0].Name != "update" || strings.Join(memory.Children[0].ValueFlags, ",") != "--phase,--next" {
		t.Fatalf("effort memory spec = %#v, found %v", memory, found)
	}
	activity, found := effort.Child("activity")
	activityNames := make([]string, len(activity.Children))
	for i, action := range activity.Children {
		activityNames[i] = action.Name
	}
	if !found || strings.Join(activityNames, ",") != "attach,heartbeat,detach" {
		t.Fatalf("effort activity spec = %#v, found %v", activity, found)
	}
	for _, action := range activity.Children {
		if !strings.Contains(strings.Join(action.BoolFlags, ","), "--json") || !strings.Contains(helpText(action), "awf effort activity "+action.Name) {
			t.Errorf("activity %s does not declare JSON-only grammar: %#v", action.Name, action)
		}
	}
	for _, removed := range []string{"rename", "complete", "abandon", "reopen", "repair", "integrated"} {
		if _, found := effort.Child(removed); found {
			t.Errorf("removed effort child %q remains", removed)
		}
	}
}

// GatedCommandNames is the exact published gated set, in table order - the
// non-Ungated commands, a group contributing only its own token.
func TestGatedCommandNames(t *testing.T) {
	want := []string{"render", "check", "read", "audit", "effort", "adr", "list", "config", "context", "topic", "new", "enable", "disable"}
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
