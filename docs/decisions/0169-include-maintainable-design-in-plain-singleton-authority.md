---
format: current-state-v2
status: Implemented
date: 2026-07-28
---
# ADR-0169: Include maintainable design in plain singleton authority

## Context

ADR-0168 added the mandatory `maintainable-code-design` document through the catalog-derived plain
singleton path. The existing `plain-singleton-via-renderkind` current-state claim exhaustively names
the mandatory documents that use that path, but the ADR-0168 transaction did not update the claim or
its backing test. The implementation works, while the retained authority and proof now describe an
incomplete set.

Implemented ADRs are frozen historical records, so the omission cannot be repaired by changing
ADR-0168. The correction must move the affected current-state claim forward and preserve the earlier
decision history.

## Decision

1. Update `plain-singleton-via-renderkind` to include `maintainable-code-design` among the mandatory
   plain singletons rendered through the shared table and common render-kind path.
2. Extend the claim's backing test with the guide's fixed output path so future catalog or rendering
   changes cannot silently restore the mismatch.
3. Keep the correction limited to authority and proof. The guide's behavior, catalog contract, and
   ADR-0168 design remain unchanged.

## State changes

- update `rendering/singletons-and-payloads:plain-singleton-via-renderkind`

## Consequences

The exhaustive current-state statement and its deterministic backing once again match rendered
behavior. The additional test expectation slightly increases the maintenance cost of changing the
mandatory singleton set, which is intentional for an exhaustive invariant.

ADR-0168 remains unchanged as historical rationale. Readers must follow this later revision for the
current singleton membership.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Edit ADR-0168 to add the omitted operation | ADR-0168 is Implemented and its meaning is frozen. |
| Generalize the claim so it no longer names singleton members | That would weaken the exhaustive contract and reduce the backing test's ability to catch omissions. |
| Leave the authority stale because runtime behavior is correct | Current-state claims are authoritative and must describe reality, not merely approximate it. |

## Status history

- 2026-07-28: Proposed
- 2026-07-28: Implemented; content-sha256: fe69ba140b6c02f691ab92ee23491eb9c5d0aa5e338b0db94a6a73db0423744e
