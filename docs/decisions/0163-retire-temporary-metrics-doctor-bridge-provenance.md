---
format: current-state-v2
status: Implemented
date: 2026-07-27
---
# ADR-0163: Retire temporary metrics doctor bridge provenance

## Context

ADR-0162 introduced effort-scoped metrics reports while the dashboard still required the
legacy top-level `awf doctor` command. Its first application batch therefore added the
focused `metrics-legacy-doctor-bridge` claim instead of applying a second update to
`metrics-command-contract`. Phase 3 removes the bridge with the dashboard transports.

A V2 ADR may name a claim only once in its State changes. Removing the temporary claim is
therefore a successor operation, not a second operation in ADR-0162.

## Decision

1. Remove `tooling/cli:metrics-legacy-doctor-bridge` in this successor ADR with the Phase 3
   dashboard-transport retirement.

## State changes

- remove `tooling/cli:metrics-legacy-doctor-bridge`

## Consequences

ADR-0162 records its scoped report contract and its temporary bridge addition exactly once.
This ADR records the later bridge removal without revising frozen ADR-0162 content or
reusing a claim operation.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Add a second bridge operation to ADR-0162 | V2 State changes permits each claim ID only once. |
| Leave the bridge claim active | The top-level command no longer exists. |

## Status history

- 2026-07-27: Proposed
- 2026-07-27: Implemented; content-sha256: cfbf6b910f91a203ed0d5f2040932e66bfed800a6e5120534f7e67ec5b9d2688; state-sequence: 60
