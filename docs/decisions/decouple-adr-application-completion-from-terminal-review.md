---
format: current-state-v4
slug: decouple-adr-application-completion-from-terminal-review
status: Implementing
date: 2026-08-04
---
# ADR-decouple-adr-application-completion-from-terminal-review: Decouple ADR Application Completion from Terminal Review


## Context

ADR-0143 introduced incremental State changes application. It made `Implementing` mean that a
nonempty strict subset of an ADR's declared operations is Applied: entering the status appends the
first batch, middle commits append more batches, and the final Applied batch must be followed
immediately by the `Implemented` status event. This keeps every claim mutation atomic with its
application record, but it couples completion of active-authority changes to lifecycle completion.

ADR-0188 later moved the final batch and `Implemented` flip into terminal implementation review so
the ADR remained amendable while findings settled. That preserved the coupling: the final claim
mutation now waits for review even when implementation needs it earlier, and terminal review owns
both an active-authority transaction and the historical status transition. A prerequisite discovered
while planning another change exposed the contradiction directly: all declared state operations
needed to become current before implementation could truthfully finish, but the parser rejects that
operation partition while status remains `Implementing`.

Incremental application also carries an unrelated choreography constraint. Applied and Reapplied
event lists must follow State changes declaration order, and later batches cannot first-apply an
operation declared before an operation that an earlier batch already applied. The declarations name
unique claim IDs, so their source order contributes no dependency or conflict semantics. It instead
makes a plan structurally invalid when implementation discovers that a later declaration must land
first, even though every intermediate claim mutation would remain atomic and every declaration
would still be Applied exactly once by completion.

The couplings are enforced in the governed-history parser, lifecycle validation, operation progress,
pair-transition shape, and authored workflow guidance. Current-state authority itself is already
operation-based: each Applied event authorizes exactly its matching claim result without depending
on terminal status or the declaration position of another unique claim ID. Chronological history
still matters for append-only events, Applied-before-Reapplied correction, and claim chains across
history occurrences and ADRs; none of those constraints requires declaration-order application.

The semantic model should therefore preserve cohesive ownership in `internal/adr`: State changes is
the complete operation set, Applied events record unordered subsets of its not-yet-applied members,
and status records implementation and review progress. Current-state checking consumes event
chronology and operation membership without inventing order inside one event. No new status or
redundant completion event is needed, because an empty Remaining partition already represents
application completion exactly.

## Decision

1. `decision: application-progress-independent-of-review` For current-state-v2,
   current-state-v3, and current-state-v4 records, operation application progress is independent of
   terminal review. `Implementing` means at least one declared operation is Applied and permits zero
   or more Remaining operations. `Implemented` remains terminal and requires every declared
   operation to be Applied.

2. `decision: declarations-are-an-unordered-completion-set` The State changes list declares the
   complete set of operations that implementation must apply, not their execution sequence. Each
   Applied event names a nonempty, duplicate-free subset of declared operations that have not been
   Applied before. Operations may be first-applied across batches in any order and may appear within
   one event in any order. Every declaration must still be Applied exactly once before
   `Implemented`; undeclared and duplicate first applications remain invalid.

3. `decision: event-chronology-remains-authoritative` Status history remains date-nondecreasing and
   prefix-append-only. Retained event bytes remain exact history, and merge reconciliation preserves
   distinct batches in ascending ADR-identity and intra-ADR history order. Applied and Reapplied
   operation lists are unordered membership within one occurrence; their list position creates no
   additional chronology, dependency, or provenance meaning.

4. `decision: final-application-precedes-terminal-status` Entering `Implementing` continues to
   append its status event and first nonempty Applied batch in one checked transaction. A
   same-status `Implementing` transaction may append any later batch, including the batch that
   exhausts Remaining operations. After all operations are Applied, a later
   `Implementing`-to-`Implemented` transition appends only the terminal status event. Existing
   histories in which declaration-ordered batches and the final Applied and Implemented events are
   adjacent remain valid.

5. `decision: correction-window-closes-at-terminal-status` An Applied add or update may be
   corrected by a material `Reapplied` event for as long as the ADR remains `Implementing`, including
   after every declaration is Applied. A Reapplied operation must have an earlier Applied occurrence,
   counts no second time toward completion, and may appear with other eligible corrections in any
   list order. `Amended` events remain legal throughout the same nonterminal window. Reapplied
   removes, operations without an earlier Applied occurrence, and all events after a terminal status
   remain refused.

6. `decision: abandonment-still-cancels-work` Explicit abandonment continues to require at least
   one unapplied operation to become Canceled. An all-Applied `Implementing` ADR therefore proceeds
   to `Implemented`; abandoning a fully applied decision is not a shorthand for undoing its active
   effects, which require an ordinary forward decision.

7. `decision: direct-terminal-shorthand-retained` Direct `Proposed`-to-`Implemented` and
   `Accepted`-to-`Implemented` transitions remain one implicit batch containing every declared
   operation and append no explicit Applied event. This shorthand remains appropriate for small
   atomic implementations and preserves existing records and workflows.

8. `decision: application-and-terminal-transactions-separated` Every explicit Applied batch,
   including the final one, continues to land atomically with exactly its matching current-state
   claim mutations. A later status-only `Implemented` transition carries no claim mutation.
   Authored workflow guidance assigns all explicit batches to implementation and assigns terminal
   review only the final status transition after findings settle.

9. `decision: additive-history-compatibility` The change broadens governed lifecycle validation
   without adding a status, history-event kind, ADR format, schema generation, or corpus migration.
   Every previously valid V2, V3, and V4 history remains valid. Existing declaration-ordered lists
   retain their exact bytes; newly legal histories may use a different operation order without
   changing append-only comparison or deterministic progress presentation.

10. `decision: publication-safe-template-rendering` Every affected template preserves coherent
    missing-key-zero rendering. An unset or empty variable produces generic usable prose and never
    emits an unresolved-value or no-value token in any rendered target or adopter output.

## State changes

- update `adr-system/adr-lifecycle:adr-status-enum-and-matrix`
- update `adr-system/adr-lifecycle:corrective-reapplication`
- update `adr-system/adr-lifecycle:applied-history-events-append-only`
- update `invariants/current-state-authority:merge-transition-ordered-aggregate`

## Consequences

- Active claim authority can become complete at the implementation transaction that establishes it,
  while the ADR stays amendable and correctable until terminal review settles.
- Plans may schedule unique declared claim operations in the order implementation needs. The checker
  still proves membership, exact-once first application, atomic claim mutation, and terminal
  completeness rather than treating declaration position as a dependency graph.
- Terminal review no longer owns a claim mutation. Its lifecycle commit records only that complete
  implementation and review have settled, making status meaning distinct from application progress.
- Explicit one-operation ADRs may enter an all-Applied `Implementing` state. Direct implicit
  completion remains available when a separate review window provides no value.
- Review findings may cause Reapplied corrections after the last first-application batch. This adds
  history lines but preserves material correction provenance and avoids a premature successor ADR.
- An all-Applied ADR cannot be Abandoned without a forward decision that changes or reverses its
  already-current effects.
- Operation-list order no longer supplies canonical history bytes for newly authored events.
  Equivalent membership can be serialized differently, but retained bytes remain immutable and
  deterministic projections continue to use declaration order for presentation.
- Authored lifecycle instructions, reviewer guidance, diagnostics, catalog data, pitfalls, tests,
  and rendered adopter outputs must distinguish unordered event membership from ordered history.
- Older binaries reject a newly legal all-Applied Implementing record, status-only terminal
  transition, or out-of-declaration-order event rather than misinterpreting it. The existing project
  binary-version lock advances with rendering; no stored-format migration is justified because the
  grammar and event kinds are unchanged.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Append an `ApplicationComplete` history event | The empty Remaining partition already records this fact mechanically; another event would duplicate state and enlarge validation. |
| Add a `Reviewing` lifecycle status | It would spread workflow phase state through every status consumer and transition matrix without adding authority information. |
| Keep the final Applied batch coupled to Implemented | Active authority would continue to wait for terminal review, and the correction window would still close at application completion rather than review settlement. |
| Keep declaration-order application | Unique claim declarations carry no ordering semantics, so this turns plan choreography into avoidable validator friction. |
| Permit repeated Applied events instead of Reapplied | It would conflate first application with correction and make completion progress ambiguous. |
| Remove Reapplied and require successor ADRs | A review-discovered correction to an already-applied add or update would need a new decision even while the owning ADR remains nonterminal. |

## Status history

- 2026-08-04: Proposed
- 2026-08-04: Implementing; content-sha256: 388dbee7d13cb3fdf6cbddaadb4092a7b65f4344816a3cfb85642eb83f4b3d08
- 2026-08-04: Applied; operations: update `adr-system/adr-lifecycle:applied-history-events-append-only`, update `adr-system/adr-lifecycle:adr-status-enum-and-matrix`, update `adr-system/adr-lifecycle:corrective-reapplication`
