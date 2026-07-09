---
status: Implemented
date: 2026-07-09
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [config, catalog, vars, init]
related: [2, 29, 51, 80]
domains: [config]
---
# ADR-0084: Catalog vars carry functional values only

## Context

The catalog's `vars:` descriptor block (ADR-0029) declares twelve string descriptors (plus two
catalog-trim multiselects, `skills`/`docs`, outside this ADR's scope). Measured by template
consumption, the string descriptors fall into two populations. The functional vars are interpolated widely
and carry values the rendered artifacts or the tooling execute or enforce: `gateCmd` (21
references across 14 template files), `gateCmdFull` (11), `checkCmd` (9), `activeMdRegenCmd` (7),
`testCmd` (3), `commitGateCmd` (2, the hook payload's command), `invariantTestPath` (2), and
`commitScopes` (routed into `audit.allowedScopes` per ADR-0051, enforced by `awf commit-gate`).
The other four are single-consumer prose knobs — each tunes the wording of exactly one sentence
in one template:

- `docCurrencyTargets` — the bugfix skill's doc-currency rule; fallback "the project's docs".
- `adrProposeCommitFmt` — the proposing-adr skill's commit-subject format; fallback
  `docs(adr): propose NNNN <short title>`. This repo's own configured value is byte-identical to
  the fallback — the knob has never expressed anything.
- `gateDuration` — an optional "(~15s)" parenthetical in the writing-plans gate-cost note; the
  sentence's point (the gate is cheap, batch related changes) stands without it.
- `modulePrefix` — the refactor-coupling-audit skill's grep example. It interpolates into a
  runnable command, but the command is illustrative, nothing enforces it, and the executing agent
  derives the real prefix trivially from `go.mod`/`package.json` at runtime; its fallback is the
  `<module-prefix>` placeholder that says exactly that.

Every one of these degraded fallbacks is already reviewed, publication-safe prose (ADR-0001,
ADR-0045), and for three of the four templates the degraded output is pinned verbatim by
ADR-0080's hand-authored unset-data fallback cases. Meanwhile each descriptor costs an `awf init`
question and a line of config surface, against the project owner's stated direction: setup should
be simple, and small things easily expressed with generic prose or pointers to awf-owned paths
should not get a knob — an adopter who wants more concrete wording overrides the section with a
convention part.

Two structural facts shape the removal. `TestVarDescriptorParity` (backing ADR-0029's
`inv: var-descriptor-parity`) is bidirectional — an unreferenced descriptor fails the gate just
as an undescribed template var does — so descriptor deletion and template edits are atomic by
construction. And the `vars:` config block is a freeform map with no unknown-key validation
anywhere in init, sync, check, upgrade, or audit, so leftover keys in adopter configs are inert.

## Decision

1. **Policy: a catalog var descriptor exists only for a functional value** — one the rendered
   artifacts or the awf tooling execute or enforce: a command (`gateCmd`, `testCmd`, `checkCmd`,
   `commitGateCmd`, `activeMdRegenCmd`, `gateCmdFull`), an enforced identifier set
   (`commitScopes`), or a structural path (`invariantTestPath`). A value that only tunes prose
   wording never gets a descriptor; the convention-part mechanism is the customization path for
   prose. Reintroducing a prose knob requires a successor ADR.
2. **Remove the four prose-knob descriptors** — `docCurrencyTargets`, `adrProposeCommitFmt`,
   `gateDuration`, `modulePrefix` — from `catalog.Standard`'s `vars:` block.
3. **Replace each template's conditional by fallback promotion**: the existing `{{ else }}`
   branch (for `gateDuration`, the empty degradation — the parenthetical disappears) becomes the
   unconditional text in `skills/bugfix`, `skills/proposing-adr`, `skills/writing-plans`, and
   `skills/refactor-coupling-audit`. No new prose is authored. These edits land in the same
   commit as item 2 (`var-descriptor-parity` forces atomicity).
4. **No schema bump, no migration.** The `vars:` block stays freeform; leftover keys in adopter
   configs are inert dead config. The changelog documents the adopter-visible effects (see
   Consequences).
5. **This repo drops the four entries from its own `.awf/config.yaml`** and accepts the promoted
   generic prose — no compensating convention parts. If a concrete pointer proves worth having
   back (e.g. the bugfix skill naming `AGENTS.md`), the fix is a part under
   `.awf/skills/parts/<skill>/`, not a var. The implementation commit flips this ADR's status and
   regenerates `docs/decisions/ACTIVE.md` via `./x sync`.

## Invariants

- Every catalog var descriptor names a value the rendered artifacts or the awf tooling execute
  or enforce; no descriptor exists solely to tune prose wording. (Textual contract — the
  functional/prose distinction is a judgment ADR review applies, not a machine check.)
- `inv: var-descriptor-set-pinned` — the catalog's string var descriptor keys are exactly the
  eight functional keys this ADR enumerates (`gateCmd`, `gateCmdFull`, `checkCmd`,
  `commitGateCmd`, `testCmd`, `commitScopes`, `activeMdRegenCmd`, `invariantTestPath`), pinned by
  a test whose set is extended only by a successor ADR (the ADR-0082 pinned-list pattern). The
  pin makes any descriptor change — reintroducing a prose knob included — a deliberate,
  ADR-recorded act; the functional/prose judgment itself stays with review.
- ADR-0029's `inv: var-descriptor-parity` continues to hold unchanged and is what makes a
  half-landed removal (descriptor without template edit, or vice versa) mechanically impossible.

## Consequences

- `awf init` asks four fewer questions; the scaffolded `config.yaml` seeds four fewer vars.
- **Adopters who set these values lose the customization silently**: nothing warns when a config
  var stops being template-referenced, so the first post-upgrade `awf sync` rewrites the affected
  skills to the generic prose with only ordinary drift output. Accepted deliberately — the
  degraded prose is coherent by the publication-safe invariant — and documented in the changelog
  with the convention-part migration path.
- **Saved init answers break on fresh scaffolds**: `initspec.Resolve` hard-errors on an answer
  key matching no descriptor, so an answers file or `--set` carrying a removed key fails
  `awf init` on a new project (on an existing project, init ignores it with a printed note).
  Changelog-documented.
- The refactor-coupling-audit grep example loses this repo's concrete module path — the most
  functionally lossy of the four sites; the executing agent reads `go.mod` instead.
- ADR-0002's instruction to "bump `gateDuration`, then re-sync" becomes historical prose;
  ADRs are append-only, so it is noted here rather than edited there.
- The freeform `vars:` map remains legal, so an adopter's own convention parts may still
  interpolate adopter-invented keys; what this ADR removes is only the catalog's advertisement.
- ADR-0080's fallback-case guard entries for these templates stay valid: all four templates
  retain other conditionals, and the pinned unset-data phrases are exactly the promoted prose.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Layout-pointer upgrade (cite `.layout.workflowRef` etc. in the promoted prose) | New wording to author and review for marginal concreteness; a project wanting concrete pointers gets them via a convention part. |
| Keep the template conditionals, remove only the descriptors | Leaves invisible magic keys — undocumented vars that still change rendered output; worse than either a knob or no knob. |
| Schema migration stripping leftover keys from adopter configs | A generation bump and migration to delete inert map entries is cost without benefit; dead keys are harmless and self-documenting to remove. |
| Warn on leftover removed-catalog keys at sync/check | Requires the binary to carry a removed-key history, and would false-positive on adopter-invented keys that adopter-authored convention parts legally interpolate (freeform `vars:` stays a feature). |
| Keep the knobs (status quo) | Each costs an init question and config surface for a sentence's wording; contradicts the simplicity direction and the convention-part mechanism's role. |
