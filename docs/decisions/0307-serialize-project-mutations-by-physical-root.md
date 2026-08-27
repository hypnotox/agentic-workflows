---
format: current-state-v4
slug: serialize-project-mutations-by-physical-root
status: Implemented
date: 2026-08-26
---
# ADR-0307: Serialize Project Mutations by Physical Root


## Context

Awf already confines and atomically replaces individual published files, and Publisher replaces the render lock after its output mutations. Those per-file guarantees do not serialize a complete project mutation. Domain and local-document operations read configuration, mutate it through direct host filesystem calls, and synchronize afterward. Concurrent operations can derive from the same authority and lose one update; a failure after configuration replacement can leave an unreported or non-convergent partial result. Direct path access can also follow a symlink outside the selected checkout.

Linked worktrees have independent tracked configuration, authored sources, generated outputs, and render locks. Git integration is the authority that reconciles those branches, so serializing every checkout under one repository-wide lock would suppress valid parallel work. Resident effort, worktree, and archive roots are different: they resolve to the primary checkout and are physically shared by linked worktrees.

The existing architecture separates neutral filesystem mechanisms, focused command operations, and Publisher policy. ADR scaffolding already uses a canonical-identity, persistent-file advisory lock for cross-process allocation. Mutation safety must preserve the dependency direction and single-home rule rather than creating either an application owner that absorbs domain, local-document, and publication policy or a second advisory-lock mechanism.

## Decision

1. `decision: lease-by-physical-root` Serialize project mutation by the physical root it can change. Tracked-tree mutation uses a checkout-local lease. Mutation of primary-resident state uses a separate lease for that shared root. An operation that changes both acquires both in deterministic order.
2. `decision: lease-covers-authority-to-outcome` Acquire every applicable lease before loading mutable authority or preparing a mutation plan, and hold it until the operation has published its complete or partial outcome. A lease around only the final write is insufficient because preparation from stale authority is unsafe.
3. `decision: preserve-focused-policy-owners` Keep confinement and one shared advisory-lock mechanism in the neutral filesystem layer, extracting or reusing the existing canonical identity, persistent lock-file, acquisition, and process-release mechanics. Focused operation packages retain their use-case and recovery policy, ADR allocation configures the shared mechanism for its directory identity, and Publisher retains output planning and publication policy. No new cross-operation application owner absorbs those responsibilities.
4. `decision: confined-transaction-primitives` Within a lease, observe and mutate project paths through the root-confined filesystem boundary. Check the loaded configuration identity before replacement, atomically replace existing authority, and create new authored files exclusively so a concurrent or pre-existing file is never clobbered.
5. `decision: explicit-partial-outcomes` A failed mutating command either preserves its pre-command tree or returns a typed outcome that completely identifies committed effects and the retry or recovery action. Operation and command boundaries must not discard Publisher or focused-operation partial results.

## State changes

- add `tooling/filesystem-access:root-scoped-project-mutation-leases`
- update `tooling/cli:cli-creation-and-inventory`
- update `tooling/cli:domain-lifecycle-commands`
- update `rendering/sync-and-drift:sync-mutations-root-confined`

## Consequences

Mutations within one checkout cannot silently lose concurrent configuration updates or publish an output lock from an interleaved plan. Linked worktrees retain independent tracked-tree concurrency, while operations that reach shared resident state serialize only that physical root. The added lease claim is an invariant with test backing for canonical root identity, deterministic dual-root acquisition, cross-process serialization, and process-exit release.

Operation setup moves inside the lease, so callers cannot prepare a Publisher plan and acquire safety afterward. Focused result and presentation models become richer where rollback cannot be guaranteed. The lease does not make a multi-file operation crash-atomic and does not replace the upgrade journal; atomic replacement, exclusive publication, rollback where safe, and explicit partial outcomes remain necessary.

A process can block behind another mutation. The lock mechanism must release ownership when its process exits without splitting waiters across replaceable lock-file identities.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| One repository-common lease | It would serialize independent tracked changes in linked worktrees even though Git already owns their reconciliation. |
| Checkout-local leases only | They would not protect primary-resident paths shared by every linked worktree. |
| Atomic file replacement without a lease | It prevents torn individual files but not stale planning, lost updates, or interleaved multi-file publication. |
| One project-mutation application package | It would collapse focused operation and Publisher policy into a new owner and reverse the established dependency structure. |

## Status history

- 2026-08-26: Proposed
- 2026-08-26: Accepted; content-sha256: e34f096e2b16a723e58062a02f443817d1bf80af66c9f70a338348a16cbe7faa
- 2026-08-26: Implementing; content-sha256: e34f096e2b16a723e58062a02f443817d1bf80af66c9f70a338348a16cbe7faa
- 2026-08-26: Applied; operations: add `tooling/filesystem-access:root-scoped-project-mutation-leases`, update `rendering/sync-and-drift:sync-mutations-root-confined`
- 2026-08-26: Applied; operations: update `tooling/cli:cli-creation-and-inventory`, update `tooling/cli:domain-lifecycle-commands`
- 2026-08-27: Implemented; content-sha256: e34f096e2b16a723e58062a02f443817d1bf80af66c9f70a338348a16cbe7faa
