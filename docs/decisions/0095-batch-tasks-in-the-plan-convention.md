---
status: Implemented
date: 2026-07-11
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [workflow, plans, convention, rendering]
related: [1, 45, 54, 80, 82, 90]
domains: [rendering]
---
# ADR-0095: Batch tasks in the plan convention

## Context

The awf plan convention mandates a single task form: exact file path, exact content or diff, and
the exact verifying command with expected output. Three rendered artifacts reinforce it in lockstep
— the `awf-writing-plans` skill (`conventions-tasks` and `conventions-no-placeholders`, the latter
forbidding "similar to task N" and requiring the change shown "verbatim"), `docs/plans/README.md`,
and the `plan-reviewer` agent, whose first focus item `step-exactness` flags any non-exact task.

That rule is correct for one-off changes and wrong for a task that applies one mechanical shape
across many sites — a rename, an API migration, a repeated edit. Followed literally, a planner
writes N near-identical diffs: high authoring cost, high review noise, and the reviewer's exactness
lens actively rewards the repetition. The skill's own `when-to-invoke` advertises "refactors
applying an already-decided pattern across many sites" as a prime reason to write a plan — the
exactness rule fights that exact use case.

The pattern that fits — show the transformation once on a representative site, enumerate (or
generate) the affected set, and prove coverage with a deterministic post-check — already appears
informally in committed plans (`2026-07-05-catalog-to-go`: "Example (representative — follow this
shape for all)"; `2026-06-29-cursor-adapter`: "let `./x gate` + check confirm correctness"). But it
is unsanctioned: an agent discovers it only by reading old plans, and the reviewer's exactness lens
may flag it when it is used.

"Exact diffs" is a proxy goal. The real goals are that the executor cannot go off-rails and the
reviewer can verify completeness. For repetitive work, a representative example plus a
coverage-proving check delivers that guarantee more cheaply — and more strongly — than N diffs a
reviewer must eyeball. `step-exactness` is a plan-reviewer focus-item *name*, not a backed `inv:`
slug (no invariant marker exists for it), so no supersession machinery is required; this is a prose
reconciliation across the catalog artifacts.

## Decision

1. **Sanction a second plan-task form, the *batch task*, as a peer of the exact-diff task.** A
   planner MAY use it when a single transformation applies across multiple sites — headline case:
   3+ sites whose change is identical modulo the site; also permitted when sites share one
   rationale with only mechanical variation. Genuinely distinct per-site edits remain exact-diff
   tasks.

2. **A batch task carries four labeled fields:**
   - **Representative (exact)** — one affected site shown with its exact diff, with the instruction
     to apply the identical shape to every affected site.
   - **Edge (exact)** — one non-uniform site shown exactly; OPTIONAL when the planner states the
     shape is identical at every site (e.g. "exact same shape at every site").
   - **Affected sites** — the completeness contract: either an exhaustive enumeration of
     paths/symbols, or a reproducing command whose output is exactly that set. The planner states
     which.
   - **Post-check** — a deterministic command (for example `./x check` clean, a named test target
     green, or `rg -c <pattern>` resolving to 0) that fails if any site is unconverted or wrong.

3. **The trigger is guidance, not a gate.** No command or check enforces when batch mode is chosen.
   Correctness is carried by the four fields plus the terminal `awf-reviewing-impl` review, not by
   policing the trigger.

4. **Reconcile the existing convention prose.** `conventions-no-placeholders` is narrowed to forbid
   vague deferral ("TBD", "implement later", "similar to task N") while explicitly permitting the
   batch task's "apply the identical shape to every affected site" — that instruction is backed by
   an enumerated or generated site set and a deterministic post-check, not a placeholder. The
   `plan-reviewer` `step-exactness` focus item is refined to accept "an exact diff, or a well-formed
   batch task (representative + affected-site set + post-check)" so a legitimate batch task is not
   flagged.

5. **Ship at catalog level.** The batch-task definition is hosted in the existing `conventions-tasks`
   section of the `awf-writing-plans` skill template — no new `awf:section` marker is added, so
   `skill-section-parity` (ADR-0054) is untouched. The change also lands in the
   `docs/plans/README.md` `structure` section and the `plan-reviewer` `step-exactness` focus item,
   refined in both the `internal/catalog` default *and* this repository's `plan-reviewer` sidecar
   (whose `focusItems` replace the catalog default wholesale). Catalog-default template prose stays
   publication-safe and generic (ADR-0001, ADR-0045, ADR-0082): it names "the project's drift check"
   rather than a concrete `./x check`, and reserves repo-specific commands for this repo's sidecar
   and rendered output. Catalog-default adopters receive the refinement on re-render; an adopter
   whose `plan-reviewer` sidecar overrides `focusItems` wholesale — the case this ADR hand-patches
   here — keeps its old `step-exactness` text and must update its own sidecar.

## Invariants

Textual contracts. Plans are hand-authored prose and awf does not parse plan bodies, so no
machine-enforceable `inv:` slug backs this ADR; the batch-task form is enforced by convention and
by the `plan-reviewer` lens, not by a source marker.

- The `awf-writing-plans` skill and the `plan-reviewer` agent never disagree on task legality: any
  task form the skill sanctions, the reviewer accepts. A batch task the skill permits is not flagged
  by `step-exactness`.
- The batch-task convention adds no placeholder latitude for single-site work: the exact-diff task
  remains mandatory whenever a transformation does not repeat across multiple sites.
- The convention is publication-safe prose: it degrades to the exact-diff task when batching does
  not apply and names no project-specific command as mandatory — post-check examples are
  illustrative.

## Consequences

- **Easier.** Repetitive multi-site plans shrink from N diffs to one representative plus a coverage
  check — less planner effort, less reviewer noise, and a stronger completeness guarantee than
  eyeballing N near-identical diffs.
- **Risk: a weak post-check.** A batch task whose post-check passes even with a missed site defeats
  the guarantee. Mitigated by the mandatory Edge field (waivable only with an explicit uniform-shape
  assertion) and by charging the refined reviewer lens to judge post-check *strength*, not merely
  its presence.
- **Cost: heavier reviewer judgment.** Trading N eyeball-able diffs for one representative plus a
  coverage check makes the `step-exactness` lens more subjective — a fresh-context or adopter
  reviewer must judge whether a post-check *actually* proves coverage, which is harder to apply
  consistently than confirming a diff is exact. Accepted as the price of removing the repetition.
- **Forward-compatible hook, not yet enforced.** The four labeled fields are shaped so a future
  `awf check` could parse a batch task and assert it names a post-check. Nothing enforces this today
  — the fields are a human convention — and that gap is deliberate, recorded here so it is not
  mistaken for an oversight.
- **Operational: catalog re-render.** The catalog edit re-renders the example adopter
  (`examples/sundial`) plan-reviewer, requiring a committed `./x sync` re-render (ADR-0090). Because
  this repo's `plan-reviewer` sidecar overrides the catalog `focusItems` wholesale, the refined
  `step-exactness` text must be written into both the catalog default and the sidecar.
- **Unblocks** cleaner plans for the recurring "apply a decided pattern across many sites" case the
  skill already advertises.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Lightweight exception — loosen the exactness rule, add no required structure | Leaves the guarantee soft: nothing requires the post-check or the site set, collapsing to "trust the executor" — the very failure exactness protected against. |
| Hard trigger — batch mode only at N≥3 identical, mechanically gated | The boundary is fuzzy (same-rationale variation is legitimate) and a mechanical gate over hand-authored prose is unenforceable; guidance plus the contract is the right altitude. |
| Tooling-first — awf parses plans and validates batch tasks | Premature: awf deliberately does not parse plan bodies today. The labeled-field convention keeps that door open without opening it now. |
| A dedicated new skill section for batch tasks | A new `awf:section` marker is a `skill-section-parity` lockstep edit (ADR-0054) for no benefit; the guidance fits the existing `conventions-tasks` section. |
