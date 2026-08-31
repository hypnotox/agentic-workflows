package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/effort/application"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
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
	resident := filepath.Join(root, ".awf", "efforts", slug)
	memoryBefore, err := os.ReadFile(filepath.Join(resident, "memory.md"))
	if err != nil {
		t.Fatal(err)
	}
	activityBefore := []byte("malformed legacy activity bytes\x00\n")
	if err := os.WriteFile(filepath.Join(resident, "activity.json"), activityBefore, 0o600); err != nil {
		t.Fatal(err)
	}

	if shown := runEffortCommand(t, root, "show", []string{slug}); !strings.Contains(shown, slug) {
		t.Fatalf("show omitted resident slug: %q", shown)
	}
	if listed := runEffortCommand(t, root, "list", nil); !strings.Contains(listed, slug) {
		t.Fatalf("list omitted resident slug: %q", listed)
	}
	if memoryAfter, err := os.ReadFile(filepath.Join(resident, "memory.md")); err != nil || !bytes.Equal(memoryAfter, memoryBefore) {
		t.Fatalf("show/list changed legacy memory: %q err=%v", memoryAfter, err)
	}
	if output := runEffortCommand(t, root, "worktree", []string{"add", slug}); !strings.Contains(output, slug) {
		t.Fatalf("worktree add omitted slug: %q", output)
	}
	if output := runEffortCommand(t, root, "worktree", []string{"remove", slug}); !strings.Contains(output, "managed worktree topology is absent") {
		t.Fatalf("worktree remove did not settle topology: %q", output)
	}
	if output := runEffortCommand(t, root, "finish", []string{slug}); !strings.Contains(output, "archived resident") || !strings.Contains(output, ".awf/effort-archive/"+"018f47a0-7b3d-4c52-8f1a-123456789abc-"+slug) {
		t.Fatalf("finish did not archive resident: %q", output)
	}
	archive := filepath.Join(root, ".awf", "effort-archive", "018f47a0-7b3d-4c52-8f1a-123456789abc-"+slug)
	if memoryAfter, err := os.ReadFile(filepath.Join(archive, "memory.md")); err != nil || !bytes.Equal(memoryAfter, memoryBefore) {
		t.Fatalf("archived memory = %q err=%v", memoryAfter, err)
	}
	if activityAfter, err := os.ReadFile(filepath.Join(archive, "activity.json")); err != nil || !bytes.Equal(activityAfter, activityBefore) {
		t.Fatalf("archived activity = %q err=%v", activityAfter, err)
	}
}

// invariant: tooling/cli:effort-command-contract (TestEffortNewExplicitSlugGrammarAndFlagCombinations)
func TestEffortNewExplicitSlugGrammarAndFlagCombinations(t *testing.T) {
	for _, args := range [][]string{
		{"effort", "new", "--slug", "ordered-input", "Ordered title"},
		{"effort", "new", "Ordered title", "--slug", "ordered-input"},
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
	}, func(context.Context, string, application.Request, func() ([]byte, error)) (application.Result, error) {
		composed = true
		return application.Result{}, errors.New("application invoked")
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
		{[]string{"effort", "new", "--json", "--slug", "json-new", "JSON new"}, "condition: awf: awf new: unknown flag \"--json\"\n"},
		{[]string{"effort", "new", "--slug", "no-worktree", "No worktree", "--no-worktree"}, "condition: awf: awf new: unknown flag \"--no-worktree\"\n"},
		{[]string{"effort", "list", "--json"}, "condition: awf: awf list: unknown flag \"--json\"\n"},
		{[]string{"effort", "show", "ordered-input", "--json"}, "condition: awf: awf show: unknown flag \"--json\"\n"},
	} {
		code, stdout, stderr := runEffortCLI(t, root, test.args...)
		if code != 2 || stdout != "" || stderr != test.want {
			t.Fatalf("%q code=%d stdout=%q stderr=%q want=%q", test.args, code, stdout, stderr, test.want)
		}
	}
	code, stdout, stderr := runEffortCLI(t, root, "effort", "new", "--slug", "readable", "Readable contract")
	newWant := "status: managed worktree added for readable\n\nmutation:\n  identity:\n    effort: readable\n    title: Readable contract\n    memory: .awf/efforts/readable/memory.md\n    worktree: " + filepath.Join(root, ".awf", "worktrees", "readable") + "\n    branch: awf/readable\n  changes:\n    completed:\n      managed path\n      git registration\n      branch\n  next actions:\n    step 1: continue the effort in " + filepath.Join(root, ".awf", "worktrees", "readable") + "\n"
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
	code, stdout, stderr = runEffortCLI(t, root, "effort", "new", "--slug", overlong, "Overlong slug")
	overlongRefusal := "condition: explicit effort slug \"" + overlong + "\" is invalid\nstate: input\ncause: slug must contain 1-32 bytes\n\ndiagnostic:\n  changed:\n    bytes: no\n  steps:\n    step 1: provide a different canonical value with `--slug`\n"
	if code != 1 || stdout != "" || stderr != overlongRefusal {
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
func runEffortCLI(t *testing.T, root string, args ...string) (int, string, string) {
	return runEffortCLIWithInput(t, root, os.Stdin, args...)
}

func runEffortCLIWithInput(t *testing.T, root string, stdin io.Reader, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := newRunner(func() (string, error) { return root, nil }, stdin, func() bool { return false }).run(append([]string{"awf"}, args...), &stdout, &stderr)
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
	testsupport.WriteAwfConfig(t, root, minimalYAML)
	gitfixture.NativeAdd(t, fixture, "tracked.txt", ".awf/config.yaml")
	gitfixture.NativeCommit(t, fixture, "base")
	if err := initializeProject(testContext(t), root, io.Discard); err != nil {
		t.Fatal(err)
	}
	gitfixture.NativeAdd(t, fixture, ".")
	gitfixture.NativeCommit(t, fixture, "initialize awf")
	return filesystem.NormalizePlatformPath(root)
}

type effortErrorWriter struct{}

func (effortErrorWriter) Write([]byte) (int, error) { return 0, os.ErrClosed }

type effortShortWriter struct{}

func (effortShortWriter) Write(value []byte) (int, error) { return len(value) - 1, nil }
func TestRetiredEffortBranchesFailOrdinaryParsing(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{
		{"effort", "memory"}, {"effort", "memory", "read", "missing"},
		{"effort", "memory", "edit", "missing"}, {"effort", "memory", "update", "missing"},
		{"effort", "activity"}, {"effort", "activity", "attach", "missing"},
		{"effort", "activity", "heartbeat", "missing"}, {"effort", "activity", "detach", "missing"},
	} {
		code, stdout, stderr := runEffortCLI(t, root, args...)
		if code != 2 || stdout != "" || !strings.Contains(stderr, "unexpected arguments") {
			t.Errorf("%v code=%d stdout=%q stderr=%q", args, code, stdout, stderr)
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
	if got, want := run("effort", "new", "--slug", "public-output", "Public output"), fmt.Sprintf("status: managed worktree added for public-output\n\nmutation:\n  identity:\n    effort: public-output\n    title: Public output\n    memory: .awf/efforts/public-output/memory.md\n    worktree: %s\n    branch: awf/public-output\n  changes:\n    completed:\n      managed path\n      git registration\n      branch\n  next actions:\n    step 1: continue the effort in %s\n", filepath.Join(root, ".awf", "worktrees", "public-output"), filepath.Join(root, ".awf", "worktrees", "public-output")); got != want {
		t.Fatalf("initial new output = %q, want %q", got, want)
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"effort", "list"}, "effort list:\n  efforts:\n    public-output: Public output\n"},
		{[]string{"effort", "show", "public-output"}, "slug: public-output\ntitle: Public output\nmemory: .awf/efforts/public-output/memory.md\n"},
		{[]string{"effort", "integrate", "public-output"}, "status: effort tip is already integrated into the target\n\nmutation:\n  next actions:\n    step 1: run `awf effort worktree remove public-output` after terminal review is settled\n"},
		{[]string{"effort", "worktree", "remove", "public-output"}, "status: managed worktree topology is absent\n\nmutation:\n  changes:\n    completed:\n      managed path\n      git registration\n      branch\n  next actions:\n    step 1: continue to retrospective, then finish the effort\n"},
		{[]string{"effort", "finish", "public-output"}, ""},
	} {
		if got := run(test.args...); test.want == "" {
			for _, want := range []string{"status: archived", "effort: public-output", "archive: .awf/effort-archive/", "archived resident", "archive parent synced", "efforts parent synced", "delete the local archive manually"} {
				if !strings.Contains(got, want) {
					t.Fatalf("%v output = %q, missing %q", test.args, got, want)
				}
			}
		} else if got != test.want {
			t.Fatalf("%v output = %q, want %q", test.args, got, test.want)
		}
	}
	code, stdout, stderr := runEffortCLI(t, root, "effort", "finish", "public-output")
	const restart = "condition: effort \"public-output\" has no active resident or finishing reservation\nstate: resident\n\ndiagnostic:\n  changed:\n    bytes: no\n  steps:\n    step 1: run `awf effort list` and use an active slug\n"
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
	if err := runEffort(ctx("removed"), nil); err == nil {
		t.Fatal("unknown effort action accepted")
	}
}

var _ = strings.Contains
