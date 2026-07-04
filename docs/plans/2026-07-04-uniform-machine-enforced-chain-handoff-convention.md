# Plan: uniform machine-enforced workflow-chain handoff convention

**ADR:** [ADR-0054](../decisions/0054-uniform-machine-enforced-workflow-chain-handoff-convention.md)
(Proposed → flips to Implemented in the final commit of this plan).

## Goal

Implement ADR-0054: make workflow-chain handoffs a uniform, machine-enforced convention. Three coupled
changes plus docs — (1) a skill/agent **section-parity guard** that lands first so it backstops the
rename; (2) rename `brainstorming`'s `terminal-handoff` section marker to `terminal-step` for chain
uniformity; (3) strengthen `internal/evals/chain_test.go` with a **positional** handoff assertion
(successor named on an invocation-verb line, replacing the ADR-0046-redundant `os.Stat`) and a
**connectivity** guard over the nine chain-progression nodes. Design rationale lives in ADR-0054 — this
plan is the execution record only.

## Architecture summary

- The parity guard lands **before** the rename (Phase 1) so it is proven green on the pre-rename state
  and then fails loudly if the rename's two-file lockstep edit is incomplete. Verified: section parity
  currently holds for every skill and agent (0 mismatches).
- The rename is exactly two source edits — `templates/skills/brainstorming/SKILL.md.tmpl` (the
  `awf:section` marker) and the `brainstorming.sections` list in `templates/catalog.yaml` — because
  `render.Assemble` derives each provenance-pointer `EditPath` from the **catalog-declared** sections,
  not the template. `./x sync` regenerates both rendered targets (`.claude/` and `.cursor/`).
- No invocation-phrasing rewrites are needed: every current handoff/dispatch line already carries an
  invocation verb (`invoke` / `Invoke` / `Dispatch` / `chains through`), and the positional matcher is
  case-insensitive and includes `chains through`. Phase 2 verifies this rather than editing.
- The evals connectivity node set is pinned to the nine progression skills; `reviewing-impl` is the
  sole terminal; task skills `bugfix`/`debugging` are excluded (still covered by the per-edge
  positional check). Verified against the current render: all nine nodes reachable from `brainstorming`,
  no orphans.
- The `reviewing-plan` / `reviewing-plan-resync` token pair collides on substring, so the positional
  matcher uses a token-boundary regex (`example-reviewing-plan` must not be followed by `-` or a word
  char) to avoid a spurious `…-plan` edge on a `…-plan-resync` line.

## Tech stack

- Go 1.26. Packages touched: `internal/project` (new parity test), `internal/evals` (strengthened
  chain test). Imports: `internal/catalog`, `internal/render`, `templates` (parity test); `regexp`,
  `os`, `strings` (evals). No new dependency.
- Template/config edits: `templates/skills/brainstorming/SKILL.md.tmpl`, `templates/catalog.yaml`,
  `.awf/agents-doc.yaml`, `.awf/docs/parts/testing/layout.md` (all re-rendered by `./x sync`).

## File structure

- **Created:** `internal/project/skill_sections_test.go`,
  `docs/plans/2026-07-04-uniform-machine-enforced-chain-handoff-convention.md` (this file).
- **Modified:** `internal/evals/chain_test.go` (rewritten), `templates/skills/brainstorming/SKILL.md.tmpl`,
  `templates/catalog.yaml`, `.awf/agents-doc.yaml`, `.awf/docs/parts/testing/layout.md`,
  `docs/decisions/0054-uniform-machine-enforced-workflow-chain-handoff-convention.md` (status flip),
  plus sync-regenerated outputs: `.claude/skills/awf-brainstorming/SKILL.md`,
  `.cursor/skills/awf-brainstorming/SKILL.md`, `AGENTS.md`, `docs/testing.md`,
  `docs/decisions/ACTIVE.md`, `docs/domains/tooling.md`, `.awf/awf.lock`.
- **Deleted:** none.

---

## Phase 1 — skill/agent section-parity guard (lands first)

### Task 1.1 — Create `internal/project/skill_sections_test.go`

Create the file with exactly this content:

```go
package project

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// templateMarkers returns the awf:section marker names declared in a template
// source (template order).
func templateMarkers(t *testing.T, tid string) []string {
	t.Helper()
	src, err := fs.ReadFile(templates.FS, tid)
	if err != nil {
		t.Fatalf("read %s: %v", tid, err)
	}
	var markers []string
	for _, s := range render.ParseSections(string(src)) {
		if s.IsSection {
			markers = append(markers, s.Name)
		}
	}
	return markers
}

// assertSectionParity fails if the template's awf:section marker set differs
// from the catalog-declared section set (order-independent).
func assertSectionParity(t *testing.T, label, tid string, sections []string) {
	t.Helper()
	want := append([]string(nil), sections...)
	got := append([]string(nil), templateMarkers(t, tid)...)
	sort.Strings(want)
	sort.Strings(got)
	if strings.Join(want, ",") != strings.Join(got, ",") {
		t.Errorf("%s: section mismatch: catalog %v vs template markers %v", label, want, got)
	}
}

// TestSkillAndAgentSectionParity asserts that for every catalog skill and agent
// the set of awf:section markers in its template source equals its
// catalog-declared sections list. Without this guard a section-slug rename that
// updates the template but not catalog.yaml (or vice versa) renders green with a
// blank-path provenance pointer that no other gate catches (ADR-0054).
//
// invariant: skill-section-parity
func TestSkillAndAgentSectionParity(t *testing.T) {
	cat, err := catalog.Load(templates.FS)
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	for name, spec := range cat.Skills {
		assertSectionParity(t, "skill "+name, fmt.Sprintf("skills/%s/SKILL.md.tmpl", name), spec.Sections)
	}
	for name, spec := range cat.Agents {
		assertSectionParity(t, "agent "+name, fmt.Sprintf("agents/%s.md.tmpl", name), spec.Sections)
	}
}
```

### Task 1.2 — Verify Phase 1

Run:

```
go test ./internal/project/ -run TestSkillAndAgentSectionParity -v
```

Expected: `--- PASS: TestSkillAndAgentSectionParity` and `ok  github.com/hypnotox/agentic-workflows/internal/project`.
(The guard passes on the current pre-rename state — parity holds for every skill and agent today.)

Then:

```
./x gate
```

Expected: final two lines `coverage: 100.0% (...)` and `0 issues.` (the test adds only `_test.go`
statements, so the coverage denominator is unchanged).

### Task 1.3 — Commit Phase 1

```
git add internal/project/skill_sections_test.go
git commit -m "test(awf): add skill/agent section-parity guard"
```

---

## Phase 2 — rename brainstorming's handoff marker (protected by the guard)

### Task 2.1 — Rename the template marker

In `templates/skills/brainstorming/SKILL.md.tmpl`, change the line:

```
<!-- awf:section terminal-handoff -->
```

to:

```
<!-- awf:section terminal-step -->
```

### Task 2.2 — Rename the catalog-declared section (lockstep)

In `templates/catalog.yaml`, under `skills.brainstorming.sections`, change the list item:

```
      - terminal-handoff
```

to:

```
      - terminal-step
```

### Task 2.3 — Verify no invocation-phrasing outliers need editing

Confirm every forward handoff/dispatch line already carries an invocation verb (no rewrite needed):

```
grep -rInE "(invoke|dispatch|hands off|chains through)" .claude/skills/awf-brainstorming/SKILL.md
```

Expected: at least the four handoff bullets under the (about-to-be-renamed) terminal section naming
`awf-proposing-adr`, `awf-writing-plans`, `awf-reviewing-adr`, `awf-reviewing-impl`. If any table-driven
handoff/dispatch pair from Phase 3 lacks a verb on its successor line, stop and raise to the user — per
ADR-0054 wording is already close and no rewrite is expected.

### Task 2.4 — Re-render and verify the guard still passes

```
./x sync
go test ./internal/project/ -run TestSkillAndAgentSectionParity -v
./x check
./x gate
```

Expected: `./x sync` → `awf sync: done`; the parity test `PASS` (both lockstep edits landed — a missed
edit would fail here); `./x check` → `awf check: clean`; `./x gate` → `coverage: 100.0% (...)` and
`0 issues.`

`./x sync` regenerates `.claude/skills/awf-brainstorming/SKILL.md`,
`.cursor/skills/awf-brainstorming/SKILL.md`, and `.awf/awf.lock` (the rendered `awf:edit terminal-step`
provenance pointer replaces `terminal-handoff`).

### Task 2.5 — Commit Phase 2

```
git add templates/skills/brainstorming/SKILL.md.tmpl templates/catalog.yaml .claude/skills/awf-brainstorming/SKILL.md .cursor/skills/awf-brainstorming/SKILL.md .awf/awf.lock
git commit -m "refactor(awf): rename brainstorming handoff section to terminal-step"
```

---

## Phase 3 — strengthen the evals chain assertions

### Task 3.1 — Rewrite `internal/evals/chain_test.go`

Replace the entire file with exactly this content:

```go
package evals

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// read reads path or fails the test.
func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// invocationVerb matches a workflow-chain invocation instruction — the verb that
// makes a line a handoff/dispatch rather than an incidental mention (ADR-0054).
// Case-insensitive so "invoke"/"Invoke"/"Dispatch"/"chains through" all anchor.
var invocationVerb = regexp.MustCompile(`(?i)(invoke|dispatch|hands off|chains through)`)

// namesOnInvocationLine reports whether body has a line carrying both an
// invocation verb and the token as a whole skill/agent name — i.e. the token is
// named in an actual instruction, not merely present somewhere in the prose
// (ADR-0053 owns mere presence) and not just as an existing target (ADR-0046
// owns that). The trailing boundary ([^-\w] or line end) stops
// "example-reviewing-plan" from matching an "example-reviewing-plan-resync" line.
func namesOnInvocationLine(body, token string) bool {
	tokenRe := regexp.MustCompile(regexp.QuoteMeta(token) + `([^-\w]|$)`)
	for _, line := range strings.Split(body, "\n") {
		if tokenRe.MatchString(line) && invocationVerb.MatchString(line) {
			return true
		}
	}
	return false
}

// assertHandoff asserts the rendered `from` skill names the prefixed `to` skill
// on an invocation-verb line — the successor sits in a real handoff instruction.
func assertHandoff(t *testing.T, root, from, to string) {
	t.Helper()
	body := read(t, skillPath(root, from))
	want := evalPrefix + "-" + to
	if !namesOnInvocationLine(body, want) {
		t.Errorf("skill %q does not hand off to %q on an invocation line", from, want)
	}
}

// TestWorkflowChainHandoffs asserts each load-bearing chain handoff names its
// successor in an actual invocation instruction in the same full-catalog render.
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
// `skill` names the reviewer `agent` on an invocation-verb line, and that agent
// carries the shared review-spine partial (ADR-0052) identified by spineToken.
func assertDispatch(t *testing.T, root, skill, agent, spineToken string) {
	t.Helper()
	if body := read(t, skillPath(root, skill)); !namesOnInvocationLine(body, agent) {
		t.Errorf("skill %q does not dispatch agent %q on an invocation line", skill, agent)
	}
	if agentBody := read(t, agentPath(root, agent)); !strings.Contains(agentBody, spineToken) {
		t.Errorf("agent %q missing spine partial token %q", agent, spineToken)
	}
}

// TestReviewerDispatchCarriesSpine asserts each reviewing skill dispatches its
// reviewer agent (on an invocation line) and that agent carries the spine partial.
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

// chainNodes is the pinned forward-chain progression node set (ADR-0054 item 3).
// chainTerminal is the sole terminal (exempt from the outgoing-edge requirement).
// Task skills bugfix/debugging are deliberately NOT nodes — their handoffs are
// covered by the per-edge positional check above.
var chainNodes = []string{
	"brainstorming", "proposing-adr", "reviewing-adr", "writing-plans",
	"reviewing-plan", "reviewing-plan-resync", "executing-plans",
	"subagent-driven-development", "reviewing-impl",
}

const (
	chainRoot     = "brainstorming"
	chainTerminal = "reviewing-impl"
)

// chainEdges returns, for each chain node, the set of other chain nodes it names
// on an invocation-verb line in the full-catalog render.
func chainEdges(t *testing.T, root string) map[string][]string {
	t.Helper()
	edges := map[string][]string{}
	for _, from := range chainNodes {
		body := read(t, skillPath(root, from))
		for _, to := range chainNodes {
			if to == from {
				continue
			}
			if namesOnInvocationLine(body, evalPrefix+"-"+to) {
				edges[from] = append(edges[from], to)
			}
		}
	}
	return edges
}

// TestChainConnectivity asserts the forward-chain handoff graph has no orphaned
// node (every non-terminal node emits >=1 outgoing invocation edge) and every
// node is reachable from the root brainstorming (ADR-0054 item 3). This catches a
// skill that loses all its handoff instructions — a whole-node failure the
// per-edge positional check cannot see.
func TestChainConnectivity(t *testing.T) {
	cat := loadCatalog(t)
	root := syncFullCatalog(t, cat)
	edges := chainEdges(t, root)

	for _, n := range chainNodes {
		if n == chainTerminal {
			continue
		}
		if len(edges[n]) == 0 {
			t.Errorf("chain node %q is orphaned: no outgoing invocation edge", n)
		}
	}

	seen := map[string]bool{chainRoot: true}
	queue := []string{chainRoot}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, to := range edges[cur] {
			if !seen[to] {
				seen[to] = true
				queue = append(queue, to)
			}
		}
	}
	for _, n := range chainNodes {
		if !seen[n] {
			t.Errorf("chain node %q is unreachable from %q", n, chainRoot)
		}
	}
}
```

### Task 3.2 — Verify Phase 3

```
go test ./internal/evals/ -v
```

Expected: `TestFullCatalogCoverage`, `TestWorkflowChainHandoffs` (5 subtests),
`TestReviewerDispatchCarriesSpine` (3 subtests), and `TestChainConnectivity` all `PASS`.

```
./x gate
```

Expected: `coverage: 100.0% (...)` and `0 issues.`

### Task 3.3 — Commit Phase 3

```
git add internal/evals/chain_test.go
git commit -m "test(awf): strengthen evals with positional and connectivity checks"
```

---

## Phase 4 — docs, invariant entry, and ADR status flip

### Task 4.1 — Add the ADR-0054 invariant entry to the agent guide

In `.awf/agents-doc.yaml`, in the `data.invariants` list, immediately after the `ADR-0053` entry
(`**Full-catalog eval coverage.** ...`), append:

```yaml
        - ref: ADR-0054
          text: '**Uniform machine-enforced chain handoffs.** Every catalog skill/agent template''s `awf:section` markers match its `catalog.yaml`-declared sections (`skill-section-parity`), so a section rename cannot half-land with a blank override path; and the `internal/evals` suite asserts each forward handoff names its successor on an invocation-verb line and that the nine-node chain graph is connected (no orphan, all reachable from `brainstorming`).'
```

### Task 4.2 — Document the convention in the testing part source

In `.awf/docs/parts/testing/layout.md`, replace the existing evals paragraph (the paragraph beginning
`Workflow-chain golden-task evals live in` and ending `... artifact (ADR-0053).`) with:

```markdown
Workflow-chain golden-task evals live in `internal/evals`, a test-only package (only `_test.go`
files, no production source). Each scenario runs a full `Project.Sync` over a fixture config derived
from the embedded catalog — every skill, agent, and doc enabled — and asserts *cross-artifact* seams a
single-template test cannot: that a skill names its handoff successor on an *invocation-verb line* (a
real handoff instruction, not an incidental mention), that a reviewing skill dispatches a reviewer agent
carrying the shared review-spine partial, and that the forward-chain handoff graph is connected — no
orphaned node, every node reachable from `brainstorming` (ADR-0053, ADR-0054). The fixture's enabled set
is catalog-derived so it cannot silently stop covering a newly-added chain artifact. A companion
section-parity guard in `internal/project` (`TestSkillAndAgentSectionParity`) asserts every skill/agent
template's `awf:section` markers match its `catalog.yaml`-declared sections, so a section-slug rename
cannot half-land with a blank-path provenance pointer.
```

### Task 4.3 — Flip ADR-0054 status to Implemented

In `docs/decisions/0054-uniform-machine-enforced-workflow-chain-handoff-convention.md`, change the
frontmatter line:

```
status: Proposed
```

to:

```
status: Implemented
```

### Task 4.4 — Re-render, verify drift + invariant backing, and gate

```
./x sync
./x check
./x gate
```

Expected: `./x sync` → `awf sync: done`; `./x check` → `awf check: clean` (now enforces the
`skill-section-parity` invariant is backed, which it is via the `// invariant:` comment in
`internal/project/skill_sections_test.go`); `./x gate` → `coverage: 100.0% (...)` and `0 issues.`

`./x sync` regenerates `AGENTS.md` (new invariant row), `docs/testing.md` (updated paragraph),
`docs/decisions/ACTIVE.md` (0054 now Implemented), `docs/domains/tooling.md` (0054 moved to the
Implemented index), and `.awf/awf.lock`.

### Task 4.5 — Commit Phase 4

```
git add .awf/agents-doc.yaml .awf/docs/parts/testing/layout.md docs/decisions/0054-uniform-machine-enforced-workflow-chain-handoff-convention.md AGENTS.md docs/testing.md docs/decisions/ACTIVE.md docs/domains/tooling.md .awf/awf.lock
git commit -m "docs(adr): mark 0054 implemented and wire handoff-convention docs"
```

---

## Done criteria

- `internal/project/skill_sections_test.go` guards skill/agent section parity (backed invariant
  `skill-section-parity`); `brainstorming`'s handoff marker is `terminal-step` across both rendered
  targets; `internal/evals` asserts positional handoffs + connectivity; `go test ./...` all green.
- `./x gate` at 100% coverage, `./x check` clean, `./x audit` clean over the branch.
- ADR-0054 is `Implemented`; `skill-section-parity` is backed and enforced; AGENTS.md and
  `docs/testing.md` document the convention.
- Terminal handoff: invoke `awf-reviewing-plan`.
