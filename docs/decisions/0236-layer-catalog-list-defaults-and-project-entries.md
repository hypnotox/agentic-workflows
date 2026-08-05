---
format: current-state-v4
slug: layer-catalog-list-defaults-and-project-entries
status: Implemented
date: 2026-08-05
---
# ADR-0236: Layer catalog list defaults and project entries


## Context

Catalog defaults and project sidecar data currently meet through a shallow top-level overlay. A
present sidecar key replaces its catalog default wholesale, including when the sidecar value is null
or an empty list. This gives adopters an off-switch, but it also makes ordinary customization discard
standard guidance silently. This repository already replaces five catalog-backed lists merely to add
project-specific test surfaces, ADR triggers, or review focuses, so improvements to those catalog
defaults never reach its rendered workflows.

The underlying data has two different ownership layers. Catalog list entries are portable standard
content that awf should improve centrally; sidecar list entries are project-specific additions that
should remain in the adopter tree. Treating both as one replaceable value conflates customization
with rejection of the standard. Null makes that conflation worse because it can mean only
suppression, yet it occupies the custom-data channel as though it were an authored list.

The effective sidecar already feeds both rendering and the artifact config hash, so layering belongs
at that existing boundary rather than in individual templates. The config tree needs an explicit
control because an empty custom list cannot distinguish "use only the defaults" from "render no
entries." Existing adopters also need a schema migration: changing merge semantics without rewriting
old replacements would prepend defaults and alter their established output during upgrade.

Some artifacts intentionally use specialized layers with different keys and domain semantics. The
glossary, for example, merges catalog `standardTerms` with project `terms` by case-insensitive term
identity. Such transforms are not evidence for a universal list deduplication rule; the generic
contract must stay limited to a catalog list and project list sharing one data key.

## Decision

1. `decision: list-data-layers` For a data key whose catalog default is a list, the effective value is the catalog list followed by the project sidecar list in authored order. An absent or empty project list keeps the complete catalog list. List composition is shallow and performs no generic deduplication, identity matching, or record merge. Non-list catalog data retains top-level sidecar replacement semantics.
2. `decision: explicit-default-suppression` Sidecars expose `dataDefaults` as a per-data-key boolean map. An absent key or `true` keeps that catalog list default; `false` suppresses it, making the effective value the project list alone, or an empty list when no project list is authored. Every map entry, whether `true` or `false`, must name a declared catalog-backed list key for that artifact; an unknown or non-list key is rejected with the sidecar and key named. `dataDefaults.<key>: false` is the sole generic way to reject that default. The control remains part of the effective sidecar and therefore of the artifact's existing config-hash and drift boundary.
3. `decision: null-list-refusal` A project value for a catalog-backed list key must be an actual list when present; null is rejected with an error naming the sidecar and key. An empty list remains valid and means "add no project entries," not suppression. This preserves an explicit distinction between absence, empty customization, and rejection of the standard.
4. `decision: fixed-snapshot-migration` The config-schema migration classifies replacements against the exact catalog list-key snapshot shipped with that migration, never against a future catalog population. Before mutation it reads every affected sidecar and refuses a non-null, non-list replacement with the sidecar and key named. For every valid existing replacement, it records `dataDefaults.<key>: false` so the established replacement output remains unchanged after the semantic cutover; a null replacement becomes suppression without a null custom value. After complete preflight, each changed sidecar is replaced atomically. The rewrite is idempotent and safely retryable after an I/O failure, preserves unrelated sidecar content, and does not promise transaction-wide atomicity across files or preemptively suppress a list default introduced by a later binary.
5. `decision: specialized-list-transforms` Differently keyed or identity-aware list layers retain their owning transforms and contracts. In particular, glossary `standardTerms` and project `terms` continue their case-insensitive term override behavior and are not exposed as a generic `dataDefaults` key. The generated configuration reference distinguishes catalog defaults, project entries, suppression, and specialized list behavior rather than describing every sidecar key as a whole-value override.

## State changes

- update `rendering/project-output-plan:sidecar-key-overrides-default`
- add `rendering/project-output-plan:catalog-list-data-layering`
- add `config/configuration:sidecar-data-defaults-control`
- add `config/migrations-and-locks:list-replacement-fixed-snapshot`

## Consequences

Projects can add local guidance without freezing or shadowing the standard guidance awf ships, while
an intentional rejection remains visible and reviewable in configuration. Catalog improvements flow
to customized adopters by default, and the existing render/hash seam keeps both list layers and the
suppression choice drift-visible.

The schema grows by one narrowly typed sidecar field and upgrade must inspect artifact identity plus
the migration's fixed catalog snapshot. The strict sidecar decoder and project validation own the
new field, the effective-data merge owns its render meaning, and the existing config-hash
serialization carries both the field and the merged result without changing the lock-manifest shape.
A new schema generation gates render and check until upgrade rewrites affected sidecars and stamps
the lock; afterward a binary too old for that generation refuses rather than ignoring the field.
Existing replacements gain explicit suppression entries, which adds configuration bytes but
preserves their rendered output. After migration, adopters may remove a suppression entry
deliberately to adopt the layered result.

Generic concatenation can produce semantically repetitive entries because awf does not infer record
identity. Catalog authors and projects remain responsible for content quality, while artifacts that
need identity-aware merging keep explicit transforms. Null list values that previously acted as an
implicit off-switch become errors outside the migration path; this is intentional because the
replacement is an unambiguous suppression control.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep whole-key replacement and document how to copy defaults | Every customized project would own a stale snapshot of standard content, preserving the friction and silent shadowing. |
| Always append defaults with no suppression | Projects could not reject irrelevant or conflicting standard entries. |
| Use null or an empty list as the suppression signal | Null conflates invalid custom content with policy, while an empty list cannot also mean the natural no-custom-entries case. |
| Add per-artifact merge strategies or generic record identities | The current need is uniform ordered composition; generalized strategy configuration would add speculative policy and still could not infer semantic identity reliably. |
| Route every list through the glossary merge | Glossary's separate keys and case-insensitive term identity are domain-specific and would impose the wrong semantics on ordered workflow lists. |

## Status history

- 2026-08-05: Proposed
- 2026-08-05: Accepted; content-sha256: cafef2a3aec9a4c0b338dbea2f4cc4c97485c7a9b3db5b57cf64ca5f01f36b14
- 2026-08-05: Implementing; content-sha256: cafef2a3aec9a4c0b338dbea2f4cc4c97485c7a9b3db5b57cf64ca5f01f36b14
- 2026-08-05: Applied; operations: update `rendering/project-output-plan:sidecar-key-overrides-default`, add `rendering/project-output-plan:catalog-list-data-layering`, add `config/configuration:sidecar-data-defaults-control`, add `config/migrations-and-locks:list-replacement-fixed-snapshot`
- 2026-08-05: Reapplied; operations: add `config/migrations-and-locks:list-replacement-fixed-snapshot`
- 2026-08-06: Implemented; content-sha256: cafef2a3aec9a4c0b338dbea2f4cc4c97485c7a9b3db5b57cf64ca5f01f36b14
