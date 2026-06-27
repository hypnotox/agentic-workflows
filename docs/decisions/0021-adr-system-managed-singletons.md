---
status: Accepted
date: 2026-06-27
supersedes: []
superseded_by: ""
tags: [rendering, adr-system, singleton]
related: [0004, 0009, 0011, 0015, 0016, 0020]
domains: [rendering, adr-system]
---
# ADR-0021: Scaffold the ADR-System Files as Managed Singletons

## Context

awf's templates reference two ADR-system static files that awf does not render: the agent guide's
document map, the workflow doc, and the `brainstorming` skill cite `.layout.adrReadme`
(`<docsDir>/decisions/README.md`); `proposing-adr` cites `.layout.adrTemplate`
(`<docsDir>/decisions/template.md`) as the skeleton to copy. Both files exist in awf's own repo only
because they were hand-authored; `internal/adr` skips them when parsing the decisions dir. An
adopter's repo has neither, so every reference is broken and `awf init` leaves the ADR workflow
non-functional out of the box.

ADR-0020 (the dead-reference gate, Accepted) makes this concrete: its check requires the README
markdown links to resolve. They resolve for awf (hand-authored file on disk) but not for adopters.
Making the README a managed artifact resolves them for everyone, so ADR-0021 is its prerequisite.

The `agents-doc`/AGENTS.md singleton (ADR-0004) is the established pattern for an always-on,
fixed-path, section-overridable rendered artifact. The grounding-check confirmed both layout
outpaths (`adrReadme`, `adrTemplate`) exist and the wiring mirrors cleanly.

## Decision

1. **Render two always-on neutral singletons**, mirroring the agents-doc singleton (ADR-0004):
   `adr-readme` → `.layout.adrReadme`, `adr-template` → `.layout.adrTemplate`. Each is emitted by a
   `RenderAll` `renderTarget` call (out-path from `layout`), suppressed only when its sidecar sets
   `local: true`. Neither is in an enable array. Sections: adr-readme
   `[intro, when, naming, frontmatter, invariants, active-md]`; adr-template `[frontmatter, body]`.

2. **Generalize singleton sidecar/part handling** from the agents-doc special case to a singleton
   set `{agents-doc, adr-readme, adr-template}`: `config.Sidecar` reads `.awf/<kind>.yaml`, and
   `config.PartPath` / `project.partRel` resolve `.awf/parts/<kind>/<section>.md` (singletons live at
   the `.awf/` root, per the ADR-0009 tree layout).

3. **Add catalog specs and validation.** New catalog `adrReadme`/`adrTemplate` entries (a
   `SkillSpec` each, `sections` only); embed the two new template dirs; add a `validateAgainstCatalog`
   block per singleton (sidecar section-override validation, mirroring agents-doc).

4. **Ship generic content, dogfood with overrides.** The templates carry content generalized from
   awf's hand-authored files: command references as `awf check`/`awf sync`/`awf invariants`, paths via
   `.layout`. awf adopts the singletons and overrides the README sections that cite commands
   (`invariants`, `active-md`) with `./x` convention parts; `template.md` ships generic. `sync`
   overwrites awf's hand-authored files with the reproduced rendered content.

5. **Add section-parity tests** for both singletons (catalog `sections` equal the template's
   `awf:section` markers; renders without `<no value>`), mirroring the domain-doc parity test. The
   pre-existing agents-doc parity gap is out of scope.

## Invariants

- `inv: adr-system-singletons-rendered` — `RenderAll` emits `<docsDir>/decisions/README.md` and
  `<docsDir>/decisions/template.md` from their always-on singletons, and omits each when its sidecar
  sets `local: true`.
- `inv: adr-singleton-section-parity` — each singleton's catalog `sections` equal its template's
  `awf:section` markers, and it renders without an unresolved-variable placeholder.
- The two singletons are not ADRs: `internal/adr` skips `README.md` and `template.md` when parsing
  the decisions dir (textual; pre-existing).

## Consequences

- The ADR workflow works out of the box for adopters: README and `template.md` exist, and every
  `.layout.adrReadme` / `.layout.adrTemplate` reference resolves. This unblocks ADR-0020 — the
  dead-reference gate then ships green for adopters, not just for awf.
- awf's hand-authored README/`template.md` become awf-rendered; `sync` replaces them with the
  reproduced content. The templates plus awf's `./x` convention parts must reproduce the current
  text section by section — verified during implementation, with a content diff guarding against
  silent loss.
- A fresh adopter with a pre-existing hand-authored README or `template.md` hits the `awf init`
  collision guard (ADR-0016) — correct; they resolve it with `--force` or `local: true`.
- `sync` writes managed outputs unconditionally — only `awf init` has the collision guard. An
  existing adopter who hand-authored a `docs/decisions/README.md` or `template.md` before this
  change has it overwritten on the first post-upgrade `sync`; this is consistent with the
  managed-file model (a hand-edit at a managed path is drift by design, ADR-0004) — they preserve a
  divergent file with `local: true` or section overrides.
- The lock/manifest format is unchanged: each singleton is an ordinary path→hash entry, as the
  agents-doc and docs renders already are. The `internal/migrate` agents-doc special-casing needs
  no parallel handling — `adr-readme`/`adr-template` never existed in a prior schema, so no upgrade
  migration is owed; the singletons simply begin rendering on the next `sync`.
- The rendered `template.md` carries the `GENERATED by awf` provenance banner (ADR-0015);
  `proposing-adr` copies section structure and writes a fresh file (it never shell-copies
  `template.md`), so the banner never lands in a real ADR.
- Two more always-on artifacts enter the rendered/lock set and the `--force`/`local` surface.

Doc-currency obligations the implementing commit(s) must satisfy:

- The status flip to `Implemented` regenerates `docs/decisions/ACTIVE.md` via `./x sync`.
- awf adopting the singletons re-renders `docs/decisions/README.md` and `docs/decisions/template.md`,
  staged with the change.
- Adding the singletons materially shifts the `rendering` and `adr-system` current states, so both
  domain narratives are refreshed in the implementing range.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| De-link the `adrReadme` references instead of scaffolding | Leaves the ADR workflow non-functional for adopters (`proposing-adr` still cites a missing `template.md`); shipping the files is the complete fix. |
| Scaffold the README only | `proposing-adr` references `template.md` too; both are awf-given ADR-system files awf does not ship. |
| Model them as catalog docs (docs taxonomy, ADR-0011) | Docs render to `<docsDir>/<name>.md`, not the `decisions/` sub-path — wrong path shape. |
| Static singletons without section override | Adopters need to customize the ADR guide and skeleton; per-section override plus `local` suppression matches the agents-doc pattern (ADR-0004). |
