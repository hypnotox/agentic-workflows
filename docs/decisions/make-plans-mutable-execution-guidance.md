---
format: current-state-v4
slug: make-plans-mutable-execution-guidance
status: Proposed
date: 2026-08-19
---
# ADR-make-plans-mutable-execution-guidance: Make Plans Mutable Execution Guidance


## Context

ADR-0286 establishes that the workflow protects the requested outcome, durable choices, material
scope, externally observable behaviour, compatibility, safety, active project constraints,
verification strength, and prohibited shortcuts. Phase and task boundaries, order, file and symbol
inventories, helper allocation, execution mode, commands, commit decomposition, and
non-load-bearing mechanisms are the execution route and remain revisable.

The plan system predates that authority and gives mixed instructions. Current authoring guidance
already calls `Latitude`, batch kind, `Representative`, and `Edge` optional aids and permits a phase
owner to resolve necessary omitted paths. Execution guidance nevertheless says not to drift from the
plan, treats every stale path or local instruction as a deviation to amend, and preserves literal
phase execution mode and ordering as though each were independently binding. This can make an owner
retain a worse route even when repository authority shows a cleaner one with the same protected
contract.

The contradiction reaches enforcement. The project plan reviewer rejects a missing
`Latitude: exact` although generic authoring guidance and the parser treat it as optional. The plan
claim calls `Representative` and `Edge` optional, while the parser requires both for every batch.
Other structure remains load-bearing: typed Decision references, phase and task selection,
Advances and Completes ownership, scope confinement, lifecycle and freeze rules, deterministic
post-checks for ambiguous populations, independently green transactions, and phase-closing commit
fences all serve authority, safety, outcome ownership, or verification.

The plan must therefore become the best known route at authoring time rather than a second source of
immutable choreography. This decision applies ADR-0286 to plans without restating its doctrine,
preserves machine-relevant structure, and changes only route authority.

## Decision

1. `decision: plan-binds-protected-contract` Under ADR-0286, a plan is binding for the requested
   outcome, its explicitly linked durable Decisions, material scope, externally observable
   behaviour, compatibility and safety constraints, required verification strength, prohibited
   shortcuts, and Definition of Done outcomes. A plan is the best known execution route at
   authoring time, not independent authority for phase or task boundaries, order, path inventories,
   local mechanisms, helper allocation, execution mode, local names, exact commands, or commit
   decomposition unless a settled decision explicitly makes one of those details load-bearing.

2. `decision: route-revision-without-reapproval` A plan owner may merge, split, reorder, add, remove,
   or replace route detail while the protected contract holds. A path omitted from the plan is not
   alone a stop condition, and a stale listed path need not be touched. Reapproval is required only
   when the protected contract would change or an unresolved material decision appears, never for a
   cleaner equivalent route.

3. `decision: material-plan-reconciliation` One shared plan-flexibility rule owns the plan-specific
   consequences of ADR-0286 and points to that doctrine rather than defining a version of its own.
   An owner records a route revision only when it materially changes implementation organization,
   path ownership, local mechanism, or work another phase or reviewer can rely on. Inconsequential
   local edits require no deviation record. While a plan remains Proposed, its owner amends stale
   material instructions before a later owner could rely on them; delegated owners report such
   revisions for parent reconciliation.

4. `decision: structure-must-protect-a-property` Keep plan fields and parser enforcement only when
   they serve authority resolution, outcome ownership, material scope or confinement, lifecycle,
   safety, compatibility, or verification. Preserve existing historical grammar and the current
   machine-relevant roles of phase and task identity, Decision references, execution projection,
   Advances and Completes, necessary Paths, deterministic post-checks, independently green
   transactions, and phase-closing commit fences. Detail that supplies examples or route
   choreography but protects none of those properties remains optional and non-binding.

## State changes

- add `rendering/workflow-skill-templates:plan-flexibility`
- update `rendering/workflow-skill-templates:phase-transaction-ownership`
- update `rendering/workflow-skill-templates:plan-task-detail-modes`
- update `rendering/workflow-skill-templates:maintainable-code-subagent-contract`
- update `rendering/workflow-skill-templates:implementer-role-contract`
- update `rendering/workflow-skill-templates:maintainable-code-review-lenses`

## Consequences

An implementation owner can correct a stale route instead of preserving it ceremonially. Two small
phases may become one coherent green transaction, an overloaded phase may split, an omitted path may
be added, an obsolete listed path may be skipped, and an equivalent local mechanism may replace the
one anticipated by the author. The protected outcome, authority, scope, safety, compatibility,
verification, and Definition of Done remain fixed.

A Proposed plan remains useful to later owners because material route changes are reconciled before
they can mislead another phase or reviewer. Notes retain changes that affect review or cross-phase
composition without becoming a ledger of inconsequential edits. Delegated owners preserve their
path and commit authority boundaries; route flexibility does not grant a helper new scope.

Independently green transactions, phase outcome ownership, typed authority references, confinement,
deterministic evidence for ambiguous populations, and lifecycle freeze rules remain. A planned phase
is still a coherent transaction boundary for the route currently recorded, but its identity is not
a durable reason to reject a cleaner regrouping before execution.

The principal risk is that an owner misclassifies a protected constraint as route detail. The
mitigation is ADR-0286's enumerated protected contract, explicit negative scenarios that still stop,
and review that distinguishes a concrete protected-contract change from mere plan drift.

The parser becomes less uniform where optional examples no longer accompany every batch. This is
intentional: examples may help a human understand a broad population, but they are not a mechanical
substitute for Paths and a deterministic Post-check.

This decision does not relax the gate, verification strength, current-state authority, generated
source ownership, worktree confinement, phase review freshness, or plan lifecycle. It does not begin
AF-004's clean-integration changes or AF-005's review-severity policy.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Remove detailed plan fields and phase structure broadly | Detail may drive projection, authority, confinement, outcome ownership, lifecycle, or verification; remove enforcement only after identifying the property it does or does not protect. |
| Keep exact plan choreography but permit case-by-case exceptions | Exceptions preserve the wrong default and grow by work shape; ADR-0286 already establishes the general authority boundary. |
| Let every plan skill explain flexibility independently | Parallel explanations would drift and violate the approved single-home rule. |
| Treat every route change as a recorded deviation | A ledger of inconsequential edits makes material cross-phase and review changes harder to see. |
| Make route changes silently even when a later owner can rely on stale prose | A mutable plan still has to remain truthful while it is the execution record consumed by other owners. |

## Status history

- 2026-08-19: Proposed
