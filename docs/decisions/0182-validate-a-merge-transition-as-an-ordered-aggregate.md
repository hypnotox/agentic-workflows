---
format: current-state-v2
status: Proposed
date: 2026-07-30
---
# ADR-0182: Validate a merge transition as an ordered aggregate

## Context

`currentstate.CheckPair` validates one HEAD-to-index transition. That is the right granularity for an
authored commit, where one commit is one authoring step. A merge commit is one Git commit but the
aggregate of a branch's commits, and two of `CheckPair`'s rules assume the two are the same thing:

- `pairOps` refuses an ADR that contributes more than one newly appended batch
  ("at most one new batch is allowed per transition");
- the same function folds every operation into a flat `map[string]pairOp`, so a claim targeted by more
  than one operation is reported as
  "claim <id> is the target of more than one operation in this transition".

Both fire on a legitimate integration. An effort that applied its ADR incrementally contributes several
batches at the merge, and an effort whose later ADR revises a claim its earlier ADR added contributes an
add and an update for one claim. The 2026-07-30 severity-simplification effort hit both: five batches
across three ADRs, three of them add-then-update pairs, all arriving at one merge.

The rules are not protecting the invariant they appear to protect.
`invariants/current-state-authority:state-impact-transition-atomic` requires a batch and its matching
claim mutations to land in one transaction rather than split across snapshot pairs. A merge cannot
violate that: every batch and every mutation it integrates arrives in the same commit. The count cap and
the flat fold are proxies for "one commit is one step", which is exactly what a merge is not.

The machinery for several batches in one transition already exists and is already correct. `pairOps`
sorts appended batches by `state-sequence` and requires them contiguous from the highest sequence
before the pair, so ordering across batches is established before either failing rule runs.

Three further observations shaped the decision.

First, `CheckPair(before, after Universe)` receives two universes and never a commit, so it cannot
detect a merge itself. The fact has to arrive from the caller, and the two callers are asymmetric:
`auditTransitions` already carries `IsMerge` (from `NumParents() > 1`), while `CheckStaged` has no
merge signal at all and `internal/git` exposes no helper for one.

Second, the flat fold cannot simply be disabled for merges. `checkMutations` classifies each claim by
its single operation and compares the claim's before and after state, so with the fold off, a claim
added and then updated across the pair would be validated as an update against a before side where it
does not exist, and an incoherent history such as two adds of one claim would pass unnoticed.

Third, `checkUpdate` requires `Revised-by` to have grown by exactly the updating ADR with the prior
list as an exact prefix. Across an aggregate a claim may be revised by several ADRs in sequence order,
so the rule is correct per step but too narrow for the folded result.

`awf effort integrate` produces exactly the merge these rules refuse: its divergent path runs
`git merge --no-ff --no-commit` and directs the operator to `awf check --staged`. So the command awf
ships for integration cannot pass its own check for any effort that applied more than one batch.

## Decision

1. A merge transition is validated as an ordered aggregate rather than refused. The pair is still
   checked; what changes is that several batches and a claim's multi-step history are legal within one
   merge, in `state-sequence` order.

2. The merge fact is supplied by the caller. `internal/currentstate` keeps receiving universes and gains
   no Git knowledge, preserving its parsed-input contract.

3. `CheckPair` keeps its signature and meaning for an authored commit. The aggregate form is a separate
   exported entry point rather than a boolean parameter on the existing one, so a caller states which
   contract it wants and neither call site reads as a flag whose meaning must be looked up.

4. The staged caller detects a merge in progress by the presence of `MERGE_HEAD` in the Git directory,
   through a new helper in `internal/git`; a merge conflict resolution is still a merge, so detection
   is by repository state rather than by anything the index contains. The audit caller passes the
   `IsMerge` it already computes.

5. In aggregate form the per-ADR batch cap does not apply. Global sequence contiguity from the highest
   sequence before the pair still does, and so does every existing per-operation check.

6. In aggregate form a claim's operations fold in `state-sequence` order into the net effect the pair
   must show: `add` followed by any number of `update`s is a net add, `add` followed by `remove` is a
   net no-op that must leave the claim absent on both sides, and `update`s alone are a net update. Any
   other chain, including a second `add` or an operation after a `remove`, remains an error.

7. `checkUpdate`'s `Revised-by` rule generalizes for the folded case: the prior list must remain an
   exact prefix and the list must have grown by exactly the updating ADRs, in the sequence order their
   batches declare. Origin preservation and the substantive-change requirement are unchanged.

8. Nothing about an authored commit changes. The one-batch cap and single-operation-per-claim rule keep
   applying to every non-merge transition, which is where the "one commit is one step" discipline they
   encode is real.

## State changes

- add `invariants/current-state-authority:merge-transition-ordered-aggregate`

## Consequences

`awf effort integrate` becomes usable for the efforts it was built for. An incrementally-applied ADR,
which ADR-0135 explicitly supports, stops being un-integrable as a single merge.

The protection that matters is retained rather than traded away. A merge that carries an incoherent
claim history still fails, because the fold validates the chain instead of ignoring it, and a merge that
introduces a claim change present on neither parent is still caught by the existing unmatched-mutation
path. What is given up is the ability to reject a well-ordered aggregate purely for being an aggregate.

Two costs are worth naming. The aggregate path is a second contract to keep correct, and a defect there
would be invisible on authored commits and appear only at integration, which is the least convenient
moment; the mitigation is that the fold is pure over parsed universes and is therefore directly
testable without Git. And `MERGE_HEAD` detection is repository state, so a caller that stages a
branch's worth of change by hand, outside a real merge, still gets the stricter authored-commit
contract. That is deliberate: the relaxation is justified by a merge's provenance, not by the shape of
its diff.

Not addressed here: validating a merge by replaying the branch's individual commits, which would be
stronger still. It needs Git history inside the check and a decision about octopus merges and about a
merge whose second parent is not reachable, so it is a larger decision than this correction.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Exempt merges from `CheckPair` entirely | A claim change introduced only in the merge would then be validated by nothing. |
| Disable the duplicate-target fold for merges without an ordered replacement | A claim added and then updated would be validated as an update against a before side lacking it, and two adds of one claim would pass. |
| Replay the branch's commits at the merge | Stronger, but needs Git history inside `internal/currentstate` and answers for octopus and unreachable-parent merges; a larger decision. |
| A boolean parameter on `CheckPair` | Both call sites would read as an unexplained flag; a named entry point states the contract. |
| Detect the merge inside `internal/currentstate` | It receives universes, never commits, and giving it Git knowledge breaks its parsed-input contract. |
| Infer aggregate mode from the diff shape, e.g. more than one batch present | Would let a hand-staged branch-sized commit silently buy the weaker contract; provenance, not shape, is the justification. |
| Leave it and integrate with `--no-verify` | The check would keep reporting a false defect at every integration, training operators to bypass it. |

## Status history

- 2026-07-30: Proposed
