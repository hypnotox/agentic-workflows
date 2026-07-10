---
status: Proposed
date: 2026-07-11
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [catalog, rendering, cli, docs, adopter-extensibility]
related: [20, 60, 61, 68, 70, 77, 86, 88]
domains: [config, rendering, tooling]
---
# ADR-0091: Project-local custom docs as a third local artifact kind

## Context

ADR-0068 gave adopters project-local *skills and agents*: an enabled name outside
`catalog.Standard`, declared by a sidecar, synthesized into a per-project catalog
clone, and rendered from an awf-owned base template. Docs were left out. Today an
adopter cannot create a doc that awf manages — the doc pool is catalog-only
(`internal/project/validate.go` `nodeEnabled` reads `Cfg.Docs` against
`NonMandatoryDocNames`), `awf new` scaffolds only `adr`/`skill`/`agent`
(`cmd/awf/new.go`), and every catalog doc has its own bespoke embedded template.

The absence is felt in awf's own dogfooding. awf keeps a hand-authored
`docs/releasing.md` that is not awf-managed — no drift check, no dead-link check
(ADR-0020), no publication-safety guarantee — and bolts it onto the AGENTS.md
document map through the `docMap` sidecar escape hatch (`.awf/agents-doc.yaml`).
The example adopter `examples/sundial` uses the same escape hatch for its README.
Every use of `docMap` is a doc awf cannot manage; retiring it means giving adopters
a first-class way to author a managed doc.

Grounding this decision against the code surfaced four facts that shape it:

- **The doc render loop resolves its template id from the package global, not the
  effective catalog.** `RenderAll`'s docs pass reads the template id via
  `mustDescriptor("docs").tid`, whose body is
  `catalog.Standard.Docs[n].TID` (`internal/project/kind.go`) — the process-wide
  global, not the per-project `p.Cat`. A synthesized local doc (absent from
  `catalog.Standard.Docs`) would resolve to an empty id and fail to render. The
  same loop already reads *sections* from `p.Cat`, so the code is internally split;
  this decision must add an effective-catalog template-id hook for docs, mirroring
  `skillTID`/`agentTID`.

- **`effectiveCatalog` clones only the `Skills` and `Agents` maps.** It leaves
  `clone.Docs` aliased to the global map (`internal/project/local.go`). Inserting a
  synthesized doc there would mutate `catalog.Standard.Docs` process-wide, breaking
  `inv: local-catalog-clone` and cross-contaminating every other project in the
  process. Doc synthesis must clone the `Docs` map too.

- **`DocEntry` already carries an explicit `TID` field** (ADR-0061), unlike
  `SkillSpec`/`TargetSpec`, which grew a `Base` flag only so a name-derived path
  could be overridden (ADR-0068 item 4). A synthesized `DocEntry` can simply set
  `TID` to the base doc template — no new struct field is needed. Doc synthesis is
  strictly simpler than the skill/agent case.

- **The document map reads `DocEntry.Title`/`Desc`, not sidecar data.**
  `resolvedDocs` (`internal/project/layout.go`) annotates each map line from
  `p.Cat.Docs[name].Title`/`Desc`, whereas the skill/agent synthesis path injects
  only `{slug}` and lets `description` fall through at render time. For a custom doc
  to appear in the map with a real title and description, synthesis must lift them
  from the declaring sidecar into the synthesized `DocEntry`. This is the one piece
  of new synthesis behaviour beyond ADR-0068.

Subfolder doc names (`guides/foo`) are in scope from the start. `docOutPath` is a
string concatenation (`docsDir/<name>.md`) and `filepath.Join`-based sidecar/part
paths preserve interior slashes, so nested output, sidecar, and part paths, and the
closed-config-tree claiming that walks them (ADR-0086), already work for a slashed
name. The single blocker is name validation: `ValidateArtifactName` rejects `/`.

## Decision

1. **A local doc is an enabled doc whose name is not in `catalog.Standard.Docs`,**
   declared by a `.awf/docs/<name>.yaml` sidecar. A non-Standard, non-`local:true`
   enabled doc name without a sidecar remains a hard "unknown doc" error, so typos
   still fail. A local doc name **may not equal** a Standard doc name (no
   shadowing). This mirrors ADR-0068 item 1 for the third kind.

2. **Local docs render from one awf-owned base doc template.** awf ships a single
   `templates/docs/_base.md.tmpl` carrying a title header and one `content`
   section. Every local doc renders from it; **adopters never author a doc
   template**, only content. It is embedded through an explicit `//go:embed` entry
   (the directory-walk form skips the reserved underscore-prefixed name).

3. **`effectiveCatalog` clones `catalog.Standard.Docs` and synthesizes local doc
   entries into the clone.** A synthesized `DocEntry` sets `Sections: ["content"]`,
   `TID` = the base doc template, `Mandatory: false`, and `Title`/`Desc` lifted from
   the declaring sidecar's `data.title`/`data.description`. `catalog.Standard` is
   **never mutated**; the `Docs` map is `maps.Clone`d before any insert, exactly as
   `Skills`/`Agents` are.

4. **Template-id resolution for docs reads the effective catalog.** A `docTID` hook
   returns `p.Cat.Docs[n].TID`, and the docs render pass uses it instead of the
   global-reading descriptor facet. Standard docs (whose `TID` is set in
   `catalog.Standard`) resolve identically to today, so their rendered output and
   drift hashes stay byte-identical; synthesized locals resolve to the base
   template. No `Base` field is added to `DocEntry`.

5. **The author surface is the `content` part plus the sidecar.** A local doc's
   body is the convention part `.awf/docs/parts/<name>/content.md`, spliced verbatim
   with `{{=awf:key}}` substitution (never Go-template-executed, `inv: parts-raw`).
   The base template sources `title` (defaulting to a value derived from the last
   name segment) and the body's `content`, each guarded so an unset value degrades
   to publication-safe generic prose, never `<no value>` (ADR-0045).

6. **Doc names are path-aware; skills and agents stay flat.** A doc-name validator
   accepts one or more `/`-separated segments, each lowercase kebab-case, rejecting
   `..`, a leading or trailing slash, an empty segment, and a `.md` suffix. It
   branches in at both call sites keyed on the docs kind — `awf new doc` and the
   synthesis path — while skills/agents keep the flat `ValidateArtifactName`. A
   local doc name equal to the reserved base-template stem (`_base`) is rejected.

7. **`awf new doc <name> "<description>"` scaffolds a complete local doc:** it
   validates the path-aware name, writes the declaring sidecar (carrying the
   description and the derived title), writes a starter `content` part seeded with
   the `awf:stub` marker (ADR-0070) whose prose points the author at the
   documentation standard, enables the name in `docs:`, and syncs. The seed's
   pointer to the documentation standard is prose or a repo-root-anchored link,
   never a file-relative markdown link — which would resolve dead from a nested
   doc's directory and trip the ADR-0020 check. The `new` help text and unknown-kind
   error (and its test) update in the same commit.

8. **`local: true` is unchanged.** The flag keeps its meaning — opt out of rendering
   for a fully hand-authored doc — and composes with local doc names. No new sidecar
   field is introduced, so no schema change and no drift-hash churn.

9. **Managed-doc coverage is inherited, not built.** Because a synthesized local doc
   lives in the effective catalog with a `Title`/`Desc`, it flows into the AGENTS.md
   document map through the existing `resolvedDocs` path and into the ADR-0020
   dead-link check through the existing rendered-doc walk — with no new doc-map or
   link-check code.

## Invariants

- `inv: local-doc-catalog-clone` — `effectiveCatalog` synthesizes local doc entries
  into a clone of `catalog.Standard.Docs`; the package global's `Docs` map is never
  mutated by opening a project.
- `inv: local-doc-requires-declaration` — a non-Standard, non-`local:true` enabled
  doc name without a declaring sidecar is a hard error at project open.
- `inv: local-doc-no-shadow` — a local (non-Standard) doc name equal to a
  `catalog.Standard.Docs` name is rejected.
- `inv: local-doc-renders-from-base` — a rendered local doc resolves its template id
  through the effective catalog to the shared base doc template, not the
  name-derived or empty path.
- `inv: local-doc-map-fields` — a synthesized local doc entry carries the `Title`
  and `Desc` lifted from its declaring sidecar, so the document map lists it with a
  non-empty title and description.
- `inv: doc-base-publication-safe` — the base doc template renders leak-free (no
  `<no value>`, no marker or leak residue) under empty data and no `content` part.
- `inv: doc-name-path-validated` — a local doc name is accepted only as one or more
  lowercase-kebab `/`-separated segments and is rejected for a path-escape (`..`), a
  leading/trailing/empty segment, or a `.md` suffix; skill and agent names remain
  flat.

## Consequences

- **Adopters get a managed custom doc for a one-line sidecar plus one content
  part.** awf owns the structure, the document-map listing, the dead-link check, and
  publication-safety; the adopter writes prose. This is the capability the `docMap`
  escape hatch only approximated.
- **The releasing gap closes by dogfooding.** A new toggleable catalog doc
  `releasing` — a single freeform `content` section whose default is a
  stub-classified authoring prompt (ADR-0070), imposing no release structure — lets
  any adopter run `awf add doc releasing`. awf enables it, ports its current
  `docs/releasing.md` prose into the `.awf/docs/parts/releasing/content.md` override
  part, and drops the `docMap` entry. The ported prose must live in the override
  part, not the template default: `docs/releasing.md` carries ADR and repo-identity
  literals the template-source residue scan (ADR-0082) would reject, and parts are
  outside the scanned templates FS. Adding a catalog doc is routine content, not a
  load-bearing decision — it rides this ADR as its motivating application.
- **Doc synthesis does one thing skill/agent synthesis does not:** it reads the
  sidecar's `data` into the synthesized spec's `Title`/`Desc`. The skill/agent path
  never needed this because the document map does not list them. It is a small,
  doc-specific addition, captured by `inv: local-doc-map-fields`.
- **The base doc template sits outside the Standard parity/eval machinery** and
  needs its own publication-safety lock (`inv: doc-base-publication-safe`), like the
  base skill/agent templates (ADR-0068). It is residue-scanned by ADR-0082 (which
  walks the whole templates FS) but is *not* auto-covered by the catalog-derived
  golden-test and leak sweeps, which range only skills and agents; a dedicated
  golden and empty-data render test cover it.
- **configspec parity extends by one entry.** The base doc template's `title`/
  `description` data keys must be described in `internal/configspec` and collected by
  the data-parity test (ADR-0088), mirroring the skills/agents `_base` `description`
  entries; otherwise the reflection-parity check fails as orphaned or undescribed.
- **`local-doc-no-shadow` is a live collision risk for generic names.** A future
  release adding a catalog doc named identically to an adopter's existing local doc
  (plausible for generic names like `releasing` or `security`) demotes the adopter's
  local sidecar and parts to unclaimed closed-tree drift with no rename guidance —
  the same trade-off ADR-0068 accepted for skills/agents, but more likely here.
- **The closed-tree drift *guidance* degrades for deeply-nested unclaimed paths.**
  `classify()` keys its advisory detail strings on fixed segment counts, so a stray
  file inside a nested doc tree falls to the generic "unclaimed" message. Pass/fail
  correctness is unaffected — the entry is still flagged; only the guidance text is
  generic. Accepted, not fixed.
- **No schema migration.** Toggleable docs are opt-in (`awf add doc`), so shipping
  `releasing` needs no seed; adopters gain it by enabling it.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Add a `Base` flag to `DocEntry` mirroring `SkillSpec`/`TargetSpec` | Unnecessary — `DocEntry` already carries an explicit `TID` (ADR-0061); synthesis sets it to the base template directly. A flag would add state with no behaviour the `TID` does not already give. |
| Section-declaring custom docs (sidecar declares its own `sections:`) | Heavier: a new sidecar field, section-parity handling for synthesized entries, more to validate — over-built for a freeform project doc, which a single `content` part already carries. A clean future increment if declared structure is ever needed. |
| Registered pass-through (awf tracks the doc for map/link coverage but does not render it) | Undershoots "awf-managed": no render or drift management. It merely formalizes the `docMap` escape hatch this ADR retires. |
| Flat doc names only, subfolders later | Rejected by the design intent — nested output, sidecar, and part paths already work via string/`filepath.Join` concatenation; only name validation blocked it, so the increment is small and worth doing once. |
| `releasing` as a bespoke multi-section catalog doc (versioning / artifacts / steps) | Forces a release structure no single project shares; a stub-default single section lets each adopter document its own process. |
| Root-output docs (README as a managed doc) | Managed docs are `docsDir`-relative; root output is the `agents-doc`/AGENTS.md special case only. Deliberately deferred — a larger, separate decision. |
