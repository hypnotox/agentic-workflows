---
format: current-state-v3
slug: agpl-3-0-only-cutover-at-the-unpublished-history-boundary
status: Proposed
date: 2026-08-02
---
# ADR-agpl-3-0-only-cutover-at-the-unpublished-history-boundary: AGPL-3.0-only cutover at the unpublished history boundary

## Context

The published project is licensed under MIT. That grant remains available for every published
version and cannot be withdrawn from recipients. Apache-2.0 would add patent terms but would remain
permissive, so it would not address the concern that a proprietary fork could incorporate the work
without publishing its changes. AGPL-3.0 permits commercial use but requires covered derivative
work to remain under the same license and extends the source-availability obligation to modified
software used over a network.

The repository has a large unpublished local DAG shared by the integration branch and several
managed-worktree branches. At investigation time GitHub's `main` was the common published boundary,
no commit carrying the test-fixture identity `T <t@example.com>` was reachable from a remote branch,
and the local refs collectively reached both signed and unsigned unpublished commits. Those counts
and heads can change while agents remain active, so execution must derive them again after all
writers stop.

A new license commit at the current tip would leave every earlier unpublished snapshot under MIT.
Independently rebasing each branch would also duplicate shared commits and manufacture divergence.
The cutover therefore needs one transaction over the complete unpublished graph. Changing a parent,
tree, author, committer, or signature changes the commit object, so the transaction must recreate and
re-sign the affected objects rather than claim their old signatures still apply.

This is a high-consequence history operation. Existing worktrees include ordinary branches,
detached scratch worktrees, dirty state, Git pseudorefs, and prior history-rewrite backup refs. A
complete copied-tree rehearsal and an external recovery artifact are prerequisites, not optional
optimizations.

## Decision

1. The project adopts AGPL-3.0-only for the first dedicated license commit after the final published
   MIT boundary and for every project snapshot descended from that commit. Published ancestry
   through that boundary remains byte-for-byte unchanged and retains its MIT grant.

2. The canonical license bytes are SPDX license-list-data v3.27.0's
   `AGPL-3.0-only.txt`, including its final newline, with SHA-256
   `d8a6cc31abc16b6748c7a21f21611f5a1ec33f67d22ca23d7da1c19b95496bee`.
   The boundary commit changes `LICENSE` and the license badge and footer in the boundary version of
   `README.md`; dependency-license metadata is not project-license metadata and remains unchanged.

3. The complete unpublished commit graph is recreated through one old-to-new mapping. The mapping
   inserts exactly one license commit between the published boundary and its unpublished child or
   children, preserves parent order and merge topology after contracting that inserted node,
   preserves messages and timestamps, and preserves every tree byte except the intended per-snapshot
   `LICENSE` and `README.md` license transformations.

4. The same recreation corrects every unpublished author or committer occurrence of the repository's
   test-fixture identity to `Josua Müller <hypnotox@pm.me>` while leaving already-correct and genuine
   third-party identities unchanged. Every recreated commit and the inserted boundary commit receives
   a fresh valid SSH signature from the configured allowed signer; no old signature is copied onto a
   changed object.

5. Before any live ref moves, the migration is executed end to end in a copied repository containing
   the complete ref and object universe. The rehearsal records the selected mechanism and proves the
   published boundary, ref mapping, topology, trees, identities, signatures, worktree-state handling,
   and recovery procedure. A mechanism that cannot satisfy every assertion is rejected rather than
   repaired during the live transaction.

6. The live transaction starts only after every agent has stopped and every worktree, detached head,
   ref namespace, pseudoref, tag, index, and uncommitted path has been inventoried and either
   checkpointed or explicitly classified. It creates an external Git bundle and separate recovery
   material for any uncommitted state before changing refs, then updates all selected refs with
   expected-old object IDs so concurrent movement aborts the transaction.

7. Temporary backup refs remain until the rewritten repository passes its structural, signature,
   content, render, and project gates and the user accepts the result. They are then removed so no
   ordinary local ref presents the superseded unpublished MIT graph as active history; the external
   recovery bundle is retained until the user deliberately retires it.

## State changes

None.

## Consequences

Every unpublished project snapshot presented by an active branch will consistently carry
AGPL-3.0-only, while existing recipients keep the exact MIT rights already granted for published
versions. Commercial use remains possible only under the AGPL obligations applicable to the work.
The dedicated boundary commit makes the legal transition visible instead of hiding it inside an
unrelated implementation commit.

All unpublished commit IDs change. Local references, worktree registrations, effort metadata, and
any out-of-band commit citations must therefore be mapped or reconciled. Existing signatures cannot
survive the rewrite, but the replacement history is uniformly signed and the false fixture identity
is removed at the same unavoidable object-recreation boundary.

The copied-tree rehearsal, external bundle, expected-old ref updates, and delayed backup cleanup make
failure recoverable and concurrent movement detectable. They add preparation and validation cost,
but avoid resolving surprises against the only live copy of a thousand-commit graph.

AGPL obligations may reduce permissive adoption and do not prohibit compliant commercial use. The
project accepts that trade-off in exchange for copyleft coverage, including the network-use clause.
This record documents project intent and implementation boundaries; it is not legal advice to users
or contributors.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep MIT | Preserves maximum reuse but does not address proprietary derivative work. |
| Adopt Apache-2.0 | Adds explicit patent terms while still permitting proprietary commercial forks. |
| Adopt AGPL-3.0-or-later | Delegates acceptance of future AGPL versions instead of requiring an explicit relicensing decision. |
| Add AGPL only at the current local tip | Leaves earlier unpublished snapshots under MIT and makes the intended cutover boundary inaccurate. |
| Rebase each local branch independently | Risks conflicts and assigns different object IDs to commits that are currently shared, creating artificial divergence. |
| Rewrite live refs without a complete rehearsal | Makes mechanism, topology, signature, and recovery defects discoverable only after destructive movement. |

## Status history

- 2026-08-02: Proposed
