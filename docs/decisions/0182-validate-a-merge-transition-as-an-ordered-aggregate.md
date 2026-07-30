---
format: current-state-v2
status: Proposed
date: 2026-07-30
---
# ADR-0182: Validate a merge transition as an ordered aggregate

## Context

`currentstate.CheckPair` validates one HEAD-to-index transition. That is the right granularity for an
authored commit, where one commit is one authoring step. A merge commit is one Git commit but the
aggregate of a branch's commits, and three of the rules reached from `CheckPair` assume the two are the
same thing:

- `pairOps` refuses an ADR that contributes more than one newly appended batch
  ("at most one new batch is allowed per transition");
- the same function folds every operation into a flat `map[string]pairOp`, so a claim targeted by more
  than one operation is reported as
  "claim <id> is the target of more than one operation in this transition";
- `checkTransitions` calls `adr.HistoryTransitionValid`, whose V2 branch accepts an append of exactly
  one or exactly two status-history events in a fixed shape, so an ADR that advances more than one
  lifecycle step across the pair fails the history-prefix rule.

The first two fire on a legitimate integration. An effort that applied its ADR incrementally
contributes several batches at the merge, and an effort whose later ADR revises a claim its earlier ADR
added contributes an add and an update for one claim. The 2026-07-30 severity-simplification effort hit
both: five batches across three ADRs, three of them add-then-update pairs, all arriving at one merge.

The third did not fire for that effort, and the reason matters. `checkTransitions` pairs records by
number, so an ADR absent from the before side has no pair to validate. All three of that effort's ADRs
were authored on the branch and absent from the target, so their whole history arrived unchecked by
this rule. An effort that instead advances an ADR the target already carries, which is the ordinary
shape when a branch implements an ADR main holds as Proposed, appends a Status event plus one or more
Applied events plus a terminal Status event and is refused. Fixing only the first two rules would leave
that case broken while appearing to solve the problem.

None of the three protects the invariant it appears to.
`invariants/current-state-authority:state-impact-transition-atomic` requires a batch and its matching
claim mutations to land in one transaction rather than split across snapshot pairs. A merge cannot
violate that: every batch and every mutation it integrates arrives in the same commit. The count cap,
the flat fold, and the fixed event-count shape are proxies for "one commit is one step", which is
exactly what a merge is not.

The machinery for several batches in one transition already exists and is already correct. `pairOps`
sorts appended batches by `state-sequence` and requires them contiguous from the highest sequence
before the pair, so ordering across batches is established before any failing rule runs. What makes the
relaxation safe is not the fold itself: `CheckPair` also re-runs the full static `Check` over the after
universe, whose `checkAppliedOp` and `checkBackward` re-derive Origin, `Revised-by` membership and
sequence ordering, presence and absence, and removed-id reuse across the whole corpus. The aggregate
therefore validates ordering and net effect over a corpus that is independently checked for internal
consistency.

Three further observations shaped the decision.

First, `CheckPair(before, after Universe)` receives two universes and never a commit, so it cannot
detect a merge itself. The fact has to arrive from the caller, and the two callers are asymmetric:
`auditTransitions` already carries `IsMerge` (from `NumParents() > 1`), while `CheckStaged` has no
merge signal and `internal/git` exposes no helper for one.

Second, the flat fold cannot simply be disabled for merges. `checkMutations` classifies each claim by
its single operation and compares the claim's before and after state, so with the fold off, a claim
added and then updated across the pair would be validated as an update against a before side where it
does not exist, and an incoherent history such as two adds of one claim would pass unnoticed.

Third, per-step claim state does not exist in either universe the check receives. Only the net before
and after are available, so a per-update substance requirement is not merely unimplemented in aggregate
form, it is unverifiable there.

`awf effort integrate`'s divergent path runs `git merge --no-ff --no-commit` and directs the operator
to `awf check --staged`. That path therefore cannot pass its own check for an effort that appended more
than one batch to a single ADR, or whose batches target one claim more than once, or that advanced an
already-present ADR by more than one lifecycle step. Its fast-forward path runs `git merge --ff-only`,
produces no merge commit, and is unaffected.

## Decision

1. A merge transition is validated as an ordered aggregate rather than refused. The pair is still
   checked; what changes is that several batches, a claim's multi-step operation history, and an ADR's
   multi-step status history are each legal within one merge when they form a legal ordered chain.

2. The merge fact is supplied by the caller. `internal/currentstate` keeps receiving universes and
   gains no Git knowledge, preserving its parsed-input contract.

3. The mode travels as a named value rather than a bare boolean or a second exported entry point.
   `CheckPair` takes an explicit transition mode whose values name the two contracts, so each call site
   states which one it wants and there is one contract to keep correct rather than two. A boolean would
   match existing precedent in `internal/git` but would read as an unexplained flag at both call sites,
   and the audit caller already holds an `IsMerge` boolean it would otherwise map onto a second
   function through an `if`.

4. The staged caller detects a merge in progress by the presence of `MERGE_HEAD`, through a new helper
   in `internal/git` that resolves the worktree-private Git directory rather than assuming
   `<root>/.git`. `MERGE_HEAD` is per-worktree, and a project root may sit below the repository root or
   inside a linked worktree, so the helper resolves the path the way `internal/worktree`'s in-progress
   probe already does for the same class of marker files. The audit caller passes the `IsMerge` it
   already computes.

5. In aggregate form the per-ADR batch cap does not apply. Global sequence contiguity from the highest
   sequence before the pair still does, and so does every existing per-operation check.

6. In aggregate form a claim's operations across the pair are validated as an ordered chain rather than
   enumerated cases: taken in `state-sequence` order the chain admits at most one `add`, which must be
   first, at most one `remove`, which must be last, and any number of `update`s between them. The net
   effect the pair must show follows from the chain: a chain beginning with `add` and ending with
   `remove` is a net no-op that must leave the claim absent on both sides, a chain beginning with `add`
   otherwise is a net add, a chain ending with `remove` otherwise is a net remove, and a chain of
   updates alone is a net update. Any other chain is an error, including a second `add`, an operation
   after a `remove`, and a claim updated more than once by the same ADR within one aggregate, which
   `update-requires-substance` already forbids by requiring an update to append its ADR once.

7. In aggregate form an ADR's appended status-history events are validated as an ordered chain by the
   same principle: the prior history must remain an exact prefix, and the appended events must replay
   as a sequence of legal V2 transitions, each event legal in the state its predecessors produce. The
   fixed one-or-two-event shape continues to govern an authored commit.

8. In aggregate form the substantive-change requirement for an update is evaluated on the net effect.
   Per-step substance is enforced where it is verifiable, at the branch's own authored commits, and is
   not re-derived at the merge because the intermediate states are not present in either universe.

9. Item 8's `Revised-by` handling governs only a chain whose net effect is an update. Membership and
   increasing sequence order of `Revised-by` are already enforced corpus-wide by the static check over
   the after universe, so the aggregate rule adds only that the prior list is preserved as an exact
   prefix and grew by exactly the updating ADRs of the chain.

10. Nothing about an authored commit changes. The one-batch cap, the single-operation-per-claim rule,
    and the fixed status-event shape keep applying to every non-merge transition, which is where the
    "one commit is one step" discipline they encode is real.

11. The added claim states that a merge transition is validated as an ordered aggregate: several
    batches are legal when globally sequence-contiguous, a claim's operations must form a legal ordered
    chain, an ADR's appended status events must replay as legal transitions over an exact prefix, and
    an authored commit keeps the stricter per-step contract. It is `Backing: test`, proved on the fold
    itself, which is pure over parsed universes and needs no Git. The `MERGE_HEAD` detection is
    deliberately outside the claim's scope: it selects which contract applies rather than being part of
    either contract.

12. The same implementation commits the documentation this falsifies: the
    `Concurrent ADR application branches may require replay before integration` pitfall, which
    currently states that `awf check --staged` correctly rejects both the multiple-batch and the
    sequence-collision conditions, is rewritten to name only the sequence collision.

## State changes

- add `invariants/current-state-authority:merge-transition-ordered-aggregate`
- update `invariants/current-state-authority:update-requires-substance`

## Consequences

`awf effort integrate`'s divergent path becomes usable for the shapes it was built for: an
incrementally-applied ADR, which ADR-0135 explicitly supports, and a branch that advances an ADR the
target already carries.

It does not become unconditionally usable, and the remaining refusal is the common one. Global sequence
contiguity is retained, so a branch whose batches were numbered before the target advanced still
collides at the merge and must be renumbered before integration. Concurrent efforts are exactly the
case managed worktrees exist to enable, so this is the normal path rather than an edge case; the
severity-simplification effort that motivated this decision needed its sequences moved even though its
ADRs were new. The replay-before-integration guidance still applies to that condition.

The protection that matters is retained rather than traded away. A merge carrying an incoherent claim
history still fails, because the fold validates the chain instead of ignoring it; an illegal lifecycle
progression still fails, because the appended events must replay as legal transitions; and a claim
change present on neither parent is still caught by the unmatched-mutation path. The static `Check`
over the after universe runs unchanged and independently re-derives provenance for the whole corpus.

What is genuinely given up is narrower than "the ability to reject an aggregate", and worth naming.
An intra-branch violation of `state-impact-transition-atomic`, where a batch was appended in one commit
and its mutations landed in another, is invisible at the merge: the aggregate sees only the endpoints.
The same holds for per-update substance under item 8. Both were previously blocked by refusing the
merge outright, and both now rely on the branch's own per-commit checks, which `--no-verify` can skip.
That is the residual risk this decision accepts, and it is bounded by the fact that the resulting
corpus must still pass the full static check.

Two further costs. The aggregate path is a second contract to keep correct, and a defect there would be
invisible on authored commits and surface only at integration, the least convenient moment; the
mitigation is that both folds are pure over parsed universes and are therefore directly testable
without Git. And detection is by repository state, so `git merge --squash`, which records no
`MERGE_HEAD`, gets the authored-commit contract despite being a merge command, as does a caller that
stages a branch's worth of change by hand. That is deliberate: the relaxation is justified by recorded
merge provenance, not by the shape of a diff.

Not addressed here: validating a merge by replaying the branch's individual commits, which would
restore both of the guarantees named above. It needs Git history inside the check and a decision about
octopus merges and about a merge whose second parent is not reachable, so it is a larger decision than
this correction.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Exempt merges from `CheckPair` entirely | A claim change introduced only in the merge would then be validated by nothing. |
| Disable the duplicate-target fold for merges without an ordered replacement | A claim added and then updated would be validated as an update against a before side lacking it, and two adds of one claim would pass. |
| Fix only the batch cap and the duplicate fold | Leaves `HistoryTransitionValid` refusing any branch that advances an ADR the target already carries, which is the ordinary shape when a branch implements a Proposed ADR. |
| Replay the branch's commits at the merge | Stronger, and would keep per-step substance and intra-branch atomicity, but needs Git history inside `internal/currentstate` and answers for octopus and unreachable-parent merges; a larger decision. |
| Keep per-update substance as a hard guarantee | Not verifiable from two universes; the intermediate states exist only in the branch's commits. It would force the replay design above. |
| A boolean parameter on `CheckPair` | Matches `internal/git` precedent but reads as an unexplained flag at both call sites. |
| A separate exported entry point for the aggregate form | Creates a second exported contract for one behavioural axis and pushes an `if` into the audit caller that already holds the boolean. |
| Detect the merge inside `internal/currentstate` | It receives universes, never commits, and giving it Git knowledge breaks its parsed-input contract. |
| Infer aggregate mode from the diff shape, e.g. more than one batch present | Would let a hand-staged branch-sized commit silently buy the weaker contract; provenance, not shape, is the justification. |
| Resolve `MERGE_HEAD` at `<root>/.git` | Wrong for a linked worktree, where it is worktree-private, and for a project root below the repository root. |
| Leave it and integrate with `--no-verify` | The check would keep reporting a false defect at every integration, training operators to bypass it. |

## Status history

- 2026-07-30: Proposed
