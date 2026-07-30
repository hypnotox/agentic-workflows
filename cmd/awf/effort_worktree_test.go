package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/worktree"
)

func TestEffortGrammarIsClosedAndHasNoForceSurface(t *testing.T) {
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
	if err := validateEffortGrammar(&cmdCtx{sub: "worktree", inv: invocation{positionals: []string{"add", "slug"}, values: map[string]string{"--base": "HEAD"}}}); err != nil {
		t.Fatal(err)
	}
}

func TestEffortWorktreeCLIComposition(t *testing.T) {
	root := commandRepo(t)
	runEffortCommand(t, root, "new", []string{"CLI worktree"}, nil)
	var output bytes.Buffer
	add := &cmdCtx{root: root, sub: "worktree", inv: invocation{positionals: []string{"add", "cli-worktree"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &output}
	if err := runEffort(add); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "changed topology: yes") {
		t.Fatalf("add output = %q", output.String())
	}
	output.Reset()
	if err := runEffort(&cmdCtx{root: root, sub: "integrate", inv: invocation{positionals: []string{"cli-worktree"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &output}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "already integrated") || !strings.Contains(output.String(), "changed topology: no") {
		t.Fatalf("integrate output = %q", output.String())
	}
	output.Reset()
	if err := runEffort(&cmdCtx{root: root, sub: "worktree", inv: invocation{positionals: []string{"remove", "cli-worktree"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &output}); err != nil {
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
				err = runEffort(&cmdCtx{root: commandRoot, sub: "integrate", inv: invocation{positionals: []string{"any-effort"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: io.Discard})
				if err == nil || !strings.Contains(err.Error(), "unknown") {
					t.Fatalf("integrate malformed config error = %v", err)
				}
			}
		})
	}
}

func TestWorktreeManagerOpenFailuresRemainSilentOnStdout(t *testing.T) {
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
		err := runEffort(&cmdCtx{root: root, sub: test.sub, inv: invocation{positionals: test.pos, bools: map[string]bool{}, values: map[string]string{}}, stdout: &stdout})
		if err == nil || stdout.Len() != 0 {
			t.Fatalf("%s err=%v stdout=%q", test.sub, err, stdout.String())
		}
	}
}
