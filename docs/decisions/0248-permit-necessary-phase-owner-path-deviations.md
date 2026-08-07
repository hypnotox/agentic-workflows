---
format: current-state-v4
slug: permit-necessary-phase-owner-path-deviations
status: Proposed
date: 2026-08-07
---
# ADR-0248: Permit Necessary Phase-Owner Path Deviations


## Context

ADR-0240 permits implementation owners to make reasoned plan-detail deviations when repository
authority determines a compliant correction. It preserves the approved outcome, material scope,
settled durable boundaries, and required verification while requiring delegated owners to report
each deviation for review and parent reconciliation.

The implementer contract still describes a dispatched task as the complete scope, however, and
phase dispatch extracts an exact path set before editing. A commit-capable phase owner that discovers
one necessary but omitted file can therefore interpret the path omission as an absolute boundary and
stop an otherwise valid transaction. The missing path is often stale implementation detail rather
than material scope: for example, a source-backed correction may require its adjacent test, fixture,
or owning declaration even though the dispatch omitted that file.

The same latitude is unsafe for commit-disabled helpers. Helper paths form an exhaustive,
path-disjoint partition; shared files remain parent-owned and later helpers may own neighboring
paths. A helper cannot know whether widening its assignment violates that ownership partition.

## Decision

1. `decision: necessary-path-deviation` Permit a commit-capable phase owner to modify or create an
   unlisted path when the path is necessary to complete the approved outcome, remains within material
   scope and settled durable boundaries, complies with repository authority, and preserves required
   verification. An omitted path alone is not a reason to stop. The owner reports every added path as
   a reasoned deviation with its rationale, governing authority, and verification.

2. `decision: helper-path-confinement` Keep commit-disabled helpers confined to the paths explicitly
   assigned by their task. A helper reports a necessary unassigned path without modifying it so the
   parent can preserve the path-disjoint partition, shared-file ownership, and deterministic
   integration.

3. `decision: no-path-classifier` Express the distinction in the existing shared implementation
   autonomy and implementer contracts, backed by rendered contract tests. Preserve missingkey-zero
   behavior and coherent token-free rendering when variables are empty. Do not introduce a path
   policy schema, automated material-scope classifier, runtime permission mechanism, or separate
   deviation ledger.

## State changes

- update `rendering/workflow-skill-templates:authority-guided-implementation-autonomy`
- update `rendering/workflow-skill-templates:maintainable-code-subagent-contract`
- update `rendering/workflow-skill-templates:implementer-role-contract`

## Consequences

Commit-capable phase owners can finish coherent green transactions when source reality exposes one
or more necessary files missing from a dispatch, instead of canceling solely because the authored
path inventory was incomplete. Their structured deviation report keeps the expansion visible to
phase review and parent reconciliation.

Path latitude remains semantic rather than open-ended. It cannot justify unrelated cleanup,
material scope expansion, a changed durable boundary, weakened verification, or a path that is only
convenient rather than necessary. Review and explicit staging make misclassification observable.

Helpers remain less autonomous because partition integrity outweighs the benefit of completing an
unassigned edit. The parent must resolve their reported omission, which may require inline
completion or a clean revised dispatch, but no helper can silently collide with parent-owned or
later-assigned work.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Treat every authored path list as absolute | It turns stale implementation detail into cancellation even when existing authority determines one compliant completion path. |
| Permit phase owners and helpers to widen paths equally | A helper lacks ownership of the complete transaction and cannot safely revise a path-disjoint partition or claim a shared file. |
| Require parent approval before every added path | It recreates the approval queue ADR-0240 removed for authority-preserving implementation details. |
| Infer allowed additions through an automated classifier | Necessity and material scope are contextual; a classifier would add false precision and a second policy mechanism. |

## Status history

- 2026-08-07: Proposed
