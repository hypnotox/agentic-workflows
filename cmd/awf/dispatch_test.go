package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRunGetwdError(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newRunner(func() (string, error) { return "", errors.New("boom") }, os.Stdin, func() bool { return false }).run([]string{"awf", "render"}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 on getwd error, got %d", code)
	}
	if out.Len() != 0 || errb.String() != "condition: awf: boom\n" {
		t.Errorf("streams stdout=%q stderr=%q", out.String(), errb.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	root := t.TempDir()
	var out, errb bytes.Buffer
	if code := runFrom(root, []string{"awf", "bogus"}, &out, &errb); code != 2 {
		t.Fatalf("expected exit 2 for unknown command, got %d", code)
	}
	if !strings.Contains(errb.String(), "unknown command") {
		t.Errorf("missing unknown-command text: %q", errb.String())
	}
}

func TestRunDispatchError(t *testing.T) {
	// Rendering in a bare directory fails Session loading, then exits 1.
	root := t.TempDir()
	var out, errb bytes.Buffer
	if code := runFrom(root, []string{"awf", "render"}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 on dispatch error, got %d", code)
	}
	if !strings.HasPrefix(errb.String(), "condition: awf:") {
		t.Errorf("expected typed diagnostic, got %q", errb.String())
	}
}

func TestRunnerComposesListAndRemoveDomainHandlers(t *testing.T) {
	runner := newRunner(os.Getwd, os.Stdin, func() bool { return false })
	root := t.TempDir()
	for _, tc := range []struct {
		name, handler string
		ctx           *cmdCtx
	}{
		{name: "list", handler: "list", ctx: &cmdCtx{ctx: testContext(t), root: root, inv: invocation{bools: map[string]bool{}, values: map[string]string{}}, stdout: io.Discard}},
		{name: "remove domain", handler: "remove", ctx: &cmdCtx{ctx: testContext(t), root: root, sub: "domain", inv: invocation{positionals: []string{"missing"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: io.Discard}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if result := runner.handlers[tc.handler](tc.ctx); result.err == nil {
				t.Fatal("bare-project handler unexpectedly succeeded")
			}
		})
	}
}
