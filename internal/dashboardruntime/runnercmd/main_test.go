package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/dashboardruntime"
)

// invariant: rendering/companion-scripts:dashboard-development-runtime-commands
func TestRunPathAndAdvance(t *testing.T) {
	oldResolve, oldAdvance := resolveRuntime, advanceRuntime
	t.Cleanup(func() { resolveRuntime, advanceRuntime = oldResolve, oldAdvance })
	resolveRuntime = func(root string, env dashboardruntime.BuildEnvironment) (dashboardruntime.Launcher, error) {
		if root != "/project" || env.Stderr == nil {
			t.Fatalf("Resolve root/env = %q %#v", root, env)
		}
		return dashboardruntime.Launcher{Path: "/cache/launcher"}, nil
	}
	advanceRuntime = func(root, revision string, env dashboardruntime.BuildEnvironment) (dashboardruntime.AdvanceResult, error) {
		if root != "/project" || revision != "HEAD" || env.Stderr == nil {
			t.Fatalf("Advance root/revision/env = %q %q %#v", root, revision, env)
		}
		return dashboardruntime.AdvanceResult{OldCommit: "old", NewCommit: "new", LauncherPath: "/cache/new-launcher"}, nil
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"path", "/project"}, &stdout, &stderr); code != 0 || stdout.String() != "/cache/launcher\n" || stderr.Len() != 0 {
		t.Fatalf("path code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	if code := run([]string{"advance", "/project"}, &stdout, &stderr); code != 0 || stdout.String() != "old commit: old\nnew commit: new\nlauncher path: /cache/new-launcher\n" || stderr.Len() != 0 {
		t.Fatalf("advance code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunExplicitRevisionErrorsAndUsage(t *testing.T) {
	oldResolve, oldAdvance := resolveRuntime, advanceRuntime
	t.Cleanup(func() { resolveRuntime, advanceRuntime = oldResolve, oldAdvance })
	resolveRuntime = func(string, dashboardruntime.BuildEnvironment) (dashboardruntime.Launcher, error) {
		return dashboardruntime.Launcher{}, errors.New("resolve failed")
	}
	advanceRuntime = func(_ string, revision string, _ dashboardruntime.BuildEnvironment) (dashboardruntime.AdvanceResult, error) {
		if revision != "commit" {
			t.Fatalf("revision = %q", revision)
		}
		return dashboardruntime.AdvanceResult{}, errors.New("advance failed")
	}
	for _, test := range []struct {
		args       []string
		code       int
		stderrPart string
	}{
		{nil, 2, "usage:"},
		{[]string{"unknown", "/project"}, 2, "usage:"},
		{[]string{"path", "/project", "extra"}, 2, "usage: awf-dashboard-runtime path"},
		{[]string{"advance", "/project", "commit", "extra"}, 2, "usage: awf-dashboard-runtime advance"},
		{[]string{"path", "/project"}, 1, "dashboard-awf-path: resolve failed"},
		{[]string{"advance", "/project", "commit"}, 1, "dashboard-awf-advance: advance failed"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(test.args, &stdout, &stderr); code != test.code || stdout.Len() != 0 || !strings.Contains(stderr.String(), test.stderrPart) {
			t.Errorf("run(%q) code=%d stdout=%q stderr=%q", test.args, code, stdout.String(), stderr.String())
		}
	}
}
