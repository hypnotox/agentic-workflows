---
format: current-state-v2
status: Proposed
date: 2026-08-01
---
# ADR-0210: Corrective re-application of an applied state operation

## Context

A governed digest-format ADR, current-state-v2 or current-state-v3, rolls its declared
operations out in batches, each batch one checked commit recorded by an Applied history
event. Review of an in-flight ADR routinely arrives in rounds, so a later round can find a
further defect in a claim whose operation an earlier batch already applied. Today that ADR
has no way to fix it.

The wall is narrow and precise. ADR-0188 already makes a V2 or V3 ADR's content amendable
through Implementing, so an ADR may amend in a new operation on a different claim, and may
freely revise an operation it has not yet applied. Only one case is closed: correcting a
claim whose own operation this ADR already applied. Two separate rules close it. ADR-0135 item 3
forbids declaring a second operation on one claim, a rule about the declaration. ADR-0182
item 6 separately forbids a claim being updated more than once by the same ADR within one
aggregate, a rule about the execution chain. ADR-0188 item 5 completes the enclosure by
forbidding any alteration of an operation an Applied event already references.

The recorded cost is real. During the ADR-numbering effort a review round found a second
problem in a claim whose update had shipped, and the correction was worked around by adding
a second test and marker rather than fixing the sentence. A separate false-enumeration
defect was deferred to the roadmap permanently, because the ADR that owned the claim could
not reach it. The available routes are a follow-up ADR or a remove-plus-add split; both are
sanctioned and both are heavier than the defect they answer.

Permitting a second declared operation was considered first and is the wrong model. An
operation declares what the ADR decided; a batch executes it. A correction is the same
intent executed twice, not a second decision, and writing it down twice would make the
declaration list report how many commits a rollout happened to take. That is the direction
ADR-0191 moved away from when it retired the global state sequence so batch positions
stopped carrying durable meaning. The provenance model refuses it for the same reason:
Revised-by lists deciding ADRs, is duplicate-free (`provenance-ordered-by-adr-number`), and
pairs one entry with one operation (`implemented-impact-bidirectional`), so two operations
against one entry cannot be represented.

The update path's machinery already permits the narrower change. `revisedByExtension` in
`internal/currentstate/transition.go` requires the after-list to equal the duplicate-free
union of the prior list and the updating ADRs, which already holds when the ADR is present
from an earlier batch. `checkUpdate` additionally requires Origin preservation and a
material change, both of which a genuine correction satisfies.

The add path has no such machinery. `checkMutations`' add branch rejects any add whose claim
is present on the before side, which is exactly the shape of a corrective re-application of
an add, and materiality is never evaluated for the add verb at all. A corrective add
therefore needs a validation case that does not exist today.

Five sites block the change. `OperationProgress` in `internal/adr/application.go` treats a
second appearance of an applied operation as an error, and the governed history validator
in `internal/adr/format.go` rejects the same shape independently through its
applied-cardinality map. `HistoryTransitionValid`'s event-shape switch does not admit a new
kind. `foldChain` in `internal/currentstate/transition.go` rejects a repeatedly updated
claim in the merge-aggregate path. And the add branch above rejects a corrective add
outright. `ApplicationBatches` is the single projection feeding both `OperationProgress`
and `currentstate.pairOps`, but it currently retains neither the contributing event kind
nor occurrence identity. Reconciliation must preserve each corrective batch while progress
continues to count the declaration once.

## Decision

1. A current-state-v2 or current-state-v3 ADR with status `Implementing` may correct the
   effect of an operation it has already applied by appending a `Reapplied` Status-history
   event. The event grammar mirrors the Applied event exactly, qualified ids in inline code
   spans and a comma-separated list permitted:
   ``- YYYY-MM-DD: Reapplied; operations: <verb> `<qualified-id>`[, ...]``. Declaration
   order and within-event uniqueness apply as they do for Applied. Every operation it names
   must already appear in an earlier Applied event of the same ADR, and the event is legal
   only while the status is `Implementing`. The same operation may appear in any number of
   later `Reapplied` events during that window; each event carries its own commit and must
   reconcile a further material correction.

2. The recognized current-state-v2 and current-state-v3 history-event kinds gain
   `Reapplied`, alongside status, Applied, and Amended. This follows the ADR-0188 item 3
   precedent, which updated the same shared-semantics claim when it introduced the Amended
   kind.

3. Re-application does not change the declaration. `## State changes` still names each
   claim at most once, the operation set is unchanged, and ADR-0135 item 3 stands. A
   `Reapplied` event is a record of execution, not of decision.

4. Only `add` and `update` operations are re-applicable. A corrected `add` is this ADR
   fixing a claim it created; a corrected `update` is this ADR fixing a revision it made. A
   `remove` is not re-applicable, because the claim it names no longer exists and there is
   nothing left to correct; an ADR that removed the wrong claim has made a decision error,
   which a later decision owns.

5. Every `Reapplied` event enters the batch projection that mutation reconciliation
   consumes, so each corrective commit has a matching operation and is not rejected as an
   unmatched mutation. The projection retains the contributing event kind and an occurrence
   identity such as the history or batch index: two corrective events for the same operation
   remain two ordered reconciliation batches rather than collapsing by operation value or
   event kind. `OperationProgress` and the governed history validator apply different
   cardinality semantics over that projection. An operation must still appear in exactly one
   Applied event, so a second Applied occurrence remains an error. Each Reapplied occurrence
   instead requires that earlier Applied occurrence, contributes no new declaration to the
   applied partition, and may repeat without double-counting progress. This split is the
   mechanism the rest of these items assume.

6. A re-applied `update` is validated by the existing update path unchanged: the claim is
   present on both sides, Origin is preserved, `revisedByExtension` passes because the ADR
   is already at its canonical position, and `claimMateriallyEqual` requires a canonical
   non-provenance field to change.

7. A re-applied `add` is validated by a new corrective case on the add branch, which the
   add branch does not have today: the claim is present on both sides rather than absent
   before, Origin is unchanged and still names this ADR, Revised-by is byte-identical, and
   a canonical field other than formatting or provenance changed. The materiality check the
   update path already runs is what the add branch gains; without it the add verb would
   carry an unverifiable obligation.

8. Provenance is written once, not once per execution. Re-applying an `update` leaves
   Revised-by unchanged, because the ADR is already present at its canonical position;
   appending it a second time is a defect. Re-applying an `add` leaves Origin unchanged and
   must not add the ADR to Revised-by, because an ADR is not a reviser of the claim it
   originated. `update-requires-substance` is amended to say the once rule is satisfied by
   the ADR's presence at its canonical position rather than by a fresh append, so a
   re-application preserves the entry instead of duplicating it.

9. Operation progress is unaffected by re-application. A re-applied operation was already
   Applied and stays Applied; it never returns to Remaining, is never counted twice, and
   never satisfies a Remaining operation. An ADR still reaches `Implemented` only when every
   declared operation has been applied at least once, and `Implementing` still requires at
   least one applied and at least one remaining operation. That precondition sets the
   window's end: the correction route closes when the final batch applies, because no
   remaining operation is left to hold the ADR in `Implementing`. A defect found in the
   final batch or in the flip commit is a terminal-status case with no re-application route.

10. A `Reapplied` event may not sit between the final Applied event and an explicit
    Implemented status event, preserving the immediate-adjacency rule the governed history
    validator enforces. This mirrors the constraint ADR-0188 item 2 placed on the Amended
    kind for the same reason.

11. Merge validation admits every re-application into the chain, for both re-applicable
    verbs. ADR-0182 item 6 carries two separate prohibitions and this decision narrows both.
    Where a claim's chain within one aggregate carries later update entries from the same
    ADR and those entries are re-applications, the fold preserves every material mutation
    while contributing that ADR to the updaters list once, consistent with the
    presence-based once rule item 8 writes into `update-requires-substance`. Where the chain
    carries later add entries from the same ADR and those entries are re-applications, the
    entries fold into one net add attributed to the chain's first step while every
    intermediate correction is reconciled in order. The requirement that an add be the
    chain's first operation is therefore satisfied by that fold rather than violated by a
    correction. Both prohibitions are about the execution chain, which is why this decision
    must change them rather than leave them standing.

12. A refusal names the route. When an author declares a second operation on a claim the
    ADR has already applied, the error states that the operation is already applied and
    that a correction is a `Reapplied` event, rather than reporting only that the claim is
    named twice. For a claim this ADR does not own, or a defect found after the window
    closes, the sanctioned routes remain a follow-up ADR, a remove-plus-add split, and an
    ADR-0188 amendment while the operation is still unapplied.

13. The contract is documented where an author meets it, not only in this record. Each of
    these lands in the same commit as the behaviour: `docs/decisions/README.md` and
    `docs/decisions/template.md`, which state the Applied event grammar and the
    per-commit event shapes this decision extends; the `.awf/` authoring sources behind the
    ADR lifecycle skill, which gain the re-application step and its window boundaries; the
    working-with-awf guide, which gains the route selection between amendment,
    re-application, and follow-up ADR; and a pitfalls entry naming the failure this
    replaces, an already applied operation discovered to be wrong.

## State changes

- add `adr-system/adr-lifecycle:corrective-reapplication`
- update `adr-system/adr-lifecycle:adr-status-enum-and-matrix`
- update `invariants/current-state-authority:update-requires-substance`

## Consequences

An in-flight ADR can now finish its own work correctly. The recorded workaround of adding a
second test and marker to avoid an unfixable sentence stops being necessary, and a defect
found in a second review round no longer chooses between a heavier follow-up record and a
permanent deferral.

The window is narrower than "while Implementing" suggests, and that is worth stating
plainly: because `Implementing` requires a remaining operation, the route closes as the
final batch lands. The corrections this buys are those found while work remains, which is
where review rounds actually find them, but a defect surfacing in the flip commit is not
covered.

The audit trail gets longer and more truthful at once. A corrected rollout shows every
correction as its own event rather than hiding it in a workaround elsewhere, so reading an
ADR's history reveals both that a batch needed fixing and whether it needed fixing again.
That visibility is the point; the cost is only that the need was previously invisible.

A new history event kind is a schema change to both governed digest formats, so several
consumers have to learn it: the event parser, the transition validator, the
application-batch projection and the operation-progress partition it feeds, the shared V2
and V3 history validator's applied-cardinality and Implemented-adjacency rules, and the
merge aggregate path. The decision index is deliberately not among them, because it renders
identity, title, and status only. The change is additive, since no existing ADR carries a
`Reapplied` event and existing records stay valid unchanged. A pending V3 record uses the
same event semantics under its slug identity, and later numbering leaves every event byte
unchanged.

The add branch gains a validation case it has never had. Today `checkMutations` treats any
add of an already-present claim as an error, and evaluates materiality for the update verb
only; item 7 adds both the corrective shape and the materiality check to that branch. This
is the largest single piece of new behaviour and the most likely place for a defect.

The boundary is `Implementing`, so an ADR that reached a terminal status keeps today's
behaviour exactly: its meaning is frozen and a later decision owns any change. That
preserves the invariant that an ADR is history rather than active authority, and it is why
this relaxation does not erode the append-only model.

Re-application can be misused to avoid thinking, by shipping a batch knowing it can be
fixed later. The mitigations are that each re-application is a separate visible event with
its own commit and its own substance requirement, that review sees the count, and that the
cheaper path for an unapplied operation remains an ADR-0188 amendment, which leaves no
event at all.

`update-requires-substance` is `Backing: unbacked` and carries a `Verify:` line, so its
amendment revises that line rather than a proof marker. The new claim covers the event
grammar across V2 and V3, the window boundaries, repeatability, the verb restriction, the
batch-projection split, and the provenance rule, and needs its own proof whose fixtures
include two ordered Reapplied events for one operation.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Permit a second declared operation on one claim per ADR | Models an execution retry as a second decision, and the provenance model cannot represent it: Revised-by is duplicate-free and pairs one entry with one operation |
| Authorize the corrective mutation against the existing Applied record, with no new event | The cheapest option and needs no schema change, but it leaves no event, so a corrected rollout becomes indistinguishable from a claim edit nobody decided, which is the audit property Consequences calls the point |
| Do nothing mechanical and improve the guidance only | The recorded cost includes one defect permanently deferred to the roadmap; guidance would name routes without making the cheap one exist |
| Extend re-application through terminal statuses | Would let an ADR edit its own effects after its meaning froze, which is the append-only model this project relies on |
| Allow re-applying a `remove` | The claim is gone, so there is nothing to correct, and re-adding a removed identity is already forbidden |
| Record the correction as a second Applied event with no new kind | Indistinguishable from an accidental duplicate application, and gives the validator no way to require that the operation was already applied |
| Require the correction to be a remove-plus-add split | Loses the claim identity, churns every proof marker and citation naming it, and reports a rename that did not happen |

## Status history

- 2026-08-01: Proposed
