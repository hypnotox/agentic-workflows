---
status: Implemented
date: 2026-07-04
supersedes: []
superseded_by: ""
tags: [commit-scopes, config-serialization]
related: [8, 17, 36, 39, 49, 51, 55]
domains: [config, rendering, tooling]
---
# ADR-0056: Structured commit-scope config with meanings

## Context

ADR-0051 made `audit.allowedScopes` (a `[]string` of scope names) the single storage for commit
scopes; ADR-0055 expanded awf's own list to eight domain-aligned scopes and derived the scope
*name list* into the agent guide and reviewing skills through the `commitScopes` render key. But a
scope also has a *meaning*: `adr-system` = "the ADR machinery code", `rendering` = "the render
engine and templates". Those meanings are documentation, and today they are hand-written in prose
(the `docs/workflow.md` commit-discipline taxonomy table), physically separate from the names they
annotate. That is a drift surface: editing `audit.allowedScopes` does not touch the table, so the
name list and the meaning table can disagree.

Removing that drift means the meanings must live *with* the names: the config becomes the single
source for name **and** meaning. This ADR is the data-shape half of that: it lets a scope entry
carry its meaning. A companion ADR (the sandboxed part-placeholder mechanism) is what lets a raw
convention part such as the workflow.md taxonomy *consume* that structured data with zero drift; it
depends on this ADR's `meaning` field and is recorded separately.

The current shape is a plain string slice in three places:
`AuditConfig.AllowedScopes []string` (`internal/config/config.go`), `audit.Settings.AllowedScopes
[]string` (`internal/audit/settings.go`), and the init/scaffold collector
`SkeletonAudit.AllowedScopes []string` (`internal/config/edit.go`). `audit.Resolve` copies the
config slice into settings, and `awf commit-gate`/`awf audit` match a commit's scope against it
(ADR-0036/ADR-0017). Config is strict-parsed (`KnownFields`), so a mapping element cannot simply be
tolerated by a `[]string` target; it errors. The `domains` field was additive and needed no
migration, but it was a genuinely new optional field; widening an *existing* field's element type
from scalar to scalar-or-mapping is a parse change, not a new field, so it needs a custom decoder
rather than "just an additive field".

## Decision

1. **A scope entry is a name with an optional meaning.** `audit.allowedScopes` accepts, per
   element, **either** a bare string (`- adr`) **or** a mapping (`- {name: adr, meaning: ADR
   markdown documents}`). The two forms are interchangeable; a bare string is exactly a mapping
   with an empty `meaning`.

2. **A named element type with a custom `UnmarshalYAML` carries both forms.** A `ScopeSpec` type
   (fields `Name`, `Meaning`) decodes a YAML scalar node into `{Name: <scalar>, Meaning: ""}` and a
   mapping node into its `name`/`meaning` keys (mapping decode stays strict: an unknown key in a
   scope mapping errors). `AuditConfig.AllowedScopes` becomes `[]ScopeSpec`. This is the only way to
   accept the mixed list under strict parsing; widening `[]string` alone cannot.

3. **The config and settings types move to the structured form; the scaffold stays scalar.**
   `AuditConfig.AllowedScopes` and `audit.Settings.AllowedScopes` both become `[]ScopeSpec`, and
   `audit.Resolve` maps `[]ScopeSpec` through so settings carry the resolved scopes with their
   meanings. The init/scaffold collector `SkeletonAudit.AllowedScopes` **stays `[]string`** and
   continues to **emit bare strings**: `awf init` writes names only, and meanings are added by
   hand or by an adopter later; scaffolding names-only keeps init output minimal and the round-trip
   stable (a bare-string list re-parses cleanly through the `ScopeSpec` decoder).

4. **The commit-gate reads `name`; `meaning` is inert metadata.** `awf commit-gate`/`awf audit`
   match a commit's scope against the set of `Name`s exactly as before; gate behaviour is
   unchanged. `Meaning` is consumed only by documentation-rendering (the companion ADR), never by
   validation. `Meaning` is empty for a bare-string element or a mapping that omits it.

5. **No schema migration, no version bump.** The change is a parse-widening that leaves every
   existing string-list config valid, so the schema generation stays at its current value and
   `minVersionBySchema` is untouched (ADR-0039/ADR-0049). Backward compatibility rests on the
   `ScopeSpec` decoder accepting the scalar form, not on the field being new.

This ADR is the data-shape decision only. awf's own `.awf/config.yaml` adopting the mapping form
with meanings for its eight scopes, and any consumer that renders those meanings, land through the
companion ADR's plan, not here.

## Invariants

- `invariant: scope-config-dual-form`: `audit.allowedScopes` decodes both a bare-string element and a
  `{name, meaning}` mapping element in the same list; `audit.Resolve` yields the `Name` for gating
  regardless of form, and `Meaning` is empty for the bare-string form (and for a mapping omitting
  it). Backed by a config/audit parse test asserting a mixed list round-trips and that a
  bare-string element resolves to an empty meaning.

## Consequences

- One source of truth for a scope's name **and** meaning: `audit.allowedScopes`. The companion ADR
  can build config-derived, meaning-bearing values (a taxonomy table) with no hand-written mirror.
- The commit-gate is untouched (it still matches on the scope name), so no adopter's commit
  workflow changes, and a project that never sets `meaning` behaves exactly as before.
- Adopters may stay entirely on the bare-string form; meanings are opt-in. `awf init` keeps writing
  bare strings.
- One real cross-ADR coupling: the meaning-bearing renderer in the companion ADR hard-depends on
  this ADR's `Meaning` field, so this ADR lands first in the shared plan.
- A small cost: `AuditConfig` and `audit.Settings` move from `[]string` to `[]ScopeSpec`, and every
  reader that only wanted names now goes through `.Name`: the commit-gate's `containsFold` match in
  `internal/audit/audit.go` and the `commitScopesDisplay` scope-list formatter in
  `internal/project/render.go`. `commitScopesDisplay` keeps emitting a names-only display string, so
  ADR-0051's `commit-scope-single-storage` and ADR-0055's `guide-scopes-derived` contracts (both
  fed by the `commitScopes` render key) are unaffected. This is mechanical but spans the config,
  audit, and **project** packages together, and all three must land in one commit to keep the build
  green.
- Forward compatibility is one-directional: an existing string-list config stays valid under the
  new decoder, but a config that opts into the mapping form is unreadable by a binary predating the
  `ScopeSpec` decoder; that binary rejects it with a raw YAML type error rather than the ADR-0039
  version-gate message, because the schema generation is unchanged. This matches every other
  additive `.awf/config.yaml` surface (e.g. the strict-parsed `domains:` field, added without a
  schema bump): the schema gate protects a new binary reading an old config, not an old binary
  reading a new config.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep `[]string`; store meanings in a parallel `audit.scopeMeanings` map keyed by name | Two structures to keep in sync (name list vs. meaning map), reintroducing the very keyed-by-name drift this removes; a meaning for a non-existent scope becomes possible. |
| Keep meanings out of config; document them only in `docs/domains/*` and prose | The workflow.md taxonomy still hand-writes tokens; the drift the effort targets survives. |
| Make `allowedScopes` a mapping `{name: meaning}` instead of a list | Loses declared ordering (the list order is the display order) and breaks every existing list-form config, forcing a migration. |
| Widen `[]string` and post-process mapping elements | Strict `KnownFields` parsing rejects a mapping element before any post-processing runs; a custom `UnmarshalYAML` is required regardless. |
