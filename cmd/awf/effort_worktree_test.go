package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/worktree"
)

func TestEffortAttachmentErrorContracts(t *testing.T) {
	cause := &worktree.RefusalError{Category: "cleanliness", Risk: "dirty", Forceable: true}
	err := newWorktreeAttachmentError("id", cause)
	if !strings.Contains(err.Error(), "category=cleanliness") || !errors.Is(err, cause) {
		t.Fatalf("attachment error=%v", err)
	}
	if !errors.Is((&worktreeAttachmentError{Cause: cause}).Unwrap(), cause) {
		t.Fatal("unwrap lost cause")
	}
	old := openWorktreeManager
	defer func() { openWorktreeManager = old }()
	openWorktreeManager = func(context.Context, string, worktree.Options) (*worktree.Manager, error) {
		return nil, errors.New("injected open")
	}
	root := commandRepo(t)
	var out bytes.Buffer
	err = runEffort(&cmdCtx{root: root, sub: "new", inv: invocation{positionals: []string{"attach"}, bools: map[string]bool{"--worktree": true}, values: map[string]string{}}, stdout: &out})
	if err == nil || !strings.Contains(err.Error(), "effortId=") || !strings.Contains(err.Error(), "category=unknown") {
		t.Fatalf("open failure=%v", err)
	}
	for _, sub := range []string{"worktree", "integrate", "integrated"} {
		if err := runEffort(&cmdCtx{root: root, sub: sub, inv: invocation{positionals: map[string][]string{"worktree": {"add", "x"}, "integrate": {"x"}, "integrated": {"x"}}[sub], bools: map[string]bool{}, values: map[string]string{}}, stdout: &out}); err == nil {
			t.Fatalf("%s accepted injected open failure", sub)
		}
	}
	if err := runEffort(&cmdCtx{root: root, sub: "worktree", inv: invocation{positionals: []string{"add"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &out}); err == nil {
		t.Fatal("worktree accepted malformed arity")
	}
}

func TestEffortWorktreeCLIComposition(t *testing.T) {
	root := commandRepo(t)
	var out bytes.Buffer
	ctx := &cmdCtx{root: root, sub: "new", inv: invocation{positionals: []string{"cli worktree"}, bools: map[string]bool{"--worktree": true}, values: map[string]string{}}, stdout: &out}
	if err := runEffort(ctx); err != nil {
		t.Fatal(err)
	}
	id := strings.Fields(out.String())[1]
	for _, tc := range []struct {
		sub   string
		pos   []string
		bools map[string]bool
		vals  map[string]string
	}{
		{"worktree", []string{"add", id}, map[string]bool{}, map[string]string{}},
		{"integrate", []string{id}, map[string]bool{}, map[string]string{}},
		{"integrated", []string{id}, map[string]bool{}, map[string]string{"--commit": "HEAD"}},
	} {
		var b bytes.Buffer
		_ = runEffort(&cmdCtx{root: root, sub: tc.sub, inv: invocation{positionals: tc.pos, bools: tc.bools, values: tc.vals}, stdout: &b})
	}
	// Complete usage and the already-attached remove path through the actual CLI.
	_ = runEffort(&cmdCtx{root: root, sub: "worktree", inv: invocation{positionals: []string{"remove", id}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &out})
	if err := runEffort(&cmdCtx{root: root, sub: "integrated", inv: invocation{positionals: []string{id}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &out}); err == nil {
		t.Fatal("integrated accepted missing commit")
	}
	for _, sub := range []string{"worktree", "integrate", "integrated"} {
		if err := runEffort(&cmdCtx{root: filepath.Join(root, "missing"), sub: sub, inv: invocation{positionals: []string{id}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &out}); err == nil {
			t.Fatalf("%s accepted invalid repository", sub)
		}
	}
	for _, sub := range []string{"integrate", "integrated"} {
		if err := runEffort(&cmdCtx{root: filepath.Join(root, "missing"), sub: sub, inv: invocation{positionals: []string{id}, bools: map[string]bool{}, values: map[string]string{"--commit": "HEAD"}}, stdout: &out}); err == nil {
			t.Fatalf("%s accepted missing root", sub)
		}
	}
	if err := runEffort(&cmdCtx{root: root, sub: "worktree", inv: invocation{positionals: []string{"bad", id}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &out}); err == nil {
		t.Fatal("bad worktree action accepted")
	}
	_ = filepath.Separator
	_ = os.ErrNotExist
}
