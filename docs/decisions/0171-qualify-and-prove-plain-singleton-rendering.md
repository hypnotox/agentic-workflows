---
format: current-state-v2
status: Implemented
date: 2026-07-28
---
# ADR-0171: Qualify and prove plain singleton rendering

## Context

ADR-0170 replaced a stale singleton inventory with the catalog predicate that production uses.
Implementation review found two remaining mismatches. The claim stated unconditional rendering even
though every plain singleton retains the established `local: true` sidecar escape hatch, and the
backing tests established membership and path presence without directly proving catalog template
identity or the shared `renderKind` call path.

ADR-0170 is Implemented and frozen, so these corrections must move authority forward again. The
runtime behavior remains correct; only the claim's precision and proof substance need repair.

## Decision

1. Qualify the plain-singleton rendering claim with the existing `local: true` suppression exception.
2. Extend the backing test to assert each unsuppressed plain singleton's catalog-derived path,
   template identity, nonempty content, and neutral declarer identity.
3. Add a focused structural assertion that `RenderAll` ranges over `plainSingletons` and passes its
   template and output-path projections through the common `renderKind` call.
4. Preserve the catalog-derived membership proof and existing suppression behavior.

## State changes

- update `rendering/singletons-and-payloads:plain-singleton-via-renderkind`

## Consequences

The claim now describes both normal output and intentional suppression accurately. Its backing fails
if a singleton bypasses the shared renderer, loses its catalog template identity, or stops producing
its expected content.

The structural source assertion deliberately couples the test to this load-bearing call shape. A
future refactor may change that shape, but must update the authority and proof together.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Remove suppression from production | Local ownership is an established contract used by all plain singletons. |
| Narrow the claim to path presence only | That would discard the common-renderer and content-identity guarantees inherited from ADR-0148. |
| Trust code review to recognize the call path | This exact proof gap survived green deterministic checks and needs a mechanical assertion. |

## Status history

- 2026-07-28: Proposed
- 2026-07-28: Implemented; content-sha256: 005aeff26e3e18040c9af38d78b66898530afc8a23cd822f8e80a1a6bbc22e78; state-sequence: 75
