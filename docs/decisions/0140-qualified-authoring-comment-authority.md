---
format: current-state-v1
status: Proposed
date: 2026-07-21
---
# ADR-0140: Qualified Authoring Comment Authority


## Context

The current-state cutover replaced the unqualified invariant scanner with qualified topic claims and
`currentState.sources`. Authoring comments remain the source-only carrier stripped before rendering,
but the active `rendering/catalog-and-targets:no-single-marker-init-descriptor` claim still named
retired `invariants.sources`. A production comment and an active rendering fixture repeated the stale
term, while the test that claimed to exercise a real embedded directive did not point to one.

This is active authority, not frozen historical prose. Correcting its canonical claim text therefore
requires an explicit update operation and matching revision provenance rather than a documentation-only
edit.

## Decision

1. Authoring comments used for current-state marker configuration carry qualified `state:`,
   `invariant:`, or `touches-state:` payloads and are discovered only through
   `currentState.sources`. No active claim, production comment, template fixture, or agent guidance
   describes `invariants.sources` or `touches-invariant:` as current behavior.

2. The embedded-template stripping regression uses a real qualified `touches-state:` authoring
   comment in an awf-owned template source and asserts that the directive is present at ingestion but
   absent after rendering. A synthetic fixture alone does not prove the embedded-template seam.

3. The implementation updates the active claim with `Revised-by: ADR-0140`, changes the production
   comment and active fixtures in the same transaction, regenerates managed outputs, and runs the
   invariant, drift, and full project gates.

## State changes

- update `rendering/catalog-and-targets:no-single-marker-init-descriptor`

## Consequences

The active authority, implementation commentary, fixtures, and rendered documentation use one
qualified marker vocabulary. The test now fails if a real embedded directive stops being stripped,
rather than remaining green through a synthetic-only path.

The ADR adds a small revision transaction late in the topic-authority hardening plan. Historical ADRs,
plans, changelog entries, and explicit compatibility fixtures retain retired terms where they describe
past encodings.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Edit the claim as documentation | Canonical claim changes require an ADR update operation and revision provenance. |
| Delete every retired literal repository-wide | Frozen history and compatibility fixtures must remain truthful. |
| Keep only the synthetic strip fixture | It does not prove that an embedded template actually carries and strips a directive. |

## Status history

- 2026-07-21: Proposed
