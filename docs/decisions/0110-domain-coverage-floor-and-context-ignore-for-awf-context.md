---
status: Proposed
date: 2026-07-13
supersedes: []
retires_invariants: [uncovered-lists-unowned-only]
superseded_by: ""
tags: [context, domains]
related: [77, 86, 88, 102, 104, 109]
domains: [tooling, config]
---
# ADR-0110: Domain-Coverage Floor and Context-Ignore for awf context

## Context

`awf context --uncovered` (ADR-0102) scans the tracked tree and reports every path owned by no
configured domain glob — the report that finds the unowned code a relevance query cannot reach. A
live run against this repo lists a broad unowned set: ~10 internal code packages (`internal/clispec`,
`configspec`, `frontmatter`, `git`, `initspec`, `pathglob`, `plan`, `project`, `refs`, `testsupport`)
*and* non-code paths (`docs/`, `examples/`, `.github/`, `.awf/`, `LICENSE`, `go.mod`, …). While
anything is uncovered, an uncovered entry is noise; only once the report reaches **zero** does a
newly-appearing unowned path become a real signal (a new code package with no domain home). The goal
is a clean coverage floor for awf's own tree.

The unowned paths are two genuinely different kinds, and conflating them is the trap:

- **Unowned code** that *should* have a domain home — the ~10 internal packages. A domain owns file
  territory, so these fold into domains.
- **Non-domain paths** that *no* domain should own — `LICENSE`, `go.mod`, the config tree, the docs
  tree, generated output. A domain has file territory *and* a generated current-state doc; `go.sum`
  has neither. Forcing a domain to "own" them strains what a domain is.

Grounding fixed the mechanics and boundaries:

- **awf already knows every path it renders.** `Project.PlannedOutputs()` returns every `RenderAll`
  output plus the generated `ACTIVE.md`, domain docs, and config reference (a read-only in-memory
  render, no writes). The rendered adapter trees (`.claude/**`, `.cursor/**`), `AGENTS.md`/`CLAUDE.md`,
  and the generated docs are *derived artifacts* — folding them into a hand-maintained ignore glob
  would be wrong and would go stale the moment a target or artifact is added. They should auto-exclude
  and stay correct with no hand edit.
- **Domains have no exclude concept.** `Uncovered` builds its match set purely from domain sidecar
  `paths:`; a path is covered iff a domain glob matches it. A non-code or test-support path can only
  leave the report through an *explicit* ignore, never a domain.
- **The five-domain / five-scope mirror is load-bearing.** ADR-0055's convention mirrors awf's five
  commit-scopes to its five domains by hand; a sixth domain breaks the mirror unless a scope is added
  too. Every uncovered code package folds naturally into one of the five, so no sixth domain is
  needed.
- **One glob dialect.** Domain `paths:` and the new ignore key share the anchored doublestar dialect
  (`internal/pathglob`, ADR-0077): `internal/plan/**` is any-depth, top-level files match by name.
- **A new top-level config key is not free.** It requires a `configspec` entry and a
  `config-reference.md` regeneration in the same commit (ADR-0088), but being optional and absent-safe
  it needs no schema-generation bump (ADR-0049), and being a Go-consumed top-level key it is outside
  ADR-0086's authored-but-unconsumed rule (which keys off `vars:`/`data:`).
- **Widening an Implemented invariant's contract needs a slug rename.** `uncovered-lists-unowned-only`
  changes meaning (the reported set now also subtracts generated and ignored paths), and
  `DeclaringADRs` rejects a same-name re-declaration before retirement runs, so it is retired and
  renamed (ADR-0031).

## Decision

1. **Fold every uncovered code package into one of the five existing domains** by extending its sidecar
   `paths:` — **no sixth domain**, preserving the ADR-0055 domain/scope mirror:
   - `rendering`: `internal/project/**`, `internal/refs/**`
   - `config`: `internal/configspec/**`, `internal/pathglob/**`
   - `tooling`: `internal/clispec/**`, `internal/initspec/**`, `internal/git/**`
   - `adr-system`: `internal/plan/**`, `internal/frontmatter/**`

   (`internal/invariants` is already owned by the `invariants` domain.) Some folds are a deliberate
   stretch — `internal/plan` and `internal/frontmatter` under `adr-system` (decision-artifact parsing;
   `frontmatter`'s importers are `adr`/`plan`/`audit`/`project`, predominantly artifact machinery),
   `internal/project` and `internal/refs` under `rendering` (the render/sync orchestration core) —
   accepted as the cost of not fracturing the mirror.

2. **Auto-exclude awf's own generated outputs.** A tracked path present in `PlannedOutputs()` is never
   reported by `--uncovered`. The rendered adapter trees and generated docs drop out, and the
   exclusion stays correct as targets and artifacts change with no hand edit.

3. **Add an absent-safe top-level config key `contextIgnore`** — a list of anchored doublestar globs
   (ADR-0077 dialect) naming genuinely non-domain source that no domain should own: `.awf/**` (config
   source), `docs/**` (ADRs and prose), `examples/**` (the example adopter's own module), `.github/**`,
   `.githooks/**`, `changelog/**`, `internal/testsupport/**` (test support the coverage/deadcode gates
   already special-case; not domain territory), and the top-level non-code files (`LICENSE`, `go.mod`,
   `go.sum`, `README.md`, `codecov.yml`, and the `.golangci.yml` / `.goreleaser.yaml` / `.gremlins.yaml`
   / `.gitignore` config). A tracked path matched by a `contextIgnore` glob is never reported; an
   absent or empty list is inert.

4. **`--uncovered` reports a tracked path iff** it is matched by no domain glob, **and** is not in
   `PlannedOutputs()`, **and** is matched by no `contextIgnore` glob. Over awf's own tree the reported
   set is empty; a future unowned code file surfaces as a genuine signal.

5. **Retire and rename the lister invariant, and register the config key.** `uncovered-lists-unowned-only`
   is retired and a renamed successor carries the widened contract; the collapse and output-parity
   invariants (ADR-0102) compose on the new set unchanged. `contextIgnore` gains its `configspec`
   entry and `config-reference.md` is regenerated in the same commit, which also updates the agent
   guide's invariant entry (sourced from `.awf/agents-doc.yaml`) — renaming the bullet to
   `uncovered-lists-unowned-unignored`, widening its contract, and re-citing ADR-0110 — re-syncs
   `AGENTS.md`, and regenerates `docs/decisions/ACTIVE.md` via `./x sync` at the eventual
   Proposed→Implemented status flip.

## Invariants

The slug below is backed by a `// invariant: <slug>` proof marker on a test in the implementing
commit; `awf check` enforces it once this ADR is `Implemented`. The retired slug's proof marker
(`internal/project/context_test.go`) and its advisory `touches-invariant` marker (the `Uncovered`
docstring in `internal/project/context.go`) are both removed and re-homed to the renamed successor in
the same commit.

- `` `invariant: uncovered-lists-unowned-unignored` `` (replaces `uncovered-lists-unowned-only`) — in
  `--uncovered` mode `awf context` reports exactly the scanned git-tracked paths (under the given scan
  roots, or the whole tree) that are matched by no configured domain glob, are not in the project's
  `PlannedOutputs()` set, and are matched by no `contextIgnore` glob; every such uncovered path is
  represented by exactly one reported entry (itself or a reported ancestor directory), and no
  domain-owned, generated, or `contextIgnore`-matched path is represented by any entry. Its proof
  includes the absent/empty-`contextIgnore` case (the key is additive-only: an absent or empty list
  subtracts nothing and changes no other behaviour, a configured one subtracts exactly its glob
  matches), so the feature is publication-safe without a separate slug.

The ADR-0102 invariants `uncovered-collapses-directories` and `uncovered-output-parity` are unchanged:
directory collapse and human/`--json` parity operate on whatever the reported set is, and that set is
now the narrowed one above.

## Consequences

- **`--uncovered` reaches zero on awf's tree, turning it into a real signal.** A new code package
  without a domain home, or a stray unowned tree anywhere, surfaces immediately instead of hiding in a
  standing wall of non-code noise.
- **Generated-output exclusion is maintenance-free.** Because it rides `PlannedOutputs()`, adding a new
  target (its rendered tree) or a new rendered doc auto-excludes with no edit to the ignore list — the
  robustness the hand-glob alternative could not offer.
- **The domain/scope mirror is preserved.** No sixth domain and no new commit scope; the price is two
  slightly-stretched folds (`internal/plan`→`adr-system`, `internal/project`→`rendering`), judged
  cheaper than fracturing the ADR-0055 convention.
- **`contextIgnore` is a small, explicit, git-auditable "not domain territory" statement.** It must be
  extended only when a genuinely-new non-code top-level tree appears (rare), and each entry is visible
  in review rather than buried in scan-root defaults.
- **`--uncovered` now runs a render pass** (`PlannedOutputs` → `RenderAll`) — heavier than a pure path
  scan, but read-only and acceptable for an advisory, non-gated query. It also inherits `RenderAll`'s
  failure modes: a render error (a malformed ADR, a corrupt sidecar) now fails `--uncovered` where
  before it was a near-pure path scan — bounded, since `awf check` is already red in that state.
- **The folds enroll the new packages in each domain's advisory surface.** Editing `internal/project`
  or `internal/refs` (rendering), or `internal/plan` or `internal/frontmatter` (adr-system), can now
  raise an ADR-0077 domain-code-staleness Warning absent a current-state co-update, and enlarges each
  domain's current-state-doc scope. Advisory only; never changes the audit exit code.
- **This ADR's own `tags: [context, domains]` are re-tagged under ADR-0109's re-curated vocabulary**
  in the shared implementation sequence; both are narrow, non-domain topics expected to survive the
  re-curation, so the two ADRs must land their vocabulary and frontmatter together.
- **`internal/testsupport` is excluded via `contextIgnore`, not a domain** — consistent with the
  coverage and dead-code gates that special-case it by name; it is production-imported (not test-only
  in the Go sense) yet is not domain territory.
- **The example adopter is excluded wholesale (`examples/**`).** Whether `examples/sundial` drives its
  *own* coverage to zero is left to its own config, out of scope here.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| One hand-maintained ignore glob covering the rendered trees too | Folds derived artifacts in with unowned source and goes stale as targets/artifacts change; `PlannedOutputs()` auto-exclusion stays correct for free. |
| A sixth domain (e.g. `project`) for `internal/project` | Breaks the ADR-0055 five-domain / five-scope mirror and adds a current-state doc to author; every orphan folds acceptably into the five. |
| Restrict `--uncovered`'s default scan to code roots (`internal/**`, `cmd/`, `x`) | Loses the whole-tree reach that can surface a stray unowned tree anywhere; explicit ignore + generated auto-exclude keeps the reach and still reaches zero. |
| Cover literally everything with domains, including docs and config files | Strains the domain concept — a domain owns file territory and a generated current-state doc, which `go.sum`/`LICENSE` have no meaningful version of. |
