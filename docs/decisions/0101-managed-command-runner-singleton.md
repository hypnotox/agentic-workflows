---
status: Proposed
date: 2026-07-12
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [config, rendering, tooling]
related: [2, 40, 48, 49, 60, 61, 84, 85, 86, 90, 92, 100]
domains: [config, rendering]
---
# ADR-0101: Managed Command Runner Singleton

## Context

awf tells every adopter to drive repo tasks through a command runner: rendered git-hook payloads
invoke `./x check` and `./x gate`, the workflow skills say "run `./x gate` before every commit",
and the command vars `gateCmd`/`gateCmdFull`/`checkCmd`/`commitGateCmd`/`testCmd` default to
`./x …`. Yet awf renders **no runner** — `awf init` scaffolds none, and ADR-0002 §5 deliberately
kept `./x` a hand-maintained repo file outside the render/lock set, explicitly deferring "the
runner/linting as part of the rendered standard" to a future ADR. This is that ADR.

Two gaps follow from the current indirection:

1. **A dangling contract.** An adopter who enables hooks gets payloads that invoke `./x`, but
   nothing renders or guarantees a `./x` exists that implements those verbs.
2. **Hand-written runners rot.** The awf-facing verbs an adopter's runner must delegate
   (`sync`, `check`, `invariants`, `audit`, `context`, `commit-gate`, `new`) grow over time, and a
   copy-pasted runner silently falls behind. The in-repo example adopter's runner
   (`examples/sundial/x`) is missing the `context` verb added by ADR-0092 — live proof.

The runner cannot be *wholly* rendered, because its highest-value verbs — `gate`, `test`, `lint`,
`build` — are project- and language-specific and awf is language-agnostic. But it splits cleanly:
the awf-verb dispatch is language-agnostic boilerplate awf should own and keep current, while the
project verbs are the adopter's own logic. ADR-0100 introduced exactly the mechanism this split
needs — **in-place-editable sections** (`awf:edit-in-place`), where awf owns most of a rendered
file while the adopter edits designated sections directly in the output, preserved across syncs.
This ADR is ADR-0100's first consumer.

Three couplings shaped the design:

- **Invocation differs by role.** Adopters run the *pinned* awf release, resolved (and checksum-
  verified) by the bootstrap shim, which prints the binary path on stdout (ADR-0040/0049/0085).
  awf-the-repo runs awf *from source* (`go run ./cmd/awf …`, ADR-0002 §3) so its dogfooded render
  never uses a stale binary. A single rendered runner cannot serve both without an invocation
  switch — so awf-the-repo opts out (below), and the rendered runner targets the pinned path only.
- **Root output is precedented.** The runner renders to `x` at the repo root, outside `.awf/`.
  `AGENTS.md` and the `.claude/*` artifacts are already root/outside-`.awf/` tracked, rendered,
  lock-checked outputs, and ADR-0086's closed-config-tree sweep is `.awf/`-scoped, so a root
  `x` is drift-guarded via its lock entry exactly as `AGENTS.md` is — no closed-tree change.
- **The singleton shape is new.** The existing config-tree singletons split two ways: `hooks`/
  `bootstrap` are toggleable but section-less (ADR-0048/0085), while section-bearing units derive
  only from mandatory doc entries (ADR-0060/0061). The runner is the first unit that is **both**
  toggleable *and* section-bearing (its in-place sections).

## Decision

1. **A new toggleable `runner` singleton** renders a single command-runner file `x` at the repo
   root when enabled. It is a co-owned file per ADR-0100: awf owns the structure and the awf-verb
   arms; the adopter owns the in-place sections. The toggle mirrors the existing `bootstrap`/`hooks`
   toggles and is **additive and default-off** — an absent key means disabled — so existing adopter
   configs are unaffected and no schema-generation migration is required; a seed migration is
   deliberately declined (adopters opt in explicitly, like `bootstrap`).
2. **awf-verb dispatch is awf-owned** and delegates to the pinned binary via the bootstrap shim —
   each of `sync`, `check`, `invariants`, `audit`, `context`, `commit-gate`, `new` runs
   `"$(bash .awf/bootstrap.sh)" <verb> "$@"`. This set tracks awf's adopter-facing verbs, so a new
   verb (as `context` was) reaches every adopter's runner on the next sync — no rot.
3. **Project verbs live in `awf:edit-in-place` sections.** The template ships two in-place
   sections — a top-of-file setup/helpers block and the project-verb `case` arms (seeded with a
   `gate`/`test` starter) — which the adopter fills with their own language-specific commands,
   preserved across syncs. The adopter extends the runner only inside these sections (ADR-0100).
4. **awf-the-repo opts out.** awf disables the `runner` singleton in its own config — as it already
   disables the `bootstrap` singleton — and keeps its hand-maintained from-source `./x`. The
   rendered runner therefore targets the pinned-via-bootstrap path only; it carries **no
   invocation-mode parameter**. Dogfooding of the primitive is provided by the example adopter, not
   by awf's own runner.
5. **The example adopter adopts it.** `examples/sundial` enables the `runner` singleton and is
   migrated onto the rendered runner: its existing `gate`/`test` bodies move into the project-verb
   in-place section, and the awf-owned dispatch supplies the `context` verb its hand-written runner
   was missing — fixing that drift as a side effect. The example must remain drift-free, invariant-
   clean, and **zero advisory notes** per ADR-0090.
6. **The rendered path is fixed at `x`.** The singleton renders exactly `x` at the repo root,
   matching the `./x …` command-var defaults; there is no filename parameter (consistent with the
   "no extra parameters" framing). An adopter who wants a different entry point wraps or symlinks
   `x` rather than renaming the rendered file — a renamed file would simply be foreign to the lock.
   The command vars remain the seam the hooks/skills/docs reference — the runner is what they now
   point at, and they keep their `./x` defaults.
7. **This ADR partially supersedes ADR-0002 Decision item 5** (the deferral of the runner into the
   standard). ADR-0002's status stays live — its linting decision is unaffected — and its legacy
   `.claude/awf.yaml`/`.claude/awf.lock` paths are the pre-relocation names of today's `.awf/`
   tree; no `supersedes` flip.

## Invariants

- `inv: runner-singleton-toggle` — with the `runner` singleton enabled, `awf sync` renders exactly
  one runner file at the repo-root path `x`; disabled (or absent), it renders none. awf's own
  project config leaves it disabled.
- `inv: runner-awf-verbs-owned` — the rendered runner's awf-verb arms (`sync`, `check`,
  `invariants`, `audit`, `context`, `commit-gate`, `new`) are awf-owned (outside any
  `awf:edit-in-place` section) and each delegates to the bootstrap-resolved pinned binary, so they
  regenerate on every sync and cannot drift out of the adopter's control.
- `inv: runner-project-verbs-in-place` — the rendered runner's project-verb region and its
  setup/helpers region are `awf:edit-in-place` sections (ADR-0100), so an adopter's project-verb
  edits survive re-sync while the awf-owned arms and structure do not.
- `inv: runner-render-publication-safe` — the runner template renders leak-free under empty data
  (no unresolved token, no stray section/marker residue), like every other awf template.
- `inv: runner-example-adopted` — `examples/sundial` enables the `runner` singleton and its
  rendered `x` is drift-free, invariant-clean, and free of advisory notes (ADR-0090).
- `inv: singleton-kinds-complete` — the runner is a dedicated config-tree render block (like
  `bootstrap`/`hooks`), not a `catalog.Standard.Docs` entry, so it stays outside `SingletonKinds()`
  / `plainSingletons`; the unified-doc-model completeness test continues to assert
  `SingletonKinds()` equals exactly the mandatory doc entries (runner excluded, still green), and
  the runner instead carries its own dedicated render/check coverage.

## Consequences

Easier:
- Every adopter gets a runner that implements the verbs the standard already tells them to run —
  the dangling `./x` contract in the hook payloads and skills is closed.
- Per-adopter rot of the awf-verb plumbing is eliminated: the awf-verb arms live in one awf-owned
  template re-rendered on every sync (and re-rendered by `awf upgrade`), so an enabled runner cannot
  fall behind the way `sundial`'s hand-written one did. Residual, stated honestly: the template's
  own verb list is not yet mechanically cross-checked against the CLI dispatch table
  (`cmd/awf/dispatch.go`), so adding a new adopter-facing verb still requires a same-change template
  edit — a candidate for a future completeness check, like the deferred hooks↔runner advisory.
- The runner is ADR-0100's first real consumer, exercising the in-place-section primitive on the
  in-repo example adopter on every `./x sync`/`check`.

Harder / accepted trade-offs:
- **awf-the-repo does not dogfood its own runner.** Because awf runs from source, its own `./x`
  stays hand-maintained and outside the render set (ADR-0002 §5 unchanged for awf itself). The
  ethos of "model what it generates" is satisfied through the example adopter, not awf's own tree —
  the same trade awf already accepts by disabling its own `bootstrap` singleton.
- **The runner/hook coupling is convention, not enforcement.** The hook payloads invoke `./x` via
  the command vars independently of whether the `runner` singleton is enabled; a project could
  enable hooks without the runner (or rename one out of step with the other). This ADR does not add
  a cross-check; it is noted as a known coupling and left to a possible future advisory.
- **First adoption of a pre-existing hand-written runner is lossy** (ADR-0100): `sundial`'s
  migration hand-ports its `gate`/`test` bodies into the in-place section; a generic adopter with
  an existing `./x` does likewise from the `*.awf-bak` backup.
- A new config-tree render unit and its template widen the render/check surface.

Ruled out:
- **Rendering the whole runner** (project verbs as config data): collides with language-agnosticism
  and ADR-0084 (vars carry functional values only, not multi-line scripts).
- **A two-file split** (awf payload + adopter delegation stub, as ADR-0048 does for hooks): rejected
  in favour of the single-file in-place model (ADR-0100) for `./x` ergonomics.
- **An invocation-mode parameter** so awf could adopt its own runner: unnecessary once awf opts out;
  keeps the template pinned-path-only and simple.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| awf-the-repo adopts its own runner via an invocation-mode parameter (source vs pinned) | Adds a template parameter and migrates awf's bespoke 117-line runner for a marginal ethos gain; awf already opts out of the analogous `bootstrap` singleton, and `sundial` provides the dogfood. |
| Two-file split (rendered payload + adopter stub) | Reuses existing machinery but loses single-file `./x` ergonomics and the reusable in-place primitive; ADR-0100 was chosen precisely to avoid the split. |
| Render the entire runner from command vars + a project-verb data block | Encoding multi-step, language-specific gate logic as config data fights language-agnosticism and ADR-0084. |
| Scaffold-once at `awf init`, no rendered singleton | Does not fix rot — a scaffolded runner still falls behind the awf-verb set (the `sundial` failure); the whole point is a re-rendered, drift-checked runner. |
| Add a hooks↔runner cross-check invariant now | Out of scope for this ADR; the coupling is noted and a lighter advisory can follow if it proves necessary. |
