package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/effort"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"github.com/hypnotox/agentic-workflows/internal/worktree"
)

// invariant: tooling/cli:effort-command-contract (TestEffortMemoryAndActivityCLIContract)
func TestEffortMemoryAndActivityCLIContract(t *testing.T) {
	root := commandRepo(t)
	code, _, stderr := runEffortCLI(t, root, "effort", "new", "Demo", "--no-worktree")
	if code != 0 {
		t.Fatalf("new: %d %s", code, stderr)
	}
	owner := "00000000-0000-4000-8000-000000000001"
	for _, action := range []string{"attach", "heartbeat", "detach"} {
		code, stdout, stderr := runEffortCLI(t, root, "effort", "activity", action, "demo", "--owner", owner, "--json")
		if code != 0 || stderr != "" {
			t.Fatalf("%s: code=%d stderr=%q", action, code, stderr)
		}
		var reply map[string]json.RawMessage
		if err := json.Unmarshal([]byte(stdout), &reply); err != nil {
			t.Fatal(err)
		}
		if got := string(reply["schemaVersion"]); got != "2" {
			t.Fatalf("%s version %s", action, got)
		}
		if action == "detach" {
			if len(reply) != 2 {
				t.Fatalf("detach envelope %#v", reply)
			}
		} else if len(reply) != 5 {
			t.Fatalf("%s envelope %#v", action, reply)
		}
	}
	// Every handled refusal has the closed three-key envelope, and only a
	// genuine resident-read failure carries cause at this JSON boundary.
	decode := func(args ...string) map[string]json.RawMessage {
		t.Helper()
		code, stdout, stderr := runEffortCLI(t, root, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("%v: code=%d stderr=%q", args, code, stderr)
		}
		var reply map[string]json.RawMessage
		if err := json.Unmarshal([]byte(stdout), &reply); err != nil {
			t.Fatal(err)
		}
		return reply
	}
	assertRefusal := func(condition string, cause bool, args ...string) {
		t.Helper()
		reply := decode(args...)
		if got := string(reply["condition"]); got != `"`+condition+`"` || len(reply) != 3 {
			t.Fatalf("%s envelope = %#v", condition, reply)
		}
		var outcome map[string]json.RawMessage
		if err := json.Unmarshal(reply["outcome"], &outcome); err != nil {
			t.Fatal(err)
		}
		if len(outcome) != map[bool]int{true: 5, false: 4}[cause] || outcome["changedActivity"] == nil || (outcome["cause"] != nil) != cause {
			t.Fatalf("%s outcome = %#v", condition, outcome)
		}
	}
	other := "00000000-0000-4000-8000-000000000002"
	decode("effort", "activity", "attach", "demo", "--owner", owner, "--json")
	assertRefusal("not-owner", false, "effort", "activity", "heartbeat", "demo", "--owner", other, "--json")
	decode("effort", "activity", "detach", "demo", "--owner", owner, "--json")
	assertRefusal("missing", false, "effort", "activity", "heartbeat", "demo", "--owner", owner, "--json")
	if err := os.WriteFile(filepath.Join(root, ".awf", "efforts", "demo", "memory.md"), []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	assertRefusal("invalid-memory", false, "effort", "activity", "attach", "demo", "--owner", owner, "--json")
	if err := os.Remove(filepath.Join(root, ".awf", "efforts", "demo", "memory.md")); err != nil {
		t.Fatal(err)
	}
	assertRefusal("invalid-memory", true, "effort", "activity", "attach", "demo", "--owner", owner, "--json")
	if err := os.WriteFile(filepath.Join(root, ".awf", "efforts", "demo", "activity.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	assertRefusal("unsafe-resident", false, "effort", "activity", "detach", "demo", "--owner", owner, "--json")

	for _, args := range [][]string{
		{"effort", "activity", "attach", "Bad", "--owner", owner, "--json"},
		{"effort", "activity", "attach", strings.Repeat("a", 64), "--owner", owner, "--json"},
		{"effort", "activity", "attach", "demo", "--owner", "AAAAAAAA-0000-4000-8000-000000000001", "--json"},
		{"effort", "activity", "resolve", "demo", "--json"},
		{"effort", "activity", "checkout", "demo", "--owner", owner, "--json"},
		{"effort", "activity", "attach", "demo", "--owner", owner, "--cwd", root, "--json"},
		{"effort", "activity", "attach", "demo", "--json"},
	} {
		code, stdout, _ := runEffortCLI(t, root, append([]string(nil), args...)...)
		if code == 0 || stdout != "" {
			t.Fatalf("removed/malformed grammar accepted: %v (%d, %q)", args, code, stdout)
		}
	}
}

func TestEffortActivityCLIRefusalsAreExactV2Envelopes(t *testing.T) {
	root := commandRepo(t)
	owner := "00000000-0000-4000-8000-000000000001"
	for _, args := range [][]string{
		{"effort", "activity", "attach", "missing", "--owner", owner, "--json"},
		{"effort", "activity", "heartbeat", "missing", "--owner", owner, "--json"},
	} {
		code, stdout, stderr := runEffortCLI(t, root, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("refusal transport: %d %q", code, stderr)
		}
		var reply map[string]json.RawMessage
		if err := json.Unmarshal([]byte(stdout), &reply); err != nil {
			t.Fatal(err)
		}
		if len(reply) != 3 || string(reply["schemaVersion"]) != "2" || reply["outcome"] == nil {
			t.Fatalf("refusal envelope: %#v", reply)
		}
	}
}

func TestEffortActivityGrammarHelpers(t *testing.T) {
	for _, action := range []string{"activity attach", "activity heartbeat", "activity detach"} {
		flags := activityRequiredFlags(action)
		if len(flags) != 1 || flags[0] != "--owner" {
			t.Fatalf("%s flags=%v", action, flags)
		}
	}
	if flags := activityRequiredFlags("activity checkout"); flags != nil {
		t.Fatalf("removed flags=%v", flags)
	}
	if err := validateEffortGrammar(&cmdCtx{sub: "activity", inv: invocation{bools: map[string]bool{}, values: map[string]string{}}}); err == nil || err.Error() != "usage: awf effort activity <attach|heartbeat|detach>" {
		t.Fatalf("grammar=%v", err)
	}
	if err := validateEffortActivityGrammar(&cmdCtx{sub: "activity attach", inv: invocation{bools: map[string]bool{"--json": true}, values: map[string]string{}}}); err == nil {
		t.Fatal("missing owner accepted")
	}
	if err := validateEffortActivityGrammar(&cmdCtx{sub: "activity attach", inv: invocation{positionals: []string{"bad_slug"}, bools: map[string]bool{"--json": true}, values: map[string]string{"--owner": "00000000-0000-4000-8000-000000000001"}}}); err == nil {
		t.Fatal("malformed slug accepted")
	}
	if err := validateEffortActivityGrammar(&cmdCtx{sub: "activity attach", inv: invocation{positionals: []string{"demo"}, bools: map[string]bool{"--json": true}, values: map[string]string{"--owner": "00000000-0000-4000-8000-000000000001", "--cwd": "/repo"}}}); err == nil {
		t.Fatal("removed activity flag accepted")
	}
}

func runEffortCLI(t *testing.T, root string, args ...string) (int, string, string) {
	t.Helper()
	t.Chdir(root)
	var stdout, stderr bytes.Buffer
	code := run(append([]string{"awf"}, args...), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func runEffortCommand(t *testing.T, root string, positionals []string, bools map[string]bool) string {
	t.Helper()
	if bools == nil {
		bools = map[string]bool{}
	}
	var out bytes.Buffer
	ctx := &cmdCtx{ctx: testContext(t), root: root, sub: "new", inv: invocation{positionals: positionals, bools: bools, values: map[string]string{}}, stdout: &out}
	if err := runEffort(ctx, openEffortComposition); err != nil {
		t.Fatalf("awf effort new: %v", err)
	}
	return out.String()
}

func commandRepo(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "command repo")
	fixture := gitfixture.InitNativeAt(t, root)
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitfixture.NativeAdd(t, fixture, "tracked.txt")
	for _, resident := range []string{"efforts", "worktrees"} {
		ignore := filepath.Join(root, ".awf", resident, ".gitignore")
		if err := os.MkdirAll(filepath.Dir(ignore), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ignore, []byte("*\n!.gitignore\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		gitfixture.NativeAdd(t, fixture, ".awf/"+resident+"/.gitignore")
	}
	gitfixture.NativeCommit(t, fixture, "base")
	return root
}

type effortErrorWriter struct{}

func (effortErrorWriter) Write([]byte) (int, error) { return 0, os.ErrClosed }

func TestEffortOutputAndGrammarBranches(t *testing.T) {
	root := commandRepo(t)
	if err := runEffort(&cmdCtx{ctx: testContext(t), root: filepath.Join(root, "missing"), sub: "list", inv: invocation{bools: map[string]bool{}, values: map[string]string{}}, stdout: &bytes.Buffer{}}, openEffortComposition); err == nil {
		t.Fatal("invalid root accepted")
	}
	for _, c := range []*cmdCtx{
		{sub: "memory", inv: invocation{bools: map[string]bool{}, values: map[string]string{}}},
		{sub: "memory update", inv: invocation{bools: map[string]bool{}, values: map[string]string{}}},
		{sub: "new", inv: invocation{bools: map[string]bool{"--no-worktree": true}, values: map[string]string{"--base": "HEAD"}}},
		{sub: "worktree", inv: invocation{positionals: []string{"add"}, bools: map[string]bool{}, values: map[string]string{}}},
		{sub: "worktree", inv: invocation{positionals: []string{"remove", "x"}, bools: map[string]bool{}, values: map[string]string{"--base": "HEAD"}}},
		{sub: "worktree", inv: invocation{positionals: []string{"other", "x"}, bools: map[string]bool{}, values: map[string]string{}}},
	} {
		if err := validateEffortGrammar(c); err == nil {
			t.Fatalf("invalid grammar accepted: %s", c.sub)
		}
	}
	if err := validateEffortGrammar(&cmdCtx{sub: "memory update", inv: invocation{bools: map[string]bool{}, values: map[string]string{"--phase": "p"}}}); err != nil {
		t.Fatal(err)
	}
	if err := validateEffortActivityGrammar(&cmdCtx{sub: "activity attach", inv: invocation{positionals: []string{"demo"}, bools: map[string]bool{}, values: map[string]string{"--owner": "00000000-0000-4000-8000-000000000001"}}}); err == nil {
		t.Fatal("non-json activity accepted")
	}
	if err := validateEffortActivityGrammar(&cmdCtx{sub: "activity attach", inv: invocation{positionals: []string{"demo"}, bools: map[string]bool{"--json": true}, values: map[string]string{"--owner": "00000000-0000-4000-8000-000000000001"}}}); err != nil {
		t.Fatal(err)
	}
	if value := effortValue(invocation{values: map[string]string{"--phase": "p"}}, "--phase"); value == nil || *value != "p" {
		t.Fatalf("value = %v", value)
	}
	if value := effortValue(invocation{values: map[string]string{}}, "--phase"); value != nil {
		t.Fatal("absent value present")
	}
	record := effort.Record{SchemaVersion: effort.SchemaVersion, Slug: "presentation", Title: "Presentation", MemoryPath: ".awf/efforts/presentation/memory.md"}
	for _, jsonOutput := range []bool{false, true} {
		var out bytes.Buffer
		if err := writeEffort(&out, record, jsonOutput); err != nil {
			t.Fatal(err)
		}
		if out.Len() == 0 {
			t.Fatal("empty effort output")
		}
	}
	if err := writeEffortText(effortErrorWriter{}, record); err == nil {
		t.Fatal("text writer error ignored")
	}
	if err := writeEffortJSON(effortErrorWriter{}, record); err == nil {
		t.Fatal("JSON writer error ignored")
	}
	if got := yesNo(true); got != "yes" {
		t.Fatalf("yes = %q", got)
	}
	if got := yesNo(false); got != "no" {
		t.Fatalf("no = %q", got)
	}
	if err := writeWorktreeResult(&bytes.Buffer{}, worktree.Result{}, os.ErrInvalid); !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("result error = %v", err)
	}
	if err := writeWorktreeResult(effortErrorWriter{}, worktree.Result{Condition: "done"}, nil); err == nil {
		t.Fatal("worktree writer error ignored")
	}
	for _, args := range [][]string{
		{"effort", "new", "Command branches", "--no-worktree", "--json"},
		{"effort", "show", "command-branches"},
		{"effort", "show", "command-branches", "--json"},
		{"effort", "list"},
		{"effort", "list", "--json"},
		{"effort", "memory", "update", "command-branches", "--phase", "one", "--next", "two"},
		{"effort", "finish", "command-branches"},
	} {
		code, _, stderr := runEffortCLI(t, root, args...)
		if code != 0 {
			t.Fatalf("command %v: %d %s", args, code, stderr)
		}
	}
	for _, c := range []*cmdCtx{
		{ctx: testContext(t), root: root, sub: "worktree", inv: invocation{positionals: []string{"add", "missing-worktree"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &bytes.Buffer{}},
		{ctx: testContext(t), root: root, sub: "worktree", inv: invocation{positionals: []string{"remove", "missing-worktree"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &bytes.Buffer{}},
		{ctx: testContext(t), root: root, sub: "integrate", inv: invocation{positionals: []string{"missing-worktree"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &bytes.Buffer{}},
	} {
		_ = runEffort(c, openEffortComposition)
	}
	for _, sub := range []string{"new", "show", "list", "finish"} {
		inv := invocation{bools: map[string]bool{}, values: map[string]string{}}
		if sub != "list" {
			inv.positionals = []string{"missing"}
		}
		err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: sub, inv: inv, stdout: effortErrorWriter{}}, openEffortComposition)
		if sub == "list" && err == nil {
			t.Fatal("list writer error ignored")
		}
	}
}

func TestRunEffortFailureDispatches(t *testing.T) {
	root := commandRepo(t)
	ctx := func(sub string, positions ...string) *cmdCtx {
		return &cmdCtx{ctx: testContext(t), root: root, sub: sub, inv: invocation{positionals: positions, bools: map[string]bool{"--json": true}, values: map[string]string{}}, stdout: &bytes.Buffer{}}
	}
	newCtx := ctx("new", "Duplicate")
	newCtx.inv.bools["--no-worktree"] = true
	if err := runEffort(newCtx, openEffortComposition); err != nil {
		t.Fatal(err)
	}
	if err := runEffort(newCtx, openEffortComposition); err == nil {
		t.Fatal("duplicate new accepted")
	}
	if err := os.WriteFile(filepath.Join(root, ".awf", "efforts", "broken"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runEffort(ctx("list"), openEffortComposition); err == nil {
		t.Fatal("unsafe list accepted")
	}
	if err := os.Remove(filepath.Join(root, ".awf", "efforts", "broken")); err != nil {
		t.Fatal(err)
	}
	if err := runEffort(ctx("show", "missing"), openEffortComposition); err == nil {
		t.Fatal("missing show accepted")
	}
	if err := runEffort(ctx("removed"), func(context.Context, string) (effortComposition, error) { return effortComposition{}, nil }); err == nil {
		t.Fatal("unknown effort action accepted")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("unknown activity action did not panic")
			}
		}()
		_ = runEffortActivity(ctx("activity removed", "duplicate"), nil)
	}()
}

var _ = strings.Contains
