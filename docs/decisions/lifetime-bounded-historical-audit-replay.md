---
format: current-state-v4
slug: lifetime-bounded-historical-audit-replay
status: Proposed
date: 2026-08-05
---
# ADR-lifetime-bounded-historical-audit-replay: Lifetime-bounded historical audit replay

## Context

ADR-0221 consolidated historical audit into one invocation-owned range operation
and narrowed committed loading to the policy projection consumed by transition
and stale-merge replay. The operation derives each required revision at most once
and shares immutable revision states between those consumers. Its cache currently
retains every derived state until the invocation returns.

That lifetime is unsafe for long authority-heavy histories. In this repository,
`v0.22.0..HEAD` contains 1,299 commits, of which 765 change conservatively
relevant authority. The selected authority across those revisions totals about
2.04 GiB before snapshot cloning, YAML parsing, ADR and topic models, source
retention, and Go allocation overhead. An audit of that range exhausted more than
15 GiB of available memory and was killed by the operating system. The existing
50-commit synthetic benchmark also allocates about 3.4 times as much for an
authority-heavy history as for a code-only history despite using tiny records.

The operation retains two independent classes of unnecessary data. Rich range
commits keep Markdown before-and-after text after aggregate rules finish, while
revision states keep selection bytes, lazy closures, parsed records, topic prose,
and source evidence after their final historical consumer. Irrelevant revisions
can also alias a first-parent state, so release cannot be correct if it treats
revision keys as independent values.

The audit must preserve one explicit invocation, one range collection,
per-commit attribution, range-aggregate rules, first-parent transition semantics,
all-parent stale-merge qualification, and at-most-once derivation. It must not
replace the memory failure with public chunking, persistent cache invalidation,
or repeated parsing at arbitrary batch boundaries. The graph scheduler therefore
becomes a load-bearing part of operation ownership rather than an incidental
optimization.

## Decision

1. `decision: compact-replay-projection` The one range collection streams each
   rich commit through audit-owned incremental commit-local and range-aggregate
   rule accumulators, then immediately retains only compact replay metadata.
   Rule-specific buffers preserve existing finding grouping and order without
   retaining all Markdown before-and-after bodies. The Git commit model remains
   rich at its owning boundary; audit owns both incremental rule state and the
   reduced replay representation.

2. `decision: explicit-dependency-schedule` Historical audit constructs its
   deterministic dependency schedule from commit identities and ordered parents,
   separately validates graph integrity, and fails on malformed evidence. It
   does not rely on a Git backend's incidental traversal order for correctness.
   The schedule accounts conservatively for result, first-parent, relevance,
   boundary-parent, and every-parent stale-merge consumers; ambiguous evidence
   extends a lifetime rather than shortening it.

3. `decision: final-consumer-release` One invocation retains compact revision
   metadata and cached load outcomes as needed, but heavy committed policy
   projections live only through their final scheduled consumers. Snapshot
   loaders release captured selections after materialization, source evidence
   may end before the remaining transition projection, and a parsed projection
   becomes unreachable after final use. The measurable peak is proportional to
   compact range metadata plus the live unique dependency frontier, not to all
   authority-changing revisions in the range.

4. `decision: alias-aware-ownership` A revision proven irrelevant may share its
   first parent's immutable state, but revision keys and heavy state ownership
   are distinct. Every remaining consumer of every alias contributes to the
   shared state's lifetime. Release occurs only after the final shared consumer,
   and each required revision state or cached load error is still derived at
   most once and never retried.

5. `decision: deterministic-interleaved-replay` Transition and stale-merge work
   executes in deterministic graph order rather than in separate operation-wide
   phases so their shared evidence can be released. Findings
   retain their existing rule grouping, ordering, severity, commit attribution,
   and exit behavior through separate result buffers. Context termination still
   propagates immediately, non-context transition projection failures remain
   advisory, and stale-merge evidence failures remain fatal. When multiple
   failures coexist, graph execution order may change which failure surfaces
   first.

6. `decision: invocation-local-boundary` Incremental range-rule state, the
   replay schedule, reduced commit projection, revision ownership, and lifetime
   accounting remain cohesive in `internal/audit`. The existing Git seam exposes
   the single streaming range walk without acquiring audit policy. No persistent
   cache, audit database, working-tree shortcut, public batch control, generic
   cache framework, or second range collection is introduced. Sparse selection,
   current-state parsing, transition policy, and merge qualification remain
   with their existing owners.

7. `decision: bounded-memory-contract` Deterministic tests back final-use
   release, alias safety, at-most-once derivation, graph shapes, finding
   compatibility, and cancellation. Synthetic measurements report both work
   and the high-water count of live heavy projections so a source-map-only or
   garbage-collector-timing improvement cannot masquerade as the bound.

## State changes

- update `tooling/audit-and-snapshots:audit-history-operation-owned`

## Consequences

Long authority-heavy ranges no longer retain either every rich Markdown change
or one complete historical policy projection per relevant revision. A mostly
linear range should keep only incremental rule summaries, compact replay
metadata, and a small heavy-state frontier; forks and merges retain the unique
states their unresolved dependencies genuinely require. Compact metadata
remains proportional to range length, and a graph with a broad live frontier can
still require proportional heavy memory, so this is a structural bound rather
than a constant byte ceiling.

The operation's ownership becomes more explicit: streamed rich evidence,
rule-specific aggregate state, compact replay metadata, revision keys, shared
heavy entries, source evidence, and parsed universes each have a named final
consumer. That replaces repeated whole-range rule scans with incremental
accumulators and removes accidental closure and slice retention, but adds a
scheduler whose accounting is correctness-critical. Conservative retention and
deterministic graph tests protect against early release, especially for aliases,
boundary parents, and merges.

Audit output remains compatible for normal evaluations. Internal interleaving
means a repository with multiple simultaneous infrastructure failures, or a
cancellation racing a different failure, can surface a different first error
than the former stale-live-transition phase sequence. This trade-off is accepted
because preserving phase failure precedence would require unbounded retention
or repeated derivation.

Runtime should remain near the existing at-most-once pipeline because the design
does not intentionally reload authority. Lifetime bookkeeping and graph
planning add work proportional to compact range metadata. Further structural
sharing or incremental parsing remains a measured follow-up rather than scope
of this decision.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Collect the full rich range and release it after aggregate rules | It leaves peak memory proportional to every retained Markdown before-and-after body even if graph replay is bounded. |
| Release only snapshot closures and source maps | It reduces constants but parsed universes and parser-retained text still accumulate across all relevant revisions. |
| Fixed internal batches with boundary reloads | It supplies a simple cap but repeats revision derivation, can substantially slow authority-heavy audit, and breaks the established at-most-once contract. |
| Preserve separate stale and transition phases with one shared cache | It preserves failure precedence but necessarily retains every merge-related state until the later phase and remains unbounded for merge-heavy history. |
| Persist or spill revision states outside the operation | It introduces invalidation, cleanup, corruption, and transport concerns that are unnecessary when final-consumer release can bound the in-process frontier. |
| Structurally share every parsed ADR and topic by blob identity | It could reduce repeated representation cost but crosses parser and package boundaries and is not required to stop operation-wide retention. |

## Status history

- 2026-08-05: Proposed
