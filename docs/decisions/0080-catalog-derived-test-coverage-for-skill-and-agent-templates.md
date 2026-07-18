---
status: Implemented
date: 2026-07-09
tags: [publication-safety, catalog-derived-tests]
related: [44, 45, 46, 50, 53, 54, 67]
domains: [rendering, tooling]
---
# ADR-0080: Catalog-derived test coverage for skill and agent templates

## Context

Three test groups hand-enumerate the skill set they cover and silently rot as
the catalog grows, recorded in `docs/pitfalls.md` ("Adding a catalog skill:
hand-enumerated test touch points") since 2026-07-06, and confirmed live on
2026-07-09:

- `TestUnsetFallbackRenders` (`internal/project/spine_test.go`), the
  publication-safety regression lock (ADR-0045), covers 5 of 16 catalog skill
  templates. Counting every conditional form after include-expansion
  (`{{ if }}`, `{{ with }}...{{ else }}`, `{{ range }}`, including conditionals
  sourced from `templates/partials/`), **11 templates carry fallback prose with
  no unset-data case at all**: their degradation is unlocked today.
- The per-skill `Test<Skill>Template` goldens are a convention with no
  enforcement; **`tdd` has no golden test** and nothing noticed.
- Chain-enabling fixtures hand-write YAML skill lists
  (`TestScopesEditReflagsReferencingArtifacts`, `internal/project/drift_test.go`);
  when `reviewing-impl` gained its unconditional `retrospective` handoff
  (ADR-0067) every such fixture had to be found by hand.

ADR-0053 already solved this shape once: the eval fixture derives its enabled
set from `catalog.Standard` so it cannot silently stop covering a new artifact.
The same derivation is owed to these sibling tests.

Two structural facts constrain the design (verified 2026-07-09):

- Unconditional `{{ .prefix }}-<skill>` cross-references are not an anomaly but
  the chain's deliberate coupling: ~10 of 19 templates name their chain
  neighbours unconditionally (plus the `plan-reviewer` agent naming
  `reviewing-plan-resync`, and every skill's own frontmatter `name:`). The
  agent guide's "disable them as a unit" rule and ADR-0046's dead-reference
  check enforce this coupling at project level, but **no machine-readable
  declaration of it exists**: a sweep banning reference residue has nothing
  to derive its exceptions from.
- An in-test exemption map re-creates the hand-enumerated list this effort
  eliminates; the user directed default-inclusion semantics instead: every new
  catalog entry is automatically covered, and any exception is an explicit
  entry that itself fails loudly when stale.

Hosting constraints: `internal/testsupport` must not import `internal/*`
(`inv: testsupport-zero-internal-deps`, ADR-0044), and non-testsupport
production code unreachable from a `main` trips the dead-code gate (ADR-0063),
so catalog-derived fixture builders live at their test call sites, not in
`testsupport` and not in `internal/catalog`.

## Decision

1. **Catalog-declared skill coupling.** `catalog.SkillSpec` and
   `catalog.TargetSpec` (agents) gain `RequiresSkills []string`: the catalog
   skills this artifact's template references unconditionally, i.e. names in
   its rendered output even when the referenced skill is not enabled. Catalog
   validation fails on a `RequiresSkills` entry that is not a catalog skill,
   and on a non-empty `RequiresSkills` on any `TargetSpec` use outside the
   agents map (the domain-doc spec shares the type; the field is meaningless
   there and a silent no-op would invite drift).
   This is the machine-readable form of the agent guide's "disable them as a
   unit" coupling; like `RequiresAgent` (ADR-0050) it is data, but unlike
   `RequiresAgent` it carries **no gated-command validation in this ADR**:
   enforcement here is test-side (Decision 2). Promoting it to `awf add`/
   `awf remove` pairing UX is deliberately deferred.

2. **Derived unset-data sweep.** A new test in `internal/project` loops every
   `catalog.Standard` skill and agent template (never a hand list) rendering
   each under empty data through the same path as the existing hand cases
   (include-expansion, section assembly, `missingkey=zero`, `assertNoLeaks`),
   with the artifact's `RequiresDoc` doc seeded (the
   `TestAllTemplatesProduceValidFrontmatter` pattern). It fails on:
   - leak residue: `<no value>`, unexecuted `{{`, residual section markers
     (via `assertNoLeaks`), and empty inline code (` `` `);
   - a `<prefix>-<skill>` reference in the output that is neither the
     artifact's own name nor in its declared `RequiresSkills`;
   - a **stale declaration**: a `RequiresSkills` entry (or any other explicit
     exemption, e.g. `proposing-adr`'s legitimate literal double-backticks)
     whose exempted residue no longer appears in the output.

   Sparse non-reference exemptions (the double-backtick case) live as explicit
   in-test entries under the same fails-when-stale rule: the Alternatives
   rejection of an "in-test exemption map" concerned the ~10-entry
   skill-reference graph specifically, which Decision 1 moves into the catalog.
3. **Conditional-case guard.** The `TestUnsetFallbackRenders` case list is
   hoisted to a package-level var, and a derived guard asserts every catalog
   skill/agent template whose **post-include-expansion** source contains any
   conditional action (`if`, `with`, `range`) has a case in that list, so
   skill-specific fallback prose stays hand-authored (only a human knows what
   the degraded prose should say) but its presence is machine-forced, with an
   error naming the missing artifact. The 11 currently-unguarded templates get
   backfilled cases in the implementation.
4. **Golden completeness guard.** A derived test asserts every catalog skill
   has a `Test<CamelCaseSkill>Template` func and every catalog agent a
   `Test<CamelCaseAgent>Agent` func in `internal/project`'s test source
   (source-scan mechanic, precedent `TestArchitectureDocNamesEveryCmd`). The
   missing `TestTddTemplate` is written as part of the implementation.
5. **Catalog-derived chain-closure fixtures.** Chain-enabling fixtures derive
   their enabled set at the call site from the catalog: the `Chain`-flagged
   skills, their transitive `RequiresSkills` closure, and the `RequiresAgent`
   agents of every skill in that combined set.
   `TestScopesEditReflagsReferencingArtifacts` switches to this derivation;
   the derived set drops `tdd` from its current hand-written list (`tdd` is
   neither `Chain` nor in any unconditional-reference closure), an acceptable
   membership delta, but the test's negative control (a skill without scope
   references is not reflagged) currently points at the rendered `tdd` skill
   and would pass vacuously once `tdd` is no longer rendered; it re-points to
   a derived-set skill that carries no scope references (e.g. `brainstorming`:
   only the reviewing-* templates reference `commitScopes`). Deliberately-minimal gating fixtures (skill-reference
   gating, doc gating, coverage fixtures) stay hand-written: minimalism is
   their point. `internal/testsupport` is untouched.
6. **Scope boundary.** The `_base` templates and `AGENTS.md.tmpl` are not
   catalog entries; their existing hand cases (including
   `inv: local-base-publication-safe`) remain as-is. The eval suite's
   hand-listed handoff pairs (`TestWorkflowChainHandoffs`) assert direction and
   invocation phrasing (richer than `RequiresSkills` membership) and are out
   of scope.
7. **Default-inclusion exemption semantics.** Across every guard in this ADR:
   a new catalog entry is always covered automatically; any exception is an
   explicit entry (a `RequiresSkills` declaration or an in-test exemption)
   that itself fails the guard when stale. No guard may grow an implicit or
   silently-skipped exclusion.

## Invariants

- `invariant: catalog-template-sweep`: every `catalog.Standard` skill and agent
  template is rendered under empty data by a catalog-derived loop that bans
  leak residue and undeclared skill-reference residue; the loop's artifact set
  derives from the catalog, never a hand-maintained list.
- `invariant: requires-skills-exact`: `RequiresSkills` declarations are exact:
  an unconditional skill reference missing from the declaration and a declared
  entry no longer present in the rendered output both fail the sweep.
- `invariant: conditional-fallback-case-guard`: every catalog template whose
  post-expansion source contains a conditional template action has a
  hand-authored unset-data case in the fallback case list.
- `invariant: golden-test-completeness`: every catalog skill and agent has its
  per-artifact golden test function in `internal/project`.

## Consequences

- The commit that flips this ADR to Implemented also adds the four new
  invariant bullets to the agent guide's Invariants section (via the
  `.awf/agents-doc.yaml` data + `./x sync`) and regenerates
  `docs/decisions/ACTIVE.md` (`./x sync`), per standing convention.
- Adding a catalog skill now fails loudly and locally at authoring time
  (pointed test failures name the missing golden, the missing fallback case,
  and any undeclared reference) instead of silently shipping an unlocked
  template. The `docs/pitfalls.md` entry converts from "known hole" to a
  description of the guards.
- The chain's coupling graph becomes catalog data, unlocking (but not yet
  committing to) `awf add`/`awf remove` skill-pairing UX and a possible future
  derivation of the eval suite's handoff pairs.
- ~11 backfilled fallback cases and `TestTddTemplate` must be authored by a
  human reading each template's degraded output: the one part that stays
  manual, now with its completeness machine-forced.
- The sweep double-covers ground `assertNoLeaks` hand cases already cover;
  accepted: it is the derived loop that closes the gap, the hand cases carry
  the prose-level assertions.
- `RequiresSkills` without gated-command validation means a project can still
  enable a skill without its referenced siblings and only learn at `awf check`
  time (ADR-0046 dead-reference check), unchanged from today; the field only
  feeds tests until a future ADR promotes it.
- Risk: the conditional-action scan and the reference-residue scan are
  heuristic (regex over template source / rendered output). A false negative
  is possible for exotic template constructs; accepted: the scan is a guard
  on top of, not a replacement for, review discipline.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Fallback want-phrases as catalog metadata (full derivation) | Moves test scripts into the production catalog; inverts ownership: the catalog describes artifacts, it does not script their tests. |
| Generic-only sweep, hand case list unguarded | Does not close the pitfall: a new conditional template still lands with its degradation unlocked. |
| In-test exemption map for unconditional references | With ~10 coupled templates it re-creates the hand-enumerated reference graph the effort eliminates; catalog declaration makes the coupling first-class data. |
| `{{ if `-only conditional guard on the raw skill file | Misses the dominant `{{ with }}...{{ else }}` fallback form and partial-sourced conditionals: 7 of the 11 unguarded templates would stay invisible. |
| Catalog-derived config builder in `internal/testsupport` | Violates `inv: testsupport-zero-internal-deps` (ADR-0044); hosting it in `internal/catalog` trips the dead-code gate (ADR-0063). Call-site derivation needs no new home. |
| Extend `RequiresAgent`-style hard validation to `RequiresSkills` now | Gated-command enforcement changes adopter-facing behaviour and deserves its own decision once the declarations have proven exact in tests. |
