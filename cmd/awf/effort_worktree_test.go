package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/effort/application"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func TestEffortGrammarIsClosedAndHasNoForceSurface(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	cases := []struct {
		name string
		ctx  cmdCtx
		want string
	}{
		{"remove base", cmdCtx{sub: "worktree", inv: invocation{positionals: []string{"remove", "slug"}, values: map[string]string{"--base": "HEAD"}}}, "--base is not allowed"},
		{"bad action", cmdCtx{sub: "worktree", inv: invocation{positionals: []string{"discard", "slug"}, values: map[string]string{}}}, "<add|remove>"},
		{"bad arity", cmdCtx{sub: "worktree", inv: invocation{positionals: []string{"add"}, values: map[string]string{}}}, "<add|remove>"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := validateEffortGrammar(&test.ctx); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v, want %q", err, test.want)
			}
		})
	}
	if err := validateEffortGrammar(&cmdCtx{ctx: testContext(t), sub: "worktree", inv: invocation{positionals: []string{"add", "slug"}, values: map[string]string{"--base": "HEAD"}}}); err != nil {
		t.Fatal(err)
	}
}

// invariant: tooling/effort-management:default-worktree-creation (TestEffortNewReportsDefaultAndOptedOutWorktrees)
func TestEffortNewReportsDefaultAndOptedOutWorktrees(t *testing.T) {
	root := commandRepo(t)
	managed := filepath.Join(root, ".awf", "worktrees", "default-cli")
	text := runNewEffortCommand(t, root, "default-cli", "Default CLI", nil)
	wantText := "status: managed worktree added for default-cli\n\nmutation:\n  identity:\n    effort: default-cli\n    title: Default CLI\n    memory: .awf/efforts/default-cli/memory.md\n    worktree: " + managed + "\n    branch: awf/default-cli\n  changes:\n    completed:\n      managed path\n      git registration\n      branch\n  next actions:\n    step 1: continue the effort in " + managed + "\n"
	if text != wantText {
		t.Fatalf("default new text =\n%q\nwant\n%q", text, wantText)
	}
	if info, err := os.Lstat(managed); err != nil || !info.IsDir() {
		t.Fatalf("managed checkout absent: %v", err)
	}
	if !effortWorktreeBranchExists(t, root, "default-cli") {
		t.Fatal("managed branch absent")
	}
	code, stdout, stderr := runEffortCLI(t, managed, "effort", "show", "default-cli")
	memory := filepath.Join(root, ".awf", "efforts", "default-cli", "memory.md")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "memory: "+memory) {
		t.Fatalf("managed show code=%d stdout=%q stderr=%q, want resolvable %s", code, stdout, stderr, memory)
	}
	if _, err := os.Stat(memory); err != nil {
		t.Fatalf("reported memory path is not resolvable: %v", err)
	}
}

// invariant: tooling/effort-management:default-worktree-creation (TestEffortNewBasesTheManagedBranchOnTheNamedRef)
func TestEffortNewBasesTheManagedBranchOnTheNamedRef(t *testing.T) {
	root := commandRepo(t)
	base := gitfixture.NativeRevParse(t, gitfixture.At(root), "HEAD")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("second\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitfixture.NativeAdd(t, gitfixture.At(root), "tracked.txt")
	gitfixture.NativeCommit(t, gitfixture.At(root), "second")
	var out bytes.Buffer
	ctx := &cmdCtx{ctx: testContext(t), root: root, sub: "new", inv: invocation{positionals: []string{"Based CLI"}, bools: map[string]bool{}, values: map[string]string{"--slug": "based-cli", "--base": base}}, stdout: &out}
	if err := runEffort(ctx, openEffortComposition); err != nil {
		t.Fatal(err)
	}
	if tip := gitfixture.NativeRevParse(t, gitfixture.At(root), "awf/based-cli"); tip != base {
		t.Fatalf("managed branch tip = %q, want the named base %q", tip, base)
	}
}

func TestEffortNewRollsBackOrRetainsPerProvenTopology(t *testing.T) {
	root := commandRepo(t)
	var out bytes.Buffer
	rolled := &cmdCtx{ctx: testContext(t), root: root, sub: "new", inv: invocation{positionals: []string{"Rolled back CLI"}, bools: map[string]bool{}, values: map[string]string{"--slug": "rolled-back-cli", "--base": "no-such-ref"}}, stdout: &out}
	err := runEffort(rolled, openEffortComposition)
	if err == nil || !strings.Contains(err.Error(), "effort rolled-back-cli rolled back") || !strings.Contains(err.Error(), "retry `awf effort new --slug \"rolled-back-cli\" \"Rolled back CLI\"`") {
		t.Fatalf("unresolvable base error = %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(root, ".awf", "efforts", "rolled-back-cli")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("rolled-back effort resident remains: %v", statErr)
	}

	gitfixture.NativeBranch(t, gitfixture.At(root), "awf/retained-cli")
	out.Reset()
	retained := &cmdCtx{ctx: testContext(t), root: root, sub: "new", inv: invocation{positionals: []string{"Retained CLI"}, bools: map[string]bool{}, values: map[string]string{"--slug": "retained-cli"}}, stdout: &out}
	err = runEffort(retained, openEffortComposition)
	if err == nil || !strings.Contains(err.Error(), "effort retained-cli retained: managed topology remains") {
		t.Fatalf("colliding branch error = %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("failed creation wrote stdout: %q", out.String())
	}
	if _, statErr := os.Lstat(filepath.Join(root, ".awf", "efforts", "retained-cli")); statErr != nil {
		t.Fatalf("retained effort resident was removed: %v", statErr)
	}
}

func effortWorktreeBranchExists(t *testing.T, root, slug string) bool {
	t.Helper()
	return gitfixture.NativeRevisionExists(t, gitfixture.At(root), "refs/heads/awf/"+slug)
}

func TestEffortWorktreeCLIComposition(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := commandRepo(t)
	runNewEffortCommand(t, root, "cli-worktree", "CLI worktree", map[string]bool{})
	if err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: "worktree", inv: invocation{positionals: []string{"remove", "cli-worktree"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &bytes.Buffer{}}, openEffortComposition); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	add := &cmdCtx{ctx: testContext(t), root: root, sub: "worktree", inv: invocation{positionals: []string{"add", "cli-worktree"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &output}
	if err := runEffort(add, openEffortComposition); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "managed path") && strings.Contains(output.String(), "git registration") && strings.Contains(output.String(), "branch") {
		t.Fatalf("add output = %q", output.String())
	}
	output.Reset()
	if err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: "integrate", inv: invocation{positionals: []string{"cli-worktree"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &output}, openEffortComposition); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "already integrated") || !strings.Contains(output.String(), "status:") {
		t.Fatalf("integrate output = %q", output.String())
	}
	output.Reset()
	if err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: "worktree", inv: invocation{positionals: []string{"remove", "cli-worktree"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &output}, openEffortComposition); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "managed path") && strings.Contains(output.String(), "git registration") && strings.Contains(output.String(), "branch") {
		t.Fatalf("remove output = %q", output.String())
	}
	if _, err := filepath.Abs(root); err != nil {
		t.Fatal(err)
	}
}

// TestEffortHandlerComposesTheProductionWiring pins the dispatch entry itself:
// the registered handler is what binds the effort command to the composition
// root, so it is exercised rather than bypassed by calling runEffort directly.
func TestEffortHandlerComposesTheProductionWiring(t *testing.T) {
	root := commandRepo(t)
	var out bytes.Buffer
	if result := newRunner(os.Getwd, os.Stdin, func() bool { return false }).handlers["effort"](&cmdCtx{ctx: testContext(t), root: root, sub: "list", inv: invocation{bools: map[string]bool{}, values: map[string]string{}}, stdout: &out}); result.err != nil {
		t.Fatal(result.err)
	}
	if out.String() != "efforts: none\n" {
		t.Fatalf("empty list = %q", out.String())
	}
}

// TestEffortCompositionRefusesAnUnusableResidentRoot covers the composition's
// own refusal path: the control roots resolve and the repository opens, but the
// resident state the effort service owns is not usable.
func TestEffortCompositionRefusesAnUnusableResidentRoot(t *testing.T) {
	root := commandRepo(t)
	worktrees := filepath.Join(root, ".awf", "worktrees")
	if err := os.RemoveAll(worktrees); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), worktrees); err != nil {
		t.Fatal(err)
	}
	if _, err := openEffortComposition(testContext(t), root, application.Request{Kind: application.List}, func() ([]byte, error) { return []byte("marker\n"), nil }); err == nil {
		t.Fatal("symlinked resident worktrees root accepted")
	}
}

func TestWorktreeCompositionFailuresRemainSilentOnStdout(t *testing.T) {
	unopenable := func(context.Context, string, application.Request, func() ([]byte, error)) (application.Result, error) {
		return application.Result{}, errors.New("injected application")
	}
	root := commandRepo(t)
	for _, test := range []struct {
		sub string
		pos []string
	}{
		{sub: "new", pos: []string{"Unopenable manager"}},
		{sub: "worktree", pos: []string{"add", "slug"}},
		{sub: "integrate", pos: []string{"slug"}},
	} {
		var stdout bytes.Buffer
		values := map[string]string{}
		if test.sub == "new" {
			values["--slug"] = "unopenable-manager"
		}
		err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: test.sub, inv: invocation{positionals: test.pos, bools: map[string]bool{}, values: values}, stdout: &stdout}, unopenable)
		if err == nil || stdout.Len() != 0 {
			t.Fatalf("%s err=%v stdout=%q", test.sub, err, stdout.String())
		}
	}
}
