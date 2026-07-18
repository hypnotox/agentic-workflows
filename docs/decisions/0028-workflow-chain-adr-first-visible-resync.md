---
status: Implemented
date: 2026-06-28
tags: [workflow-chain, chain-handoffs]
related: [4]
domains: [adr-system]
---
# ADR-0028: ADR-first ordering and a visible plan-ADR resync loop in the workflow chain

## Context

The project's documented workflow chain is internally inconsistent, in two independent ways.

**Ordering.** Both guide templates, `templates/agents-doc/AGENTS.md.tmpl` and
`templates/docs/workflow.md.tmpl` (rendered to `AGENTS.md` and `docs/workflow.md`), present the
chain as `brainstorming → planning (if warranted) → ADR (if warranted) → review → implementation
→ review`. But the `awf-brainstorming` terminal handoff
(`templates/skills/brainstorming/SKILL.md.tmpl`) prescribes the opposite for load-bearing +
complex work: propose and settle the ADR first, *then* write the plan. The guide and the skill
that drives the chain disagree about whether the plan or the ADR comes first.

**Review presentation.** ADR-0004 Decision item 1 chose to present reviews as *lightweight* ("the
grounding-check inside `*-brainstorming` subsumes plan/ADR review, and `*-reviewing-impl` is the
single terminal review") and deliberately kept `reviewing-plan-resync` out of the high-level
chain string (enforced by `internal/project/spine_test.go`, which asserts the rendered Workflow
section "must not present reviewing-plan-resync as a primary step"). In practice the skill chain is
heavier than that presentation admits: `awf-reviewing-adr` and `awf-reviewing-plan` are real
per-artifact review nodes, and `awf-reviewing-plan-resync` runs before implementation whenever both
an ADR and a plan exist. The guide therefore *understates* the process the skills implement, and
gives no name to the plan↔ADR reconciliation that is load-bearing for getting the two artifacts to
agree.

Three facts about the relationship shape the fix (raised during brainstorming):

- An ADR is execution-independent: the plan is detail derived from the decision, so sequencing a
  plan against an unsettled design encodes guesses.
- A single plan links **zero or more** ADRs (many-to-one), not exactly one. Several skill templates
  word this as "the linked ADR" (singular): `writing-plans` (×2), `reviewing-plan`,
  `reviewing-plan-resync`.
- Planning can itself surface a new load-bearing decision; when it does, the chain must loop back
  to propose or amend an ADR rather than press on.

Grounding (verified against source): the old chain string appears only in the two `.tmpl` files
(plus rendered outputs and two frozen `docs/plans/` records, which are historical and untouched);
no `.awf/` override part shadows the targeted sections; editing template *content* needs no schema
bump (`./x sync` regenerates). `README.md` (lines 48, 87) and `docs/decisions/0022-curated-init-default.md`
(line 78) also state the old `brainstorm → plan → ADR` order in prose: the README is
hand-maintained and is corrected here; ADR-0022 is Implemented and append-only, so its historical
prose is left as-is. One further order-bearing site exists: the `adr-system` domain narrative
(`.awf/domains/parts/adr-system/current-state.md`, rendered to `docs/domains/adr-system.md`)
abbreviates the chain as `brainstorm → plan/ADR → review → impl → review` (plan ahead of ADR). It is
a live awf-managed part (not append-only), and this ADR declares `domains: [adr-system]`, so it is
reconciled to the ADR-first order in the implementing range (see Downstream work).

**Lifecycle states (surfaced while planning this ADR).** ADR-first also exposes a latent
contradiction in the ADR/plan *lifecycle* rules. `awf-reviewing-adr` step 7 flips an ADR
`Proposed → Accepted` immediately after review, whereas `awf-proposing-adr` (note: "stays
`Proposed` through the implementation sequence"), `awf-executing-plans` (flip in the final
implementation commit; plan "mutable until `Implemented`"), and `awf-writing-plans` all treat the
ADR as `Proposed` through implementation. Under plan-first this was latent; under ADR-first (where
the ADR is reviewed *before* the plan is written) the early flip, combined with `awf-writing-plans`'
"freeze when the ADR flips to `Accepted`" rule, would leave every plan **born frozen**. The states
must be reconciled for ADR-first to be coherent. (`awf-proposing-adr`, `awf-executing-plans`, and
`awf-subagent-driven-development` already describe the correct model; `awf-reviewing-adr` and
`awf-writing-plans` are out of line, and two further skills carry stale references to the post-review
flip: `awf-reviewing-plan-resync` (its "the ADR status is flipped" precondition) and
`awf-reviewing-impl` (its "Accepted → Implemented" final-commit transition note).)

## Decision

1. **The workflow chain is ADR-first.** Both guide templates present:
   ```
   brainstorming → ADR (if warranted) → plan (if warranted) → resync (when both) → implementation → review
   ```
   An ADR, when warranted, is written **and reviewed to a settled state** before planning begins,
   because the plan is execution detail derived from the decision. The `(if warranted)` qualifiers
   preserve all cases: load-bearing + complex (ADR then plan), load-bearing + simple (ADR only),
   complex-only (plan only, no ADR), and neither.

2. **The plan↔ADR resync is a visible, first-class chain step**, overriding the hide-it choice in
   ADR-0004 Decision item 1 (`refines: ADR-0004#1`; "presents reviews as lightweight"). The guide prose describes the real
   review topology: each written artifact gets a fresh-context review (`reviewing-adr`,
   `reviewing-plan`); when both an ADR and a plan exist, a plan↔ADR **resync** reconciles them
   before implementation, looping until they converge; `reviewing-impl` is the terminal review. The
   "grounding-check subsumes plan/ADR review / single terminal review" framing is removed.

3. **The ADR↔plan relationship is many-to-one.** One plan links zero or more ADRs. Skill templates
   that say "the linked ADR" are reworded to "the linked ADR(s)"; `reviewing-plan-resync` fires when
   at least one ADR and a plan exist.

4. **Planning that surfaces a load-bearing decision loops back.** `awf-writing-plans` instructs:
   if planning surfaces an ADR-worthy decision, pause, invoke `awf-proposing-adr` (or amend a
   still-`Proposed` ADR via `awf-adr-lifecycle`), then resume the plan. The resync step
   (Decision 2) catches the resulting drift, so convergence is normally fast.

5. **`spine_test.go` enforces the new presentation:** the rendered Workflow section presents ADR
   before plan and surfaces the resync step. The prior assertion (resync must not appear as a
   primary step) is removed.

6. **ADR/plan lifecycle states are reconciled for ADR-first.** "Reviewed to a settled state before
   planning" (Decision 1) means the design is *review-converged*, not that the status is `Accepted`.
   The ADR stays `Proposed` through the implementation sequence and flips to `Accepted` (or directly
   `Implemented`) only in the final implementation commit, owned by `awf-executing-plans` /
   `awf-subagent-driven-development` (per `awf-adr-lifecycle`). When no plan exists (load-bearing +
   simple), the direct-implementation final commit performs the `Proposed → Accepted`/`Implemented`
   flip via `awf-adr-lifecycle`: the one-flip-owner invariant forbids reinstating a review-time
   flip for that case. `awf-reviewing-adr` no longer flips
   the status after review: it hands off to plan resync (when a plan exists) or to implementation
   with the ADR still `Proposed`; its step-8 "status is flipped" wording is corrected to match. The
   plan-freeze trigger in `awf-writing-plans` becomes `Implemented` (matching `awf-executing-plans`'
   "mutable until `Implemented`"); its "Accepted/Implemented" freeze wording is corrected to
   "Implemented". `awf-proposing-adr`, `awf-executing-plans`, and `awf-subagent-driven-development`
   already encode this model and are left unchanged. Two further skills carry stale references to the
   now-removed review-time flip and are corrected: `awf-reviewing-plan-resync` (drop "and the ADR
   status is flipped" from its when-fires precondition) and `awf-reviewing-impl` (its final-commit
   transition note `Accepted → Implemented` becomes `Proposed → Accepted`/`Implemented`).

## Invariants

- `invariant: workflow-chain-adr-before-plan`: the rendered AGENTS.md / workflow.md chain string presents
  the ADR step before the plan step.
- `invariant: workflow-chain-surfaces-resync`: the rendered Workflow chain names the plan↔ADR resync
  step; it is no longer hidden from the high-level chain.
- The "single terminal review" / "grounding-check subsumes plan/ADR review" framing appears in no
  rendered awf-managed doc. (textual)
- Skill templates referring to a plan's ADR do so in a form that admits zero or more ADRs. (textual)
- The ADR/plan lifecycle rules across all chain skills name one freeze trigger (`Implemented`) and
  one flip-owner (the implementation step's final commit); no skill flips the status at review time.
  (textual)

## Consequences

Easier: the guide and the skills tell the same story; a fresh agent reading `AGENTS.md` learns the
true order and the resync loop instead of a lighter process the skills don't follow. The resync
step gains a name in the chain, so "is the ADR and the plan in sync?" becomes an explicit gate
rather than folklore buried in a skill.

Harder / accepted: the high-level chain is marginally heavier to read than ADR-0004 intended: the
lightweight presentation was a deliberate readability choice, now traded for accuracy. "resync"
enters the high-level prose and therefore earns a glossary entry.

Downstream work created: the two guide templates re-rendered to `AGENTS.md` and `docs/workflow.md`
(`./x sync`); a `resync` entry in `docs/glossary.md`; the flipped assertion in
`internal/project/spine_test.go` (with `// invariant:` markers backing the two tagged invariants);
the `README.md` order correction; the `adr-system` domain part reconciled to ADR-first (re-rendering
`docs/domains/adr-system.md`). The lifecycle reconciliation (Decision 6) edits four skill templates:
`awf-reviewing-adr` (drop the post-review status flip; fix the step-8 wording), `awf-writing-plans`
(plan-freeze trigger `Accepted/Implemented → Implemented`), `awf-reviewing-plan-resync` (drop "and the
ADR status is flipped" from its when-fires precondition), and `awf-reviewing-impl` (final-commit
transition note corrected from `Accepted → Implemented` to `Proposed → Accepted`/`Implemented`). When this ADR's status lands as Accepted
or Implemented, the same commit regenerates `docs/decisions/ACTIVE.md` via `./x sync`. ADR-0004 stays `Implemented`; only its
Decision item 1 "presents reviews as lightweight" clause is overridden, recorded here as
partial-item supersedence (`related: [0004]`, predecessor status unchanged). ADR-0022's historical
prose is left under the append-only rule.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Reorder only; keep resync hidden (stay within ADR-0004) | Preserves the lightweight guide, but leaves it understating the skills and gives the sync gate no name; accuracy was chosen over lightness. |
| Fully supersede ADR-0004 | Most of ADR-0004 (section shape, docs module, config-driven prose) is unaffected; a full supersede would wrongly retire live decisions. Partial-item supersedence fits. |
| Branching chain diagram (all three warrant-cases shown explicitly) | More precise but over-heavy for a one-line orientation string; the `(if warranted)` qualifiers already carry the cases. |
| Rewrite ADR-0022's prose to the new order | ADR-0022 is Implemented; ADRs are append-only historical records. Editing its prose would violate the append-only invariant. |
| Record the lifecycle reconciliation (Decision 6) as a separate follow-up ADR, or defer it | The lifecycle states are an inseparable consequence of ADR-first (Decision 1): splitting them off would fragment one decision, and deferring would ship an internally contradictory chain (plans born frozen). Folded in via amendment while still `Proposed`. |
