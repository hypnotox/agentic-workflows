---
format: current-state-v4
slug: decouple-adr-application-completion-from-terminal-review
status: Proposed
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

The coupling is enforced in three places. Governed-history validation rejects an all-Applied
`Implementing` record and requires an explicit `Implemented` event immediately after the final
Applied event. Operation progress likewise requires both Applied and Remaining partitions while
Implementing. Authored lifecycle, execution, review, and plan-review guidance assigns the final
batch and status flip to one terminal-review transaction. Current-state authority itself is already
operation-based: each Applied event authorizes exactly its matching claim result without depending
on terminal status.

The domain-document staleness audit has the opposite stale coupling. It warns when an ADR first
reaches `Implemented`, even though active authority may have changed in earlier incremental batches.
Once terminal review becomes a status-only transition, that trigger would both arrive late and
incorrectly imply that the review commit should refresh current-state narrative.

The semantic model should therefore preserve cohesive ownership: `internal/adr` owns lifecycle and
application progress, current-state checking consumes Applied operations, and audit asks the ADR
model when authority first becomes applied. No new status or redundant completion event is needed,
because an empty Remaining partition already represents application completion exactly.

## Decision

1. `decision: application-progress-independent-of-review` For current-state-v2,
   current-state-v3, and current-state-v4 records, operation application progress is independent of
   terminal review. `Implementing` means at least one declared operation is Applied and permits zero
   or more Remaining operations. `Implemented` remains terminal and requires every declared
   operation to be Applied.

2. `decision: final-application-precedes-terminal-status` Entering `Implementing` continues to
   append its status event and first nonempty Applied batch in one checked transaction. A
   same-status `Implementing` transaction may append any later batch, including the batch that
   exhausts Remaining operations. After all operations are Applied, a later
   `Implementing`-to-`Implemented` transition appends only the terminal status event. Existing
   histories in which the final Applied event and Implemented event are adjacent remain valid.

3. `decision: correction-window-closes-at-terminal-status` An Applied add or update may be
   corrected by a material `Reapplied` event for as long as the ADR remains `Implementing`, including
   after every declaration is Applied. `Amended` events remain legal throughout the same
   nonterminal window. Reapplied removes, operations without an earlier Applied occurrence, and all
   events after a terminal status remain refused. Terminal review, rather than final application,
   closes the correction window.

4. `decision: abandonment-still-cancels-work` Explicit abandonment continues to require at least
   one unapplied operation to become Canceled. An all-Applied `Implementing` ADR therefore proceeds
   to `Implemented`; abandoning a fully applied decision is not a shorthand for undoing its active
   effects, which require an ordinary forward decision.

5. `decision: direct-terminal-shorthand-retained` Direct `Proposed`-to-`Implemented` and
   `Accepted`-to-`Implemented` transitions remain one implicit batch containing every declared
   operation and append no explicit Applied event. This shorthand remains appropriate for small
   atomic implementations and preserves existing records and workflows.

6. `decision: application-and-terminal-transactions-separated` Every explicit Applied batch,
   including the final one, continues to land atomically with exactly its matching current-state
   claim mutations. A later status-only `Implemented` transition carries no claim mutation.
   Authored workflow guidance assigns all explicit batches to implementation and assigns terminal
   review only the final status transition after findings settle.

7. `decision: staleness-follows-first-applied-authority` The advisory domain-document staleness
   rule triggers when an ADR changes from no Applied authority to nonempty Applied authority, not
   when it later becomes Implemented. It remains branch-aggregate by domain: a warning is emitted
   only when no in-range commit refreshes that domain's current-state narrative. Later batches and a
   status-only terminal transition do not retrigger it; a direct implicit terminal transition does.

8. `decision: audit-uses-bounded-semantic-pass` Domain-document staleness uses a dedicated strict
   ADR semantic pass to derive operation progress while existing audit rules retain their
   lightweight frontmatter parser. Before and after records are paired by persistent identity,
   including retained slug across pending-to-numbered renames, so numbering cannot resemble first
   authority acquisition. A record that cannot be parsed semantically is skipped by this advisory
   rule and remains the responsibility of the existing governed transition diagnostics.

9. `decision: additive-history-compatibility` The change broadens governed lifecycle validation
   without adding a status, history-event kind, ADR format, schema generation, or corpus migration.
   Every previously valid V2, V3, and V4 history remains valid. The operation partition and existing
   event stream remain the single stored representation of progress.

## State changes

- update `adr-system/adr-lifecycle:adr-status-enum-and-matrix`
- update `adr-system/adr-lifecycle:corrective-reapplication`
- update `tooling/audit-and-snapshots:audit-domain-doc-staleness`

## Consequences

- Active claim authority can become complete at the implementation transaction that establishes it,
  while the ADR stays amendable and correctable until terminal review settles.
- Terminal review no longer owns a claim mutation. Its lifecycle commit records only that complete
  implementation and review have settled, making status meaning distinct from application progress.
- Explicit one-operation ADRs may enter an all-Applied `Implementing` state. Direct implicit
  completion remains available when a separate review window provides no value.
- Review findings may cause Reapplied corrections after the last first-application batch. This adds
  history lines but preserves material correction provenance and avoids a premature successor ADR.
- An all-Applied ADR cannot be Abandoned without a forward decision that changes or reverses its
  already-current effects.
- The staleness audit becomes aligned with the first active-authority change. Its semantic pass is
  intentionally narrower and stricter than the shared frontmatter path, and malformed records do
  not create speculative advisory findings.
- Authored lifecycle instructions, agent guidance, diagnostics, catalog data, pitfalls, tests, and
  rendered adopter outputs must describe the independent progress model consistently.
- Older binaries reject a newly legal all-Applied Implementing record rather than misinterpreting
  it. The existing project binary-version lock advances with rendering; no stored-format migration
  is justified because the grammar and event kinds are unchanged.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Append an `ApplicationComplete` history event | The empty Remaining partition already records this fact mechanically; another event would duplicate state and enlarge validation. |
| Add a `Reviewing` lifecycle status | It would spread workflow phase state through every status consumer and transition matrix without adding authority information. |
| Keep the final Applied batch coupled to Implemented | Active authority would continue to wait for terminal review, and the correction window would still close at application completion rather than review settlement. |
| Infer audit timing from status alone | No existing status identifies first Applied authority across incremental, direct, and later terminal transitions. |
| Replace the audit's shared frontmatter parser with strict full-record parsing | Unrelated advisory rules intentionally tolerate bodies they do not need; widening their failure boundary is unnecessary. |

## Status history

- 2026-08-04: Proposed
