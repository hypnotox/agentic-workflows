---
status: Implemented
date: 2026-07-04
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [doc-singletons, adopter-reference]
related: [4, 11, 13, 20, 34, 43, 57, 58]
domains: [rendering, config]
---
# ADR-0059: Mandatory working-with-awf usage doc singleton

## Context

awf renders a suite of docs into an adopter's repo, but none is a *usage guide* for the adopted
project. `docs/` holds internals (`architecture`, `development`, `glossary`, `pitfalls`, `testing`)
and authoring *standards* (`doc-standard`, `agents-md-standard`, `workflow`). The only "how to work
with awf" material is a terse four-bullet `awf-setup` section baked into the generated `AGENTS.md`
guide (`templates/agents-doc/AGENTS.md.tmpl`; this repo overrides it at
`.awf/parts/agents-doc/awf-setup.md`), enough to point at, not enough to learn from.

The gap is sharpest for the ADR-0057 `{{=awf:...}}` placeholder registry: it has no adopter-facing
reference at all. Its keys and rules live only in `docs/architecture.md` and the ADRs, so an adopter
discovers the feature only by triggering the fail-loud error that lists the available keys. A
capability whose entire purpose is adopter overrides needs a place an adopter can read.

ADR-0043 established the mandatory always-on doc singletons (`workflow`, `doc-standard`,
`agents-md-standard`), awf-authored, rendered for every project, outside the toggleable `docs:`
catalog. A usage guide belongs in exactly that family: it is generic awf-usage guidance, the same
for every adopter, and every adopter needs it. Grounding confirmed a brand-new always-on singleton
needs **no schema migration or version bump**, unlike ADR-0043, which migrated (`{To: 6}`) only
because it *relocated existing* sidecars and stripped a `docs:` array member; a new singleton never
lived in `docs:` and adds no config field.

## Decision

1. **A new mandatory always-on doc singleton `working-with-awf`.** A post-adoption usage guide
   rendered into every adopter repo, joining the `plainSingletons` family alongside `workflow` /
   `doc-standard` / `agents-md-standard`: suppressible only via a `local: true` sidecar, covered
   automatically by the dead-internal-link scan (ADR-0020) and the validate/scaffold loops.
   **Adoption and `awf init` are out of scope**: first-time adoption is documented in the awf repo
   itself, not rendered into adopter projects. Concretely this adds a new template default
   `templates/docs/working-with-awf.md.tmpl` (its `awf:section` markers matching item 2), a
   `working-with-awf` entry in `templates/catalog.yaml`'s `singletons:` map (carrying the item-2
   sections) and in `catalog.SingletonKinds`, and a `plainSingletons` entry in
   `internal/project/singleton.go`; no `internal/config` change, so `IsSingletonKind` picks it up by
   membership with no schema touch.

2. **Section taxonomy** (five sections):
   - `overview`: the generate-from-config model: every rendered file comes from the `.awf/` tree and
     is never hand-edited.
   - `commands`: the day-to-day CLI (`sync`, `check`, `add`/`remove`, `upgrade`, `audit`, `list`,
     `new`, `changelog`, `commit-gate`); not `init`/adoption.
   - `config-and-overrides`: the `.awf/` tree (`config.yaml`, sidecars) and overriding a section
     with a *raw* convention part (ADR-0034).
   - `placeholders`: the `{{=awf:...}}` registry reference: the keys (`commitScopeList`,
     `commitScopeTable`, `commitScopeSentence`, `prefix`, `gateCmd`, `checkCmd`), the
     available-only-when-non-empty rule, and the ADR-0058 backslash escape. Because the doc is a
     template *default*, it shows the literal token via `text/template` escaping (`{{ "{{=awf:key}}" }}`).
   - `sync-and-drift`: the `sync` → `check` → gate loop and what drift means.

3. **The agent guide's `awf-setup` section becomes a pointer.** Its terse bullets stay but reference
   `working-with-awf` for the full story instead of being the sole home. Both the template default
   (`templates/agents-doc/AGENTS.md.tmpl`) and this repo's override
   (`.awf/parts/agents-doc/awf-setup.md`) update together.

4. **The doc is discoverable and linkable.** It joins `AGENTS.md`'s document map, and its path is
   exposed on `.layout` (a `WorkingWithAwf` field plus a `templateMap` key) so any artifact links it
   publication-safely.

## Invariants

- `invariant: working-with-awf-mandatory`: the `working-with-awf` doc renders as an always-on singleton
  for every project (present in `plainSingletons` and `catalog.SingletonKinds`), suppressible only
  via a `local: true` sidecar. Backed by the singleton-set test extended to assert its presence.
- **Partial-item supersedence of ADR-0043's `document-map-lists-mandatory-docs` invariant.** This
  ADR overrides that single invariant to widen its scope to `working-with-awf`: its backing test
  (`TestAgentsDocDocumentMapListsMandatorySingletonsUnconditionally`) asserts all **four** mandatory
  docs appear in the rendered `AGENTS.md` document map. This is partial-item supersedence in the
  project's lifecycle vocabulary, not amendment (which is reserved for a still-`Proposed` ADR editing
  its own body) and not full supersedence: `related: [0043]`, no status flip, and the successor prose
  names the overridden item. ADR-0043 stays `Implemented`; no invariant is retired. Its frozen text
  still enumerates the original three docs, so this successor is the authoritative record of the
  fourth.

## Consequences

- Every adopter gets a real usage guide and, in particular, an adopter-facing reference for the
  `{{=awf:...}}` placeholder registry, not just a fail-loud error message.
- The agent guide de-duplicates: `awf-setup` points at the guide rather than re-explaining, keeping
  the always-loaded guide lean.
- No migration and no version bump: adopters simply gain a new rendered file on the next `awf sync`.
- Count-drift discipline: the two live narrative sources that say "six" non-agent-guide singletons
  (`.awf/docs/parts/architecture/components.md`, `.awf/domains/parts/rendering/current-state.md`)
  move to seven; the historical "six" in ADR-0043 and the completed singleton plan stay untouched
  (append-only). The stale `singleton.go` "7th plain singleton" comment and the by-name enumeration
  at `render.go` are updated.
- Coverage (ADR-0012): the new singleton is exercised by the existing iterating singleton tests plus
  the extended set / document-map assertions; the doc content is prose with no new Go branches. The
  hand-coded `TestLayout` fixture (which enumerates the mandatory-doc paths and `templateMap` keys)
  gains the new field.
- This repo dogfoods the doc: its own `working-with-awf.md` renders and is drift-checked.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Toggleable catalog doc (adopters opt in with `awf add doc`) | Usage of awf is not optional for an adopted project; a guide every adopter needs belongs in the mandatory-singleton family, and opt-in leaves the placeholder reference undiscoverable for those who don't enable it. |
| Expand the AGENTS.md `awf-setup` section in place | Bloats the always-loaded guide with a six-key table and multi-section prose; the guide should point, not carry the whole reference. |
| Document placeholders only, in an existing doc (e.g. `doc-standard`) | Misfiles usage guidance under an authoring *standard*, and still leaves no home for the broader command/override/sync guidance an adopter needs. |
| A new internal/awf-repo-only doc, not rendered to adopters | Adopters are exactly who need the usage guide and the placeholder reference in their own repo; keeping it awf-only defeats the purpose. |
