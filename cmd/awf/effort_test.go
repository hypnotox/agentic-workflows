package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/effort"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"github.com/hypnotox/agentic-workflows/internal/worktree"
)

func TestEffortCheckoutResolutionAdapterNormalizesGitErrors(t *testing.T) {
	for category, want := range map[string]effort.CheckoutResolutionKind{
		"symlink": effort.CheckoutUnsafe, "foreign-owner": effort.CheckoutUnsafe, "file-type": effort.CheckoutUnsafe, "resident-permissions": effort.CheckoutUnsafe,
		"repository-identity": effort.CheckoutRepositoryMismatch, "bare-repository": effort.CheckoutRepositoryMismatch, "missing-primary": effort.CheckoutRepositoryMismatch, "ambiguous-primary": effort.CheckoutRepositoryMismatch, "unconfined": effort.CheckoutRepositoryMismatch,
	} {
		err := normalizeCheckoutResolutionError(&awfgit.HardSafetyError{Category: category})
		if err.Kind() != want || errors.Unwrap(err) == nil {
			t.Fatalf("%s = %#v", category, err)
		}
		var hard *awfgit.HardSafetyError
		if errors.As(err, &hard) {
			t.Fatalf("%s leaked HardSafetyError", category)
		}
	}
	err := normalizeCheckoutResolutionError(&awfgit.CommandError{Args: []string{"worktree", "list"}, ExitCode: 1, Stderr: "failed"})
	var command *awfgit.CommandError
	if errors.As(err, &command) {
		t.Fatalf("command error leaked: %v", err)
	}
}

func TestEffortProtocol2CLIFromPrimaryAndLinkedWorktrees(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	// invariant: tooling/cli:effort-command-contract (TestEffortProtocol2CLIFromPrimaryAndLinkedWorktrees)
	// invariant: tooling/effort-management:effort-record-authority (TestEffortProtocol2CLIFromPrimaryAndLinkedWorktrees)
	primary := filepath.Join(t.TempDir(), "primary with spaces")
	fixture := gitfixture.InitNativeAt(t, primary)
	if err := os.WriteFile(filepath.Join(primary, "tracked.txt"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitfixture.NativeAdd(t, fixture, "tracked.txt")
	gitfixture.NativeCommit(t, fixture, "base")
	linked := filepath.Join(filepath.Dir(primary), "linked with spaces")
	gitfixture.NativeWorktreeAddDetached(t, fixture, linked, "HEAD")

	createdJSON := runNewEffortCommand(t, primary, "cli-outcome", "CLI outcome", map[string]bool{"--json": true, "--no-worktree": true})
	var created struct {
		SchemaVersion int                  `json:"schemaVersion"`
		Effort        effort.Record        `json:"effort"`
		Worktree      *effortWorktreeFacts `json:"worktree"`
	}
	if err := json.Unmarshal([]byte(createdJSON), &created); err != nil {
		t.Fatal(err)
	}
	if created.SchemaVersion != 2 || created.Effort.Slug != "cli-outcome" || created.Effort.Title != "CLI outcome" || created.Effort.MemoryPath != ".awf/efforts/cli-outcome/memory.md" || created.Worktree != nil {
		t.Fatalf("new JSON = %#v", created)
	}
	resident := filepath.Join(primary, ".awf", "efforts", "cli-outcome")
	if _, err := os.Stat(filepath.Join(resident, "state.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(resident, "memory.md")); err != nil {
		t.Fatal(err)
	}
	shownJSON := runEffortCommand(t, linked, "show", []string{"cli-outcome"}, map[string]bool{"--json": true})
	var shown struct {
		SchemaVersion int           `json:"schemaVersion"`
		Effort        effort.Record `json:"effort"`
	}
	if err := json.Unmarshal([]byte(shownJSON), &shown); err != nil {
		t.Fatal(err)
	}
	primaryMemory := filepath.Join(primary, ".awf", "efforts", "cli-outcome", "memory.md")
	qualified := created.Effort
	qualified.MemoryPath = primaryMemory
	if shown.SchemaVersion != 2 || shown.Effort != qualified {
		t.Fatalf("show JSON from the linked checkout = %#v, want the created effort with memoryPath %q", shown, primaryMemory)
	}
	if text := runEffortCommand(t, linked, "show", []string{"cli-outcome"}, nil); !strings.Contains(text, "memory="+primaryMemory) {
		t.Fatalf("linked show text = %q, want the primary-root-qualified memory fact", text)
	}
	listedJSON := runEffortCommand(t, linked, "list", nil, map[string]bool{"--json": true})
	var listed struct {
		SchemaVersion int             `json:"schemaVersion"`
		Efforts       []effort.Record `json:"efforts"`
	}
	if err := json.Unmarshal([]byte(listedJSON), &listed); err != nil {
		t.Fatal(err)
	}
	if listed.SchemaVersion != 2 || len(listed.Efforts) != 1 || listed.Efforts[0].Slug != "cli-outcome" || listed.Efforts[0].MemoryPath != primaryMemory {
		t.Fatalf("list JSON = %#v", listed)
	}
	if text := runEffortCommand(t, primary, "show", []string{"cli-outcome"}, nil); !strings.Contains(text, "effort cli-outcome") || !strings.Contains(text, "memory=.awf/efforts/cli-outcome/memory.md") {
		t.Fatalf("show text = %q", text)
	}

	finished := runEffortCommand(t, primary, "finish", []string{"cli-outcome"}, nil)
	if !strings.Contains(finished, "changed active rename: yes") || !strings.Contains(finished, "changed cleanup: yes") {
		t.Fatalf("finish output = %q", finished)
	}
	if _, err := os.Stat(resident); !os.IsNotExist(err) {
		t.Fatalf("finished resident remains: %v", err)
	}
}

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

	if shown := runEffortCommand(t, root, "show", []string{slug}, map[string]bool{"--json": true}); !strings.Contains(shown, slug) {
		t.Fatalf("show omitted resident slug: %q", shown)
	}
	if listed := runEffortCommand(t, root, "list", nil, map[string]bool{"--json": true}); !strings.Contains(listed, slug) {
		t.Fatalf("list omitted resident slug: %q", listed)
	}
	code, stdout, stderr := runEffortCLI(t, root, "effort", "memory", "update", slug, "--phase", "Still operable")
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("memory update code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	owner := "128f47a0-7b3d-4c52-8f1a-123456789abc"
	for _, args := range [][]string{
		{"effort", "activity", "attach", slug, "--owner", owner, "--cwd", root, "--role", "receiving", "--receiving-checkout", root, "--json"},
		{"effort", "activity", "detach", slug, "--owner", owner, "--json"},
	} {
		code, stdout, stderr = runEffortCLI(t, root, args...)
		if code != 0 || !strings.Contains(stdout, `"schemaVersion":1`) || stderr != "" {
			t.Fatalf("%q code=%d stdout=%q stderr=%q", args, code, stdout, stderr)
		}
	}
	if output := runEffortCommand(t, root, "worktree", []string{"add", slug}, nil); !strings.Contains(output, slug) {
		t.Fatalf("worktree add omitted slug: %q", output)
	}
	if output := runEffortCommand(t, root, "worktree", []string{"remove", slug}, nil); !strings.Contains(output, "managed worktree topology is absent") {
		t.Fatalf("worktree remove did not settle topology: %q", output)
	}
	if output := runEffortCommand(t, root, "finish", []string{slug}, nil); !strings.Contains(output, "changed cleanup: yes") {
		t.Fatalf("finish did not clean resident: %q", output)
	}
}

// invariant: tooling/cli:effort-command-contract (TestEffortNewExplicitSlugGrammarAndFlagCombinations)
func TestEffortNewExplicitSlugGrammarAndFlagCombinations(t *testing.T) {
	for _, args := range [][]string{
		{"effort", "new", "--slug", "ordered-input", "--json", "Ordered title", "--no-worktree"},
		{"effort", "new", "Ordered title", "--no-worktree", "--json", "--slug", "ordered-input"},
		{"effort", "new", "--slug", "ordered-input", "--base", "HEAD", "Ordered title", "--json"},
		{"effort", "new", "Ordered title", "--json", "--base", "HEAD", "--slug", "ordered-input"},
	} {
		root := commandRepo(t)
		code, stdout, stderr := runEffortCLI(t, root, args...)
		var reply struct {
			SchemaVersion int           `json:"schemaVersion"`
			Effort        effort.Record `json:"effort"`
		}
		decodeErr := json.Unmarshal([]byte(stdout), &reply)
		if code != 0 || stderr != "" || decodeErr != nil || reply.SchemaVersion != 2 || reply.Effort.Slug != "ordered-input" || reply.Effort.Title != "Ordered title" {
			t.Fatalf("%q code=%d stdout=%q stderr=%q reply=%#v decode=%v", args, code, stdout, stderr, reply, decodeErr)
		}
	}
	root := commandRepo(t)
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
	overlong := strings.Repeat("s", 33)
	code, stdout, stderr := runEffortCLI(t, root, "effort", "new", "--slug", overlong, "Overlong slug", "--no-worktree")
	if code == 0 || stdout != "" || !strings.Contains(stderr, "1-32 bytes") || !strings.Contains(stderr, "changed bytes: no") || !strings.Contains(stderr, "--slug") {
		t.Fatalf("33-byte slug code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Lstat(filepath.Join(root, ".awf", "efforts", overlong)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("33-byte new slug changed residents: %v", err)
	}
	writePersistedEffortFixture(t, root, overlong)
	if shown := runEffortCommand(t, root, "show", []string{overlong}, map[string]bool{"--json": true}); !strings.Contains(shown, overlong) {
		t.Fatalf("same 33-byte persisted slug is not selectable: %q", shown)
	}
}

func TestEffortNestedGrammarDispatch(t *testing.T) {
	cmd, top, sub, rest, ok := resolve([]string{"effort", "activity", "heartbeat", "example", "--owner", "018f47a0-7b3d-4c52-8f1a-123456789abc", "--json"})
	if !ok || top.Name != "effort" || cmd.Name != "heartbeat" || sub != "activity heartbeat" {
		t.Fatalf("nested resolve = cmd=%#v top=%#v sub=%q ok=%v", cmd, top, sub, ok)
	}
	if _, err := parseArgs(cmd, rest); err != nil {
		t.Fatalf("valid activity grammar rejected: %v", err)
	}
	if _, err := parseArgs(cmd, []string{"example", "--owner", "owner", "--json", "--cwd", "/irrelevant"}); err == nil {
		t.Fatal("irrelevant activity flag accepted")
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"awf", "help", "effort", "activity", "attach"}, &stdout, &stderr); code != 0 || stdout.String() != "Usage: awf effort activity attach <slug> --owner <uuid> --cwd <absolute-path> --role <managed|receiving> --receiving-checkout <absolute-path> --json\n" || stderr.Len() != 0 {
		t.Fatalf("nested help code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

// invariant: tooling/cli:effort-command-contract (TestEffortMemoryAndActivityCLIContract)
func TestEffortMemoryAndActivityCLIContract(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"awf", "help", "effort", "memory"}, "Usage: awf effort memory update <slug> [--phase <text>] [--next <text>]\n\nUpdate one or both mutable memory metadata fields. At least one of --phase and\n--next is required.\n"},
		{[]string{"awf", "help", "effort", "memory", "update"}, "Usage: awf effort memory update <slug> [--phase <text>] [--next <text>]\n\nUpdate one or both mutable memory metadata fields. At least one of --phase and\n--next is required.\n"},
		{[]string{"awf", "help", "effort", "activity"}, "Usage: awf effort activity resolve <slug> --destination <managed|receiving> [--receiving-checkout <absolute-path>] --json\n       awf effort activity attach <slug> --owner <uuid> --cwd <absolute-path> --role <managed|receiving> --receiving-checkout <absolute-path> --json\n       awf effort activity heartbeat <slug> --owner <uuid> --json\n       awf effort activity checkout <slug> --owner <uuid> --cwd <absolute-path> --role <managed|receiving> --json\n       awf effort activity detach <slug> --owner <uuid> --json\n\nActivity replies are protocol-1 JSON only. Each action accepts only the flags\nshown in its usage form.\n"},
		{[]string{"awf", "help", "effort", "activity", "resolve"}, "Usage: awf effort activity resolve <slug> --destination <managed|receiving> [--receiving-checkout <absolute-path>] --json\n"},
		{[]string{"awf", "help", "effort", "activity", "attach"}, "Usage: awf effort activity attach <slug> --owner <uuid> --cwd <absolute-path> --role <managed|receiving> --receiving-checkout <absolute-path> --json\n"},
		{[]string{"awf", "help", "effort", "activity", "heartbeat"}, "Usage: awf effort activity heartbeat <slug> --owner <uuid> --json\n"},
		{[]string{"awf", "help", "effort", "activity", "checkout"}, "Usage: awf effort activity checkout <slug> --owner <uuid> --cwd <absolute-path> --role <managed|receiving> --json\n"},
		{[]string{"awf", "help", "effort", "activity", "detach"}, "Usage: awf effort activity detach <slug> --owner <uuid> --json\n"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(test.args, &stdout, &stderr); code != 0 || stdout.String() != test.want || stderr.Len() != 0 {
			t.Fatalf("help %q: code=%d stdout=%q stderr=%q", test.args[3:], code, stdout.String(), stderr.String())
		}
	}

	primary := filepath.Join(t.TempDir(), "primary")
	fixture := gitfixture.InitNativeAt(t, primary)
	if err := os.WriteFile(filepath.Join(primary, "tracked.txt"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitfixture.NativeAdd(t, fixture, "tracked.txt")
	for _, resident := range []string{"efforts", "worktrees"} {
		ignore := filepath.Join(primary, ".awf", resident, ".gitignore")
		if err := os.MkdirAll(filepath.Dir(ignore), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ignore, []byte("*\n!.gitignore\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		gitfixture.NativeAdd(t, fixture, ".awf/"+resident+"/.gitignore")
	}
	gitfixture.NativeCommit(t, fixture, "base")
	linked := filepath.Join(filepath.Dir(primary), "linked")
	gitfixture.NativeWorktreeAddDetached(t, fixture, linked, "HEAD")

	runNewEffortCommand(t, primary, "activity-contract", "Activity contract", map[string]bool{"--no-worktree": true})
	slug := "activity-contract"
	memoryPath := filepath.Join(primary, ".awf", "efforts", slug, "memory.md")
	body := "## Brief\r\n\r\nbody bytes stay exact\r\n"
	legacy := "Effort: activity-contract\nPhase: old\nNext: old next\nUpdated: Not yet updated.\n\n" + body
	if err := os.WriteFile(memoryPath, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	phase, next := "Implementing", "Exercise the attached checkout."
	code, memoryOut, memoryErr := runEffortCLI(t, primary, "effort", "memory", "update", slug, "--phase", phase, "--next", next)
	if code != 0 || memoryOut != "" || memoryErr != "" {
		t.Fatalf("memory update code=%d stdout=%q stderr=%q", code, memoryOut, memoryErr)
	}
	migrated, err := os.ReadFile(memoryPath)
	if err != nil || !strings.HasPrefix(string(migrated), "---\neffort: activity-contract\n") || !strings.Contains(string(migrated), "phase: Implementing\n") || !strings.Contains(string(migrated), "next: Exercise the attached checkout.\n") || !strings.Contains(string(migrated), "updated: \"") || !strings.Contains(string(migrated), "Z\"\n---\n") || !strings.HasSuffix(string(migrated), body) {
		t.Fatalf("legacy memory migration err=%v memory=%q", err, migrated)
	}
	updatedLine := strings.Split(string(migrated), "\n")[4]
	updatedText := strings.Trim(strings.TrimPrefix(updatedLine, "updated: "), "\"")
	if updated, err := time.Parse(time.RFC3339Nano, updatedText); err != nil || updated.Location() != time.UTC {
		t.Fatalf("memory updated timestamp=%q parsed=%v err=%v", updatedLine, updated, err)
	}

	owner := "018f47a0-7b3d-4c52-8f1a-123456789abc"
	reply := runActivityCommand(t, linked, "activity resolve", map[string]string{"--destination": "receiving"})
	assertActivityFields(t, reply, effort.ActivityReady, "effort", "memory", "destination")
	reply = runActivityCommand(t, linked, "activity resolve", map[string]string{"--destination": "receiving", "--receiving-checkout": "."})
	assertActivityFields(t, reply, effort.ActivityRepositoryMismatch, "outcome")
	var relativeOutcome effort.ActionableOutcome
	if err := json.Unmarshal(reply["outcome"], &relativeOutcome); err != nil || relativeOutcome.ChangedActivity || relativeOutcome.ChangedMemory || relativeOutcome.ChangedCWD {
		t.Fatalf("relative receiving checkout outcome=%#v err=%v", relativeOutcome, err)
	}
	reply = runActivityCommand(t, linked, "activity attach", map[string]string{"--owner": owner, "--cwd": linked, "--role": "receiving", "--receiving-checkout": linked})
	assertActivityFields(t, reply, effort.ActivityAttached, "effort", "memory", "activity")
	reply = runActivityCommand(t, primary, "activity heartbeat", map[string]string{"--owner": owner})
	assertActivityFields(t, reply, effort.ActivityHeartbeat, "effort", "memory", "activity")
	reply = runActivityCommand(t, linked, "activity checkout", map[string]string{"--owner": owner, "--cwd": linked, "--role": "receiving"})
	assertActivityFields(t, reply, effort.ActivityCheckoutUpdated, "effort", "memory", "activity")
	reply = runActivityCommand(t, primary, "activity detach", map[string]string{"--owner": owner})
	assertActivityFields(t, reply, effort.ActivityDetached, "effort")

	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"effort", "memory", "update", slug}, "usage: awf effort memory update"},
		{[]string{"effort", "memory", "update", "missing-effort", "--phase", phase}, "next action"},
		{[]string{"effort", "activity", "resolve", slug, "--destination", "receiving"}, "requires --json"},
		{[]string{"effort", "activity", "attach", slug, "--owner", owner}, "requires --json"},
		{[]string{"effort", "activity", "heartbeat", slug, "--owner", owner}, "requires --json"},
		{[]string{"effort", "activity", "checkout", slug, "--owner", owner}, "requires --json"},
		{[]string{"effort", "activity", "detach", slug, "--owner", owner}, "requires --json"},
		{[]string{"effort", "activity", "heartbeat", slug, "--owner", owner, "--json", "--json"}, "given more than once"},
		{[]string{"effort", "activity", "heartbeat", slug, "--owner", owner, "--cwd", primary, "--json"}, "unknown flag"},
	} {
		code, stdout, stderr := runEffortCLI(t, primary, test.args...)
		if code == 0 || stdout != "" || !strings.Contains(stderr, test.want) {
			t.Errorf("%q failure code=%d stdout=%q stderr=%q", test.args, code, stdout, stderr)
		}
	}

	if err := os.WriteFile(memoryPath, []byte("---\neffort: activity-contract\nphase: \"\"\nnext: valid\nupdated: 2026-08-02T12:00:00Z\n---\n"+body), 0o600); err != nil {
		t.Fatal(err)
	}
	reply = runActivityCommand(t, linked, "activity resolve", map[string]string{"--destination": "receiving"})
	assertActivityRefusal(t, reply, effort.ActivityInvalidMemory, false, "./awf effort memory update activity-contract --phase <replacement-phase>", false)
	if err := os.WriteFile(memoryPath, []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	reply = runActivityCommand(t, linked, "activity resolve", map[string]string{"--destination": "receiving"})
	assertActivityRefusal(t, reply, effort.ActivityInvalidMemory, false, "repair .awf/efforts/activity-contract/memory.md manually: preserve its body and restore a matching effort identity with a recognized canonical or legacy metadata boundary", false)
	if err := os.Remove(memoryPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(memoryPath, 0o700); err != nil {
		t.Fatal(err)
	}
	reply = runActivityCommand(t, linked, "activity resolve", map[string]string{"--destination": "receiving"})
	assertActivityRefusal(t, reply, effort.ActivityInvalidMemory, false, "repair .awf/efforts/activity-contract/memory.md manually: preserve its body and restore a matching effort identity with a recognized canonical or legacy metadata boundary", true)
	if err := os.Remove(memoryPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(memoryPath, migrated, 0o600); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		sub string
		pos []string
	}{
		{"show", []string{slug}}, {"list", nil}, {"worktree", []string{"add", slug}}, {"integrate", []string{slug}}, {"worktree", []string{"remove", slug}}, {"finish", []string{slug}},
	} {
		if output := runEffortCommand(t, primary, test.sub, test.pos, nil); output == "" {
			t.Fatalf("unrelated %s command produced no availability result", test.sub)
		}
	}
}

func assertActivityRefusal(t *testing.T, reply map[string]json.RawMessage, condition effort.ActivityCondition, changedCWD bool, nextAction string, cause bool) {
	t.Helper()
	assertActivityFields(t, reply, condition, "effort", "outcome")
	var outcome effort.ActionableOutcome
	if err := json.Unmarshal(reply["outcome"], &outcome); err != nil || outcome.ChangedActivity || outcome.ChangedMemory || outcome.ChangedCWD != changedCWD || len(outcome.NextActions) != 1 || outcome.NextActions[0] != nextAction || (outcome.Cause != "") != cause {
		t.Fatalf("refusal outcome=%#v err=%v", outcome, err)
	}
}

func runActivityCommand(t *testing.T, root, sub string, values map[string]string) map[string]json.RawMessage {
	t.Helper()
	args := []string{"effort"}
	args = append(args, strings.Fields(sub)...)
	args = append(args, "activity-contract")
	for _, flag := range []string{"--destination", "--owner", "--cwd", "--role", "--receiving-checkout"} {
		if value, ok := values[flag]; ok {
			args = append(args, flag, value)
		}
	}
	args = append(args, "--json")
	code, stdout, stderr := runEffortCLI(t, root, args...)
	if code != 0 || stderr != "" {
		t.Fatalf("%s: code=%d stdout=%q stderr=%q", sub, code, stdout, stderr)
	}
	var reply map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &reply); err != nil {
		t.Fatalf("%s JSON %q: %v", sub, stdout, err)
	}
	return reply
}

// runEffortCLI exercises the public driver, including nested resolution,
// parse-once validation, project guards, gates, and effort dispatch.
func runEffortCLI(t *testing.T, root string, args ...string) (int, string, string) {
	t.Helper()
	t.Chdir(root)
	var stdout, stderr bytes.Buffer
	code := run(append([]string{"awf"}, args...), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func assertActivityFields(t *testing.T, reply map[string]json.RawMessage, condition effort.ActivityCondition, present ...string) {
	t.Helper()
	var gotCondition effort.ActivityCondition
	if err := json.Unmarshal(reply["condition"], &gotCondition); err != nil || gotCondition != condition {
		t.Fatalf("condition=%q err=%v reply=%v", gotCondition, err, reply)
	}
	if _, ok := reply["schemaVersion"]; !ok {
		t.Fatalf("reply has no schemaVersion: %v", reply)
	}
	want := map[string]bool{}
	for _, name := range present {
		want[name] = true
	}
	for _, name := range []string{"effort", "memory", "destination", "activity", "priorClaim", "outcome"} {
		_, got := reply[name]
		if got != want[name] {
			t.Errorf("%s presence=%v want %v in %v", name, got, want[name], reply)
		}
	}
}

func TestEffortJSONFailuresWriteNoStdoutAndRejectProtocol1(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := commandRepo(t)
	for _, test := range []struct {
		sub string
		pos []string
	}{
		{sub: "new", pos: []string{"界🙂"}},
		{sub: "show", pos: []string{"missing-effort"}},
	} {
		var stdout bytes.Buffer
		values := map[string]string{}
		if test.sub == "new" {
			values["--slug"] = "bad_slug"
		}
		err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: test.sub, inv: invocation{positionals: test.pos, bools: map[string]bool{"--json": true}, values: values}, stdout: &stdout}, openEffortComposition)
		if err == nil || stdout.Len() != 0 || !strings.Contains(err.Error(), "next action") {
			t.Fatalf("%s error=%v stdout=%q", test.sub, err, stdout.String())
		}
	}

	dir := filepath.Join(root, ".awf", "efforts", "legacy-effort")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "memory.md"), []byte("Effort: legacy-effort\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(`{"schemaVersion":1,"id":"018f47a0-7b3d-4c52-8f1a-123456789abc","slug":"legacy-effort","title":"Legacy effort","createdAt":"2026-07-29T12:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: "show", inv: invocation{positionals: []string{"legacy-effort"}, bools: map[string]bool{"--json": true}, values: map[string]string{}}, stdout: &stdout}, openEffortComposition)
	if err == nil || stdout.Len() != 0 || !strings.Contains(err.Error(), "unsupported schemaVersion 1") {
		t.Fatalf("protocol-1 error=%v stdout=%q", err, stdout.String())
	}
}

type effortErrorWriter struct{}

func (effortErrorWriter) Write([]byte) (int, error) { return 0, os.ErrClosed }

func TestEffortCommandUsageAndOutputErrors(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := commandRepo(t)
	if err := runEffort(&cmdCtx{ctx: testContext(t), root: filepath.Join(root, "missing"), sub: "list", inv: invocation{bools: map[string]bool{}, values: map[string]string{}}, stdout: &bytes.Buffer{}}, openEffortComposition); err == nil {
		t.Fatal("invalid repository accepted")
	}
	for _, bools := range []map[string]bool{{}, {"--no-worktree": true}} {
		if err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: "new", inv: invocation{positionals: []string{" "}, bools: bools, values: map[string]string{"--slug": "blank-title"}}, stdout: &bytes.Buffer{}}, openEffortComposition); err == nil {
			t.Fatalf("blank title accepted with bools %v", bools)
		}
	}
	runNewEffortCommand(t, root, "output-errors", "Output errors", map[string]bool{"--no-worktree": true})
	for _, test := range []struct {
		sub   string
		pos   []string
		bools map[string]bool
	}{
		{sub: "new", pos: []string{"Another output"}, bools: map[string]bool{"--no-worktree": true}},
		{sub: "new", pos: []string{"JSON output"}, bools: map[string]bool{"--json": true, "--no-worktree": true}},
		{sub: "show", pos: []string{"output-errors"}},
		{sub: "show", pos: []string{"output-errors"}, bools: map[string]bool{"--json": true}},
		{sub: "list"},
		{sub: "list", bools: map[string]bool{"--json": true}},
	} {
		if test.bools == nil {
			test.bools = map[string]bool{}
		}
		values := map[string]string{}
		if test.sub == "new" {
			values["--slug"] = strings.ToLower(strings.ReplaceAll(test.pos[0], " ", "-"))
		}
		err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: test.sub, inv: invocation{positionals: test.pos, bools: test.bools, values: values}, stdout: effortErrorWriter{}}, openEffortComposition)
		if err == nil {
			t.Errorf("%s output error ignored", test.sub)
		}
	}
	if err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: "finish", inv: invocation{positionals: []string{"output-errors"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: effortErrorWriter{}}, openEffortComposition); err == nil {
		t.Fatal("finish output error ignored")
	}
	if err := writeWorktreeResult(&bytes.Buffer{}, worktree.Result{}, os.ErrInvalid); !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("worktree error = %v", err)
	}
	if err := writeWorktreeResult(effortErrorWriter{}, worktree.Result{Condition: "done"}, nil); err == nil {
		t.Fatal("worktree output error ignored")
	}
	if got := yesNo(false); got != "no" {
		t.Fatalf("yesNo(false) = %q", got)
	}
	for _, ctx := range []*cmdCtx{
		{root: root, sub: "worktree", inv: invocation{positionals: []string{"add"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &bytes.Buffer{}},
		{root: root, sub: "worktree", inv: invocation{positionals: []string{"remove", "output-errors"}, bools: map[string]bool{}, values: map[string]string{"--base": "HEAD"}}, stdout: &bytes.Buffer{}},
		{root: root, sub: "worktree", inv: invocation{positionals: []string{"other", "output-errors"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &bytes.Buffer{}},
	} {
		if err := runEffort(ctx, openEffortComposition); err == nil {
			t.Fatal("invalid worktree grammar accepted")
		}
	}
	corruptRoot := commandRepo(t)
	if err := os.MkdirAll(filepath.Join(corruptRoot, ".awf", "efforts"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(corruptRoot, ".awf", "efforts", "foreign"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runEffort(&cmdCtx{ctx: testContext(t), root: corruptRoot, sub: "list", inv: invocation{bools: map[string]bool{}, values: map[string]string{}}, stdout: &bytes.Buffer{}}, openEffortComposition); err == nil {
		t.Fatal("corrupt list accepted")
	}
	emptyRoot := commandRepo(t)
	if output := runEffortCommand(t, emptyRoot, "list", nil, nil); output != "" {
		t.Fatalf("empty text list = %q", output)
	}
	if err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: "finish", inv: invocation{positionals: []string{"missing-effort"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &bytes.Buffer{}}, openEffortComposition); err == nil {
		t.Fatal("missing finish accepted")
	}
	if err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: "unknown", inv: invocation{bools: map[string]bool{}, values: map[string]string{}}, stdout: &bytes.Buffer{}}, openEffortComposition); err == nil {
		t.Fatal("unknown effort child accepted")
	}
}

func TestEffortCompositionGrammarAndPresentationBranches(t *testing.T) {
	if _, err := resolveEffortCheckout(testContext(t), t.TempDir()); err == nil {
		t.Fatal("checkout resolution accepted a non-checkout")
	}
	if got := effortValue(invocation{values: map[string]string{}}, "--phase"); got != nil {
		t.Fatalf("absent effort value = %q", *got)
	}
	for _, test := range []struct {
		sub  string
		want string
	}{
		{sub: "memory", want: "usage: awf effort memory update <slug> [--phase <text>] [--next <text>]"},
		{sub: "activity", want: "usage: awf effort activity <resolve|attach|heartbeat|checkout|detach>"},
	} {
		if err := validateEffortGrammar(&cmdCtx{sub: test.sub, inv: invocation{bools: map[string]bool{}, values: map[string]string{}}}); err == nil || err.Error() != test.want {
			t.Fatalf("%s grammar = %v", test.sub, err)
		}
	}
	if err := validateEffortActivityGrammar(&cmdCtx{sub: "activity attach", inv: invocation{bools: map[string]bool{"--json": true}, values: map[string]string{}}}); err == nil || err.Error() != "usage: awf effort activity attach requires --owner" {
		t.Fatalf("activity required-flag grammar = %v", err)
	}

	record := effort.Record{SchemaVersion: effort.SchemaVersion, Slug: "presentation", Title: "Presentation", MemoryPath: ".awf/efforts/presentation/memory.md"}
	var jsonOut bytes.Buffer
	if err := writeEffortNew(&jsonOut, record, worktree.Result{}, true); err != nil {
		t.Fatal(err)
	}
	var reply struct {
		Worktree *effortWorktreeFacts `json:"worktree"`
	}
	if err := json.Unmarshal(jsonOut.Bytes(), &reply); err != nil || reply.Worktree != nil {
		t.Fatalf("no-worktree JSON = %q, reply=%#v, err=%v", jsonOut.String(), reply, err)
	}
	var textOut bytes.Buffer
	result := worktree.Result{Path: "/managed/presentation", Branch: "awf/presentation", Condition: "managed"}
	if err := writeEffortNew(&textOut, record, result, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(textOut.String(), "worktree=/managed/presentation branch=awf/presentation") {
		t.Fatalf("managed-worktree text = %q", textOut.String())
	}
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

func runEffortCommand(t *testing.T, root, sub string, positionals []string, bools map[string]bool) string {
	t.Helper()
	if bools == nil {
		bools = map[string]bool{}
	}
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
	// An adopted project commits the resident .gitignore files awf renders, which
	// is what keeps owned effort and worktree state out of the cleanliness
	// oracle's view; the fixture carries them for the same reason.
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
