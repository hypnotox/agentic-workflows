---
format: current-state-v4
slug: make-plans-mutable-execution-guidance
status: Implementing
date: 2026-08-19
---
# ADR-make-plans-mutable-execution-guidance: Make Plans Mutable Execution Guidance


## Context

ADR-0286 defines the protected contract, distinguishes it from the revisable execution route, and
requires every other surface to point to that single authored doctrine rather than restating it.

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

1. `decision: plan-binds-protected-contract` A plan binds the protected contract defined by
   ADR-0286. Its plan-specific authority consists of its linked durable Decisions and its assignment
   of Definition of Done outcomes. The plan records the best known execution route at authoring
   time; recorded route metadata is not independent authority unless a settled decision explicitly
   makes a detail load-bearing.

2. `decision: route-revision-without-reapproval` A plan owner may merge, split, reorder, add, remove,
   or replace recorded route detail while the protected contract holds. A path omitted from the plan
   is not alone a stop condition, and a stale listed path need not be touched. Reapproval is required
   only when the protected contract would change or an unresolved material decision appears, never
   for a cleaner equivalent route.

3. `decision: material-plan-reconciliation` One shared plan-flexibility rule owns the plan-specific
   consequences of ADR-0286 and points to that doctrine rather than defining a version of its own.
   An owner records a route revision only when another phase or reviewer can rely on the affected
   implementation organization, path ownership, or local mechanism. Inconsequential and
   independently local edits require no deviation record. While a plan remains Proposed, its owner
   amends stale material instructions before a later owner could rely on them; delegated owners
   report such revisions for parent reconciliation.

4. `decision: structure-must-protect-a-property` Keep plan fields and parser enforcement only when
   they serve authority resolution, outcome ownership, material scope or confinement, lifecycle,
   safety, compatibility, or verification. Preserve existing historical grammar and the current
   machine-relevant roles of phase and task identity, Decision references, execution projection,
   Advances and Completes, necessary Paths, deterministic post-checks, independently green
   transactions, and phase-closing commit fences. Plans record ordering only where an actual
   dependency constrains a protected property; absence of such a dependency is not a review defect.
   Optional examples remain optional, so a batch does not require `Representative` or `Edge` when
   its Paths and deterministic Post-check already confine and verify the population.

5. `decision: plan-flexibility-proof` The added plan-flexibility invariant has `Backing: test`.
   Deterministic scenarios cover permitted route revisions, protected-contract stops, Proposed-plan
   reconciliation, and optional batch examples across Core and Full and both supported runtimes.
   Every affected template renders coherently with empty variables under missingkey-zero semantics
   and emits no unresolved token.

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

The claim operations divide by owned contract. The new `plan-flexibility` claim owns the single
plan-specific rule and its consumer and scenario coverage. `phase-transaction-ownership` changes so
a planned phase remains independently green without making its recorded grouping immutable.
`plan-task-detail-modes` changes the authority and enforcement of route fields.
`maintainable-code-subagent-contract` and `implementer-role-contract` narrow deviation reporting to
parent reconciliation without weakening helper confinement. `maintainable-code-review-lenses`
changes plan review from recorded-choreography adherence to protected-contract adherence.

This decision does not relax the gate, verification strength, current-state authority, generated
source ownership, missingkey-zero rendering, worktree confinement, phase review freshness, or plan
lifecycle. It does not begin AF-004's clean-integration changes or AF-005's review-severity policy.

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
- 2026-08-19: Implementing; content-sha256: 77cd10cb7437550b1d96b6381104f7461fc5efb94625020bebd14fbcf935bc10
- 2026-08-19: Applied; operations: add `rendering/workflow-skill-templates:plan-flexibility`, update `rendering/workflow-skill-templates:phase-transaction-ownership`, update `rendering/workflow-skill-templates:plan-task-detail-modes`, update `rendering/workflow-skill-templates:maintainable-code-subagent-contract`, update `rendering/workflow-skill-templates:implementer-role-contract`, update `rendering/workflow-skill-templates:maintainable-code-review-lenses`
