---
format: current-state-v2
status: Proposed
date: 2026-07-27
---
# ADR-0166: Phase transaction ownership

## Context

Plan authoring defines phases as the independently green units that close with commits, but the
execution workflows currently treat checkbox tasks as the implementation, dispatch, review,
checkpoint, and commit boundaries. In subagent-driven execution this means serial commit-disabled
implementers can leave several tasks accumulated in one dirty shared checkout, with no one agent
owning the complete transaction from a known green baseline through the gate and commit. The
ADR-0164 implementation plan exposed the mismatch when execution dispatched an intermediate task
inside a phase rather than the complete phase.

A task remains useful as an ordered implementation step, especially for exact edits, tests, and
repeatable batch transformations. It is not necessarily an independently committable concern.
Making every task a transaction therefore conflicts with the plan convention that a phase's closing
commit must pass the gate on its own and represent one coherent concern.

ADR-0152 made intermediate implementation tasks routine checkpoint and Pi handoff boundaries. That
rule is frozen history and must be corrected forward through the claims it established. ADR-0164 is
already Implementing and removes runtime phase enforcement, effort lifecycle routing, and required
telemetry association. This decision follows that post-ADR-0164 architecture: phase ownership is
workflow guidance, not a promise that the runtime enforces or records every implementation phase.
Its implementation and overlapping claim operations must be sequenced after ADR-0164's relevant
batches.

Pi currently serializes implementation children and requires an implementation tool call to occupy
an exclusive parent tool batch. Those runtime constraints are sound and do not solve transaction
ownership by themselves. Concurrent mutation of one checkout would additionally require safe scope
enforcement, incidental-write handling, failure attribution, and deterministic integration, which
are outside this decision.

## Decision

1. A plan phase is the independently green implementation transaction. Its checkbox tasks are
   ordered steps within that transaction, not default commit, checkpoint, dispatch, or ownership
   boundaries. The containing phase must represent one coherent concern and end with its declared
   gate and closing commit.

2. Plan authors select execution ownership independently for each phase. One plan may mix inline
   phases and subagent-driven phases. The plan records the selected mode and enough phase-level
   context for an executor to take ownership without reconstructing prior conversation.

3. In subagent-driven mode, the parent dispatches one commit-capable implementer for the complete
   phase from a known green baseline. That implementer performs every ordered task, stages the full
   transaction, runs the required staged check and gate, and creates the phase's closing commit.
   The parent then obtains report-only phase review, resolves findings through focused follow-up
   commits, and checkpoints only after the phase is settled.

4. In inline mode, the parent owns the complete phase transaction, integrates all work, runs the
   checks, obtains review where applicable, and commits. An explicit batch task may be executed by
   the parent, by one sequential commit-disabled helper, or by several sequential commit-disabled
   helpers over declared subsets. Helpers never become transaction owners and never commit.

5. A batch task keeps its representative transformation, edge transformation, exhaustive affected
   sites, and deterministic post-check. Optional worker partitions assign every affected site to the
   parent or exactly one helper. Helper subsets are path-disjoint, shared files remain parent-owned,
   and focused mutating commands must be confined to the assigned subset.

6. The coupled-phase exception is removed. Plan authors merge or reorder horizontally sliced phases
   so production machinery lands with its first production consumer. No dead-code exception is
   introduced for a definition whose first production use would otherwise appear in a later phase.

7. An unfinished dirty phase is never handed casually to a successor implementer. The parent either
   completes it inline, restores the known green baseline and redispatches the whole phase, or stops
   when safety, scope, or authority requires user input. A stopped implementer may leave a dirty
   intermediate state, so the parent first inventories and evaluates it rather than assuming the
   implementer's report is complete.

8. Ownership transfer after a stopped implementer is explicit. The successor receives the complete
   revised phase, the dirty-state inventory, completed and remaining work, prior concerns, and the
   required recovery verification. A blind instruction to continue an individual task is forbidden.

9. Plan authoring and plan review enforce the phase transaction boundary, per-phase mode selection,
   coherent phase concern, clean-baseline requirement, and batch partition contract. Execution
   guidance for inline and subagent-driven modes consumes that same contract rather than inferring a
   whole-plan mode from task coupling.

10. Routine implementation checkpoints and Pi handoff guidance move from per-task boundaries to
    settled phase boundaries. ADR-0152 remains unchanged as historical record; this decision updates
    its active current-state claims. The mandatory approval boundaries and the routine attention
    classification remain unchanged.

11. Pi's sequential implementation dispatch and exclusive implementation-tool batch behavior remain
    unchanged. Concurrent same-checkout batch helpers and worktree-isolated or patch-producing
    parallel implementation are not introduced. The unresolved concurrent-helper design is recorded
    as a roadmap idea rather than implied by this workflow contract.

12. Tests prove phase-level ownership across plan authoring, plan review, inline execution, and
    subagent-driven execution. They cover mixed per-phase modes, a known green baseline, complete
    phase implementer briefs, commit-capable phase owners, helper-only batch workers, explicit dirty
    recovery, phase-level checkpoints, and a regression plan shape in which several checkbox tasks
    lead to one phase-closing gate and commit. Existing Pi serialization and commit-policy tests stay
    unchanged.

13. Implementation follows ADR-0164. This ADR's updates to
    `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage` and
    `rendering/pi-workflows:pi-session-handoff-workflow` are Applied only after ADR-0164 has Applied
    its pending updates to the same claims and the post-ADR-0164 workflow sources are on the base
    branch.

## State changes

- add `rendering/workflow-skill-templates:phase-transaction-ownership`
- update `rendering/workflow-skill-templates:plan-task-detail-modes`
- update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`
- update `rendering/pi-workflows:pi-session-handoff-workflow`

## Consequences

Each green implementation commit gains one clear owner and one reviewable phase concern. Plans can
still use detailed tasks and can choose the most suitable execution mode phase by phase, while fresh
context no longer fragments a single transaction among serial implementers. Phase-level recovery
also makes dirty intermediate work visible instead of silently passing it forward.

Phase implementers receive larger briefs and may hold more context than task implementers. Inline
parents retain integration responsibility when helpers are used, and explicit batch partitions add
planning detail. Removing coupled phases may require authors to merge or reorder work into larger
phases, but it preserves the repository's green-commit and dead-code invariants without an escape
hatch.

The workflow contract remains guidance after ADR-0164 rather than runtime phase enforcement. Agents
can still violate it, so rendering, reviewer, and regression tests must make the intended boundary
prominent and mechanically checked where the project can do so. Concurrent mutation remains slower
than it might be because helpers stay sequential; the deferred design work is preferable to unsafe
same-checkout concurrency.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep one implementer, review, checkpoint, and commit per task | It conflicts with phases as independently green concerns and leaves no owner for the complete phase transaction. |
| Select inline or subagent-driven mode once for the whole plan | Different phases can have materially different coupling and context needs; a plan-wide choice needlessly weakens one or more phases. |
| Allow concurrent same-checkout batch helpers now | Scope confinement, incidental writes, failure attribution, and integration are not sufficiently designed. |
| Use isolated worktrees for parallel phase implementation | It adds branch and integration complexity and was explicitly rejected for this workflow. |
| Have parallel helpers produce patches for the parent | Patch production adds orchestration and conflict-handling complexity without solving a present requirement. |
| Preserve coupled phases or add a dead-code escape hatch | Merging or reordering phases keeps every commit green and production definitions live when introduced. |
| Discard dirty work whenever an implementer stops | Automatic restoration can destroy useful progress; recovery requires inspection and an explicit ownership choice. |

## Status history

- 2026-07-27: Proposed
