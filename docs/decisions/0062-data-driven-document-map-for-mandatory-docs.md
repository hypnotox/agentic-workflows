---
status: Implemented
date: 2026-07-05
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [doc-model]
related: [43, 59, 61]
domains: [rendering]
---
# ADR-0062: Data-driven document map for mandatory docs

## Context

ADR-0061 unified the catalog's toggleable docs and always-on singletons into one `DocEntry`
collection, but explicitly deferred one item (ADR-0061 lines 137, 183–184, 192): the `AGENTS.md`
"## Document map" section still **hardcodes** the mandatory-doc lines in
`templates/agents-doc/AGENTS.md.tmpl`, while the toggleable docs already render data-driven from
catalog `Title`/`Desc` via `{{ range .docs }}`. So adding a mandatory doc means hand-editing a
document-map line in the template — the last hand-step the unified model left open.

The document map today has three groups: an **ADR-system cluster** (ADR index → `.layout.adrReadme`,
Active ADRs → `.layout.activeMd`, Plans → `.layout.plansDir`), four **hardcoded mandatory-doc lines**
(Workflow, Documentation Standard, Authoring AGENTS.md, Working with awf — the `DocumentMap: true`
entries), then the data-driven toggleable block and a per-project `data.docMap` extras block. Of the
ADR-system cluster, only `adr-readme` is a `DocEntry`; `activeMd` is a generated index and `plansDir`
is a directory — neither is a doc.

Grounding confirmed: the toggleable block reads `{{ .title }}`/`{{ .path }}`/`{{ .desc }}` from
`resolvedDocs` (sorted by name, `docsDir`-relative path); the injected `data["docs"]` key is read only
by the `agents-doc` render and does **not** enter any `ConfigHash` (it is a top-level data key, like
`.docs`); the `document-map-lists-mandatory-docs` backing test already iterates `catalog.Standard.Docs`
`DocumentMap` entries (since ADR-0061); and the toggleable catalog `Desc`s carry no trailing period
while the four hardcoded lines do.

## Decision

1. **The four mandatory-doc lines render data-driven from the catalog.** Give the four
   `DocumentMap: true` `DocEntry`s (`workflow`, `doc-standard`, `agents-md-standard`,
   `working-with-awf`) a `Title` and `Desc` carrying today's document-map label and description,
   **normalized to no trailing period** to match the toggleable-doc `Desc`s. A new
   `(*Project).documentMapDocs()` returns those entries as a name-sorted `[]map[string]any`
   (`title`/`path`/`desc`, `path` = `docsDir`-relative, mirroring `resolvedDocs`), injected as
   `data["mandatoryDocs"]` beside `data["docs"]` in the `agents-doc` render. The template replaces the
   four hardcoded lines with one `{{ range .mandatoryDocs }}` block, above the toggleable
   `{{ range .docs }}` and the `data.docMap` extras.

2. **`documentMapDocs()` renders unconditionally.** Unlike `resolvedDocs`, it does **not** skip a
   `local: true` sidecar: a mandatory singleton's file exists at its path even when hand-maintained,
   and the document-map lines have always rendered regardless of sidecars. This is the one deliberate
   divergence from the toggleable path.

3. **The ADR-system cluster stays hardcoded.** ADR index / Active ADRs / Plans keep their template
   lines: `activeMd` and `plansDir` are not `DocEntry`s (a generated index and a directory), and
   keeping `adr-readme`'s "ADR index" line with them keeps the data-driven mandatory-doc lines
   contiguous — no interleaving. `adr-readme`/`adr-template`/`plans-readme` stay `DocumentMap: false`.

4. **Ordering is alphabetical by name**, consistent with the toggleable block. The accepted output
   change is limited to `AGENTS.md`'s document map: the four lines reorder (to `agents-md-standard`,
   `doc-standard`, `workflow`, `working-with-awf`) and lose their four trailing periods. No other
   rendered artifact changes; the lock churns for `AGENTS.md` only (its `TemplateHash`/`OutputHash`,
   not `ConfigHash`).

## Invariants

- The `document-map-lists-mandatory-docs` invariant (ADR-0043, widened to four docs by ADR-0059)
  **keeps its slug and contract** — `AGENTS.md`'s document map unconditionally cites every
  `DocumentMap` mandatory doc regardless of the `docs:` array. Only its *backing mechanism* moves from
  hardcoded template lines to the data-driven `{{ range .mandatoryDocs }}` block; the existing test
  (`TestAgentsDocDocumentMapListsMandatorySingletonsUnconditionally`) already iterates the catalog's
  `DocumentMap` entries and stays green. No invariant is added or retired.

## Consequences

- Adding a mandatory doc is now a single `DocEntry` end to end — it appears in the document map with
  no template edit, closing the last hand-step ADR-0061 left open. The whole document map (minus the
  non-doc ADR-system cluster) renders through one uniform catalog-driven mechanism.
- `DocEntry.Title`/`Desc` on a mandatory entry are consumed only by `documentMapDocs()` — nothing else
  reads them (toggleable-only `resolvedDocs` iterates `Cfg.Docs`), so the change is inert outside the
  document map.
- One-time adopter-visible change: the four document-map lines reorder to alphabetical and drop their
  trailing periods (catalog-wide description consistency). Rendered output is otherwise unchanged; this
  repo re-renders its own `AGENTS.md` in the same commit.
- The `data.docMap` extras block and its `**{{ .path }}:**` label are untouched — a pre-existing
  cosmetic inconsistency left as-is.
- When this ADR flips to `Accepted`/`Implemented`, the same commit regenerates
  `docs/decisions/ACTIVE.md` (and the `rendering` per-domain decision index) via `./x sync`, staged
  alongside the template and catalog change (docs-travel-with-change).

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Preserve the exact current order (byte-identical) | Non-alphabetical order needs a `DocMapOrder` field or a hand-maintained ordered list in Go — reintroducing the hand-maintenance this removes. Since the lock churns at the next release anyway, an alphabetical reorder is the cleaner cost. |
| Keep singleton descriptions' trailing periods (byte-identical descriptions) | Leaves the catalog inconsistent (some `Desc`s end with a period, some do not); normalizing to no-period matches the toggleable docs and is free given the accepted reorder. |
| Data-drive the ADR-system cluster too (Active ADRs / Plans) | `activeMd` is a generated index and `plansDir` a directory — neither is a `DocEntry`; synthesizing entries for them adds machinery for no gain and would interleave the doc-backed lines. |
| Fold `documentMapDocs` and `resolvedDocs` into one helper | Their source set (`Cfg.Docs` vs catalog `DocumentMap` entries) and `local:`-skip behavior diverge; two small sibling methods read clearer than a flag-driven merge. |
