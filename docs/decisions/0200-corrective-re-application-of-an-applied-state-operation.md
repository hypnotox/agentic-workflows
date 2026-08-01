---
format: current-state-v2
status: Proposed
date: 2026-08-01
---
# ADR-0200: Corrective re-application of an applied state operation

## Context

A current-state-v2 ADR rolls its declared operations out in batches, each batch one checked
commit recorded by an Applied history event. Review of an in-flight ADR routinely arrives
in rounds, so a second round can find a further defect in a claim whose operation an
earlier batch already applied. Today that ADR has no way to fix it.

The wall is narrow and precise. ADR-0188 already makes a v2 ADR's content amendable through
Implementing, so an ADR may amend in a new operation on a different claim, and may freely
revise an operation it has not yet applied. Only one case is closed: correcting a claim
whose own operation this ADR already applied. ADR-0188 item 5 forbids altering an operation
already referenced by an Applied event, and one operation per claim per ADR (ADR-0135 item
3, restated by ADR-0182 item 6) forbids declaring a second one.

The recorded cost is real. During the ADR-numbering effort a review round found a second
problem in a claim whose update had shipped, and the correction was worked around by adding
a second test and marker rather than fixing the sentence. A separate false-enumeration
defect was deferred to the roadmap permanently, because the ADR that owned the claim could
not reach it. The available routes are a follow-up ADR or a remove-plus-add split; both are
sanctioned and both are heavier than the defect they answer.

Permitting a second operation was considered first and is the wrong model. An operation
declares what the ADR decided; a batch executes it. A correction is the same intent executed
twice, not a second decision, and writing it down twice would make the declaration list
report how many commits a rollout happened to take. That is the direction ADR-0191 moved
away from when it retired the global state sequence so batch positions stopped carrying
durable meaning. The provenance model refuses it for the same reason: Revised-by lists
deciding ADRs, is duplicate-free (`provenance-ordered-by-adr-number`), and pairs one entry
with one operation (`implemented-impact-bidirectional`), so two operations against one
entry cannot be represented.

The mechanics permit the narrower change. `revisedByExtension`
(`internal/currentstate/transition.go:405-431`) requires the after-list to equal the
duplicate-free union of the prior list and the updating ADRs, which already holds when the
ADR is present from the earlier batch. `checkUpdate` (`transition.go:368-381`) additionally
requires Origin preservation and a material change, both of which a genuine correction
satisfies. What blocks it is bookkeeping: `OperationProgress`
(`internal/adr/application.go:83-88`) treats a second appearance of an applied operation as
an error, and `foldChain` (`transition.go:294-321`) rejects a twice-updated claim in the
merge-aggregate path.

## Decision

1. A current-state-v2 ADR with status `Implementing` may correct the effect of an operation
   it has already applied, by appending a `Reapplied` Status-history event naming that
   operation. The event grammar mirrors the Applied event:
   `- YYYY-MM-DD: Reapplied; operations: <verb> <qualified-id>`. Each operation it names
   must already appear in an earlier Applied event of the same ADR, and the event is legal
   only while the status is `Implementing`. A `Reapplied` event carries its own commit,
   exactly as an Applied batch does.

2. Re-application does not change the declaration. `## State changes` still names each
   claim at most once, the operation set is unchanged, and ADR-0135 item 3 and ADR-0182
   item 6 stand. A `Reapplied` event is a record of execution, not of decision.

3. Only `add` and `update` operations are re-applicable. A corrected `add` is this ADR
   fixing a claim it created; a corrected `update` is this ADR fixing a revision it made. A
   `remove` is not re-applicable, because the claim it names no longer exists and there is
   nothing left to correct; an ADR that removed the wrong claim has made a decision error,
   which a later decision owns.

4. The substance requirement is unchanged and applies per re-application: the claim must
   change in a canonical field other than formatting or provenance, so a `Reapplied` event
   whose commit changes nothing material is rejected exactly as a hollow update is.

5. Provenance is written once, not once per execution. Re-applying an `update` leaves
   Revised-by unchanged, because the ADR is already present at its canonical position;
   appending it a second time is a defect. Re-applying an `add` leaves Origin unchanged and
   must not add the ADR to Revised-by, because an ADR is not a reviser of the claim it
   originated. `update-requires-substance` is amended to say the once rule is satisfied by
   the ADR's presence at its canonical position rather than by a fresh append, so a
   re-application preserves the entry instead of duplicating it.

6. Operation progress is unaffected by re-application. A re-applied operation was already
   Applied and stays Applied; it never returns to Remaining, is never counted twice, and
   never satisfies a Remaining operation. An ADR still reaches `Implemented` only when every
   declared operation has been applied at least once, and `Implementing` still requires at
   least one applied and at least one remaining operation.

7. Merge validation folds a re-application into the net effect. The operation chain for one
   claim within one ADR collapses to that claim's net change across the compared universes,
   which is the evaluation ADR-0191 already established for merges, so a claim corrected
   mid-rollout presents as one update by one ADR. `foldChain`'s prohibition on one ADR
   updating a claim twice narrows to declared operations, which item 2 leaves singular.

8. A refusal names the route. When an author declares a second operation on a claim the ADR
   has already applied, the error states that the operation is already applied and that a
   correction is a `Reapplied` event, rather than reporting only that the claim is named
   twice. For a claim this ADR does not own, or a defect found after a terminal status, the
   sanctioned routes remain a follow-up ADR, a remove-plus-add split, and an ADR-0188
   amendment while the operation is still unapplied.

9. The contract is documented where an author meets it, not only in this record. The ADR
   lifecycle guidance gains the re-application step and its `Implementing`-only boundary,
   the day-to-day usage guide gains the route selection between amendment, re-application,
   and follow-up ADR, and a pitfalls entry names the failure this replaces: an already
   applied operation discovered to be wrong.

## State changes

- add `adr-system/adr-lifecycle:corrective-reapplication`
- update `invariants/current-state-authority:update-requires-substance`

## Consequences

An in-flight ADR can now finish its own work correctly. The recorded workaround of adding a
second test and marker to avoid an unfixable sentence stops being necessary, and a defect
found in a second review round no longer chooses between a heavier follow-up record and a
permanent deferral.

The audit trail gets longer and more truthful at once. A corrected rollout shows its
correction as an event rather than hiding it in a workaround elsewhere, so reading an ADR's
history reveals that a batch needed fixing. That visibility is the point; the cost is only
that the need was previously invisible.

A new history event kind is a schema change to a governed format, so every consumer of
Status history has to learn it: the event parser, the transition validator, the merge
aggregate path, and the decision index. The change is additive, since no existing ADR
carries a `Reapplied` event and existing records stay valid unchanged.

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
grammar, the `Implementing` boundary, the verb restriction, and the provenance rule, and
needs its own proof.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Permit a second declared operation on one claim per ADR | Models an execution retry as a second decision, and the provenance model cannot represent it: Revised-by is duplicate-free and pairs one entry with one operation |
| Do nothing mechanical and improve the guidance only | The recorded cost includes one defect permanently deferred to the roadmap; guidance would name routes without making the cheap one exist |
| Extend re-application through terminal statuses | Would let an ADR edit its own effects after its meaning froze, which is the append-only model this project relies on |
| Allow re-applying a `remove` | The claim is gone, so there is nothing to correct, and re-adding a removed identity is already forbidden |
| Record the correction as a second Applied event with no new kind | Indistinguishable from an accidental duplicate application, and gives the validator no way to require that the operation was already applied |
| Require the correction to be a remove-plus-add split | Loses the claim identity, churns every proof marker and citation naming it, and reports a rename that did not happen |

## Status history

- 2026-08-01: Proposed
