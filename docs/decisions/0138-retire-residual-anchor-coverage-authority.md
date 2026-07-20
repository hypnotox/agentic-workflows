---
format: current-state-v1
status: Implemented
date: 2026-07-20
---
# ADR-0138: Retire residual anchor coverage authority

## Context

The current-state cutover removed supersession and anchor-coverage authority from normal project
checks, but final implementation review found that `internal/adr/coverage.go` and its corpus facet
still survived for the historical schema-12 supersession-key migration. Two migrated current-state
claims consequently described that retired model as live behavior.

The migration remains necessary for adopters upgrading from an older schema. It needs only a local,
one-shot list of each target ADR's numbered Decision items and declared invariant slugs; it does not
need a permanent corpus coverage model or an exported anchor API.

## Decision

1. Delete the anchor-coverage model, its corpus facet, and its exported anchor query.
2. Keep the historical supersession-key migration operational by deriving its temporary token list
   locally in the existing order: parsed Decision items, any synthetic bookkeeping item pending for a
   target that is itself a supersession carrier, then declared invariant slugs. Preserve the exact
   numbered-item/slug token grammar and emitted migration output.
3. Retire `corpus-model-not-rebuilt` because migration-local enumeration intentionally replaces the
   centralized construction it requires.
4. Retire `supersession-model-single-source` because the supersession coverage model no longer exists;
   its residual identifier ban has no independent authority once that model is deleted.
5. Update the three surviving corpus-boundary claims to remove references to deleted renderers and
   supersession consumers while retaining their enforced ParseDir, Sections, and raw-byte seams.
6. Keep the migration's behavior and regression fixtures unchanged; this decision removes permanent
   authority, not the ability to upgrade an old tree.

## State changes

- remove `adr-system/adr-lifecycle:corpus-model-not-rebuilt`
- remove `adr-system/adr-lifecycle:supersession-model-single-source`
- update `adr-system/adr-lifecycle:corpus-owns-field-reads`
- update `adr-system/adr-lifecycle:corpus-parsed-once`
- update `adr-system/adr-lifecycle:corpus-raw-access-enumerated`

## Consequences

The permanent ADR package no longer constructs anchor-derived authority, while schema-12 upgrades
still emit the same legacy retirement tokens. The migration owns a deliberately local compatibility
calculation that has no runtime consumer after that migration completes.

The removed claim ids cannot be reused. Tests and proof markers dedicated to the deleted model leave
with the claims; migration behavior remains covered by its migration-specific tests.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Delete the schema-12 migration | Existing pre-schema-12 adopters would lose a supported upgrade path. |
| Keep the corpus coverage facet as migration support | It leaves retired authority in a central permanent model and keeps false current-state claims alive. |
| Move the whole legacy model into another package | The migration needs only a small deterministic token list, not the old derived model. |

## Status history

- 2026-07-20: Proposed
- 2026-07-20: Implemented; content-sha256: 862c9f5c08f168065f0a4045ee2490972b688f82ea98c0576d743bb0efb2194a; state-sequence: 2
