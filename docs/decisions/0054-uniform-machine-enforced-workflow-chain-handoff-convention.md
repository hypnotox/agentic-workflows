---
status: Proposed
date: 2026-07-04
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [testing, evals, workflow, conventions]
related: [0053, 0046, 0015, 0022, 0012]
domains: [tooling]
---
# ADR-0054: Uniform machine-enforced workflow-chain handoff convention

## Context

[ADR-0053](0053-deterministic-workflow-chain-golden-task-eval-suite.md) added the `internal/evals`
suite: a full-catalog `Project.Sync` render plus cross-artifact assertions that the composed harness
still hangs together. A review of that suite found its two chain assertions weaker than they read, and
tracing them against the codebase confirmed it:

- **The `os.Stat` target-existence half is redundant.** `assertHandoff` checks the handoff-target
  skill file exists. But ADR-0046's `checkDeadSkillRefs` (`internal/project/check.go:373`) already
  fails `awf check` on any rendered `<prefix>-<name>` token pointing to a skill outside the enabled
  set — so for any config that passes `check`, closure is already guaranteed. Under the full-catalog
  fixture, where every skill is enabled, the check is doubly trivial.
- **The body-`Contains` half is near-redundant and semantically weak.** `spine_test.go` already asserts
  per-skill that the successor token appears somewhere in an isolated single-template render (e.g.
  `spine_test.go:266` for `writing-plans → example-reviewing-plan`). And "appears somewhere" is not
  "appears in a handoff instruction": a template edit that dropped the real handoff but left
  `example-proposing-adr` in a "do not confuse with…" aside would pass every existing test while the
  chain is semantically severed. Catching *that* — workflow-walkability — is exactly the evals suite's
  mandate, and nothing covers it today.

The genuinely uncovered surface is therefore a **positional** property (the successor token sits in an
actual invocation instruction) and a **graph** property (no chain node is orphaned; the chain is
connected) — both over the full-catalog render, neither owned by `spine_test.go` (plain `Contains`,
single-template) or `checkDeadSkillRefs` (reference-to-absent-skill, not reachability).

**A positional test needs a stable anchor, and the rendered skills do not currently offer a uniform
one.** Two anchor candidates exist, and grounding both against the templates shaped the decision:

- *Section-marker slug.* awf renders an `<!-- awf:edit <slug> … -->` provenance pointer per template
  section; the slug is the adopter override-API key (`.awf/skills/parts/<skill>/<slug>.md` overrides
  that section — ADR-0015). But the slugs are **not uniform**: `brainstorming` alone uses
  `terminal-handoff` while the other four progression skills (`proposing-adr`, `writing-plans`,
  `executing-plans`, `subagent-driven-development`) use `terminal-step`; and the task skills `bugfix`
  and `debugging` carry *no* handoff-section marker at all — their handoff is an inline numbered
  procedure step. So a slug anchor cannot cover every handoff uniformly.
- *Invocation wording.* In every case the successor token sits on a line carrying an invocation verb —
  "invoke `awf-reviewing-adr`", "Invoke `awf-reviewing-impl`" (capitalized in `bugfix`), "Dispatch the
  `code-reviewer`", and "…which chains through `awf-reviewing-adr`". This anchor *is* uniform across all
  skills including the marker-less task skills, and it anchors on the semantic instruction rather than
  render structure.

A **surfacing discovery** shaped the scope further: renaming a section-marker slug requires **two**
lockstep source edits — the template marker (`templates/skills/<skill>/SKILL.md.tmpl`) and the catalog
`sections` list (`templates/catalog.yaml`). `render.Assemble` derives each pointer's `EditPath` from the
**catalog-declared** sections, not the template (`internal/project/render.go`); a catalog miss yields a
provenance pointer with a **blank** override path (`create  to override`). And there is **no
skill/agent section-parity guard** — only docs, ADR singletons, and the domain doc have a
template-marker↔catalog parity test (`internal/project/docs_sections_test.go`). So a template-only
rename renders green, `./x sync` regenerates the goldens, and `./x check` stays clean while shipping a
malformed pointer. The rename this ADR proposes would itself be vulnerable to that trap.

Constraints carried from ADR-0053 still bind: no live agent, no new command, strict non-redundancy,
and the 100% coverage gate (ADR-0012).

## Decision

1. **Uniform forward-chain handoff convention.** The five chain-*progression* skills — `brainstorming`,
   `proposing-adr`, `writing-plans`, `executing-plans`, `subagent-driven-development` — share the
   handoff section-marker slug `terminal-step`. Concretely, `brainstorming`'s current
   `terminal-handoff` marker is renamed to `terminal-step` (two lockstep edits:
   `templates/skills/brainstorming/SKILL.md.tmpl` and the `brainstorming.sections` list in
   `templates/catalog.yaml`). Handoff/dispatch lines follow a canonical invocation phrasing so a
   positional matcher has a stable anchor: a skill→skill handoff reads `invoke <code>example-<skill></code>`
   and a reviewing-skill→reviewer-agent dispatch reads `Dispatch the <code>agent</code>`. Outliers are
   aligned; wording is already close. `reviewing-plan-resync` keeps its distinct
   `dispatch-subagent-narrowed` marker and is **excluded from the forward-chain graph** — it reconciles
   a plan against a settled ADR rather than progressing the chain.

2. **Positional handoff assertion replaces the redundant existence check.** In
   `internal/evals/chain_test.go`, `assertHandoff` / `assertDispatch` drop the `os.Stat` existence
   check (owned by ADR-0046) and instead require the successor/agent token to appear **on a line that
   also carries an invocation verb**. The verb set is `invoke`, `dispatch`, `hands off`, `chains
   through`, matched **case-insensitively** (so `bugfix`'s capitalized "Invoke" and `brainstorming`'s
   "chains through" both anchor). This asserts the successor is named in an actual instruction, not
   merely present in the body.

3. **Connectivity guard over the full-catalog render.** A new assertion builds the forward-chain
   handoff graph from the invocation-verb lines of the full-catalog `Sync` render (edges = a skill's
   body invoking another chain skill) and asserts two graph properties: **no orphaned node** (every
   non-terminal forward-chain skill has ≥1 outgoing invocation edge) and **reachability** (every
   forward-chain node is reachable from `brainstorming`). `reviewing-plan-resync` is excluded from the
   node set. This catches a skill that loses all its handoff instructions — a whole-node failure the
   per-edge positional check cannot see.

4. **Skill/agent section-parity guard.** A new test in `internal/project` (mirroring
   `docs_sections_test.go`) asserts, for every catalog skill and agent, that the set of `awf:section`
   markers in its template exactly equals its `catalog.yaml`-declared `sections` list. This closes the
   silent-drift gap that would otherwise let a section-slug rename half-land with a blank override path,
   and it backs the rename in Decision item 1 with a gate that fails on a lockstep miss. It is a backed
   invariant (`inv: skill-section-parity`).

5. **Strict non-redundancy preserved.** Every new assertion depends on the full-catalog render and
   asserts a property no existing test owns: the positional check asserts *instruction placement* (not
   mere presence, which `spine_test.go` owns); the connectivity guard asserts *graph reachability* (not
   reference-closure, which `checkDeadSkillRefs` owns). The suite does not re-assert single-artifact
   content.

6. **Document the convention.** The uniform handoff convention and the section-parity guard are
   recorded in `docs/testing.md` (via `.awf/docs/parts/testing/layout.md`) and the `tooling` domain
   narrative in the implementing range; the generated docs are never hand-edited (ADR-0005/0020).

## Invariants

- `inv: skill-section-parity` — for every catalog skill and agent, the set of `awf:section` markers in
  its template source equals its `catalog.yaml`-declared `sections` list (set equality,
  order-independent), so a section rename cannot half-land with a blank-path provenance pointer. Backed
  by `// invariant: skill-section-parity` in an `internal/project` `_test.go` (matching
  `invariants.sources` glob `*.go`).
- The five forward-chain progression skills share the `terminal-step` handoff section-marker slug and
  the canonical invocation phrasing — a textual contract, enforced by the section-parity guard (marker
  presence) and the positional evals assertion (phrasing) running in the gate.
- Every forward-chain handoff names its successor on an invocation-verb line in the full-catalog render
  — enforced by the positional assertion in `internal/evals` running in the gate.
- The forward-chain handoff graph has no orphaned node and every node is reachable from `brainstorming`
  (`reviewing-plan-resync` excluded) — enforced by the connectivity assertion running in the gate.

## Consequences

- **Easier:** a template edit that demotes a handoff out of its invocation instruction, or that strips a
  chain skill's handoffs entirely, now fails `go test ./...` — even though each artifact individually
  still renders and passes frontmatter and dead-reference checks. The evals suite now asserts
  workflow-*walkability*, not just token presence.
- **Easier (bonus):** the section-parity guard protects **all** future skill/agent section renames from
  the blank-path trap, not just this one — a general gate improvement surfaced by this work.
- **Harder / cost:** the positional and connectivity matchers couple to a small, bounded vocabulary of
  invocation verbs and the canonical phrasing. This is the same brittleness ADR-0053 accepted for its
  matcher tokens; it is bounded by keeping the verb set to load-bearing instruction words.
- **Adopter-facing break:** renaming `brainstorming`'s `terminal-handoff` marker to `terminal-step`
  changes the override-API key. An adopter with a `.awf/skills/parts/brainstorming/terminal-handoff.md`
  override would find it silently stop applying (and flagged as an orphan part by `awf check` once the
  catalog is updated). No such override exists in this repo; adopters upgrading must rename any such
  file. This adopter-facing surface is what makes the rename load-bearing and this ADR warranted.
- **No new command, no config schema change, no new dependency, no gate tier.** All new code is ordinary
  Go test code in the existing gate. The section-parity guard adds only `_test.go` statements, so the
  100% coverage denominator (ADR-0012) is unchanged.
- **Relationship to ADR-0053:** this extends ADR-0053's suite rather than superseding it; ADR-0053
  remains Implemented and its `evals-full-catalog-coverage` invariant unchanged. This ADR strengthens
  the two chain assertions ADR-0053 introduced and adds the parity backstop the rename requires.

Doc-currency obligations the implementing commit(s) must satisfy:

- The status flip to `Implemented` regenerates `docs/decisions/ACTIVE.md` via `./x sync`.
- The new backed invariant `skill-section-parity` gains an AGENTS.md invariants entry (the
  `data.invariants` list in `.awf/agents-doc.yaml`), citing this ADR, in the implementing range.
- `docs/testing.md` documents the convention and the parity guard via its part source
  `.awf/docs/parts/testing/layout.md`; the `tooling` domain narrative is refreshed if the change
  materially shifts it.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the existence check, just add the positional check | The `os.Stat` check is redundant with ADR-0046 and near-always-true under the full-catalog fixture; keeping it is dead weight. Replacing it is net-simpler. |
| Anchor the positional check on the `awf:edit` marker slug | Slugs are not uniform (`terminal-handoff` vs `terminal-step`) and the task skills carry no handoff marker at all, so a slug anchor cannot cover every handoff. Invocation-verb wording is uniform across all skills. |
| Do the marker rename without the section-parity guard | The rename is exactly the failure mode (a two-file lockstep edit) that renders green with a blank path and no gate catches. Shipping the rename without the backstop would risk silently corrupting a provenance pointer. |
| Normalize `reviewing-plan-resync` into the graph too | Resync reconciles a plan against a settled ADR; it is not a forward-chain progression step. Folding it in would conflate a reconcile pass with handoffs and needlessly widen the adopter-API change to a second slug. |
| Literal end-to-end reachability only (`brainstorming → … → reviewing-impl`) | `brainstorming` has a *direct* edge to `reviewing-impl` (the "neither" path), so end-to-end reachability is near-trivially true and barely bites. The orphaned-node + reachable-from-root guard adds real signal. |
| Split into two ADRs (convention + parity guard) | The parity guard is not independently load-bearing — it exists to make the rename safe. It is a facet of the single "uniform, machine-enforced handoff convention" commitment, so one ADR is the right altitude. |
| A separate ADR family / live-agent walkability | Out of lane and cost-prohibitive, already ruled out by ADR-0053; this stays in the deterministic `go test` lane. |
