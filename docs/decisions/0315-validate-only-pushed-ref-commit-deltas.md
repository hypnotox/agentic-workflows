---
format: current-state-v4
slug: validate-only-pushed-ref-commit-deltas
status: Proposed
date: 2026-08-28
---
# ADR-0315: Validate only pushed ref commit deltas

## Context

ADR-0228 made the generated pre-push payload pass each complete local target to the common commit-policy verifier. The verifier consequently reevaluates every target ancestor after `grandfatheredThrough`, including commits already accepted by the destination remote. This repository exposed the mismatch when a push of conforming descendants was refused because two GitHub-created merge commits already reachable from remote `main` had GitHub committer identities and signatures outside the local allowlist. The exact range from the advertised remote `main` tip to the local tip conformed.

The pre-push protocol supplies both old and new object IDs for an existing destination ref. A newly created ref has no old destination tip, while awf already requires an `integrationBranch` and this repository sets it to `main`. Commit policy must continue to fail closed when the evidence needed to identify the pushed delta is unavailable. Reference-transaction checking remains a distinct earlier boundary over commits introduced to local branches.

## Decision

1. `decision: pushed-ref-delta-enforcement` The generated pre-push payload checks only the union of commits introduced by the pushed ref updates. For an existing ref, its advertised remote tip is the base. For a new commit-bearing branch or tag, the freshly resolved destination integration-branch tip is the base.

2. `decision: pushed-ref-delta-evidence` Deletions contribute no commits. Multiple updates are unioned and deduplicated, and commit-bearing tag targets are recursively peeled before selection. Missing, malformed, unresolvable, or contradictory required remote or object evidence refuses the push rather than widening, skipping, or consulting stale remote-tracking refs.

## State changes

- update `rendering/singletons-and-payloads:commit-policy-hook-payloads`

## Consequences

A conforming descendant push no longer re-rejects history already behind the destination ref. A new branch or tag checks its divergence from the destination integration branch, so the policy still evaluates work introduced away from the repository's integration line. Force updates use the same old-tip delta and cannot make a newly reachable commit escape evaluation.

New-ref pushes require one fresh destination integration-tip lookup and can add authentication latency. Failure to obtain exact evidence blocks the push. A client hook still cannot make its remote observation atomic with the receive transaction, so remote protection remains the final publication boundary.

The explicit commit-policy preview command and reference-transaction hook retain their existing selection semantics. The configured grandfather baseline remains unchanged; it continues to define tolerated pre-policy ancestry rather than being advanced to conceal already-published exceptions.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Advance `grandfatheredThrough` past the GitHub merges | Weakens the repository-wide policy boundary to mask a pre-push selection error. |
| Keep checking every complete pushed target | Re-rejects already-accepted remote history and can permanently block conforming descendants. |
| Exclude everything reachable from any advertised remote ref | Broader and more expensive than the accepted per-ref delta, and obscures the integration-branch boundary for new refs. |
| Use local remote-tracking refs as bases | They may be stale and are not authority for the destination state. |

## Status history

- 2026-08-28: Proposed
