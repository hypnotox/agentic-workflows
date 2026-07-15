# Plan: workflow-chain golden-task eval suite

**ADR:** [ADR-0053](../decisions/0053-deterministic-workflow-chain-golden-task-eval-suite.md) (Proposed → flips to Implemented in the final commit of this plan).

## Goal

Add a deterministic, internal Go regression suite (`internal/evals`) that renders the real embedded
templates via a full `Project.Sync` over a catalog-derived fixture config and asserts **cross-artifact
workflow-chain seams** no existing test covers: a skill's terminal handoff naming a skill present in
the same rendered set, and a reviewing skill's dispatched reviewer agent carrying the shared
review-spine partial. Design rationale lives in ADR-0053; this plan is the execution record only.

## Architecture summary

- A new **test-only** Go package `internal/evals` (only `_test.go` files, `package evals`, no production
  `.go`). It contributes zero coverable statements, so ADR-0012's 100% gate is satisfied vacuously.
- The fixture config's enabled skill/agent/doc set is derived from `catalog.Load(templates.FS)` (every
  catalog skill, agent, and doc enabled) so the suite stays exhaustive as the catalog grows and the
  `roadmap`-gated `roadmap-graduation` skill (ADR-0013 `requiresDoc`) is not silently dropped.
- Scenarios are code-expressed, table-driven, with small matcher helpers. No YAML DSL, no new command.
- The suite reuses the exported `internal/testsupport` primitives + `project.Open`/`Sync` (the
  package-private `scaffold` helper in `internal/project` is not importable from an external package).

## Tech stack

- Go 1.26. Packages touched: new `internal/evals`; imports `internal/catalog`, `internal/project`,
  `internal/testsupport`, and the `templates` embed FS. No new dependency.
- Config-tree edits: `.awf/docs/parts/testing/layout.md`, `.awf/agents-doc.yaml` (both re-rendered by
  `./x sync`).

## File structure

- **Created:** `internal/evals/fixture_test.go`, `internal/evals/chain_test.go`,
  `docs/plans/2026-07-04-workflow-chain-golden-task-eval-suite.md` (this file).
- **Modified:** `docs/decisions/0053-deterministic-workflow-chain-golden-task-eval-suite.md` (status
  flip), `.awf/docs/parts/testing/layout.md`, `.awf/agents-doc.yaml`, and their re-rendered outputs
  (`docs/testing.md`, `AGENTS.md`), plus sync-regenerated `docs/decisions/ACTIVE.md`,
  `docs/domains/tooling.md`, `.awf/awf.lock`.
- **Deleted:** none.

---

## Phase 1: evals package: full-catalog fixture + coverage guard

### Task 1.1: Create `internal/evals/fixture_test.go`

Create the file with exactly this content:

```go
package evals

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/templates"
)

// evalPrefix is the skill-name prefix the golden-task fixture renders under.
// Rendered skill dirs are ".claude/skills/<evalPrefix>-<name>/SKILL.md"; agents
// are unprefixed at ".claude/agents/<name>.md".
const evalPrefix = "example"

// loadCatalog loads the embedded catalog or fails the test.
func loadCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	cat, err := catalog.Load(templates.FS)
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	return cat
}

// sortedKeys returns m's keys in deterministic order.
func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// writeList appends a "key:\n  - v\n" YAML block to b.
func writeList(b *strings.Builder, key string, vals []string) {
	b.WriteString(key + ":\n")
	for _, v := range vals {
		b.WriteString("  - " + v + "\n")
	}
}

// fullCatalogConfig builds a .awf/config.yaml enabling every catalog skill,
// agent, and doc, the deliberate inverse of the curated awf init default
// (ADR-0022), so the rendered set exercises every workflow-chain seam. The
// enabled set is derived from the catalog (never hand-listed) so it cannot
// silently rot as the catalog grows (ADR-0053).
func fullCatalogConfig(cat *catalog.Catalog) string {
	var b strings.Builder
	b.WriteString("prefix: " + evalPrefix + "\n")
	b.WriteString("targets:\n  - claude\n")
	writeList(&b, "skills", sortedKeys(cat.Skills))
	writeList(&b, "agents", sortedKeys(cat.Agents))
	writeList(&b, "docs", sortedKeys(cat.Docs))
	return b.String()
}

// syncFullCatalog scaffolds a temp project with the full-catalog config, runs a
// real Project.Sync, and returns the project root. It reuses the exported
// testsupport primitives rather than internal/project's package-private
// scaffold helper (ADR-0053 Decision item 5).
func syncFullCatalog(t *testing.T, cat *catalog.Catalog) string {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, fullCatalogConfig(cat))
	p, err := project.Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("sync: %v", err)
	}
	return root
}

// skillPath returns the rendered claude-target SKILL.md path for a skill name.
func skillPath(root, name string) string {
	return filepath.Join(root, ".claude", "skills", evalPrefix+"-"+name, "SKILL.md")
}

// agentPath returns the rendered claude-target agent path for an agent name.
func agentPath(root, name string) string {
	return filepath.Join(root, ".claude", "agents", name+".md")
}

// read reads path or fails the test.
func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// TestFullCatalogCoverage proves the full-catalog fixture actually renders an
// artifact for every catalog skill and agent, so no chain artifact is silently
// dropped (e.g. by a requiresDoc gate). This is the guard that keeps the eval
// suite exhaustive as the catalog grows.
//
// invariant: evals-full-catalog-coverage
func TestFullCatalogCoverage(t *testing.T) {
	cat := loadCatalog(t)
	root := syncFullCatalog(t, cat)
	for _, s := range sortedKeys(cat.Skills) {
		if _, err := os.Stat(skillPath(root, s)); err != nil {
			t.Errorf("catalog skill %q not rendered: %v", s, err)
		}
	}
	for _, a := range sortedKeys(cat.Agents) {
		if _, err := os.Stat(agentPath(root, a)); err != nil {
			t.Errorf("catalog agent %q not rendered: %v", a, err)
		}
	}
}
```

### Task 1.2: Verify Phase 1

Run:

```
go test ./internal/evals/ -run TestFullCatalogCoverage -v
```

Expected: `--- PASS: TestFullCatalogCoverage` and `ok  github.com/hypnotox/agentic-workflows/internal/evals`.

Then run the gate:

```
./x gate
```

Expected: final two lines `coverage: 100.0% (...)` and `0 issues.` (the test-only package adds no
coverable statements, so the 100% ratio is unchanged).

### Task 1.3: Commit Phase 1

```
git add internal/evals/fixture_test.go
git commit -m "test(awf): add internal/evals full-catalog fixture and coverage guard"
```

---

## Phase 2: cross-artifact workflow-chain scenarios

### Task 2.1: Create `internal/evals/chain_test.go`

Create the file with exactly this content:

```go
package evals

import (
	"os"
	"strings"
	"testing"
)

// assertHandoff asserts a cross-artifact seam: the rendered `from` skill names
// the prefixed `to` skill AND the `to` skill is itself present in the rendered
// set. Neither spine_test.go (single-template render, no target-existence
// check) nor ADR-0046 (reference to an *absent* skill) covers "handoff to a
// present skill"; that seam is this suite's mandate (ADR-0053).
func assertHandoff(t *testing.T, root, from, to string) {
	t.Helper()
	body := read(t, skillPath(root, from))
	want := evalPrefix + "-" + to
	if !strings.Contains(body, want) {
		t.Errorf("skill %q does not hand off to %q", from, want)
	}
	if _, err := os.Stat(skillPath(root, to)); err != nil {
		t.Errorf("handoff target %q not present in rendered set: %v", want, err)
	}
}

// TestWorkflowChainHandoffs asserts each load-bearing chain handoff resolves to
// a skill present in the same full-catalog render.
func TestWorkflowChainHandoffs(t *testing.T) {
	cat := loadCatalog(t)
	root := syncFullCatalog(t, cat)
	for _, tc := range []struct{ from, to string }{
		{"brainstorming", "proposing-adr"},
		{"brainstorming", "writing-plans"},
		{"proposing-adr", "reviewing-adr"},
		{"writing-plans", "reviewing-plan"},
		{"bugfix", "reviewing-impl"},
	} {
		t.Run(tc.from+"_to_"+tc.to, func(t *testing.T) {
			assertHandoff(t, root, tc.from, tc.to)
		})
	}
}

// assertDispatch asserts a skill->agent->partial seam: the rendered reviewing
// `skill` names the reviewer `agent`, and that agent carries the shared
// review-spine partial (ADR-0052) identified by spineToken. This spans three
// artifacts no single existing test composes.
func assertDispatch(t *testing.T, root, skill, agent, spineToken string) {
	t.Helper()
	if body := read(t, skillPath(root, skill)); !strings.Contains(body, agent) {
		t.Errorf("skill %q does not dispatch agent %q", skill, agent)
	}
	if agentBody := read(t, agentPath(root, agent)); !strings.Contains(agentBody, spineToken) {
		t.Errorf("agent %q missing spine partial token %q", agent, spineToken)
	}
}

// TestReviewerDispatchCarriesSpine asserts each reviewing skill dispatches its
// reviewer agent and that agent carries the review-spine partial.
func TestReviewerDispatchCarriesSpine(t *testing.T) {
	cat := loadCatalog(t)
	root := syncFullCatalog(t, cat)
	for _, tc := range []struct{ skill, agent string }{
		{"reviewing-impl", "code-reviewer"},
		{"reviewing-adr", "adr-reviewer"},
		{"reviewing-plan", "plan-reviewer"},
	} {
		t.Run(tc.skill, func(t *testing.T) {
			assertDispatch(t, root, tc.skill, tc.agent, "## Classification rules")
		})
	}
}
```

### Task 2.2: Verify Phase 2

```
go test ./internal/evals/ -v
```

Expected: all of `TestFullCatalogCoverage`, `TestWorkflowChainHandoffs` (5 subtests), and
`TestReviewerDispatchCarriesSpine` (3 subtests) `PASS`.

```
./x gate
```

Expected: `coverage: 100.0% (...)` and `0 issues.`

### Task 2.3: Commit Phase 2

```
git add internal/evals/chain_test.go
git commit -m "test(awf): add workflow-chain cross-artifact golden-task scenarios"
```

---

## Phase 3: doc currency + AGENTS.md invariant entry + status flip

### Task 3.1: Document the new test category in the testing part source

Edit `.awf/docs/parts/testing/layout.md`. After the first paragraph (the one ending
`... in \`cmd/awf/*_test.go\`.`), insert this new paragraph:

```markdown

Workflow-chain golden-task evals live in `internal/evals`, a test-only package (only `_test.go`
files, no production source). Each scenario runs a full `Project.Sync` over a fixture config derived
from the embedded catalog (every skill, agent, and doc enabled) and asserts *cross-artifact* seams a
single-template test cannot: that a skill's terminal handoff names a skill present in the same rendered
set, and that a reviewing skill's dispatched reviewer agent carries the shared review-spine partial. The
fixture's enabled set is catalog-derived so it cannot silently stop covering a newly-added chain
artifact (ADR-0053).
```

### Task 3.2: Add the ADR-0053 invariants entry to the agent guide

Edit `.awf/agents-doc.yaml`. In the `data.invariants` list, immediately after the `ADR-0051` entry
(the last one in the list: `**Single commit-scope storage.** ...`), append:

```yaml
        - ref: ADR-0053
          text: '**Full-catalog eval coverage.** The golden-task eval suite (`internal/evals`) renders every catalog skill and agent via a full `Project.Sync` and asserts cross-artifact workflow seams; its fixture enabled-set is derived from `catalog.Load` so it cannot silently stop covering a new chain artifact.'
```

### Task 3.3: Flip ADR-0053 status to Implemented

In `docs/decisions/0053-deterministic-workflow-chain-golden-task-eval-suite.md`, change the
frontmatter line:

```
status: Proposed
```

to:

```
status: Implemented
```

### Task 3.4: Re-render, verify drift + invariant backing, and gate

```
./x sync
./x check
./x gate
```

Expected: `./x sync` → `awf sync: done`; `./x check` → `awf check: clean` (this now enforces the
`evals-full-catalog-coverage` invariant is backed, which it is via the `// invariant:` comment in
`internal/evals/fixture_test.go`); `./x gate` → `coverage: 100.0% (...)` and `0 issues.`

`./x sync` regenerates `AGENTS.md` (new invariant row), `docs/testing.md` (new paragraph),
`docs/decisions/ACTIVE.md` (0053 now Implemented), `docs/domains/tooling.md` (0053 index entry), and
`.awf/awf.lock`.

### Task 3.5: Commit Phase 3

```
git add .awf/docs/parts/testing/layout.md .awf/agents-doc.yaml docs/decisions/0053-deterministic-workflow-chain-golden-task-eval-suite.md AGENTS.md docs/testing.md docs/decisions/ACTIVE.md docs/domains/tooling.md .awf/awf.lock
git commit -m "docs(adr): mark 0053 implemented and wire eval-suite docs"
```

---

## Done criteria

- `internal/evals` renders the full catalog and asserts handoff + reviewer-dispatch-carries-spine
  seams; `go test ./internal/evals/ -v` all green.
- `./x gate` at 100% coverage, `./x check` clean, `./x audit` clean over the branch.
- ADR-0053 is `Implemented`; `evals-full-catalog-coverage` is backed and enforced; AGENTS.md and
  `docs/testing.md` document the suite.
- Terminal handoff: invoke `awf-reviewing-impl`.
