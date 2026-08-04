package main

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

const readCommandV2Plan = `---
format: plan-v2
date: 2026-08-03
adrs: [fixture, context, third]
status: Proposed
---
# Plan: Read command V2

## Goal

Project the exact selected work without leaking unselected decisions.

## Architecture summary

Keep plan parsing and decision resolution at their existing boundaries.

## Phase 1: Project

**Execution mode: inline.**

Advances: ["advanced"]
Completes: ["completed"]

### Task 1.1: Read selected
Applying: ["fixture:first"]
Context: ["context:second"]

Keep this exact task body.

### Task 1.2: Keep unselected
Applying: ["context:second"]
Context: ["third:third"]

Do not leak this task or its decision into task 1.1.

### Phase close

Run the exact gate.

` + "```commit\nfeat(plans): project task context\n```" + `

## Definition of done

- ` + "`dod: advanced`" + ` Advance the exact projection.
- ` + "`dod: completed`" + ` Complete the exact projection.

## Notes

Keep the exact note.
`

func readCommandV4ADRFor(t *testing.T, slug, title, decision string) string {
	t.Helper()
	proposed := "---\nformat: current-state-v4\nstatus: Proposed\ndate: 2026-08-03\nslug: " + slug + "\n---\n# ADR-" + slug + ": " + title + "\n\n## Context\n\nFrozen context.\n\n## Decision\n\n1. " + decision + "\n\n## State changes\n\nNone.\n\n## Consequences\n\nNone.\n\n## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-08-02: Proposed\n"
	record, err := adr.ParseV4(slug+".md", []byte(proposed))
	if err != nil {
		t.Fatal(err)
	}
	implemented := strings.Replace(proposed, "status: Proposed", "status: Implemented", 1)
	return strings.Replace(implemented, "- 2026-08-02: Proposed\n", "- 2026-08-02: Proposed\n- 2026-08-03: Implemented; content-sha256: "+adr.ContentDigest(record.Sections)+"\n", 1)
}

const readCommandPromotionPlan = `---
format: plan-v2
date: 2026-08-03
adrs: [fixture, context]
status: Proposed
---
# Plan: Promotion order

## Goal

Preserve first-authored Decision order across promotion.

## Architecture summary

Keep reference ordering in the project composition boundary.

## Phase 1: Project

**Execution mode: inline.**

### Task 1.1: Introduce context
Context: ["context:second"]

Use the earlier context.

### Task 1.2: Promote context
Applying: ["fixture:first", "context:second"]

Promote the earlier context after another Applying reference.

### Phase close

Run the gate.

` + "```commit\nfeat(plans): preserve promotion order\n```" + `

## Definition of done

- ` + "`dod: ordered`" + ` Preserve first-authored order.
`

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
	parsed, err := plan.Resolve(filepath.Dir(path), filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	wantPayload, err := plan.RenderProjection(parsed, "1.1")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stdout.Bytes(), wantPayload) {
		t.Fatalf("read plan changed payload bytes with a presentation prefix, suffix, or normalization:\n--- got ---\n%s--- want ---\n%s", stdout.Bytes(), wantPayload)
	}
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

	v2Path := filepath.Join(root, "docs/plans/2026-08-03-read-command-v2.md")
	adrPath := filepath.Join(root, "docs/decisions/fixture.md")
	contextADRPath := filepath.Join(root, "docs/decisions/context.md")
	thirdADRPath := filepath.Join(root, "docs/decisions/third.md")
	fixtureADR := readCommandV4ADRFor(t, "fixture", "Fixture decisions", "`decision: first` First exact Decision block.")
	contextADR := readCommandV4ADRFor(t, "context", "Context decisions", "`decision: second` Second exact Decision block.")
	thirdADR := readCommandV4ADRFor(t, "third", "Third decisions", "`decision: third` Third unselected Decision block.")
	testsupport.WriteFile(t, v2Path, readCommandV2Plan)
	testsupport.WriteFile(t, adrPath, fixtureADR)
	testsupport.WriteFile(t, contextADRPath, contextADR)
	testsupport.WriteFile(t, thirdADRPath, thirdADR)
	v2Before, err := os.ReadFile(v2Path)
	if err != nil {
		t.Fatal(err)
	}
	adrBefore, err := os.ReadFile(adrPath)
	if err != nil {
		t.Fatal(err)
	}
	contextADRBefore, err := os.ReadFile(contextADRPath)
	if err != nil {
		t.Fatal(err)
	}
	thirdADRBefore, err := os.ReadFile(thirdADRPath)
	if err != nil {
		t.Fatal(err)
	}
	v2Hash, adrHash := sha256.Sum256(v2Before), sha256.Sum256(adrBefore)
	contextADRHash, thirdADRHash := sha256.Sum256(contextADRBefore), sha256.Sum256(thirdADRBefore)

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"awf", "read", "plan", "2026-08-03-read-command-v2", "1.1"}, &stdout, &stderr); code != 0 {
		t.Fatalf("v2 task read exit = %d, stderr = %q", code, stderr.String())
	}
	v2Task := stdout.String()
	for _, want := range []string{
		"## Goal", "## Architecture summary", "## Applying decisions", "### ADR-fixture: Fixture decisions (Implemented)",
		"First exact Decision block.", "## Context decisions", "### ADR-context: Context decisions (Implemented)", "Second exact Decision block.", "## Phase 1: Project",
		"Scope notice: only this task is in scope", "### Task 1.1: Read selected", "### Phase close",
		"### Advanced outcomes (phase-owner context)", "- `dod: advanced` Advance the exact projection.",
		"### Completed outcomes (phase-owner context)", "- `dod: completed` Complete the exact projection.", "## Notes",
	} {
		if !strings.Contains(v2Task, want) {
			t.Errorf("v2 task projection lacks %q:\n%s", want, v2Task)
		}
	}
	ordered := []string{"## Goal", "## Architecture summary", "## Applying decisions", "## Context decisions", "## Phase 1: Project", "### Task 1.1: Read selected", "### Phase close", "### Advanced outcomes (phase-owner context)", "### Completed outcomes (phase-owner context)", "## Notes"}
	for i := 1; i < len(ordered); i++ {
		if strings.Index(v2Task, ordered[i-1]) >= strings.Index(v2Task, ordered[i]) {
			t.Errorf("v2 task projection order %q before %q:\n%s", ordered[i-1], ordered[i], v2Task)
		}
	}
	if strings.Count(v2Task, "### ADR-fixture: Fixture decisions (Implemented)") != 1 ||
		strings.Count(v2Task, "### ADR-context: Context decisions (Implemented)") != 1 ||
		strings.Contains(v2Task, "### Task 1.2") || strings.Contains(v2Task, "Third unselected Decision block.") ||
		strings.Contains(v2Task, "## Definition of done") || stderr.Len() != 0 {
		t.Fatalf("v2 task projection leaked or duplicated scope; stdout=%q stderr=%q", v2Task, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"awf", "read", "plan", "2026-08-03-read-command-v2", "1"}, &stdout, &stderr); code != 0 {
		t.Fatalf("v2 phase read exit = %d, stderr = %q", code, stderr.String())
	}
	v2Phase := stdout.String()
	if strings.Count(v2Phase, "### ADR-fixture: Fixture decisions (Implemented)") != 1 ||
		strings.Count(v2Phase, "### ADR-context: Context decisions (Implemented)") != 1 ||
		strings.Count(v2Phase, "### ADR-third: Third decisions (Implemented)") != 1 ||
		strings.Index(v2Phase, "First exact Decision block.") > strings.Index(v2Phase, "Second exact Decision block.") ||
		strings.Index(v2Phase, "Second exact Decision block.") > strings.Index(v2Phase, "Third unselected Decision block.") ||
		!strings.Contains(v2Phase, "## Context decisions") || stderr.Len() != 0 {
		t.Fatalf("v2 phase projection did not first-author dedupe and promote Applying decisions; stdout=%q stderr=%q", v2Phase, stderr.String())
	}

	promotionPath := filepath.Join(root, "docs/plans/2026-08-03-promotion-order.md")
	testsupport.WriteFile(t, promotionPath, readCommandPromotionPlan)
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"awf", "read", "plan", "2026-08-03-promotion-order", "1"}, &stdout, &stderr); code != 0 {
		t.Fatalf("promotion-order phase read exit = %d, stderr = %q", code, stderr.String())
	}
	promotion := stdout.String()
	if contextAt, fixtureAt := strings.Index(promotion, "Second exact Decision block."), strings.Index(promotion, "First exact Decision block."); contextAt < 0 || fixtureAt < 0 || contextAt > fixtureAt {
		t.Fatalf("promoted Context lost first-authored order; stdout=%q", promotion)
	}

	for _, source := range []struct {
		path string
		want []byte
		hash [sha256.Size]byte
	}{{v2Path, v2Before, v2Hash}, {adrPath, adrBefore, adrHash}, {contextADRPath, contextADRBefore, contextADRHash}, {thirdADRPath, thirdADRBefore, thirdADRHash}} {
		got, readErr := os.ReadFile(source.path)
		if readErr != nil || !bytes.Equal(got, source.want) || sha256.Sum256(got) != source.hash {
			t.Fatalf("read command mutated %s: err=%v bytes-equal=%v hash-equal=%v", source.path, readErr, bytes.Equal(got, source.want), sha256.Sum256(got) == source.hash)
		}
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"awf", "read", "plan", "2026-08-03-read-command-v2", "01"}, &stdout, &stderr); code != 1 || stdout.Len() != 0 || stderr.String() != "condition: awf: plan selector \"01\" must be canonical positive P or P.T; available: 1, 1.1, 1.2\n" {
		t.Fatalf("v2 typed selector failure: exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	brokenPath := filepath.Join(root, "docs/plans/2026-08-03-broken-reference.md")
	testsupport.WriteFile(t, brokenPath, strings.Replace(readCommandV2Plan, "fixture:first", "missing:first", 1))
	stdout.Reset()
	stderr.Reset()
	const missingReferenceError = "condition: awf: plan 2026-08-03-broken-reference.md task 1.1 Applying \"missing:first\": ADR not found\n"
	if code := run([]string{"awf", "read", "plan", "2026-08-03-broken-reference", "1.1"}, &stdout, &stderr); code != 1 || stdout.Len() != 0 || stderr.String() != missingReferenceError {
		t.Fatalf("broken v2 reference did not preserve task coordinates: exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	absentPath := filepath.Join(root, "docs/plans/2026-08-03-absent-applying.md")
	absentPlan := strings.Replace(readCommandV2Plan, "adrs: [fixture, context, third]", "adrs: [context, third]", 1)
	testsupport.WriteFile(t, absentPath, absentPlan)
	stdout.Reset()
	stderr.Reset()
	const absentApplyingError = "condition: awf: plan 2026-08-03-absent-applying.md task 1.1 Applying \"fixture:first\": Applying ADR is absent from adrs\n"
	if code := run([]string{"awf", "read", "plan", "2026-08-03-absent-applying", "1.1"}, &stdout, &stderr); code != 1 || stdout.Len() != 0 || stderr.String() != absentApplyingError {
		t.Fatalf("Applying outside plan adrs did not block read: exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
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
		args       []string
		wantStderr string
	}{
		{[]string{"awf", "read", "plan", "missing", "1"}, "condition: awf: plan name \"missing\" not found; available: 2026-08-02-read-command, 2026-08-02-read-command.md\n"},
		{[]string{"awf", "read", "plan", "2026-08-02-read-command", "01"}, "condition: awf: plan selector \"01\" must be canonical positive P or P.T; available: 1, 1.1, 1.2\n"},
		{[]string{"awf", "read", "plan", "2026-08-02-read-command"}, ""},
	}
	for _, tc := range cases {
		var stdout, stderr bytes.Buffer
		if code := run(tc.args, &stdout, &stderr); code == 0 || stdout.Len() != 0 || stderr.Len() == 0 {
			t.Errorf("run(%v) exit=%d stdout=%q stderr=%q", tc.args, code, stdout.String(), stderr.String())
		}
		if tc.wantStderr != "" && stderr.String() != tc.wantStderr {
			t.Errorf("run(%v) stderr = %q, want exact available-values diagnostic %q", tc.args, stderr.String(), tc.wantStderr)
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
