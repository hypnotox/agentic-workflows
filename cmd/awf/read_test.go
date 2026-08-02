package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

const readCommandPlan = `---
format: plan-v1
date: 2026-08-02
adrs: []
status: Proposed
---
# Plan: Read command

## Goal

Return a bounded projection without widening scope.

## Architecture summary

Keep Markdown rendering in internal/plan.

## Phase 1: Read

**Execution mode: inline.**

### Task 1.1: Return bytes

Write the projection unchanged.

### Task 1.2: Keep omitted

This task is omitted from a task projection.

### Phase close

Run the gate.

` + "```commit\nfeat(plans): add read command\n```" + `

## Definition of done

- The command returns the executable closure.

## Notes

No deviations.
`

// invariant: tooling/cli:plan-read-command (TestReadPlanCommand)
func TestReadPlanCommand(t *testing.T) {
	root := syncedGitProject(t)
	path := filepath.Join(root, "docs/plans/2026-08-02-read-command.md")
	testsupport.WriteFile(t, path, readCommandPlan)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"awf", "read", "plan", "2026-08-02-read-command", "1.1"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	got := stdout.String()
	for _, want := range []string{"format: plan-v1", "# Plan: Read command", "## Goal", "## Architecture summary", "## Phase 1: Read", "**Execution mode: inline.**", "### Task 1.1: Return bytes", "### Phase close", "## Definition of done", "## Notes"} {
		if !strings.Contains(got, want) {
			t.Errorf("projection lacks %q", want)
		}
	}
	if strings.Contains(got, "### Task 1.2") || stderr.Len() != 0 {
		t.Fatalf("stdout = %q, stderr = %q", got, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"awf", "read", "plan", "2026-08-02-read-command.md", "1"}, &stdout, &stderr); code != 0 {
		t.Fatalf("filename phase read exit = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "### Task 1.1") || !strings.Contains(stdout.String(), "### Task 1.2") || stderr.Len() != 0 {
		t.Fatalf("phase stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("read command mutated source")
	}
	t.Run("direct failures", TestRunReadPlanRejectsArityAndUnadoptedRoot)
	t.Run("typed failures", TestReadPlanCommandFailuresKeepStdoutEmpty)
	t.Run("version gate", TestReadPlanCommandIsVersionGated)
}

func TestRunReadPlanRejectsArityAndUnadoptedRoot(t *testing.T) {
	var stdout bytes.Buffer
	if err := runReadPlan(testContext(t), t.TempDir(), nil, &stdout); err == nil || !strings.Contains(err.Error(), "usage: awf read plan") {
		t.Fatalf("arity error = %v", err)
	}
	if err := runReadPlan(testContext(t), t.TempDir(), []string{"plan", "1"}, &stdout); err == nil || !strings.Contains(err.Error(), "awf init") {
		t.Fatalf("open error = %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestReadPlanCommandFailuresKeepStdoutEmpty(t *testing.T) {
	root := syncedGitProject(t)
	testsupport.WriteFile(t, filepath.Join(root, "docs/plans/2026-08-02-read-command.md"), readCommandPlan)
	t.Chdir(root)
	cases := []struct {
		args           []string
		availableValue string
	}{
		{[]string{"awf", "read", "plan", "missing", "1"}, "2026-08-02-read-command"},
		{[]string{"awf", "read", "plan", "2026-08-02-read-command", "01"}, "1.1"},
		{[]string{"awf", "read", "plan", "2026-08-02-read-command"}, ""},
	}
	for _, tc := range cases {
		var stdout, stderr bytes.Buffer
		if code := run(tc.args, &stdout, &stderr); code == 0 || stdout.Len() != 0 || stderr.Len() == 0 {
			t.Errorf("run(%v) exit=%d stdout=%q stderr=%q", tc.args, code, stdout.String(), stderr.String())
		}
		if tc.availableValue != "" && (!strings.Contains(stderr.String(), "available:") || !strings.Contains(stderr.String(), tc.availableValue)) {
			t.Errorf("run(%v) did not list exact available value %q: %q", tc.args, tc.availableValue, stderr.String())
		}
	}
}

func TestReadPlanCommandIsVersionGated(t *testing.T) {
	root := syncedGitProject(t)
	repinLockVersion(t, root, "99.0.0")
	testsupport.WriteFile(t, filepath.Join(root, "docs/plans/2026-08-02-read-command.md"), readCommandPlan)
	t.Chdir(root)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"awf", "read", "plan", "2026-08-02-read-command", "1"}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "behind") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
