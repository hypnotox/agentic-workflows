---
status: Implemented
date: 2026-06-28
supersedes: []
superseded_by: ""
tags: [tooling, rendering, refactor, dispatch]
related: [0009, 0016, 0024]
domains: [rendering, tooling]
---
# ADR-0027: Unified Kind Descriptor for Per-Kind Dispatch

## Context

awf renders five target *kinds* — `skills`, `agents`, `docs`, `hooks`, and the freeform
`domains` — and the mapping from a kind to its four facets (the config enable array, the
catalog pool, the declared sections, and the rendered output path) plus its display labels is
hand-maintained as parallel `switch`/`map` blocks in at least six sites:

- `cmd/awf/list_add.go:18` (`kindKey`, singular→plural), `:27` (`kindsOrdered`, display order),
  `:34-47` (`enabledNames`, enable-array accessor), `:51-64` (`catalogNames`, catalog-pool
  accessor with a `hasPool` bool), and the sidecar-driven `targetState` at `:183`.
- `internal/project/check.go:18-27` (`localOutPath`, skills/agents output path) and the
  `declaredSections` switch (`:130-142`).
- `internal/project/validate.go:34-51` (three near-identical `checkKind` calls differing only in
  the `p.Cat.X[name]` map they close over), with hooks handled by a fourth, separate block at
  `:53-61`.

Adding a runtime kind, or changing how one facet resolves, means editing every one of these in
lockstep; nothing mechanically couples them, so they drift independently (the audit that
prompted this ADR found exactly this class of latent inconsistency). The kinds are *not*
uniform, which is why the duplication has resisted consolidation so far:

1. **Catalog shape differs.** `internal/catalog` exposes `Skills`/`Agents`/`Docs` as
   `map[string]Entry` but `Hooks` as a plain `[]string` (`catalog.go:36-44`), so `catalogNames`
   branches between `maps.Keys` and `slices.Values`.
2. **Output path differs.** Only skills and agents are *adapter* artifacts with a
   `Target`-supplied path (ADR-0016); `localOutPath` returns `""` for docs/hooks, which are
   neutral.
3. **`domains` has no catalog *pool*.** It is freeform: `catalogNames` returns `(nil, false)` and no
   per-name catalog presence applies. Its *declared sections* are **not** absent, though — they come
   from the shared `DomainDoc` singleton: `declaredSections("domains", …)` returns
   `p.Cat.DomainDoc.Sections` (`check.go:138-139`), which `orphans()` uses to flag domain
   convention-part sections that are not catalog-declared. So `domains` participates in the
   CLI-facing facets (key, order, enable array) **and** in the section facet (via the singleton), but
   not the catalog-pool facets (pool listing, per-name presence).

A naive struct-of-values table cannot express these asymmetries. The existing
`// invariant: cli-config-kinds` marker (ADR-0024, Implemented) currently backs `kindKey` as the
single enumeration of CLI kinds; any consolidation must keep that invariant backed.

## Decision

1. **One ordered descriptor table is the single source of per-kind dispatch.** Introduce a
   `kindDescriptor` value and a package-level ordered table in `internal/project`, keyed by the
   plural kind name, with one entry per kind (`skills`, `agents`, `docs`, `hooks`, `domains`).
   Every dispatch site listed in Context resolves through this table instead of its own switch.

2. **Facets are accessor functions, not values, so the table absorbs the asymmetries.** Each
   descriptor carries function-typed fields rather than flat data:
   - `poolNames func(*catalog.Catalog) []string` — returns the sorted catalog pool, hiding the
     `map` vs `[]string` difference; `nil` for `domains` (no pool).
   - `sections func(*catalog.Catalog, name string) ([]string, bool)` — declared sections and a
     presence bool; the `bool` is `false` for a name absent from a catalog-backed pool and for
     every `domains` name (freeform). For `domains` the `[]string` is **not** `nil` — it stays
     `p.Cat.DomainDoc.Sections`, so `orphans()`'s section-orphan check is unchanged; only the
     presence `bool` is `false` (no per-name catalog membership).
   - `outPath func(t Target, prefix, name string) string` — the rendered path; returns `""` for
     neutral kinds (docs/hooks/domains handled by their own neutral-layer formulas, unchanged).
   - `singular string` — the singular label (replacing `strings.TrimSuffix(kind, "s")` at
     `check.go:48` and `validate.go:26`, which is fragile for already-singular forms).
   A kind without a facet sets that field to `nil`/`""`; callers treat the zero value as "facet
   absent" exactly as the current `hasPool`/`""` returns already do.

3. **Scope is the dispatch tables only.** The descriptor replaces the per-kind switches in
   `list_add.go`, `check.go` (`localOutPath`, `declaredSections`), and `validate.go`
   (`validateAgainstCatalog`). It does **not** absorb (a) the `RenderAll` per-kind render *loops*
   in `render.go` — they carry a skills-only doc-gate and other render-time asymmetry and remain a
   possible future consumer, not part of this change; nor (b) the always-on singleton/bridge
   targets (`agents-doc`, `adr-readme`, `adr-template`, `plans-readme`, `CLAUDE.md`), which are
   not kinds and keep their dedicated handling.

4. **`cmd/awf` consumes the table through an exported accessor.** `cmd/awf` already imports
   `internal/project` (`list_add.go:13`), so `enabledNames`/`catalogNames`/`kindKey`/`kindsOrdered`
   are reframed as thin wrappers over an exported `project` lookup (e.g. `project.KindDescriptor(kind)`
   / `project.Kinds()`); no new dependency edge is introduced.

5. **Re-home ADR-0024's `cli-config-kinds` backing.** Consolidating `kindKey` into the descriptor
   table would orphan the `// invariant: cli-config-kinds` marker (ADR-0024 is Implemented). The
   implementing commit moves that marker onto the descriptor table's kind enumeration, keeping
   ADR-0024's invariant backed; ADR-0024 otherwise stands.

## Invariants

- `inv: kind-dispatch-single-table` — every per-kind facet (enable array, catalog pool, declared
  sections, output path, singular/plural labels) resolves through the single `internal/project`
  descriptor table; no other site hand-rolls a parallel per-kind `switch` over the kind set. A
  test enumerates the table and asserts its kind set equals the catalog's kind set plus
  `domains`, so adding a catalog kind without a descriptor entry fails.
- **`cli-config-kinds` stays owned by ADR-0024 — only its source marker moves.** ADR-0024 remains the
  sole declaring ADR for that slug; re-declaring it here (as a leading `inv:` bullet) would trip the
  duplicate-slug guard in `internal/invariants` (`invariants.go:70-72` errors on a slug declared by two
  Implemented ADRs) the moment this ADR flips to `Implemented`, and ADR-0024 is append-only so its
  declaration cannot be removed. The implementing commit therefore adds **no** second declaration; it
  only relocates the existing `// invariant: cli-config-kinds` comment from `kindKey` onto the
  descriptor table's kind enumeration (Decision 5), which keeps the slug backed (the backing scan is
  marker-location-agnostic).

## Consequences

- Adding a runtime kind becomes one descriptor entry instead of edits to six switches; the
  `kind-dispatch-single-table` test fails loudly if a catalog kind is added without one.
- The table is more abstract than six literal switches: a reader must follow a function-typed
  field to see how one kind resolves a facet. This is the deliberate cost of removing the drift
  class — the asymmetries (map-vs-slice catalog, adapter-vs-neutral path, catalog-less `domains`)
  are now stated once in the descriptor rather than re-encoded per site.
- `internal/project` gains the descriptor type and the exported accessor; `cmd/awf`'s kind maps
  shrink to wrappers. No change to rendered output, the lock format, or the config schema — this
  is an internal dispatch refactor with no behavioural change to any awf command.
- The `RenderAll` render loops remain duplicated for now (out of scope per Decision 3); a later
  ADR may fold them into the descriptor once the render-time doc-gate asymmetry is modelled.

Doc-currency obligations the implementing commit(s) must satisfy:

- `docs/architecture.md`'s `internal/project` entry gains the descriptor table as the per-kind
  dispatch source (via `.awf/docs/parts/architecture/components.md`).
- The `rendering` and `tooling` domain narratives note the single-table dispatch.
- The status flip to `Implemented` regenerates `docs/decisions/ACTIVE.md` via `./x sync`.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Three-kind descriptor (skills/agents/docs) with hooks + domains kept separate | The grounding-check's conservative option. Leaves a parallel hooks switch in each of list/add/check/validate — a partial win that keeps the exact drift class the ADR targets. The function-accessor table absorbs the hooks `[]string` shape, so excluding hooks buys nothing. |
| Struct-of-values table (flat fields, no functions) | Cannot express the map-vs-slice catalog shape, the adapter-vs-neutral path, or catalog-less `domains` without sentinel values and caller-side branching — pushing the asymmetry back to call sites and defeating the consolidation. |
| Fold the `RenderAll` render loops in too | Over-scopes: the render loops carry a skills-only doc-gate and render-time concerns absent from the dispatch switches. Bundling them couples two refactors and enlarges blast radius; deferred to a future ADR (Decision 3). |
| Leave the switches as-is | Accepts ongoing lockstep maintenance across six sites and the independent-drift risk the prompting audit found. |
