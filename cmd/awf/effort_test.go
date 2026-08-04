package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/effort"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"github.com/hypnotox/agentic-workflows/internal/worktree"
)

func writePersistedEffortFixture(t *testing.T, root, slug string) {
	t.Helper()
	resident := filepath.Join(root, ".awf", "efforts", slug)
	if err := os.MkdirAll(resident, 0o700); err != nil {
		t.Fatal(err)
	}
	state := fmt.Sprintf(`{"schemaVersion":2,"id":"018f47a0-7b3d-4c52-8f1a-123456789abc","slug":%q,"title":"Persisted resident","createdAt":"2026-08-03T00:00:00Z"}`, slug)
	memory := fmt.Sprintf("---\neffort: %s\nphase: Persisted compatibility\nnext: Exercise resident operations\nupdated: %q\n---\n## Brief\n\nPersisted compatibility fixture.\n\n## Decision log\n\n## Observations\n\n## Handoff log\n", slug, "2026-08-03T00:00:00Z")
	if err := os.WriteFile(filepath.Join(resident, "state.json"), []byte(state), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(resident, "memory.md"), []byte(memory), 0o600); err != nil {
		t.Fatal(err)
	}
}

// invariant: tooling/effort-management:effort-record-authority (TestPersisted63ByteEffortRemainsOperable)
func TestPersisted63ByteEffortRemainsOperable(t *testing.T) {
	root := commandRepo(t)
	slug := strings.Repeat("r", 63)
	writePersistedEffortFixture(t, root, slug)

	if shown := runEffortCommand(t, root, "show", []string{slug}); !strings.Contains(shown, slug) {
		t.Fatalf("show omitted resident slug: %q", shown)
	}
	if listed := runEffortCommand(t, root, "list", nil); !strings.Contains(listed, slug) {
		t.Fatalf("list omitted resident slug: %q", listed)
	}
	code, stdout, stderr := runEffortCLI(t, root, "effort", "memory", "update", slug, "--phase", "Still operable")
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("memory update code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	owner := "128f47a0-7b3d-4c52-8f1a-123456789abc"
	for _, args := range [][]string{
		{"effort", "activity", "attach", slug, "--owner", owner, "--json"},
		{"effort", "activity", "detach", slug, "--owner", owner, "--json"},
	} {
		code, stdout, stderr = runEffortCLI(t, root, args...)
		if code != 0 || !strings.Contains(stdout, `"schemaVersion":2`) || stderr != "" {
			t.Fatalf("%q code=%d stdout=%q stderr=%q", args, code, stdout, stderr)
		}
	}
	if output := runEffortCommand(t, root, "worktree", []string{"add", slug}); !strings.Contains(output, slug) {
		t.Fatalf("worktree add omitted slug: %q", output)
	}
	if output := runEffortCommand(t, root, "worktree", []string{"remove", slug}); !strings.Contains(output, "managed worktree topology is absent") {
		t.Fatalf("worktree remove did not settle topology: %q", output)
	}
	if output := runEffortCommand(t, root, "finish", []string{slug}); !strings.Contains(output, "finishing cleanup") {
		t.Fatalf("finish did not clean resident: %q", output)
	}
}

// invariant: tooling/cli:effort-command-contract (TestEffortNewExplicitSlugGrammarAndFlagCombinations)
func TestEffortNewExplicitSlugGrammarAndFlagCombinations(t *testing.T) {
	for _, args := range [][]string{
		{"effort", "new", "--slug", "ordered-input", "Ordered title", "--no-worktree"},
		{"effort", "new", "Ordered title", "--no-worktree", "--slug", "ordered-input"},
		{"effort", "new", "--slug", "ordered-input", "--base", "HEAD", "Ordered title"},
		{"effort", "new", "Ordered title", "--base", "HEAD", "--slug", "ordered-input"},
	} {
		root := commandRepo(t)
		code, stdout, stderr := runEffortCLI(t, root, args...)
		if code != 0 || stderr != "" || !strings.Contains(stdout, "effort: ordered-input") || !strings.Contains(stdout, "title: Ordered title") {
			t.Fatalf("%q code=%d stdout=%q stderr=%q", args, code, stdout, stderr)
		}
	}
	root := commandRepo(t)
	composed := false
	err := runEffort(&cmdCtx{
		ctx:    testContext(t),
		root:   root,
		sub:    "new",
		inv:    invocation{positionals: []string{"Missing slug"}, bools: map[string]bool{}, values: map[string]string{}},
		stdout: &bytes.Buffer{},
	}, func(_ context.Context, _ string) (effortComposition, error) {
		composed = true
		return effortComposition{}, errors.New("composer invoked")
	})
	if err == nil || !strings.Contains(err.Error(), "--slug is required") || composed {
		t.Fatalf("missing slug err=%v composed=%t", err, composed)
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"effort", "new", "Missing slug"}, want: "--slug is required"},
		{args: []string{"effort", "new", "Valueless slug", "--slug"}, want: "needs a value"},
		{args: []string{"effort", "new", "--slug", "one", "Duplicate slug", "--slug", "two"}, want: "given more than once"},
		{args: []string{"effort", "new", "--slug", "flag-title", "-title"}, want: "unknown flag"},
	} {
		code, stdout, stderr := runEffortCLI(t, root, test.args...)
		if code == 0 || stdout != "" || !strings.Contains(stderr, test.want) {
			t.Fatalf("%q code=%d stdout=%q stderr=%q, want %q", test.args, code, stdout, stderr, test.want)
		}
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"effort", "new", "--json", "--slug", "json-new", "JSON new", "--no-worktree"}, "condition: awf: awf new: unknown flag \"--json\"\n"},
		{[]string{"effort", "list", "--json"}, "condition: awf: awf list: unknown flag \"--json\"\n"},
		{[]string{"effort", "show", "ordered-input", "--json"}, "condition: awf: awf show: unknown flag \"--json\"\n"},
	} {
		code, stdout, stderr := runEffortCLI(t, root, test.args...)
		if code != 2 || stdout != "" || stderr != test.want {
			t.Fatalf("%q code=%d stdout=%q stderr=%q want=%q", test.args, code, stdout, stderr, test.want)
		}
	}
	code, stdout, stderr := runEffortCLI(t, root, "effort", "new", "--slug", "readable", "Readable contract", "--no-worktree")
	newWant := "status: no managed worktree\n\nmutation:\n  identity:\n    effort: readable\n    title: Readable contract\n    memory: .awf/efforts/readable/memory.md\n  next actions:\n    step 1: continue the effort in " + root + "\n"
	if code != 0 || stderr != "" || stdout != newWant {
		t.Fatalf("readable new code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"effort", "list"}, "effort list:\n  efforts:\n    readable: Readable contract\n"},
		{[]string{"effort", "show", "readable"}, "slug: readable\ntitle: Readable contract\nmemory: .awf/efforts/readable/memory.md\n"},
	} {
		code, stdout, stderr = runEffortCLI(t, root, test.args...)
		if code != 0 || stderr != "" || stdout != test.want {
			t.Fatalf("readable %q code=%d stdout=%q stderr=%q", test.args, code, stdout, stderr)
		}
	}
	overlong := strings.Repeat("s", 33)
	code, stdout, stderr = runEffortCLI(t, root, "effort", "new", "--slug", overlong, "Overlong slug", "--no-worktree")
	if code == 0 || stdout != "" || !strings.Contains(stderr, "1-32 bytes") || !strings.Contains(stderr, "changed bytes: no") || !strings.Contains(stderr, "--slug") {
		t.Fatalf("33-byte slug code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Lstat(filepath.Join(root, ".awf", "efforts", overlong)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("33-byte new slug changed residents: %v", err)
	}
	writePersistedEffortFixture(t, root, overlong)
	if shown := runEffortCommand(t, root, "show", []string{overlong}); !strings.Contains(shown, overlong) {
		t.Fatalf("same 33-byte persisted slug is not selectable: %q", shown)
	}
}

// invariant: tooling/cli:effort-command-contract (TestEffortMemoryAndActivityCLIContract)
func TestEffortMemoryAndActivityCLIContract(t *testing.T) {
	// The protocol writer, rather than a decoded map, owns these bytes. Fixed
	// values prove its field order, all transport values, JSON escaping, and
	// newline framing independently of clock-backed CLI activity mutations.
	at := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
	for name, reply := range map[string]struct {
		reply effort.ActivityReply
		want  string
	}{
		"attach": {
			reply: effort.ActivityReply{SchemaVersion: 2, Condition: effort.ActivityAttached, Effort: &effort.ActivityEffort{Slug: "demo", Title: "Demo \\\"quoted\\\" <tag>"}, Memory: &effort.MemoryMetadata{Effort: "demo", Phase: "Phase  4", Next: "Next\tstep", Updated: "2024-01-02T03:04:05Z"}, Activity: &effort.Activity{SchemaVersion: 2, Owner: "00000000-0000-4000-8000-000000000001", AttachedAt: at, HeartbeatAt: at}},
			want:  "{\"schemaVersion\":2,\"condition\":\"attached\",\"effort\":{\"slug\":\"demo\",\"title\":\"Demo \\\\\\\"quoted\\\\\\\" \\u003ctag\\u003e\"},\"memory\":{\"effort\":\"demo\",\"phase\":\"Phase  4\",\"next\":\"Next\\tstep\",\"updated\":\"2024-01-02T03:04:05Z\"},\"activity\":{\"schemaVersion\":2,\"owner\":\"00000000-0000-4000-8000-000000000001\",\"attachedAt\":\"2024-01-02T03:04:05Z\",\"heartbeatAt\":\"2024-01-02T03:04:05Z\"}}\n",
		},
		"heartbeat": {
			reply: effort.ActivityReply{SchemaVersion: 2, Condition: effort.ActivityHeartbeat, Effort: &effort.ActivityEffort{Slug: "demo", Title: "Demo"}, Memory: &effort.MemoryMetadata{Effort: "demo", Phase: "Phase", Next: "Next", Updated: "2024-01-02T03:04:05Z"}, Activity: &effort.Activity{SchemaVersion: 2, Owner: "00000000-0000-4000-8000-000000000001", AttachedAt: at, HeartbeatAt: at.Add(time.Second)}},
			want:  "{\"schemaVersion\":2,\"condition\":\"heartbeat\",\"effort\":{\"slug\":\"demo\",\"title\":\"Demo\"},\"memory\":{\"effort\":\"demo\",\"phase\":\"Phase\",\"next\":\"Next\",\"updated\":\"2024-01-02T03:04:05Z\"},\"activity\":{\"schemaVersion\":2,\"owner\":\"00000000-0000-4000-8000-000000000001\",\"attachedAt\":\"2024-01-02T03:04:05Z\",\"heartbeatAt\":\"2024-01-02T03:04:06Z\"}}\n",
		},
		"detach": {
			reply: effort.ActivityReply{SchemaVersion: 2, Condition: effort.ActivityDetached},
			want:  "{\"schemaVersion\":2,\"condition\":\"detached\"}\n",
		},
		"refusal": {
			reply: effort.ActivityReply{SchemaVersion: 2, Condition: effort.ActivityInvalidMemory, Outcome: &effort.ActionableOutcome{Category: "operation", Condition: "memory cannot be \\\"read\\\"", ChangedActivity: false, NextActions: []string{"inspect <resident>", "repair\\now"}, Cause: "read <failure>"}},
			want:  "{\"schemaVersion\":2,\"condition\":\"invalid-memory\",\"outcome\":{\"category\":\"operation\",\"condition\":\"memory cannot be \\\\\\\"read\\\\\\\"\",\"changedActivity\":false,\"nextActions\":[\"inspect \\u003cresident\\u003e\",\"repair\\\\now\"],\"cause\":\"read \\u003cfailure\\u003e\"}}\n",
		},
	} {
		t.Run("protocol bytes/"+name, func(t *testing.T) {
			var out bytes.Buffer
			if err := writeActivityReply(&out, reply.reply); err != nil {
				t.Fatal(err)
			}
			if got := out.String(); got != reply.want {
				t.Fatalf("reply bytes = %q, want %q", got, reply.want)
			}
		})
	}

	root := commandRepo(t)
	code, _, stderr := runEffortCLI(t, root, "effort", "new", "--slug", "demo", "Demo", "--no-worktree")
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
	assertRefusal := func(condition, observed string, actions []string, cause bool, args ...string) {
		t.Helper()
		reply := decode(args...)
		if got := string(reply["schemaVersion"]); got != "2" || string(reply["condition"]) != `"`+condition+`"` || len(reply) != 3 {
			t.Fatalf("%s envelope = %#v", condition, reply)
		}
		var outcome struct {
			Category        string   `json:"category"`
			Condition       string   `json:"condition"`
			ChangedActivity bool     `json:"changedActivity"`
			NextActions     []string `json:"nextActions"`
			Cause           string   `json:"cause"`
		}
		if err := json.Unmarshal(reply["outcome"], &outcome); err != nil {
			t.Fatal(err)
		}
		var rawOutcome map[string]json.RawMessage
		if err := json.Unmarshal(reply["outcome"], &rawOutcome); err != nil {
			t.Fatal(err)
		}
		if len(rawOutcome) != map[bool]int{true: 5, false: 4}[cause] || outcome.Category != "operation" || outcome.Condition != observed || outcome.ChangedActivity || !slices.Equal(outcome.NextActions, actions) || (outcome.Cause != "") != cause {
			t.Fatalf("%s outcome = %#v raw=%#v", condition, outcome, rawOutcome)
		}
	}
	other := "00000000-0000-4000-8000-000000000002"
	decode("effort", "activity", "attach", "demo", "--owner", owner, "--json")
	assertRefusal("not-owner", "the resident owner differs from this invocation", []string{"confirm the active session owner", "attach this session owner to take over"}, false, "effort", "activity", "heartbeat", "demo", "--owner", other, "--json")
	decode("effort", "activity", "detach", "demo", "--owner", owner, "--json")
	assertRefusal("missing", "the requested activity is absent", []string{"inspect the requested activity resident", "attach a new activity"}, false, "effort", "activity", "heartbeat", "demo", "--owner", owner, "--json")
	if err := os.WriteFile(filepath.Join(root, ".awf", "efforts", "demo", "memory.md"), []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	memoryActions := []string{"inspect .awf/efforts/demo/memory.md", "repair .awf/efforts/demo/memory.md manually"}
	assertRefusal("invalid-memory", "the effort memory metadata is invalid", memoryActions, false, "effort", "activity", "attach", "demo", "--owner", owner, "--json")
	if err := os.Remove(filepath.Join(root, ".awf", "efforts", "demo", "memory.md")); err != nil {
		t.Fatal(err)
	}
	assertRefusal("invalid-memory", "the effort memory cannot be read", memoryActions, true, "effort", "activity", "attach", "demo", "--owner", owner, "--json")
	if err := os.WriteFile(filepath.Join(root, ".awf", "efforts", "demo", "activity.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	assertRefusal("unsafe-resident", "the activity resident cannot be safely used", []string{"inspect the unsafe resident", "repair the unsafe resident"}, false, "effort", "activity", "detach", "demo", "--owner", owner, "--json")

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

func runNewEffortCommand(t *testing.T, root, slug, title string, bools map[string]bool) string {
	t.Helper()
	if bools == nil {
		bools = map[string]bool{}
	}
	var out bytes.Buffer
	ctx := &cmdCtx{ctx: testContext(t), root: root, sub: "new", inv: invocation{positionals: []string{title}, bools: bools, values: map[string]string{"--slug": slug}}, stdout: &out}
	if err := runEffort(ctx, openEffortComposition); err != nil {
		t.Fatalf("awf effort new: %v", err)
	}
	return out.String()
}

func runEffortCommand(t *testing.T, root, sub string, positionals []string) string {
	t.Helper()
	bools := map[string]bool{}
	var out bytes.Buffer
	ctx := &cmdCtx{ctx: testContext(t), root: root, sub: sub, inv: invocation{positionals: positionals, bools: bools, values: map[string]string{}}, stdout: &out}
	if err := runEffort(ctx, openEffortComposition); err != nil {
		t.Fatalf("awf effort %s: %v", sub, err)
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
	var out bytes.Buffer
	if err := writeEffort(&out, record); err != nil {
		t.Fatal(err)
	}
	if out.Len() == 0 {
		t.Fatal("empty effort output")
	}
	if err := writeEffort(effortErrorWriter{}, record); err == nil {
		t.Fatal("text writer error ignored")
	}
	if err := writeEffortJSON(effortErrorWriter{}, record); err == nil {
		t.Fatal("JSON writer error ignored")
	}
	if err := writeWorktreeResult(&bytes.Buffer{}, worktree.Result{}, os.ErrInvalid); !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("result error = %v", err)
	}
	if err := writeWorktreeResult(effortErrorWriter{}, worktree.Result{Condition: "done"}, nil); err == nil {
		t.Fatal("worktree writer error ignored")
	}
	for _, args := range [][]string{
		{"effort", "new", "--slug", "command-branches", "Command branches", "--no-worktree"},
		{"effort", "show", "command-branches"},
		{"effort", "list"},
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
		if sub == "new" {
			inv.values["--slug"] = "missing"
		}
		err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: sub, inv: inv, stdout: effortErrorWriter{}}, openEffortComposition)
		if sub == "list" && err == nil {
			t.Fatal("list writer error ignored")
		}
	}
}

func TestEffortPublicTextProtocol(t *testing.T) {
	root := commandRepo(t)
	run := func(args ...string) string {
		t.Helper()
		code, stdout, stderr := runEffortCLI(t, root, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("%v: code=%d stdout=%q stderr=%q", args, code, stdout, stderr)
		}
		return stdout
	}
	if got, want := run("effort", "new", "--slug", "public-output", "Public output", "--no-worktree"), fmt.Sprintf("status: no managed worktree\n\nmutation:\n  identity:\n    effort: public-output\n    title: Public output\n    memory: .awf/efforts/public-output/memory.md\n  next actions:\n    step 1: continue the effort in %s\n", root); got != want {
		t.Fatalf("initial new output = %q, want %q", got, want)
	}
	managed := filepath.Join(root, ".awf", "worktrees", "public-output")
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"effort", "list"}, "effort list:\n  efforts:\n    public-output: Public output\n"},
		{[]string{"effort", "show", "public-output"}, "slug: public-output\ntitle: Public output\nmemory: .awf/efforts/public-output/memory.md\n"},
		{[]string{"effort", "worktree", "add", "public-output"}, fmt.Sprintf("status: managed worktree added for public-output\n\nmutation:\n  identity:\n    worktree: %s\n    branch: awf/public-output\n  changes:\n    completed:\n      managed topology\n  next actions:\n    step 1: continue the effort in %s\n", managed, managed)},
		{[]string{"effort", "integrate", "public-output"}, "status: effort tip is already integrated into the target\n\nmutation:\n  next actions:\n    step 1: run `awf effort worktree remove public-output` after terminal review is settled\n"},
		{[]string{"effort", "worktree", "remove", "public-output"}, "status: managed worktree topology is absent\n\nmutation:\n  changes:\n    completed:\n      managed topology\n  next actions:\n    step 1: continue to retrospective, then finish the effort\n"},
		{[]string{"effort", "finish", "public-output"}, "status: completed\n\nmutation:\n  identity:\n    effort: public-output\n  changes:\n    completed:\n      active resident\n      finishing cleanup\n  next actions:\n    step 1: continue without this finished effort\n"},
	} {
		if got := run(test.args...); got != test.want {
			t.Fatalf("%v output = %q, want %q", test.args, got, test.want)
		}
	}
	code, stdout, stderr := runEffortCLI(t, root, "effort", "finish", "public-output")
	const restart = "condition: awf: effort \"public-output\" has no active resident or finishing reservation; changed bytes: no; next action: run `awf effort list` and use an active slug\n"
	if code != 1 || stdout != "" || stderr != restart {
		t.Fatalf("restarted finish: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestRunEffortFailureDispatches(t *testing.T) {
	root := commandRepo(t)
	ctx := func(sub string, positions ...string) *cmdCtx {
		return &cmdCtx{ctx: testContext(t), root: root, sub: sub, inv: invocation{positionals: positions, bools: map[string]bool{"--json": true}, values: map[string]string{}}, stdout: &bytes.Buffer{}}
	}
	newCtx := ctx("new", "Duplicate")
	newCtx.inv.bools["--no-worktree"] = true
	newCtx.inv.values["--slug"] = "duplicate"
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
