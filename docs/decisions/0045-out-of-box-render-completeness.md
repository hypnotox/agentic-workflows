---
status: Implemented
date: 2026-07-01
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [rendering, catalog, publication-safety, adoption]
related: [1, 6, 11, 12, 22, 29, 39]
domains: [rendering, config]
---
# ADR-0045: Out-of-box render completeness

## Context

A fresh `awf init` produces degenerate rendered artifacts. The failure is by construction,
not by accident:

- `ScaffoldConfig` deliberately seeds every referenced var as `""`
  (`internal/project/scaffold.go`, `invariant: scaffold-seeds-all-vars`) so the strict
  render passes on the silent path ([ADR-0029](0029-interactive-agent-prefillable-init.md)
  chose silent-path-seeds-empty), and every catalog var descriptor carries `default: ""`.
- The publication-safety net ([ADR-0001](0001-template-overlay-rendering-engine.md)) is a
  single `strings.Contains(content, "<no value>")` check
  (`internal/project/render.go:275-277`). With `missingkey=zero`, a *missing* key prints
  `<no value>` (caught), but a var seeded as `""` prints nothing (uncaught) and a `range`
  over an absent `.data` key iterates zero times (uncaught).
- All `.data.*` content lives in optional sidecar YAMLs (`.awf/<kind>/<name>.yaml`) that
  `init` never scaffolds and the adopter docs barely mention. The catalog has no default-data
  mechanism (`internal/catalog/catalog.go`: `SkillSpec{Sections, RequiresDoc, Core}` — nothing
  else).

The observable result in a default install: `proposing-adr` renders "**Commit everything in
one commit.** Format: ``." and "**Required sections:** in that order."; `adr-lifecycle`
renders a state table with a header and zero rows; `writing-plans` renders "`` (~) runs
before every commit"; the reviewer agents render doc-currency checklists over empty lists.
A sweep counts 22 unguarded `{{ .vars.* }}` prose interpolations across 13 templates and 9
unguarded `range .data.*` blocks. `awf check` reports clean on all of it — the drift oracle
validates render fidelity, not config completeness.

Grounding discoveries that shape the design:

- `artifactConfigHash` (`internal/project/confighash.go:30-54`) hashes prefix, layout,
  referenced vars, the raw sidecar struct, and consumed part bytes — catalog content appears
  nowhere, and `checkLockedFiles` never byte-compares a fresh render against disk when hashes
  match. Catalog-supplied default data that is not folded into the hash would change rendered
  output at sync time while `check` reports clean.
- `renderTarget` and `artifactConfigHash` receive the same sidecar value in `renderKind`
  (`internal/project/render.go:114` and `:279`), so merging catalog defaults into that shared
  value covers both consumers at one site — with one exception: `agents-doc` resolves its
  sidecar outside `renderKind` (`internal/project/render.go:169-181`), so the merge must be a
  helper applied at both sidecar-resolution sites, not code inlined in `renderKind`.
- Default content has precedent: the docs module ships full per-section default bodies
  ([ADR-0011](0011-docs-default-content-and-section-taxonomy.md)); the unshipped remainder is
  exactly the data-driven content of skills, agents, and the singletons.
- `awf check` findings are uniformly failing (`cmd/awf/check.go:32-42`); the only non-failing
  output precedent is the "binary ahead" note line (`cmd/awf/check.go:16-19`).
- ADR-0001's invariant "Required vars declared in `catalog.yaml` are validated before
  rendering, not discovered by inspecting render output" was never implemented — no
  required-var mechanism exists anywhere. This ADR must reckon with that bullet explicitly
  rather than silently contradict it.
- The catalog loader uses non-strict `yaml.Unmarshal` (`internal/catalog/catalog.go:77`), so
  a new `data:` key parses without breaking older binaries — though the binary-version gate
  ([ADR-0039](0039-binary-version-compatibility-gate.md)) governs that seam anyway.
- Universal-standard content is separable from project-specific content: awf's own
  `.awf/skills/adr-lifecycle.yaml` (`adrStates`) and the `adrSections` key of
  `proposing-adr.yaml` describe the standard itself and belong to every adopter; awf's
  reviewer sidecars (`focusItems`, `docCurrencyItems`, `correctnessTraps`) name awf files
  and stay project-specific overrides.

The user chose graceful degradation over the two loud alternatives (visible TODO tokens,
required-at-init): the render must always be coherent, with `awf check` nudging toward
configuration.

## Decision

1. **Catalog default data.** `templates/catalog.yaml` gains an optional per-artifact `data:`
   block (skills, agents, and the sidecar-capable singletons), parsed into the catalog specs.
   Universal-standard content ships as catalog defaults: the ADR lifecycle states, the ADR
   required-section list, and freshly-authored *generic* defaults for the reviewer focus and
   doc-currency lists, TDD test surfaces, and ADR triggers. Generic means written for any
   project — never a copy of awf's own sidecar values, which remain as this repo's overrides.

2. **Sidecar-overrides-default merge.** The effective render data for an artifact is the
   catalog default overlaid by the sidecar per top-level key: a key absent from the sidecar
   falls through to the catalog default; a key *present* in the sidecar — even with a null or
   empty value — replaces the default entirely (the explicit off-switch). No deep merging.
   The merge happens once per artifact at sidecar resolution — a single helper applied in
   `renderKind` and in the direct `agents-doc` render path — upstream of both `renderTarget`
   and `artifactConfigHash`, so catalog default data participates in the config hash and a
   catalog-data change flags the artifact stale exactly like a template change.

3. **Graceful-fallback contract.** Every var or data interpolation in running prose must be
   guarded so an unset value degrades to coherent generic prose (`{{ with .vars.gateCmd }}run
   `{{ . }}`{{ else }}run the project's gate command{{ end }}`), never an empty inline code
   span, a zero-row table, or a dangling list-introduction sentence. This extends ADR-0001's
   publication-safety contract from "no unresolved-value token" to "coherent prose at every
   configuration level". It applies to frontmatter `description` values as well, which must
   stay non-empty ([ADR-0006](0006-frontmatter-parser-and-skill-validation.md)). This item
   **supersedes ADR-0001's "required vars validated before rendering" invariant bullet**
   (recorded via `related`; ADR-0001 stays `Accepted`): no required-var mechanism will exist —
   completeness is advisory, not validation. The implementing change updates the AGENTS.md
   publication-safety invariant entry (the ADR-0001 line in `.awf/agents-doc.yaml`
   `data.invariants`) to the extended contract, citing this ADR, in the same commit.

4. **Render-completeness advisory.** `awf check` prints a non-failing, note-style line per
   enabled artifact that references unset vars — derived statically from the template's
   referenced-vars set intersected with empty config values, following the existing
   "binary ahead" note precedent. The exit code is unaffected; `./x gate` behaviour is
   unchanged. No severity model is introduced — findings remain uniformly failing, notes
   remain uniformly informational.

## Invariants

- `inv: empty-init-coherent-render` — a non-interactive `awf init` with no answers renders
  artifacts containing no empty inline code spans, no tables without body rows, and no
  list-introduction sentences followed by nothing.
- `inv: catalog-data-in-confighash` — a change to an artifact's catalog default data changes
  its lock `configHash`, so `awf check` flags the artifact stale.
- `inv: sidecar-key-overrides-default` — a sidecar data key that is present (including null
  or empty) fully replaces the catalog default for that key; an absent key falls through to
  the default.
- `inv: completeness-advisory-nonfailing` — unset-var notes never affect `awf check`'s exit
  code.
- `inv: catalog-defaults-generic-denylist` — no catalog default data value contains `./x` or
  `hypnotox/agentic-workflows` (mechanical backstop; the full generic-content contract remains
  a textual contract, audited at review).

## Consequences

Easier:
- A default install renders coherent, usable instructions at every configuration level; the
  universal parts of the standard (ADR states, required sections) are maintained centrally in
  the catalog instead of being every adopter's homework.
- Unset configuration becomes visible (`check` notes) without becoming blocking, matching the
  language-agnostic adoption posture ([ADR-0022](0022-curated-init-default.md)).

Harder / accepted trade-offs:
- Fallback prose hides missing configuration from an adopter who never reads `check` output;
  the advisory is the only nudge. Accepted — the loud alternatives were rejected by design.
- awf's own sidecars override several defaults, so this repo stops exercising the generic
  default content in its own rendered output; golden tests must render the defaults directly.
- Template churn across 13+ templates; every guarded site needs unset/set golden coverage
  under the 100% gate ([ADR-0012](0012-full-coverage-gate-and-conventions.md)).
- Existing adopters see every data-bearing artifact flagged stale after upgrading (the config
  hash gains catalog data); `awf sync` resolves it — the normal upgrade path per ADR-0039.
- Advisory notes are unsuppressible: an adopter who deliberately stays at the generic level
  sees the same unset-var notes on every `awf check`. Accepted — a suppression knob, if the
  noise proves real, is its own ADR.
- No config-schema migration: the config tree's shape is unchanged (schema generation stays
  at 6). Sidecar files keep their exact syntax; only their semantics gain a fall-through.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Visible `TODO(awf: set vars.X)` tokens in the render | Loud, but ships degraded prose agents read until configured; user chose coherent-always. |
| Required vars at init (implementing ADR-0001's bullet) | Breaks the try-it-fast path and the ADR-0029 silent-empty seeding contract; unhelpful for existing half-configured trees. |
| Warning-severity field on check findings | Introduces a severity model for one consumer; note-lines follow an existing precedent. If more advisories accumulate, a severity model is its own ADR. |
| Detecting fired fallbacks by scanning rendered output | Generic fallback prose is by design indistinguishable from intentional prose; referenced-vars ∩ empty-config is statically derivable and needs no render instrumentation. |
| Keep data sidecar-only, document it better | Universal-standard content (ADR states, section lists) is not project configuration; documentation cannot fix a wrong ownership boundary. |
| Scaffold default sidecar files at `awf init` | Freezes universal content as per-project copies at init time; upgrades cannot improve them and every adopter's copy diverges — the ownership boundary stays wrong, just better hidden. |
