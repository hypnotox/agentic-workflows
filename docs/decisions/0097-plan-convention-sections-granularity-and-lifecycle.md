---
status: Implemented
date: 2026-07-12
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [workflow, plans, convention, lifecycle]
related: [95]
domains: [rendering]
---
# ADR-0097: Plan Convention: Sections, Granularity, and Lifecycle

## Context

The plan convention — the `awf-writing-plans` skill and the `plans-readme` doc — specifies
task-level discipline well (exact diffs, no placeholders, test-first, and the batch-task form of
ADR-0095) but leaves a plan's *header* and *tail* loosely defined. A retrospective over this
project's own 67 dogfooded plans found drift concentrated precisely in those unspecified margins,
consistent across the whole corpus:

- **No canonical section shape.** The four header field *names* (goal, architecture summary, tech
  stack, file structure) are present in nearly every plan, but their presentation never converged —
  `## Goal` vs bold-inline `**Goal:**` (split even between plans written the same day), a stray
  `**Date:**` field (redundant with the filename) in ~8 plans, at least six distinct plan→ADR
  citation forms, and file-structure sub-buckets that sprouted `Renamed:` / `Relocated:` /
  `Regenerated:` variants.
- **An under-specified tail.** Fifteen-to-nineteen of every nineteen plans bolt on a trailing
  section the convention never names — "Verification" / "Done when" / "Done criteria" / "Notes" —
  under a different label each time. A near-universal invention marks a missing convention slot.
- **Wall-clock task sizing is the most-violated rule.** "~2-5 min bite-sized" is ignored
  corpus-wide; single tasks legitimately author whole 100–230-line files. The exact-diff and
  no-placeholder rules, by contrast, are honored throughout — so the friction is the *unit*, not the
  discipline.
- **The self-contained-phase rule visibly breaks** whenever a change cannot be sliced (a signature
  threaded through many callers, a struct-shape rewrite): plans openly collapse phases into one
  atomic commit, or carry a deliberately-stale marker across phases, because the project's own gates
  fail a lone phase.
- **The lifecycle is prose-only and keyed off the linked ADR.** Plans track their ADR's status,
  leaving ADR-less plans without a status driver and requiring an ad-hoc
  `# Implementation complete (YYYY-MM-DD)` line as the freeze marker.

This ADR settles the plan-authoring *convention* — the section taxonomy, the granularity rule, the
phase-atomicity exception, and the lifecycle. A companion ADR (structured plan artifact) mechanizes
it with machine-readable frontmatter, a scaffolded template, and the checks that enforce it.

## Decision

1. **Canonical plan section taxonomy.** A plan is, in order: its frontmatter (schema defined by the
   companion structured-artifact ADR); a title heading `# Plan: <Title>`; a narrative header of
   exactly four fields — **Goal**; **Architecture summary** (the *execution shape*, not the
   rationale, which lives in the linked ADR); **Tech stack**; **File structure** (`created` /
   `modified` / `deleted` only); one or more **Phases** of tasks, each closing with a commit; an
   **optional Verification** section (whole-effort end-state checks beyond the per-phase gates); and
   an **optional Notes** section (out-of-scope items, follow-ups, and implementation findings). No
   other top-level sections. The stray `Date:` and `Branch:` header fields are dropped — the date is
   frontmatter, the branch is git.

2. **Task granularity is defined by reviewability, not wall-clock.** A task is one reviewable,
   logically-coherent change: a single new file authored in full is one task; a mechanical edit is
   one task. The "~2-5 min" wording is retired. The exact-content / exact-diff rule and the
   no-placeholder rule are unchanged.

3. **Coupled-phase escape.** The default remains self-contained phases — each phase's closing commit
   passes the project gate on its own. When a change genuinely cannot be sliced into
   independently-gate-passing phases (a signature threaded through many callers, a struct-shape
   rewrite, a rename gated as one unit), the coupled phases share a single closing commit, explicitly
   marked as a coupled group with the reason it cannot be sliced. This is the exception, invoked only
   when slicing is impossible — never a convenience.

4. **Two-state, plan-own lifecycle.** A plan's status is `Proposed` or `Implemented`, carried in its
   frontmatter and independent of any linked ADR's status. A plan is mutable while `Proposed`
   (through review *and* implementation) and frozen at `Implemented`. The `status: Implemented` flip
   is the freeze marker, replacing the former `# Implementation complete (YYYY-MM-DD)` prose line for
   all plans, ADR-driven or not. Amendments during implementation are expected: findings surfaced by
   execution — a wrong diff, an unsliceable phase, a bad estimate — are recorded in the plan's Notes
   section, and the freeze lands in the same final commit that records them, so a frozen plan is an
   honest record of how the change actually rolled out. The executing skills own the flip and the
   findings-recording; `awf-retrospective` reflects but does not perform the flip.

## Invariants

The constraints below are **textual contracts** enforced by the plan-review step; the companion
structured-artifact ADR supplies their machine backing (frontmatter validity, template
section-parity, and the plan→ADR link check). This ADR introduces no tagged, code-backed invariant
of its own.

- A plan carries exactly the canonical sections: a `# Plan:` title, the four narrative header fields
  (goal / architecture summary / tech stack / file structure), phases of tasks, and at most the two
  optional tails (Verification, Notes).
- The architecture summary states execution shape only; design rationale lives in the linked ADR(s).
- A task is one reviewable, logically-coherent change; no step defers content with a placeholder.
- Self-contained phases are the default; a multi-phase single commit is the marked exception,
  carrying the reason the change cannot be sliced.
- A plan's frozen state is exactly `status: Implemented`; the former `# Implementation complete`
  prose line is not used.

## Consequences

- **Easier:** plans acquire one uniform shape, so adopters (and this project) stop re-improvising the
  header, the tail, and the freeze marker. Granularity guidance stops contradicting practice.
  Un-sliceable changes gain a sanctioned, visible path instead of silent rule-breaking.
- **Trade-offs accepted:** the coupled-phase escape is a judgment call that could be abused as a
  convenience — mitigated by requiring an explicit marked reason and keeping self-contained phases
  the default the reviewer checks first. Decoupling plan status from ADR status means a reader must
  not infer ADR state from plan state; the two co-flip in the usual single-effort flow but are
  formally independent.
- **Ruled out:** an `Accepted` middle state for plans (meaningless while plans stay mutable through
  implementation), wall-clock task sizing, and the `# Implementation complete` prose freeze marker.
- **Migration:** the 67 existing dogfooded plans (no frontmatter, `# Implementation complete`
  markers, stray `Date:` / `Branch:` fields) are grandfathered as frozen historical records — never
  retrofitted. The convention binds new plans only; the companion ADR's frontmatter parse-validation
  and plan→ADR link check must exempt pre-convention frozen plans, which carry no frontmatter to
  validate.
- **Downstream:** the companion structured-artifact ADR (β) mechanizes this convention —
  frontmatter, the `plans-template` singleton, `awf new plan`, and the plan→ADR link check. Because
  Decision 1's title/frontmatter split and Decision 4's `status:` freeze marker both ride on β's
  frontmatter mechanism, α's convention cannot be *fully* implemented until β lands; the single
  implementing plan links both ADRs and sequences β's frontmatter ahead of (or co-ships it with) α's
  lifecycle wiring. The updated artifacts are the `awf-writing-plans` skill, the `plans-readme` doc,
  the executing skills (which own the `status` flip that replaces the `# Implementation complete`
  line), and the **`plan-reviewer` agent** — whose `executability` lens still hard-codes the retired
  "~2-5 min" sizing and the unconditional self-contained-phase rule, and must retire the wall-clock
  wording, add the coupled-phase exception, and gain focus items for the section taxonomy, the
  coupled-group marker, and the two-state freeze. These edits land in the `internal/catalog`
  defaults, so they re-render `examples/sundial` (which must stay zero-notes per ADR-0090), the
  catalog-default prose stays publication-safe and generic (ADR-0001, ADR-0045, ADR-0082), and any
  wholesale-override sidecar (e.g. the plan-reviewer `focusItems`) is patched in lockstep.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| One combined ADR (convention + structured artifact) | Fuses two separable decisions — a prose convention and a data model plus tooling — into one large record; splitting lets each review independently. |
| Keep wall-clock sizing with a higher time bound | Any time bound is the wrong unit; reviewability is what actually sizes a task, and a bound would keep contradicting practice. |
| Three-state lifecycle mirroring the ADR (Proposed/Accepted/Implemented) | `Accepted` means body-freeze, which is meaningless for plans that stay amendable through implementation; two states give uniform mutability. |
| Derive plan mutability from the linked ADR's status (status quo) | Leaves ADR-less plans without a status driver and forces the ad-hoc `# Implementation complete` freeze marker. |
