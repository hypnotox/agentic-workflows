---
format: current-state-v4
slug: validate-only-local-reference-transaction-commit-deltas
status: Implementing
date: 2026-08-29
---
# ADR-validate-only-local-reference-transaction-commit-deltas: Validate only local reference-transaction commit deltas

## Context

ADR-0228 made the generated reference-transaction payload check every policy-era commit reachable from a new local branch. That selection is broader than the local change when Git creates another branch name at inherited history. In this repository, `git worktree add -b` from `main` consequently revalidated two already-hosted GitHub merge commits and aborted before creating the worktree.

ADR-0315 corrected the analogous pre-push mismatch by using the destination integration branch as the base for a new pushed ref, but deliberately retained reference-transaction selection. The local hook remains useful because it sees the final signed commit object during Git's prepared phase and can reject a nonconforming commit before a branch moves. It is an early accidental-error boundary rather than remote authority.

A prepared reference transaction supplies an old and new object ID for each ref. Existing local branches therefore have an exact old tip, while a new branch has a zero old object ID. The project already declares an integration branch. Local checking can use its pre-transaction local tip as the inherited-history base without acquiring a remote dependency or treating proposed updates as evidence.

## Decision

1. `decision: local-reference-delta-enforcement` The generated reference-transaction payload checks only commits introduced by prepared local branch updates. An existing branch uses its old tip as the base. A new branch uses the configured integration branch's local pre-transaction tip as the base.

2. `decision: local-integration-evidence` The integration base resolves only from the exact local integration branch. The hook does not contact a remote, consult a remote-tracking ref, fall back to another ref, or substitute a proposed update from the same transaction. Missing, malformed, unresolvable, or non-commit required evidence refuses the complete transaction rather than widening or skipping selection.

3. `decision: local-reference-delta-safety` Deletions and backward-only updates contribute no commits. Multiple updates remain buffered and deduplicated before the common commit-policy verifier runs, and policy refusal occurs before refs move without rewriting history. The payload preserves invoking-worktree resolution and publication-safe rendering when integration-branch data is unset.

## State changes

- update `rendering/singletons-and-payloads:commit-policy-hook-payloads`

## Consequences

Creating a branch or managed worktree from the local integration line no longer revalidates inherited integration history. A new branch that diverges from that line still checks its introduced commits, while existing branch updates retain their exact old-to-new ranges.

New-branch transactions now depend on a valid local integration branch. A missing or invalid local base blocks the transaction even when the same transaction proposes creating or updating that branch. This stable pre-transaction evidence avoids record-order dependence and network access.

Pre-push retains its destination-derived evidence and remote publication role. The explicit commit-policy preview, policy baseline, configuration schema, and common verifier remain unchanged.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Remove the reference-transaction check and rely on pre-push | Allows nonconforming local commits to accumulate and loses rejection before a branch moves. |
| Keep checking every post-baseline commit for a new branch | Revalidates inherited history and prevents ordinary branch and worktree creation when accepted integration history contains policy exceptions. |
| Subtract commits reachable from every existing local ref | Makes arbitrary ref topology the policy base instead of using the project's declared integration line. |
| Query the remote integration branch | Adds network and remote-availability dependencies to a local pre-ref-movement operation. |
| Use a proposed integration update from the same transaction | Makes evidence depend on transaction contents instead of stable pre-transaction local state. |

## Status history

- 2026-08-29: Proposed
- 2026-08-29: Implementing; content-sha256: fb7b34279af5a8290bf2c473ce6f781ef15d965d0a2f17ae1ef5b275d25db823
- 2026-08-29: Applied; operations: update `rendering/singletons-and-payloads:commit-policy-hook-payloads`
