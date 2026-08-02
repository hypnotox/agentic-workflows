package plan_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/plan"
)

const projectionPreamble = `---
format: plan-v1
date: 2026-08-02
adrs: []
status: Proposed
---
# Plan: Example

## Goal

Deliver the thing without widening its scope.

## Goal detail

This level-two heading remains opaque Goal Markdown.

## Architecture summary

Keep parsing and rendering in the model owner.

`

const projectionPhase1 = `## Phase 1: Parse

**Execution mode: inline.**

### Task 1.1: Build it
Kind: batch
Latitude: exact
Paths: ["glob:internal/plan/*.go", "pathspec::(top)internal/plan", "docs/plans/template.md"]
Representative: Cover normal input.
Edge: Cover invalid input.
Post-check: go test ./internal/plan

Implement the parser.

### Task 1.2: Investigate
Kind: spike
Question: Which errors are stable?

### Phase close

Run the staged check and gate.

` + "```commit\nfeat(plans): parse plans\n```" + `

`

const projectionTask11 = `## Phase 1: Parse

**Execution mode: inline.**

### Task 1.1: Build it
Kind: batch
Latitude: exact
Paths: ["glob:internal/plan/*.go", "pathspec::(top)internal/plan", "docs/plans/template.md"]
Representative: Cover normal input.
Edge: Cover invalid input.
Post-check: go test ./internal/plan

Implement the parser.

### Phase close

Run the staged check and gate.

` + "```commit\nfeat(plans): parse plans\n```" + `

`

const projectionTail = `## Definition of done

- A valid plan parses and projects.

## Notes

The spike established stable typed diagnostics.
`

// invariant: adr-system/plan-artifacts:plan-executable-projection (TestPlanExecutableProjection)
func TestPlanExecutableProjection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "2026-08-02-example.md")
	writePlan(t, dir, filepath.Base(path), v1Plan)
	beforeBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	beforeHash := sha256.Sum256(beforeBytes)

	parsed, err := plan.Resolve(dir, "2026-08-02-example")
	if err != nil {
		t.Fatal(err)
	}
	phase, err := plan.RenderProjection(parsed, "1")
	if err != nil {
		t.Fatal(err)
	}
	task, err := plan.RenderProjection(parsed, "1.1")
	if err != nil {
		t.Fatal(err)
	}
	if want := projectionPreamble + projectionPhase1 + projectionTail; !bytes.Equal(phase, []byte(want)) {
		t.Fatalf("phase projection:\n%s\nwant:\n%s", phase, want)
	}
	if want := projectionPreamble + projectionTask11 + projectionTail; !bytes.Equal(task, []byte(want)) {
		t.Fatalf("task projection:\n%s\nwant:\n%s", task, want)
	}
	if bytes.Contains(task, []byte("### Task 1.2")) || bytes.Contains(task, []byte("## Phase 2")) {
		t.Fatalf("task projection contains unselected content:\n%s", task)
	}
	afterBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	afterHash := sha256.Sum256(afterBytes)
	if beforeHash != afterHash || !bytes.Equal(parsed.Source, beforeBytes) {
		t.Fatal("projection mutated source bytes")
	}
	t.Run("parse failure", TestPlanResolutionPropagatesParseFailure)
	t.Run("exact names", TestPlanResolutionUsesExactNamesAndSortedAvailability)
	t.Run("symlink confinement", TestPlanResolutionConfinesSymlinkTargets)
	t.Run("ambiguous name", TestPlanResolutionReportsAmbiguousStemFilenameCollision)
	t.Run("selectors", TestPlanProjectionSelectorsAreTypedAndCanonical)
	t.Run("legacy boundary", TestLegacyPlanHasNoExecutableProjection)
}

func TestPlanResolutionPropagatesParseFailure(t *testing.T) {
	dir := t.TempDir()
	writePlan(t, dir, "2026-08-02-broken.md", replaceOnceForTest(v1Plan, "format: plan-v1", "format: plan-v2"))
	_, err := plan.Resolve(dir, "2026-08-02-broken")
	var diagnostics *plan.DiagnosticsError
	if !errors.As(err, &diagnostics) {
		t.Fatalf("Resolve error = %v, want *plan.DiagnosticsError", err)
	}
}

func TestPlanResolutionUsesExactNamesAndSortedAvailability(t *testing.T) {
	dir := t.TempDir()
	writePlan(t, dir, "2026-08-02-example.md", v1Plan)
	writePlan(t, dir, "2026-08-01-other.md", replaceOnceForTest(v1Plan, "# Plan: Example", "# Plan: Other"))
	writePlan(t, dir, "2026-08-03-name..dots.md", replaceOnceForTest(v1Plan, "# Plan: Example", "# Plan: Dots"))
	for _, name := range []string{"2026-08-02-example.md", "2026-08-02-example", "2026-08-03-name..dots.md", "2026-08-03-name..dots"} {
		if _, err := plan.Resolve(dir, name); err != nil {
			t.Errorf("Resolve(%q): %v", name, err)
		}
	}
	wantAvailable := []string{
		"2026-08-01-other", "2026-08-01-other.md",
		"2026-08-02-example", "2026-08-02-example.md",
		"2026-08-03-name..dots", "2026-08-03-name..dots.md",
	}
	for _, name := range []string{"", "../example", "/tmp/example", `C:\\tmp\\example`, `dir\\example`, "Example", "example", "2026-08"} {
		_, err := plan.Resolve(dir, name)
		var notFound *plan.NotFoundError
		if !errors.As(err, &notFound) || notFound.Kind != "name" || notFound.Value != name || !reflect.DeepEqual(notFound.Available, wantAvailable) {
			t.Errorf("Resolve(%q) error = %#v", name, err)
		}
	}
}

func TestPlanResolutionConfinesSymlinkTargets(t *testing.T) {
	dir := t.TempDir()
	insideTarget := filepath.Join(dir, "fixture.md")
	if err := os.WriteFile(insideTarget, []byte(v1Plan), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("fixture.md", filepath.Join(dir, "2026-08-02-inside.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Resolve(dir, "2026-08-02-inside"); err != nil {
		t.Fatalf("inside symlink Resolve: %v", err)
	}

	outsideDir := t.TempDir()
	outsideTarget := filepath.Join(outsideDir, "outside.md")
	if err := os.WriteFile(outsideTarget, []byte(v1Plan), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideTarget, filepath.Join(dir, "2026-08-03-outside.md")); err != nil {
		t.Fatal(err)
	}
	_, err := plan.Resolve(dir, "2026-08-03-outside")
	var diagnostics *plan.DiagnosticsError
	if !errors.As(err, &diagnostics) || len(diagnostics.Diagnostics) != 1 || diagnostics.Diagnostics[0].Category != "path" || diagnostics.Diagnostics[0].Path != "2026-08-03-outside.md" {
		t.Fatalf("outside symlink error = %#v", err)
	}
}

func TestPlanResolutionReportsAmbiguousStemFilenameCollision(t *testing.T) {
	dir := t.TempDir()
	writePlan(t, dir, "2026-08-02-collision.md", v1Plan)
	writePlan(t, dir, "2026-08-02-collision.md.md", replaceOnceForTest(v1Plan, "# Plan: Example", "# Plan: Collision"))
	_, err := plan.Resolve(dir, "2026-08-02-collision.md")
	var ambiguous *plan.AmbiguousError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("error = %v, want *plan.AmbiguousError", err)
	}
	want := []string{"2026-08-02-collision", "2026-08-02-collision.md", "2026-08-02-collision.md.md"}
	if ambiguous.Value != "2026-08-02-collision.md" || !reflect.DeepEqual(ambiguous.Available, want) {
		t.Fatalf("ambiguous = %#v", ambiguous)
	}
	if got := ambiguous.Error(); got != "plan name \"2026-08-02-collision.md\" is ambiguous; available: 2026-08-02-collision, 2026-08-02-collision.md, 2026-08-02-collision.md.md" {
		t.Fatalf("AmbiguousError.Error() = %q", got)
	}
}

func TestPlanProjectionSelectorsAreTypedAndCanonical(t *testing.T) {
	dir := t.TempDir()
	writePlan(t, dir, "2026-08-02-example.md", v1Plan)
	parsed, err := plan.Resolve(dir, "2026-08-02-example")
	if err != nil {
		t.Fatal(err)
	}
	wantAvailable := []string{"1", "1.1", "1.2", "2", "2.1"}
	for _, selector := range []string{"", "0", "01", "+1", "1.", ".1", "1.0", "1.01", "1.1.1", "x"} {
		_, err := plan.RenderProjection(parsed, selector)
		var invalid *plan.InvalidSelectorError
		if !errors.As(err, &invalid) || invalid.Value != selector || !reflect.DeepEqual(invalid.Available, wantAvailable) {
			t.Errorf("selector %q error = %#v", selector, err)
		}
	}
	for _, selector := range []string{"3", "1.3"} {
		_, err := plan.RenderProjection(parsed, selector)
		var notFound *plan.NotFoundError
		if !errors.As(err, &notFound) || notFound.Kind != "selector" || notFound.Value != selector || !reflect.DeepEqual(notFound.Available, wantAvailable) {
			t.Errorf("selector %q error = %#v", selector, err)
		}
	}
}

func TestLegacyPlanHasNoExecutableProjection(t *testing.T) {
	dir := t.TempDir()
	writePlan(t, dir, "2026-08-02-legacy.md", "---\ndate: 2026-08-02\nadrs: []\nstatus: Proposed\n---\n# Plan: Legacy\n")
	parsed, err := plan.Resolve(dir, "2026-08-02-legacy")
	if err != nil {
		t.Fatal(err)
	}
	_, err = plan.RenderProjection(parsed, "1")
	var diagnostic *plan.Diagnostic
	if !errors.As(err, &diagnostic) || diagnostic.Category != "projection" || diagnostic.Path != "2026-08-02-legacy.md" {
		t.Fatalf("error = %#v", err)
	}
}
