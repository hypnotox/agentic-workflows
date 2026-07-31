package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

// invariant: tooling/effort-management:default-worktree-creation
func TestEffortNewReportsDefaultAndOptedOutWorktrees(t *testing.T) {
	root := commandRepo(t)
	managed := filepath.Join(root, ".awf", "worktrees", "default-cli")
	text := runEffortCommand(t, root, "new", []string{"Default CLI"}, nil)
	wantText := "effort default-cli title=\"Default CLI\" memory=.awf/efforts/default-cli/memory.md worktree=" + managed + " branch=awf/default-cli\n" +
		"managed worktree added for default-cli; changed topology: yes; next action: continue the effort in " + managed + "\n"
	if text != wantText {
		t.Fatalf("default new text =\n%q\nwant\n%q", text, wantText)
	}
	if info, err := os.Lstat(managed); err != nil || !info.IsDir() {
		t.Fatalf("managed checkout absent: %v", err)
	}
	if !effortWorktreeBranchExists(t, root, "default-cli") {
		t.Fatal("managed branch absent")
	}

	createdJSON := runEffortCommand(t, root, "new", []string{"JSON CLI"}, map[string]bool{"--json": true})
	var created struct {
		Worktree *effortWorktreeFacts `json:"worktree"`
	}
	if err := json.Unmarshal([]byte(createdJSON), &created); err != nil {
		t.Fatal(err)
	}
	jsonManaged := filepath.Join(root, ".awf", "worktrees", "json-cli")
	if created.Worktree == nil || created.Worktree.Path != jsonManaged || created.Worktree.Branch != "awf/json-cli" {
		t.Fatalf("default new JSON worktree = %#v, want %q on awf/json-cli", created.Worktree, jsonManaged)
	}

	absent := runEffortCommand(t, root, "new", []string{"Opted out"}, map[string]bool{"--no-worktree": true})
	wantAbsent := "effort opted-out title=\"Opted out\" memory=.awf/efforts/opted-out/memory.md worktree=none\n" +
		"no managed worktree; changed topology: no; next action: continue the effort in " + root + "\n"
	if absent != wantAbsent {
		t.Fatalf("--no-worktree text =\n%q\nwant\n%q", absent, wantAbsent)
	}
	if _, err := os.Lstat(filepath.Join(root, ".awf", "worktrees", "opted-out")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("--no-worktree created topology: %v", err)
	}
	absentJSON := runEffortCommand(t, root, "new", []string{"Opted out JSON"}, map[string]bool{"--json": true, "--no-worktree": true})
	if !strings.Contains(absentJSON, `"worktree":null`) {
		t.Fatalf("--no-worktree JSON = %q, want an explicit null worktree", absentJSON)
	}
}

// invariant: tooling/effort-management:default-worktree-creation
func TestEffortNewBasesTheManagedBranchOnTheNamedRef(t *testing.T) {
	root := commandRepo(t)
	base := gitfixture.NativeRevParse(t, gitfixture.At(root), "HEAD")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("second\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitfixture.NativeAdd(t, gitfixture.At(root), "tracked.txt")
	gitfixture.NativeCommit(t, gitfixture.At(root), "second")
	var out bytes.Buffer
	ctx := &cmdCtx{ctx: testContext(t), root: root, sub: "new", inv: invocation{positionals: []string{"Based CLI"}, bools: map[string]bool{}, values: map[string]string{"--base": base}}, stdout: &out}
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
	rolled := &cmdCtx{ctx: testContext(t), root: root, sub: "new", inv: invocation{positionals: []string{"Rolled back CLI"}, bools: map[string]bool{}, values: map[string]string{"--base": "no-such-ref"}}, stdout: &out}
	err := runEffort(rolled, openEffortComposition)
	if err == nil || !strings.Contains(err.Error(), "effort rolled-back-cli rolled back") || !strings.Contains(err.Error(), "retry `awf effort new`") {
		t.Fatalf("unresolvable base error = %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(root, ".awf", "efforts", "rolled-back-cli")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("rolled-back effort resident remains: %v", statErr)
	}

	gitfixture.NativeBranch(t, gitfixture.At(root), "awf/retained-cli")
	out.Reset()
	retained := &cmdCtx{ctx: testContext(t), root: root, sub: "new", inv: invocation{positionals: []string{"Retained CLI"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &out}
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

// invariant: tooling/effort-management:default-worktree-creation
func TestEffortNewRejectsBaseWithoutAWorktree(t *testing.T) {
	root := commandRepo(t)
	var out bytes.Buffer
	ctx := &cmdCtx{ctx: testContext(t), root: root, sub: "new", inv: invocation{positionals: []string{"Invalid CLI"}, bools: map[string]bool{"--no-worktree": true}, values: map[string]string{"--base": "HEAD"}}, stdout: &out}
	err := runEffort(ctx, openEffortComposition)
	if err == nil || !strings.Contains(err.Error(), "--base is invalid with --no-worktree") {
		t.Fatalf("--no-worktree --base error = %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(root, ".awf", "efforts", "invalid-cli")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("refused creation published a resident: %v", statErr)
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
	runEffortCommand(t, root, "new", []string{"CLI worktree"}, map[string]bool{"--no-worktree": true})
	var output bytes.Buffer
	add := &cmdCtx{ctx: testContext(t), root: root, sub: "worktree", inv: invocation{positionals: []string{"add", "cli-worktree"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &output}
	if err := runEffort(add, openEffortComposition); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "changed topology: yes") {
		t.Fatalf("add output = %q", output.String())
	}
	output.Reset()
	if err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: "integrate", inv: invocation{positionals: []string{"cli-worktree"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &output}, openEffortComposition); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "already integrated") || !strings.Contains(output.String(), "changed topology: no") {
		t.Fatalf("integrate output = %q", output.String())
	}
	output.Reset()
	if err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: "worktree", inv: invocation{positionals: []string{"remove", "cli-worktree"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &output}, openEffortComposition); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "changed topology: yes") {
		t.Fatalf("remove output = %q", output.String())
	}
	if _, err := filepath.Abs(root); err != nil {
		t.Fatal(err)
	}
}

func TestIntegrationGateCommandUsesOnlyAConfiguredString(t *testing.T) {
	for _, test := range []struct {
		name       string
		configYAML string
		want       string
		wantErr    bool
	}{
		{name: "absent config"},
		{name: "configured", configYAML: "prefix: example\nvars: {gateCmd: make gate}\n", want: "make gate"},
		{name: "blank", configYAML: "prefix: example\nvars: {gateCmd: \"  \"}\n"},
		{name: "non-string", configYAML: "prefix: example\nvars: {gateCmd: [make, gate]}\n"},
		{name: "malformed", configYAML: "unknown: value\n", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if test.configYAML != "" {
				if err := os.Mkdir(filepath.Join(root, ".awf"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte(test.configYAML), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := integrationGateCommand(root)
			if (err != nil) != test.wantErr || got != test.want {
				t.Fatalf("integrationGateCommand() = %q, %v; want %q, error=%v", got, err, test.want, test.wantErr)
			}
			if test.wantErr {
				commandRoot := commandRepo(t)
				if err := os.MkdirAll(filepath.Join(commandRoot, ".awf"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(commandRoot, ".awf", "config.yaml"), []byte(test.configYAML), 0o644); err != nil {
					t.Fatal(err)
				}
				err = runEffort(&cmdCtx{ctx: testContext(t), root: commandRoot, sub: "integrate", inv: invocation{positionals: []string{"any-effort"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: io.Discard}, openEffortComposition)
				if err == nil || !strings.Contains(err.Error(), "unknown") {
					t.Fatalf("integrate malformed config error = %v", err)
				}
			}
		})
	}
}

// TestEffortHandlerComposesTheProductionWiring pins the dispatch entry itself:
// the registered handler is what binds the effort command to the composition
// root, so it is exercised rather than bypassed by calling runEffort directly.
func TestEffortHandlerComposesTheProductionWiring(t *testing.T) {
	root := commandRepo(t)
	var out bytes.Buffer
	if err := handlers["effort"](&cmdCtx{ctx: testContext(t), root: root, sub: "list", inv: invocation{bools: map[string]bool{}, values: map[string]string{}}, stdout: &out}); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("empty list wrote %q", out.String())
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
	if _, err := openEffortComposition(testContext(t), root); err == nil {
		t.Fatal("symlinked resident worktrees root accepted")
	}
}

func TestWorktreeCompositionFailuresRemainSilentOnStdout(t *testing.T) {
	unopenable := func(context.Context, string) (effortComposition, error) {
		return effortComposition{}, errors.New("injected open")
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
		err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: test.sub, inv: invocation{positionals: test.pos, bools: map[string]bool{}, values: map[string]string{}}, stdout: &stdout}, unopenable)
		if err == nil || stdout.Len() != 0 {
			t.Fatalf("%s err=%v stdout=%q", test.sub, err, stdout.String())
		}
	}
}
