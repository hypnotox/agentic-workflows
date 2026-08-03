---
format: current-state-v3
slug: agpl-3-0-only-cutover-at-the-unpublished-history-boundary
status: Implemented
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
optimizations. Relicensing also requires authority over every affected contribution: an author label
is evidence to investigate, not proof of copyright ownership, and notices required by retained
third-party work survive a project-license change.

## Decision

1. The project adopts AGPL-3.0-only for the first dedicated license commit after the final published
   MIT boundary and for every project snapshot descended from that commit. Execution freezes the
   advertised heads, tags, and releases of every configured remote, records their object IDs in the
   transaction manifest, and derives the common final published boundary from that complete set.
   Every commit selected for recreation must descend from that boundary; a remote movement, unrelated
   history, or selected commit outside the boundary aborts for explicit user disposition. Published
   ancestry through the boundary remains byte-for-byte unchanged and retains its MIT grant.

2. The canonical license bytes are fetched, never transcribed, from SPDX license-list-data commit
   `d46e94e2c78ceede1cfc63cfa0396472d2798d4c` at
   `https://raw.githubusercontent.com/spdx/license-list-data/d46e94e2c78ceede1cfc63cfa0396472d2798d4c/text/AGPL-3.0-only.txt`.
   The fetched file must be exactly 34,020 bytes, end with one newline, and have SHA-256
   `d8a6cc31abc16b6748c7a21f21611f5a1ec33f67d22ca23d7da1c19b95496bee`;
   any mismatch aborts rather than inviting a manual correction. The boundary commit changes
   `LICENSE` and the license badge and footer in the boundary version of `README.md`;
   dependency-license metadata is not project-license metadata and remains unchanged.

3. Before rehearsal, the contributor-rights audit classifies every author, committer, imported file,
   and retained notice in the selected graph. The cutover proceeds only if the project has relicensing
   authority for every affected contribution or has obtained explicit permission. Required MIT or
   other third-party notices remain with their contributions. Any unresolved ownership or notice case
   stops for a user decision rather than being inferred from Git metadata.

4. The complete unpublished commit graph is recreated through one old-to-new mapping. The mapping
   inserts exactly one license commit between the published boundary and its unpublished child or
   children, preserves parent order and merge topology after contracting that inserted node,
   preserves messages and timestamps, and preserves every tree byte except the intended per-snapshot
   `LICENSE` and `README.md` license transformations.

5. The same recreation corrects every unpublished author or committer occurrence of the repository's
   test-fixture identity to `Josua Müller <hypnotox@pm.me>` while leaving already-correct and audited
   genuine third-party identities unchanged. Every recreated commit and the inserted boundary commit
   receives a fresh valid SSH signature from the configured allowed signer; no old signature is
   copied onto a changed object.

6. Before any live ref moves, the migration is executed end to end in a copied repository containing
   the complete ref and object universe. The rehearsal records the selected mechanism and proves the
   published boundary, ref mapping, topology, trees, identities, signatures, worktree-state handling,
   and recovery procedure. A mechanism that cannot satisfy every assertion is rejected rather than
   repaired during the live transaction.

7. The live transaction starts only after every agent has stopped and every local branch, linked or
   detached worktree HEAD, lightweight or annotated tag, custom ref namespace, remote-tracking ref,
   `refs/original/*` backup, pseudoref, index, and uncommitted path has one manifest disposition.
   Active branches, active worktree heads, and tags that present selected unpublished commits are
   recreated through the one map; annotated tags are recreated and re-signed when their target moves.
   Remote-tracking refs remain frozen evidence of advertised remote state. Recovery-only refs and
   pseudorefs are archived externally and either updated when they remain operationally meaningful or
   removed from active presentation after acceptance. No class is silently selected or ignored.

8. The transaction creates an external Git bundle and separate patches or archives for every retained
   uncommitted state before changing refs. Ref updates use expected-old object IDs so concurrent
   movement aborts. Repository-local and worktree-local `user.name` and `user.email` overrides are
   removed or corrected, and every worktree must resolve the approved effective author and committer
   identity before work resumes.

9. Temporary backup refs remain until the rewritten repository passes its structural, signature,
   content, render, and project gates and the user accepts the result. They are then removed so no
   ordinary local ref presents the superseded unpublished MIT graph as active history; the external
   recovery bundle is retained until the user deliberately retires it.

10. The historical cutover and the current-state application form one ordered activation with a
    dedicated-boundary exception. The rewrite transaction first inserts the Decision 2 boundary
    commit, whose tree changes only `LICENSE` and the README license badge and footer, so it cannot
    also carry later current-state machinery. After that rewritten history is validated, one
    application transaction authors `tooling/project-license:project-license-agpl` as a
    `Backing: test` invariant together with `TestProjectLicenseAGPL` and its proof marker. The test
    pins the exact `LICENSE` hash and newline, README badge and footer, release-package license
    inclusion, and absence of obsolete project MIT references while excluding dependency metadata.
    That application transaction carries every remaining project-license artifact, the matching
    Applied and Implemented history, and rendered authored `.awf/` sources so
    `docs/decisions/INDEX.md` and all generated documentation are current before acceptance. Neither
    transaction may be accepted independently: failure before the application transaction completes
    retains the recovery posture and blocks acceptance.

## State changes

- add `tooling/project-license:project-license-agpl`

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

The copied-tree rehearsal, contributor-rights audit, external bundle, expected-old ref updates, and
delayed backup cleanup make failure recoverable and concurrent movement detectable. They add
preparation and validation cost, but avoid resolving surprises against the only live copy of a
thousand-commit graph or silently removing notices the project must retain.

The dedicated-boundary exception creates a controlled interim state after rewritten history is
presented and before the application transaction lands. During that interval the trees carry the
canonical AGPL bytes, but the ordinary project gates do not yet prove the complete project-license
invariant and the ADR is not terminal. The repository therefore remains unaccepted: temporary backup
refs and external recovery artifacts stay retained, publication stays blocked, and any application
failure returns to reconciliation rather than permitting cleanup or acceptance.

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
- 2026-08-03: Accepted; content-sha256: 135fa5e0798b6f91fcb067684df525cff038a606c2641c4560601839ee94d517
- 2026-08-03: Implemented; content-sha256: 135fa5e0798b6f91fcb067684df525cff038a606c2641c4560601839ee94d517
