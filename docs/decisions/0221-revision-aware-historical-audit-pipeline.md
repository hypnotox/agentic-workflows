---
format: current-state-v3
slug: revision-aware-historical-audit-pipeline
status: Implementing
date: 2026-08-02
---
# ADR-0221: Revision-aware historical audit pipeline

## Context

`awf audit` evaluates workflow conformance over every commit reachable from the
range head but not the range base. In this repository, a nominal
`HEAD~5..HEAD` range can contain 69 commits because the range includes merged
history rather than only the first-parent chain.

The current implementation collects that commit range in `internal/audit` and
then collects it again for current-state transition replay in
`internal/project`. Transition replay materializes the complete result and
first-parent repository trees for every commit. Each materialization reads all
regular blobs, copies them into an immutable snapshot, parses historical
configuration, ADRs, and topics, and scans configured marker sources. Stale
merge authorization separately materializes the merge result and every parent.
The repository currently has about 1,454 tracked files and 16 MB of content per
tree.

A prebuilt binary takes about 1.7 seconds for one commit, 68 seconds for a
69-commit range, and 85 seconds for an 85-commit range on the observed
checkout. Building through the repository wrapper adds less than one second,
so compilation is not the dominant cost. Runtime grows at roughly one second
per commit because complete historical trees and current-state inputs are read,
copied, and parsed repeatedly.

The historical transition policy compares `currentstate.Universe` values.
That projection contains ADR records, ADR source bytes, and topic claims. It
does not contain marker indexes, coverage paths, or domain ownership paths.
Nevertheless, the broad historical loader currently validates those discarded
inputs. A malformed marker source or configured domain sidecar can therefore
become a transition warning or stale-merge failure even though neither policy
consumes it. Repository and staged checks already own those broader
validations. Historical transition loading also converts context cancellation
into a warning and continues rather than aborting the operation.

The audit must retain per-commit historical attribution, range-aggregate rules,
first-parent transition semantics, and all-parent stale merge qualification.
Its derived history belongs to one invocation: a persistent or project-owned
cache would introduce invalidation obligations and stale evidence. Git access
must continue through `internal/git`, and a sparse committed view must not be
indistinguishable from `snapshot.Tree`, whose absence semantics describe a
complete selected file set.

## Decision

1. One historical audit invocation owns one immutable range operation. It opens
   the repository once, collects the requested commits once, preserves their
   evaluation order, and threads that range to commit-local, range-aggregate,
   transition, and stale-merge rules.

2. The range operation owns lazy revision state for its lifetime. A revision
   state contains the historical configuration and layout, lock-derived schema
   boundary, reduced current-state universe, and any load error. Each revision
   state is derived at most once and is shared by transition and stale-merge
   replay. The cache is neither global nor stored on `Project`.

3. Historical transition and stale-merge replay load the policy projection they
   consume: ADR records and source bytes plus topic definitions and claims.
   They do not load or validate marker indexes, coverage paths, or domain
   ownership paths. Malformed or incomplete data that matters only to those
   omitted projections remains the responsibility of repository and staged
   checks and no longer creates a historical transition warning or stale-merge
   failure.

4. `internal/currentstate` and `internal/topic` separate topic and Universe
   assembly from marker indexing. Full working-tree and staged loaders retain
   marker and coverage behavior. The historical loader reuses the same ADR and
   topic parsers and assembly policy; it does not reimplement their grammar in
   `internal/audit`.

5. A revision may reuse its first parent's immutable state when changed-path
   evidence proves that no input to the historical projection changed. Any
   `.awf/**` change is conservatively relevant. A change under either
   historically applicable decisions directory is relevant. Configuration and
   layout are read from committed evidence, not the working tree. A parent
   outside the selected range is loaded directly as a boundary rather than
   causing an unbounded ancestry walk.

6. First-parent changed-path evidence for a merge is a separate range-operation
   facet. It does not populate or redefine `git.Commit.Changes`, because
   ordinary audit rules intentionally do not treat a merge as another copy of
   every integrated file change. Transitions compare a merge result with its
   first parent, while stale-merge qualification still compares the result,
   first parent, and every incoming parent.

7. `internal/git` supplies the minimum backend-neutral committed-tree evidence
   needed to enumerate paths without eager blob reads, select committed bytes,
   and obtain first-parent merge changes. Every new exported entrypoint receives
   a backend-neutral contract suite and registration in the mechanically checked
   Git entrypoint registry. `internal/snapshot` represents sparse committed
   content with an explicit selection boundary that cannot be passed
   accidentally as a complete `snapshot.Tree`. Existing complete snapshot
   entrypoints and their isolation guarantees remain unchanged.

8. `internal/audit` owns historical range evaluation, relevance policy, lazy
   revision state, transition replay, and stale-merge replay.
   `internal/project` remains the outer composition point that resolves project
   settings and layout, then invokes the one audit operation. Historical
   transition orchestration no longer performs a second range walk from
   `internal/project`.

9. Existing audit findings retain their rule names, ordering, severity, commit
   attribution, and exit behavior except for the narrowed validation boundary
   in item 3. A transition projection failure remains a warning for its commit,
   and a stale-merge evidence failure remains fatal. Context cancellation and
   deadline expiry are operation failures and propagate immediately rather
   than being converted into findings. Cached load errors are not retried.

10. All four state changes declared below are backed by tests. Historical audit
    operation tests prove one range collection, at most one state derivation per
    required revision, state reuse across irrelevant commits, and sharing
    between historical rules. Current-state and audit regressions prove the
    reduced projection and cancellation behavior. Snapshot tests and the Git
    contract suites prove explicit sparse selection. Each proving test carries
    the exact invariant marker required by the current-state claim.

11. Synthetic benchmarks cover code-only, authority-heavy, and merge-heavy
    ranges of at least 50 commits. Real-repository before-and-after measurements
    are reported, with less than 10 seconds considered a substantial improvement
    and less than 2 seconds an aspirational outcome for a representative 50-plus
    commit span. Neither duration is a hard acceptance threshold: measurements
    and profiles determine whether structural sharing or incremental corpus
    parsing warrants a follow-up.

12. The implementation updates the authored architecture description and the
    tooling domain current-state source in the same implementation transaction
    as the ownership and data-flow changes, then regenerates their rendered
    outputs.

## State changes

- add `tooling/audit-and-snapshots:audit-history-operation-owned`
- add `tooling/audit-and-snapshots:audit-history-policy-projection`
- add `tooling/audit-and-snapshots:audit-cancellation-propagates`
- add `tooling/audit-and-snapshots:sparse-snapshot-explicit-selection`

## Consequences

Audit work becomes proportional to commits that change historical authority
rather than to the product of every commit and the complete repository size.
Transition and stale-merge checks share one view of each revision, eliminating
both the second range walk and independently reconstructed parent snapshots.
Code-only commits can reuse authority state even when marker-bearing Go files
change, because historical transition policy does not consume marker backing.

The historical contract becomes narrower and easier to explain: repository and
staged checks validate the complete current-state model, while historical audit
replays only the authority needed for cross-commit transitions and stale ADR
imports. A repository with malformed marker or domain-path history can therefore
produce fewer historical warnings than before. This is intentional; current
invalid state remains rejected at its owning boundary.

The change introduces a revision-aware operation and a distinct sparse-content
boundary across several packages. Contract tests and operation-count tests must
pin those seams so later changes cannot restore eager full-tree reads silently.
Historical configuration changes, layout changes, merges, renames, deletions,
and boundary parents require conservative relevance handling. A false negative
in relevance would be a correctness defect, so ambiguous evidence reloads
rather than reuses.

No persistent cache, cache invalidation command, audit database, or working-tree
shortcut is introduced. Performance remains reproducible from committed Git
evidence. Wall-clock benchmarks remain diagnostic rather than a flaky gate, so
the first architecture correction can land with its measured result instead of
being held to a target chosen before its remaining costs are profiled.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Add a second-walk cache and skip obvious commits without changing ownership | This reduces some duplicate work but retains the full-tree snapshot and broad-loader model that dominates runtime. |
| Make default audit run only range-aggregate and live rules, with full replay behind a flag | This is faster but loses default detection of bypassed hooks, outdated hooks, and imported historical violations before the exact replay pipeline has been streamlined. |
| Preserve historical marker, coverage, and domain-sidecar validation incrementally | Those inputs are not consumed by the historical policies. Maintaining their complete state across every revision adds substantial complexity for incidental behavior already owned by repository and staged checks. |
| Audit only the first-parent chain | This avoids merged-history fan-out by dropping the individual commits integrated from side branches, losing the per-commit attribution and validation that historical audit exists to provide. |
| Keep complete snapshots but memoize them by revision | Adjacent commits normally have distinct trees, so memoization only removes duplicate parent loads and still reads and copies the whole repository once per commit. |
| Store a persistent cross-invocation audit cache | Persistence creates invalidation, object-lifetime, and corruption concerns that are unnecessary for eliminating duplicate work inside one invocation. |

## Status history

- 2026-08-02: Proposed
- 2026-08-02: Implementing; content-sha256: 757ce6f2a758c9f0b0a97f628f98ea4c0cd050eb9257a6c4aaa02fe2e026479a
- 2026-08-02: Applied; operations: add `tooling/audit-and-snapshots:audit-history-operation-owned`
- 2026-08-02: Applied; operations: add `tooling/audit-and-snapshots:audit-history-policy-projection`
- 2026-08-02: Applied; operations: add `tooling/audit-and-snapshots:audit-cancellation-propagates`
