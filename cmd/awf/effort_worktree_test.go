package main

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/worktree"
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

func TestEffortWorktreeCLIComposition(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := commandRepo(t)
	runEffortCommand(t, root, "new", []string{"CLI worktree"}, nil)
	var output bytes.Buffer
	add := &cmdCtx{ctx: testContext(t), root: root, sub: "worktree", inv: invocation{positionals: []string{"add", "cli-worktree"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &output}
	if err := runEffort(add); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "changed topology: yes") {
		t.Fatalf("add output = %q", output.String())
	}
	output.Reset()
	if err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: "integrate", inv: invocation{positionals: []string{"cli-worktree"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &output}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "already integrated") || !strings.Contains(output.String(), "changed topology: no") {
		t.Fatalf("integrate output = %q", output.String())
	}
	output.Reset()
	if err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: "worktree", inv: invocation{positionals: []string{"remove", "cli-worktree"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &output}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "changed topology: yes") {
		t.Fatalf("remove output = %q", output.String())
	}
	if _, err := filepath.Abs(root); err != nil {
		t.Fatal(err)
	}
}

func TestWorktreeManagerOpenFailuresRemainSilentOnStdout(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	old := openWorktreeManager
	defer func() { openWorktreeManager = old }()
	openWorktreeManager = func(context.Context, string, worktree.Options) (*worktree.Manager, error) {
		return nil, errors.New("injected open")
	}
	root := commandRepo(t)
	for _, test := range []struct {
		sub string
		pos []string
	}{
		{sub: "worktree", pos: []string{"add", "slug"}},
		{sub: "integrate", pos: []string{"slug"}},
	} {
		var stdout bytes.Buffer
		err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: test.sub, inv: invocation{positionals: test.pos, bools: map[string]bool{}, values: map[string]string{}}, stdout: &stdout})
		if err == nil || stdout.Len() != 0 {
			t.Fatalf("%s err=%v stdout=%q", test.sub, err, stdout.String())
		}
	}
}
