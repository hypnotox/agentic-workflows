---
status: Proposed
date: 2026-07-11
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [cli, workflow, domains, invariants, query]
related: [8, 19, 24, 39, 49, 77, 86, 88]
domains: [tooling, config, invariants, adr-system]
---
# ADR-0092: Read-Only Context Query Command

## Context

awf is a *generator*: it renders static artifacts from `.awf/`, drift-checks them,
and gets out of the way. The binary is **absent during the workflow** — the agent
runs against committed markdown; awf only reappears at `sync`/`check`/`gate` time.
A consequence is that the workflow skills instruct the agent to reconstruct, by
reading and grepping, facts awf already holds structurally. `awf-brainstorming`
step 1 is the clearest case: *"Read `AGENTS.md` … Check domain docs under
`docs/domains`. Identify which packages and which existing ADRs the work touches."*
That is probabilistic archaeology — an agent grepping to rederive a path's owning
domain, its backing invariants, and its related ADRs — over data that is a
deterministic function of committed config. A missed domain or a stale grep is a
silent workflow-quality loss.

The fault line this decision sits on: **anything that *generates* committed
artifacts stays at sync time; anything that *answers questions about* committed
state can be a live read-only query.** A read-only query preserves awf's trust
model whole — its output is a pure function of committed config, as reproducible as
a rendered file — where runtime *rendering* of skills would not be (what ran would
no longer be the drift-checked artifact). Runtime/proxy skill rendering is therefore
explicitly out of scope and recorded as future work; this ADR takes only the
read-only-query half.

Grounding the design against the code surfaced facts that shape it:

- **The four joins already exist as committed data, and three of four readers are
  reusable.** Domain sidecars carry anchored `paths:` globs, modeled as
  `Sidecar.Paths` (`internal/config/config.go:36`); `internal/project` already
  builds `domainPaths` from them (`internal/project/project.go:308-322`).
  `pathglob.Match` (`internal/pathglob/pathglob.go:23`, `inv: pathglob-anchored`,
  ADR-0077) plus `audit.matchesAny` (`internal/audit/audit.go:440`) is the exact
  path→domain matcher, already used by the domain-code-staleness rule. ADR frontmatter
  carries `domains:` (`internal/adr/adr.go:40`), and `adr.RenderDomainIndex`
  (`internal/adr/domain.go:15`) already filters ADRs by a domain — a reusable
  domain→ADRs predicate.

- **The path→invariant-marker join is the one piece that is genuinely new.**
  `internal/invariants.scanTags` (`internal/invariants/invariants.go:141`) discards
  each marker's *location* and filters by the `invariants.sources` globs, which are
  unrelated to domain `paths:`. So "which invariants live under these paths" needs a
  new path-filtered marker walk (the slug regexes are reusable). The *other* half —
  `inv:` slugs declared in a related ADR's Invariants section — is directly reusable
  via the existing `declRe` scan.

- **The gated static-fallback pattern exists verbatim and is the template to
  mirror.** `runConfig` (`cmd/awf/config.go:17-42`): `os.Stat(config.ConfigPath)` →
  `fs.ErrNotExist` yields a static fallback (`inv: config-command-static-fallback`),
  any other stat error is hard, else `gate(cwd)` (the binary-version gate,
  `inv: version-compat-gate`, ADR-0039) then `project.Open`. `context` mirrors it
  line-for-line.

- **The current-state pointer cannot be a sidecar field.** Domain sidecars hold only
  `paths:`; ADR-0086 makes any non-`paths:` field on a domain sidecar refuse at
  project open. The pointer must be *derived* by the rendered-path convention
  (`<docsDir>/domains/<name>.md`), never stored.

- **Skills already call bare `awf` subcommands unconditionally** — `awf sync`,
  `awf new`, `awf check` across `.claude/skills/` — so a bare `awf context`
  reference introduces no novel dependency class. The prerequisite (a resolvable
  `awf` on PATH: bootstrap has run once, or `./x install` in this repo) is the same
  one committing already imposes via the hook → gate.

- **`--json` has zero precedent in `cmd/awf`.** It is awf's first machine-output
  surface; `commandOrder`↔`argSpecs` help-parity (`cmd/awf/help_test.go`) and the
  100% coverage gate (every text/JSON/no-arg/degrade/range branch) apply.

- **The example adopter re-renders skills but runs the pinned release.**
  `examples/sundial` (ADR-0090, "example zero-notes") re-renders its skills with the
  source-built awf yet executes the pinned release binary, so `awf context` only
  *works* there once a release ships it. New `awf context` prose in skill templates
  is note-clean immediately (it is prose), but the command's availability lags a
  release — which shapes the rollout below.

## Decision

1. **A new read-only subcommand `awf context <path>...`.** For the union of the
   given repo-relative paths it reports, per path set: owning domain(s), backing
   invariants, related ADRs, and each owning domain's current-state doc pointer. It
   never writes to disk or mutates config.

2. **Explicit paths are the core contract; git flags are sugar.** `--staged` and
   `--range <a>..<b>` resolve to a path set via git. No-arg + no-flag is an error
   with a hint — `context` never guesses "current changes", because the intended
   moment differs between brainstorm (no diff yet) and review (a diff exists).

3. **Human-readable output by default; `--json` for brief assembly.** Both renderings
   derive from one assembled context struct, so they cannot diverge. The JSON shape
   is a documented struct carrying no pre-1.0 stability promise (Identity); it is
   awf's first `--json` surface and sets the convention for future query output.

4. **The assembler composes existing readers; only the marker join is new.**
   path→domain reuses `domainPaths` + `matchesAny`; domain→ADRs reuses the
   `RenderDomainIndex` predicate; ADR→`inv:` slugs reuses `declRe`; path→markers is a
   new path-filtered walk reusing the invariants slug regexes. No existing reader is
   reinvented, and the assembler is the single source both renderings read.

5. **The current-state pointer is derived, never stored.** It is the rendered-path
   convention `<docsDir>/domains/<name>.md`; `context` reads no new domain-sidecar
   field, so ADR-0086 holds unchanged. A domain configured without `paths:` is
   unreachable by path query — documented as an explicit, accepted limitation.

6. **Gating mirrors `runConfig`.** Outside an adopted tree (`ConfigPath` absent)
   `context` degrades to a static/empty answer rather than refusing; inside one it
   runs the binary-version gate then opens the project. `context` joins the
   enumerated gated command set — the ADR-0039 enumeration and the AGENTS.md
   gated-command invariant line update in the same commit that lands the command
   (no mechanical test fails otherwise).

7. **Skills invoke it directly, with no fallback prose.** This is consistent with
   the bare-`awf` calls skills already make; the binary is already a commit-time
   dependency, so a "grep if awf is absent" fallback would guard a case that does not
   arise in a working tree while re-bloating the very skills the feature slims.

8. **Rollout is two-staged.** (a) The command, `--json`, and its tests land first —
   shipping the capability with no skill edits. (b) Skill-template adoption follows
   in a later effort, after a release exposes `awf context` to pinned-release
   adopters like `examples/sundial`. This keeps the code landing independent of the
   cross-skill rewrite and of the release-availability lag.

## Invariants

- `inv: context-read-only` — the `context` command never writes to disk or mutates
  config or the lock; it only reads committed state.
- `inv: context-gated-static-fallback` — `awf context` outside an adopted tree
  degrades to a static/empty answer (never a refusal), and inside one runs the
  binary-version gate before opening the project — mirroring
  `config-command-static-fallback` + `version-compat-gate`.
- `inv: context-output-parity` — the human and `--json` renderings of `awf context`
  report the same underlying context set for the same inputs, because both derive
  from one assembled struct.

## Consequences

- **The workflow gains a deterministic context primitive.** A skill line that told
  the agent to grep out a path's domain, invariants, and ADRs becomes one exact
  `awf context` call. Reliability rises (an exact answer replaces an agent that may
  miss a domain) and the skills slim — the feature's whole point, and the concrete
  form of awf's "deterministic checks that wrap the probabilistic agent" identity.
- **A new internal assembler unit and the path-filtered marker walk are net-new
  code** under the 100% coverage gate — every branch (text, JSON, no-arg error,
  outside-tree degrade, git-range resolution, multi-domain path, unowned path) needs
  a test. Composition of existing readers keeps the surface small, but it is not zero.
- **`--json` is awf's first machine-output surface.** It commits to no schema
  stability pre-1.0 but sets a precedent that later query commands (`audit --json`,
  focused `--only <aspect>` carve-outs) can follow — deliberately deferred here.
- **The dependency surface is named, not novel.** Skills already assume a resolvable
  `awf`; this ADR only makes the prerequisite explicit. No publication-safety
  invariant is engaged — that invariant governs render-time token degradation, not
  runtime tool availability.
- **The example adopter constrains rollout, not correctness.** New `awf context`
  prose renders note-clean in `examples/sundial` immediately, but the command works
  there only post-release — hence the staged rollout; stage (b) waits for a release.
- **Runtime/proxy skill rendering is explicitly deferred.** It strains the
  committed-and-drift-checked trust model and largely duplicates the harness's native
  skill-body lazy-load and awf's config-time enabled-set curation; its useful kernel
  (progressive disclosure) is what this query serves. Recorded as future work, not a
  commitment of this ADR.
- **Doc-currency obligations land in the implementing/status-flip commit(s):** back
  the three new `inv:` slugs with source markers + tests; update the ADR-0039
  gated-command enumeration and the AGENTS.md gated-command invariant line; add
  `context` to the CLI help and `working-with-awf`; and regenerate
  `docs/decisions/ACTIVE.md` via `./x sync`.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Focused verbs (`awf domains` / `awf invariants` / `awf adr`, each `--for <path>`) | 3–4× the CLI surface to gate and test; the workflow wants the whole bundle per brief. A `--only <aspect>` flag carves a focused view later without a new command. |
| Query dispatcher (`awf query <aspect> --for <path>`) | The wrapper earns nothing over a focused verb — `query domains` is a clunkier `domains` — while still paying the multi-aspect surface cost. |
| On-demand / proxy skill rendering (runtime materialization) | Strains the committed-and-drift-checked trust model — what runs is no longer the drift-checked artifact — and largely duplicates the harness's native skill-body lazy-load plus awf's config-time enabled-set curation. Deferred as future work. |
| Optional accelerator with fallback prose (skills grep if `awf` is absent) | Re-bloats the skills the feature slims and makes the deterministic path the untaken one; the binary is already a commit-time dependency, so the fallback guards a case that does not arise in a working tree. |
| Git-derived paths by default (no-arg = working-tree changes) | Guesses intent across the brainstorm (no diff yet) and review (diff exists) moments; explicit paths plus `--staged`/`--range` is unambiguous. |
| Store the current-state pointer as a domain-sidecar field | ADR-0086 refuses any non-`paths:` field on a domain sidecar at open; the pointer is derivable by the rendered-path convention, so no field — and no schema change — is needed. |
