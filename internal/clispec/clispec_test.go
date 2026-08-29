package clispec

import (
	"bytes"
	"fmt"
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

func TestCheckCommitSpecDescribesProfileApplicableAuthorization(t *testing.T) {
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
	if !strings.Contains(commit.Summary, "profile-applicable merge authorization") {
		t.Fatalf("summary = %q", commit.Summary)
	}
	for _, text := range []string{"shared commit rules", "profile-applicable", "unchanged", "git commit"} {
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

func TestPartAuthoringCommandGrammar(t *testing.T) {
	edit, ok := Lookup("edit")
	if !ok || edit.Gating != Gated || edit.MinPos != 3 || edit.MaxPos != 3 {
		t.Fatalf("edit spec = %#v, found %v", edit, ok)
	}
	if got := strings.Join(edit.ValueFlags, ","); got != "--content" {
		t.Fatalf("edit value flags = %q", got)
	}
	if got := strings.Join(edit.BoolFlags, ","); got != "--stdin" {
		t.Fatalf("edit bool flags = %q", got)
	}
	reset, ok := Lookup("reset")
	if !ok || reset.Gating != Gated || reset.MinPos != 3 || reset.MaxPos != 3 || len(reset.ValueFlags)+len(reset.BoolFlags) != 0 {
		t.Fatalf("reset spec = %#v, found %v", reset, ok)
	}
	for _, command := range []Command{edit, reset} {
		for _, token := range []string{"<kind>", "<name>", "<part>"} {
			if !strings.Contains(helpText(command), token) {
				t.Errorf("%s help missing %s", command.Name, token)
			}
		}
	}
}

func TestConfigurationSurfaceGrammar(t *testing.T) {
	for _, retired := range []string{"enable", "disable", "target"} {
		if _, ok := Lookup(retired); ok {
			t.Errorf("retired top-level command %q remains declared", retired)
		}
	}
	newCommand, ok := Lookup("new")
	if !ok {
		t.Fatal("new command is missing")
	}
	var children []string
	for _, child := range newCommand.Children {
		children = append(children, child.Name)
	}
	if got, want := strings.Join(children, ","), "adr,plan,topic,domain,doc,pitfall"; got != want {
		t.Fatalf("new children = %q, want %q", got, want)
	}
}

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

// assertPositionalSemantics compares every documented usage form with the
// parser's bounds. A union such as <base>|<a>..<b> is one parser slot, while
// each named identity in it witnesses its HelpItem. Value-flag operands are
// skipped, so --owner <uuid> is never counted as a command positional.
func assertPositionalSemantics(t *testing.T, path string, command Command) {
	t.Helper()
	for _, problem := range positionalSemanticsProblems(path, command) {
		t.Error(problem)
	}
}

type usageCardinality struct {
	min, max   int // max is -1 when the usage form is variadic.
	identities map[string]bool
}

func positionalSemanticsProblems(path string, command Command) []string {
	documented := make(map[string]bool, len(command.Help.Positionals))
	var problems []string
	for _, positional := range command.Help.Positionals {
		if _, duplicate := documented[positional.Name]; duplicate {
			problems = append(problems, fmt.Sprintf("%s repeats positional %s", path, positional.Name))
		}
		documented[positional.Name] = false
	}

	aggregateMin, aggregateMax := -1, -1
	for _, usage := range command.Help.Usage {
		cardinality, err := usagePositionalCardinality(usage, "awf "+path, command.ValueFlags, command.Help.Positionals)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s usage %q: %v", path, usage, err))
			continue
		}
		if cardinality.min < command.MinPos || (command.MaxPos >= 0 && (cardinality.max < 0 || cardinality.max > command.MaxPos)) {
			problems = append(problems, fmt.Sprintf("%s usage %q has positional cardinality %d..%d outside parser bounds %d..%d", path, usage, cardinality.min, cardinality.max, command.MinPos, command.MaxPos))
		}
		if aggregateMin < 0 {
			aggregateMin, aggregateMax = cardinality.min, cardinality.max
		} else {
			if cardinality.min < aggregateMin {
				aggregateMin = cardinality.min
			}
			if aggregateMax < 0 || cardinality.max < 0 {
				aggregateMax = -1
			} else if cardinality.max > aggregateMax {
				aggregateMax = cardinality.max
			}
		}
		for name := range cardinality.identities {
			documented[name] = true
		}
	}
	for name, found := range documented {
		if !found {
			problems = append(problems, fmt.Sprintf("%s documents positional %s outside its parser grammar", path, name))
		}
	}
	if aggregateMin != command.MinPos {
		problems = append(problems, fmt.Sprintf("%s documented minimum positional cardinality is %d, parser minimum is %d", path, aggregateMin, command.MinPos))
	}
	if aggregateMax != command.MaxPos {
		problems = append(problems, fmt.Sprintf("%s documented maximum positional cardinality is %d, parser maximum is %d", path, aggregateMax, command.MaxPos))
	}
	return problems
}

func usagePositionalCardinality(usage, prefix string, valueFlags []string, documented []HelpItem) (usageCardinality, error) {
	if !strings.HasPrefix(usage, prefix) || (len(usage) > len(prefix) && usage[len(prefix)] != ' ') {
		return usageCardinality{}, fmt.Errorf("does not start with exact command prefix %q", prefix)
	}
	cardinality := usageCardinality{identities: make(map[string]bool, len(documented))}
	tokens := strings.Fields(strings.TrimSpace(strings.TrimPrefix(usage, prefix)))
	for i := 0; i < len(tokens); i++ {
		raw := tokens[i]
		token := strings.Trim(raw, "[]")
		if strings.HasPrefix(token, "--") {
			if contains(valueFlags, token) && i+1 < len(tokens) {
				i++
			}
			continue
		}
		isPositional := strings.Contains(raw, "<") || hasLiteralAlternative(documented, raw)
		if !isPositional {
			continue
		}
		for _, item := range documented {
			if positionalTokenMatches(item.Name, raw) {
				cardinality.identities[item.Name] = true
			}
		}
		if !strings.HasPrefix(raw, "[") {
			cardinality.min++
		}
		if strings.HasSuffix(strings.Trim(raw, "[]"), "...") {
			cardinality.max = -1
		} else if cardinality.max >= 0 {
			cardinality.max++
		}
	}
	return cardinality, nil
}

func TestUsagePositionalCardinality(t *testing.T) {
	items := []HelpItem{{Name: "<base>"}, {Name: "<a>"}, {Name: "<b>"}, {Name: "<slug>"}, {Name: "<uuid>"}}
	for _, test := range []struct {
		name, usage, prefix string
		min, max            int
		identities          []string
	}{
		{"union is one slot", "awf audit <base>|<a>..<b>", "awf audit", 1, 1, []string{"<base>", "<a>", "<b>"}},
		{"optional positional", "awf adr number [<slug>]", "awf adr number", 0, 1, []string{"<slug>"}},
		{"variadic positional", "awf adr number <slug>...", "awf adr number", 1, -1, []string{"<slug>"}},
		{"optional variadic positional", "awf adr number [<slug>...]", "awf adr number", 0, -1, []string{"<slug>"}},
		{"value option operand is skipped", "awf effort activity attach <slug> --owner <uuid>", "awf effort activity attach", 1, 1, []string{"<slug>"}},
		{"exact command prefix", "awf test <slug>", "awf tests", 0, 0, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := usagePositionalCardinality(test.usage, test.prefix, []string{"--owner"}, items)
			if test.name == "exact command prefix" {
				if err == nil {
					t.Fatal("accepted a non-exact command prefix")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.min != test.min || got.max != test.max {
				t.Fatalf("cardinality = %d..%d, want %d..%d", got.min, got.max, test.min, test.max)
			}
			for _, identity := range test.identities {
				if !got.identities[identity] {
					t.Errorf("identity %s not represented", identity)
				}
			}
		})
	}
}

func TestPositionalSemanticsCatchesBoundChanges(t *testing.T) {
	command := Command{MinPos: 1, MaxPos: 1, Help: Help{Usage: []string{"awf test <thing>"}, Positionals: []HelpItem{{Name: "<thing>"}}}}
	if problems := positionalSemanticsProblems("test", command); len(problems) != 0 {
		t.Fatalf("valid grammar problems = %v", problems)
	}
	command.MinPos = 0
	if problems := positionalSemanticsProblems("test", command); !strings.Contains(strings.Join(problems, "\n"), "minimum positional cardinality") {
		t.Fatalf("MinPos 1 -> 0 problems = %v", problems)
	}
	command.MinPos, command.MaxPos = 1, -1
	if problems := positionalSemanticsProblems("test", command); !strings.Contains(strings.Join(problems, "\n"), "maximum positional cardinality") {
		t.Fatalf("MaxPos 1 -> -1 problems = %v", problems)
	}
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

// invariant: tooling/cli:pitfall-scaffold (TestLookup)
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
	if pitfall, ok := newCmd.Child("pitfall"); !ok {
		t.Error("new.Child(pitfall) missing")
	} else if pitfall.MinPos != 1 || pitfall.MaxPos != 1 || !strings.Contains(helpText(pitfall), "without rendering") {
		t.Errorf("new pitfall spec = %#v", pitfall)
	}
	if topic, ok := newCmd.Child("topic"); !ok {
		t.Error("new.Child(topic) missing")
	} else if topic.MinPos != 2 || topic.MaxPos != -1 || !strings.Contains(helpText(topic), "without syncing") {
		t.Errorf("new topic spec = %#v", topic)
	}
	if _, ok := newCmd.Child("nope"); ok {
		t.Error("new.Child(nope) should miss")
	}
	if topic, ok := Lookup("topic"); ok {
		t.Fatalf("legacy topic spec remains: %#v", topic)
	}
	effort, ok := Lookup("effort")
	if !ok || len(effort.Children) != 8 {
		t.Fatalf("effort spec = %#v, found %v", effort, ok)
	}
	if newEffort, found := effort.Child("new"); !found || strings.Join(newEffort.BoolFlags, ",") != "--no-worktree" || strings.Join(newEffort.ValueFlags, ",") != "--slug,--base" || !strings.Contains(helpText(newEffort), "awf effort new --slug <slug> <outcome-title>") || !strings.Contains(helpText(newEffort), "1 through 32 bytes") {
		t.Fatalf("effort new spec = %#v, found %v", newEffort, found)
	}
	wantEffortChildren := []string{"new", "list", "show", "finish", "worktree", "integrate", "memory", "activity"}
	for index, name := range wantEffortChildren {
		if effort.Children[index].Name != name {
			t.Errorf("effort child %d = %q, want %q", index, effort.Children[index].Name, name)
		}
	}
	memory, found := effort.Child("memory")
	if !found || len(memory.Children) != 3 || memory.Children[0].Name != "read" || memory.Children[1].Name != "edit" || memory.Children[2].Name != "update" || strings.Join(memory.Children[0].ValueFlags, ",") != "--offset,--limit,--owner" || strings.Join(memory.Children[2].ValueFlags, ",") != "--phase,--next,--owner" {
		t.Fatalf("effort memory spec = %#v, found %v", memory, found)
	}
	// Only a mutation may declare --preview: a read has nothing to preview, and
	// an accepted but ignored flag is a silently wrong surface.
	for index, wantBoolFlags := range []string{"--json", "--json,--preview", "--json,--preview"} {
		command := memory.Children[index]
		if got := strings.Join(command.BoolFlags, ","); got != wantBoolFlags || !strings.Contains(helpText(command), "--owner") {
			t.Errorf("memory %s bool flags = %q, want %q: %#v", command.Name, got, wantBoolFlags, command)
		}
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
	want := []string{"render", "edit", "reset", "check", "read", "resolve", "audit", "effort", "adr", "list", "config", "new", "remove"}
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
