---
status: Proposed
date: 2026-07-05
supersedes: []
retires_invariants: [mandatory-docs-not-in-docs-catalog]
superseded_by: ""
tags: [catalog, docs, singleton, rendering, config, refactor]
related: [0004, 0021, 0022, 0027, 0031, 0037, 0043, 0059, 0060]
domains: [rendering, config]
---
# ADR-0061: Unified catalog doc model for toggleable docs and singletons

## Context

awf renders two kinds of documentation artifact, and today they are described by two separate catalog
maps plus a scatter of hand-maintained Go projections:

- **Toggleable docs** — `catalog.Docs` (`architecture`, `testing`, `development`, `debugging`,
  `pitfalls`, `glossary`, `roadmap`): an adopter-selectable pool, rendered only when named in the
  config `docs:` array, listed in `AGENTS.md`'s document map via `resolvedDocs`.
- **Always-on singletons** — `catalog.Singletons` (`agents-doc`, `adr-readme`, `adr-template`,
  `plans-readme`, `workflow`, `doc-standard`, `agents-md-standard`, `working-with-awf`): rendered for
  every project unless a `local: true` sidecar suppresses them.

The mandatory subset is hand-declared in ~6 places that drift together whenever a doc is added or
promoted: `catalog.SingletonKinds` (the kind list), `project.plainSingletons` (the render/validate
table), `Layout`'s typed singleton path fields + `templateMap` camelCase keys, the hardcoded
`TestAgentsDocDocumentMapListsMandatorySingletonsUnconditionally` and `TestLayoutDerivesFromDocsDir`,
and a by-name enumeration comment in `render.go`. ADR-0043 (promotion of three docs to singletons)
and ADR-0059 (the `working-with-awf` singleton) each had to touch every one of these; adding the next
mandatory doc will too. ADR-0043's invariants encode the split itself —
`mandatory-docs-not-in-docs-catalog` literally asserts the `docs:` block contains no singleton and
that `DocSpec` carries no `Core` field.

ADR-0060 moved the whole catalog into a compile-time Go value (`catalog.Standard`). That removes the
runtime-parse workaround that forced `SingletonKinds` to exist as a hand-list separate from the
loaded map, and makes deriving every projection from one collection ordinary Go code. This ADR builds
directly on that: with the data already in Go, the two maps can become one collection whose entries
carry their own nature, and the ~6 hand-maintained sites collapse into projections over it.

Grounding surfaced two sharp edges. The first is a **pool leak**: several consumers derive the
toggleable-doc pool from the doc-map's keys. Three of them funnel through the single `docs`
`kindDescriptor` `poolNames` facet (`kind.go` — `slices.Sorted(maps.Keys(c.Docs))`):
`validateAgainstCatalog`, `CatalogNames` for `awf list`/`add`/`remove`, and `scaffold`'s
var-collection loop — so one `!Mandatory` filter at that facet covers all three (scaffold still seeds
each mandatory doc's vars through the `plainSingletons` loop, so the filter drops a harmless
double-seed, not real coverage; scaffold's `docs:` array is built from the catalog-trim, never from
the pool). Two more read `cat.Docs` directly and each needs its own `!Mandatory` filter: the
`awf init` catalog-docs multiselect (`initspec.go:50`) and the evals fixture (`fixture_test.go:60`,
which seeds a `docs:` array). Miss one and a mandatory singleton leaks into the toggleable-doc CLI,
validation, or init surface. `agents-doc` is the irregular member — it renders to root `AGENTS.md`
(not a `docsDir` path), has no document-map entry of its own (it *is* the map), and drives the
CLAUDE.md bridge from a bespoke `RenderAll` branch — so a uniform loop must special-case it.

The second edge is **template-path reconstruction**. Merging every singleton into `Docs` brings in
keys whose templates do *not* live at `docs/<name>.md.tmpl` — `adr-readme`
(`adr-readme/README.md.tmpl`), `adr-template`, `plans-readme`, and `agents-doc`. Every site that
rebuilds a template id as `docs/<name>.md.tmpl` from a doc-map key breaks on them: the `docs`
`poolNames` facet's `tid` closure (protected once the pool is filtered to `!Mandatory`) and — more
sharply — three live invariant-backing tests that iterate `cat.Docs` raw and would `t.Fatalf` reading
a non-existent `docs/adr-readme.md.tmpl`: `TestDocsSectionParity` (`inv: docs-section-parity`),
`TestVarDescriptorParity` (`inv: var-descriptor-parity`), and `scaffold_test.go`'s var-parity check
(`inv: scaffold-seeds-all-vars`). These read the raw map, not the filtered pool, so they must switch
to each entry's `TID` or filter `!Mandatory`; `DocEntry.TID` (Decision item 1) is that authority.

## Decision

1. **One unified doc collection in the Go catalog.** Merge `catalog.Docs` and `catalog.Singletons`
   into a single `Docs map[string]DocEntry` on `catalog.Catalog`. A `DocEntry` carries the existing
   `Title` / `Desc` / `Sections` / `Data` plus the metadata currently hardcoded in Go: `Mandatory
   bool`, `Path` (docsDir-relative suffix; empty for `agents-doc`), `TemplateKey` (the `.layout`
   camelCase key; empty when the doc is not layout-exposed), `TID` (embedded template id),
   `DocumentMap bool`, and `AgentsDoc bool`. Skills and agents keep `TargetSpec`; only the doc surface
   unifies.

2. **Every projection derives from the collection.** No independent hand-maintained doc/singleton
   list remains:
   - `catalog.SingletonKinds` derives from entries where `Mandatory` is true (`agents-doc` included).
   - `project.plainSingletons` derives from entries where `Mandatory && !AgentsDoc`.
   - `Layout`'s singleton path fields and `templateMap` keys derive from each mandatory entry's `Path`
     and `TemplateKey`; template-facing `.layout.*` names are unchanged (no contract break).
   - The `docs` `kindDescriptor` pool and its ~6 consumers derive from entries where `!Mandatory`.
   - `RenderAll`'s render loops, the document-map assembly, and the `render.go` enumeration comment
     iterate the collection; every template id is read from each entry's `TID` rather than rebuilt as
     `docs/<name>.md.tmpl`, since merged-in singletons render from non-`docs/` templates.
   - The two formerly-hardcoded tests re-back by nature, not by iterating the collection into their
     own expectations: `TestAgentsDocDocumentMapListsMandatorySingletonsUnconditionally` renders
     `AGENTS.md` and asserts each mandatory `DocumentMap` entry's link appears in the *rendered output*
     (a real check, not a tautology), while `TestLayoutDerivesFromDocsDir` stays a concrete-value
     fixture — its expected paths remain literal strings (gaining the new entry's path) — because a
     layout test that derived its expectations from the same collection it checks would prove nothing.

3. **Mandatory entries are excluded from the toggleable-doc pool.** The toggleable pool
   (`CatalogNames`, `validateAgainstCatalog`, the `awf init` catalog-docs options, `scaffold`
   var-seeding, the evals fixture) contains only `!Mandatory` entries. A mandatory doc is never
   addable or removable through `awf add doc` / `awf remove doc`, and never seeds a `docs:` array —
   matching today's behaviour, now enforced by one filter rather than a separate `docs:` block.

4. **`agents-doc` is special-cased by its `AgentsDoc` flag.** It is a mandatory entry with an empty
   `Path`/`TemplateKey` and `DocumentMap: false`; it keeps its bespoke `RenderAll` branch (root
   `AGENTS.md`, `resolvedDocs` injection, CLAUDE.md bridge). No projection that expects a `docsDir`
   path or a document-map slot iterates it. `DocumentMap: true` is set on exactly the four standard
   docs the `document-map-lists-mandatory-docs` invariant names (`workflow`, `doc-standard`,
   `agents-md-standard`, `working-with-awf`); `adr-readme`, `adr-template`, and `plans-readme` are
   `DocumentMap: false` because their map lines ("ADR index", "Active ADRs", "Plans") stay hardcoded
   from `.layout.adrReadme`/`.activeMd`/`.plansReadme`, not driven by a `DocEntry` projection — so the
   re-backed document-map test iterates exactly the four the invariant contracts, no wider.

5. **Invariant reconciliation.** `mandatory-docs-not-in-docs-catalog` is retired via this ADR's
   `retires_invariants` frontmatter and its backing test (`TestCatalogDocsExcludeSingletonKinds`,
   `internal/project/singleton_test.go`) is removed in the same change — its premise, a separate
   `docs:` block and a `DocSpec.Core` field, no longer exists. It is replaced by
   `mandatory-doc-pool-exclusion` (item 3) and the overarching `unified-doc-model` invariant (item 2).
   The surviving ADR-0043/0059 invariants keep their wording and are re-backed to iterate the
   collection: `singleton-kind-single-source` (both projections still derive from one source),
   `plain-singleton-via-renderkind` (the now-derived `plainSingletons` still renders each plain
   singleton through `renderKind`), `document-map-lists-mandatory-docs` (the document-map test now
   iterates mandatory `DocumentMap` entries instead of a hardcoded list), and
   `singleton-doc-migration-relocates-parts` (guards a frozen past migration; untouched).

6. **Document-map description templating is out of scope.** The `AGENTS.md` template keeps its
   hardcoded singleton document-map link text and descriptions this pass; the `.layout.*` references
   still resolve from the catalog-derived `Layout`, so the merge does not force touching them. Driving
   the whole document map from catalog `Title`/`Desc` is a clean follow-up, not part of this ADR.

## Invariants

- `inv: unified-doc-model` — the entire doc surface, toggleable and mandatory, derives from the single
  `catalog` doc collection. `SingletonKinds`, `plainSingletons`, the `Layout` singleton paths +
  `templateMap` keys, and the toggleable-doc pool are all projections over it, with no independent
  hand-maintained doc/singleton list. Backed by tests that iterate the collection and assert each
  projection matches (a mandatory entry appears in `SingletonKinds`; a non-`agents-doc` mandatory
  entry appears in `plainSingletons`; each mandatory `TemplateKey`/`Path` appears in `templateMap`).
- `inv: mandatory-doc-pool-exclusion` — no `Mandatory` entry appears in the toggleable-doc pool: it is
  absent from `CatalogNames("doc")`, rejected by `awf add doc` / `awf remove doc`, and never seeds a
  scaffolded `docs:` array. Backed by a test asserting the `!Mandatory` filter across the pool
  consumers.
- The surviving invariants `singleton-kind-single-source`, `plain-singleton-via-renderkind`,
  `document-map-lists-mandatory-docs` (ADR-0043, widened by ADR-0059), and
  `singleton-doc-migration-relocates-parts` keep their contracts; their backings are updated to
  iterate the unified collection where they previously read a split map.

## Consequences

- Adding a mandatory doc becomes a single `DocEntry` (with `Mandatory: true` and its `Path`/
  `TemplateKey`/`TID`) plus its template and sections — every projection follows automatically. The
  ~6-place drift shape that ADR-0043 and ADR-0059 each paid is gone; the formerly-hardcoded tests
  iterate the collection and cannot silently stop covering a new entry.
- Promoting a toggleable doc to mandatory (the ADR-0043 move) becomes flipping `Mandatory` on one
  entry plus a migration to relocate its sidecar/parts — no catalog-map surgery.
- The pool-leak hazard is the real cost: the `!Mandatory` filter must be applied at every toggleable-
  pool consumer, and `mandatory-doc-pool-exclusion` exists to catch a missed one. The `agents-doc`
  special-case is preserved, not unified away.
- ConfigHash / `.awf/awf.lock`: rendered output stays byte-identical (the `.layout.*` values and
  document-map text are unchanged), so no adopter-visible drift is expected; per ADR-0060 a one-time
  lock regeneration is acceptable if a hash shifts, since output equality is the contract.
- Live awf-managed docs describing the doc/singleton split — `docs/architecture.md`,
  `docs/domains/rendering.md`, and the convention parts under `.awf/docs/parts/` and
  `.awf/domains/parts/rendering/` that name "singletons" vs "toggleable docs" as two mechanisms —
  update in the same commit to describe one collection (docs-travel-with-change). Frozen ADRs/plans
  stay as written; ADR-0043 and ADR-0059 remain `Implemented`, their retired invariant recorded here.
- The three `docs/<name>.md.tmpl`-reconstructing parity tests (`docs-section-parity`,
  `var-descriptor-parity`, `scaffold-seeds-all-vars`) are updated in the same change to read `TID` or
  filter `!Mandatory`; they `t.Fatalf` the moment a non-`docs/`-template singleton lands in `Docs`, so
  the fix is not optional cleanup but a landing prerequisite.
- When this ADR flips to `Accepted`/`Implemented`, the same commit regenerates
  `docs/decisions/ACTIVE.md` via `./x sync` and removes the retired invariant's backing test in
  lockstep with the flip — retirement is inert until the retiring ADR is `Implemented` (ADR-0031).
- Document-map descriptions remain hardcoded in the template, so a future entry still needs its
  document-map line added there by hand until the deferred templating follow-up lands.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep two catalog maps, unify only via a Go view | Leaves `mandatory-docs-not-in-docs-catalog`'s split premise and the two-map surface in place; the "one structure for all docs" goal and the flag-flip promotion both want a single collection, and ADR-0060 already made the Go merge cheap. |
| Move only the mandatory singletons to a registry, leave toggleable docs as-is | Closes the ~6-place drift but keeps two doc mechanisms and two mental models; a promotion still means moving an entry between collections rather than flipping a flag. |
| Unify the document map fully (data-drive singleton link text now) | Forces authoring every singleton's document-map description into the catalog byte-for-byte in the same change; deferrable because `.layout.*` references still resolve, so it is sequenced as a follow-up rather than bundled. |
| Fold `agents-doc` into the uniform render loop | Its root output path, absent document-map slot, and CLAUDE.md-bridge coupling make it genuinely irregular; a flag-guarded special case is simpler and safer than generalising the loop to accommodate one outlier. |
