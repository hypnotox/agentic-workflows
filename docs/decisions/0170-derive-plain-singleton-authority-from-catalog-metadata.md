---
format: current-state-v2
status: Implemented
date: 2026-07-28
---
# ADR-0170: Derive plain singleton authority from catalog metadata

## Context

The `plain-singleton-via-renderkind` claim names an exhaustive list of mandatory plain singletons.
ADR-0169 corrected the list for the maintainable-code-design guide, but retrospective inspection
found that the older `plans-template` member was also absent. The backing test repeated a manually
maintained path subset, so both authority and proof could stay green while catalog membership grew.

Production already avoids this drift: `plainSingletons` is derived from catalog entries marked
mandatory that are neither the agents document nor generated output. Current-state authority and its
backing should describe and test the same projection instead of maintaining a second enumeration.

## Decision

1. Define the `plain-singleton-via-renderkind` claim by the catalog predicate `Mandatory &&
   !AgentsDoc && !Generated`, rather than naming current members.
2. Derive the backing test's expected output paths from `plainSingletons`, while retaining the
   independent unified-doc-model proof that the table exactly matches the catalog predicate.
3. Keep the existing local-sidecar suppression checks and common render-kind behavior unchanged.

## State changes

- update `rendering/singletons-and-payloads:plain-singleton-via-renderkind`

## Consequences

Adding another mandatory plain singleton automatically expands both the authoritative category and
the render-path backing test. A separate claim update is no longer needed merely because catalog
membership grows.

The claim no longer offers an inline inventory. The catalog and generated configuration reference
remain the authoritative inventory, while the predicate is more precise and resistant to drift.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Add only `plans-template` to the claim and test | A third catalog addition could repeat the same omission. |
| Keep the named claim list but derive only the test | The authority text could still become stale while the test passed. |
| Remove the exhaustive behavior from the claim | Common render-path coverage is load-bearing and should remain explicit. |

## Status history

- 2026-07-28: Proposed
- 2026-07-28: Implemented; content-sha256: fdf9596f7dca7460231c9aeffd1b5750731e309ebd00820c47748dde5739aeae
