---
format: current-state-v4
slug: validate-authored-transactions-by-observable-operations
status: Implementing
date: 2026-08-05
---
# ADR-0233: Validate Authored Transactions by Observable Operations

## Context

`currentstate.CheckPair` compares a before and after authority universe. For an ordinary
HEAD-to-index transition it currently treats one Git commit as exactly one authoring step:
one ADR may append at most one application batch, one claim may be targeted by at most one
operation occurrence, and Status history may grow only by a fixed one- or two-event shape.
The first and third restrictions reject a transaction even when every appended event forms
a legal ordered history and every operation has its independently observable required claim
result, including the empty mutation set required for a dominated update.

[ADR-0182](0182-validate-a-merge-transition-as-an-ordered-aggregate.md) introduced an
ordered aggregate contract for merges. It preserves an exact history prefix, replays every
appended lifecycle event, orders application batches, folds legal operation chains, checks
their net claim effect, and then runs the complete static after-state validation. That
record deliberately retained the stricter authored contract because merge provenance, not
diff size, justified losing access to intermediate claim bytes.

The distinction contains two separate policies. Same-claim operation chains genuinely need
an observable boundary between authored applications: without the intermediate claim bytes,
the checker can prove only a net endpoint, not that every update or corrective re-application
made its own material change. The one-batch cap and fixed event-count shape do not supply that
proof when operation targets are distinct. Each distinct operation still maps independently
to its required before-to-after result, which is empty for a dominated update, and the
governed history parser can replay several status and application events without inventing
claim state.

This difference matters beyond staging. Historical audit selects the authored contract for
every non-merge parent-to-commit pair, so changing it also changes how the current binary
interprets retained commits. The resulting rule must therefore state the durable observable
boundary, not merely make a current command more convenient.

## Decision

1. `decision: observable-authored-transaction` An authored HEAD-to-index transaction may append any number of application or re-application batches and any number of Status-history events when the prior history remains an exact prefix, the appended events replay as a legal ordered lifecycle, and every operation occurrence has its matching required claim result in that same transaction. A dominated update's matching result is the empty mutation set. A Git commit is an authority transaction, not a required one-event workflow step.

2. `decision: same-claim-authored-boundary` One claim ID may still be the target of at most one operation occurrence in an authored transaction. This boundary is load-bearing: it preserves an observable before and after for every authored update and corrective re-application, so each occurrence proves its own material effect rather than borrowing one net endpoint for several recorded operations. Multiple batches in one authored transaction are therefore legal only across distinct claim IDs.

3. `decision: merge-chain-boundary` Merge transitions keep the ordered aggregate operation-chain contract established by ADR-0182. Recorded merge provenance justifies validating same-claim chains by their legal net effect because the authored commits remain the boundaries that proved per-occurrence materiality; an ordinary authored or squash-shaped transaction does not receive that relaxation.

4. `decision: historical-authored-interpretation` Historical audit applies the relaxed authored transaction contract to every non-merge parent-to-commit pair it replays. A retained non-merge commit with several distinct-target batches or a legal multi-event history may therefore become clean under a newer awf binary, while same-claim chains, rewritten history, unmatched mutations, illegal lifecycle events, and invalid final authority remain findings.

## State changes

- update `invariants/current-state-authority:merge-transition-ordered-aggregate`

## Consequences

A coherent authored transaction no longer has to split solely to satisfy a batch count or
Status-event count. This permits, for example, several independently observable claim
operations from one ADR to land together and a legal lifecycle progression to be recorded
without manufacturing intermediate commits.

The relaxation does not turn ordinary commits into merge aggregates. Two updates to one
claim, an application followed by its corrective re-application, and cross-ADR add/update
chains still require separately checked authored transactions. That cost preserves the
substantive meaning of every operation occurrence and the existing corrective
re-application contract.

Staged and historical callers share the revised authored interpretation. Audit results over
old commits can change when their only defect was authored cardinality, an intentional
forward interpretation change. Final-state validation, operation provenance, frozen
content, assigned identity, append-only history, stale-format merge authorization, and lock
authority are unchanged.

The transition implementation retains two semantic contracts: authored transactions reject
same-claim chains, while recorded merges may fold them. This is less mechanically simple
than one universal aggregate contract, but it keeps the stronger proof exactly where the
required intermediate bytes are observable.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the authored contract unchanged | The one-batch and fixed-event caps enforce workflow choreography even when every durable operation and lifecycle edge remains independently verifiable. |
| Apply the full merge aggregate contract to every transaction | Same-claim chains and repeated corrective events would prove only one net endpoint, weakening operation atomicity and per-occurrence substance. |
| Relax batches but retain the fixed Status-event shape | Leaves the same one-commit-one-step proxy in a second form and still rejects histories the parser can replay completely. |
| Relax Status-event cardinality but retain the one-batch cap | Leaves the batch-count proxy refusing independently observable operations without adding evidence about any operation. |
| Reconstruct intermediate claim states from Status events | Operations do not carry claim bodies, so the missing intermediate bytes cannot be derived from the ADR history. |

## Status history

- 2026-08-05: Proposed
- 2026-08-05: Implementing; content-sha256: 3e0b319a49115e0473e9083ddc49249e8e65532b9cdc45d0e21034f275427853
- 2026-08-05: Applied; operations: update `invariants/current-state-authority:merge-transition-ordered-aggregate`
