---
status: Superseded by ADR-0032
date: 2026-06-24
supersedes: []
superseded_by: "0032"
tags: [tooling]
related: [2]
domains: [tooling]
---
# ADR-0003: awf Binary Delivery and Hook-Activation Setup

## Context

[ADR-0002](0002-static-linting-and-x-command-runner.md) wired the gate through `./x gate`
and noted, as a deferred follow-up, that the shared pre-commit hook template still renders a
hardcoded bare `awf check`. That bareness is the seam this ADR addresses.

The `awf` binary is consumed in two contexts, and the hardcoded `awf check` is correct for
one and wrong for the other:

- **Adopter projects** — other Go repos that adopt the awf standard. They obtain `awf` via
  `go install <module>/cmd/awf@latest` (see `README.md`), so a bare `awf` resolves on PATH and
  `awf check` works.
- **This repo (dogfooding)** — `awf` *is* the local module; per ADR-0002 we run it from source
  via `go run ./cmd/awf` (wrapped by the repo-local `./x`) precisely to avoid the stale-binary
  problem. A bare `awf check` does **not** resolve here, so activating git hooks
  (`git config core.hooksPath .githooks`) would break every commit.

The hook gate command is already parameterised (`gateCmd` → `./x gate` in this repo), but the
adjacent **check** command in the same pre-commit hook is not — an asymmetry. That asymmetry is
why the hooks remain dormant (`core.hooksPath` unset), which in turn leaves ADR-0002's
"pre-commit blocks lint failures" invariant unbound.

A second gap: when a project adopts awf, `awf init`/`sync` renders `.githooks/` into its tree
but **never activates them** — and git hooks are not activated by `git clone` (`core.hooksPath`
is local, uncommitted config). So both adopters and fresh clones of any awf repo need a
one-time, idempotent activation step that today is undocumented manual `git config`.

**Stated assumption (not changed by this ADR):** the adopter-delivery convention already
documented in `README.md` — adopters obtain `awf` via `go install <module>/cmd/awf@latest`,
putting `awf` on PATH — is what the default hook check command (`awf check`) relies on. The
concrete go-gettable module path is resolved at the Phase-4 publish step. This ADR depends on
that convention; it does not introduce or alter it.

Grounding discoveries that shape the design:

- **No catalog-level var defaults exist.** Vars are sourced from `.claude/awf.yaml`; `awf init`
  seeds the referenced-var union as empty strings (`internal/project/scaffold.go`). The render
  executor uses `missingkey=zero`, which renders an unset/empty var as the literal `<no value>`,
  and a `<no value>` token in output is a hard render error (`internal/project/project.go`). A
  default therefore must be expressed **inline in the template**, not via the catalog.
- The CLI dispatches on `os.Args[1]` (`cmd/awf/main.go`); `runInit` already ends by calling
  `runSync`. There is no existing `os/exec` use in `cmd/awf`, and no temp-git-repo test helper.

## Decision

1. **Parameterise the hook check command via `checkCmd`.** Introduce a template var `checkCmd`.
   `templates/hooks/pre-commit.tmpl` renders it with an **inline default**:
   `{{ if .vars.checkCmd }}{{ .vars.checkCmd }}{{ else }}awf check{{ end }}`. Adopters (whose
   seeded `checkCmd` is empty) get `awf check`; this repo sets `checkCmd: "./x check"` in
   `.claude/awf.yaml`. The inline `{{ else }}` is mandatory because `missingkey=zero` would
   otherwise emit a render-failing `<no value>`. This mirrors the existing `gateCmd` parameter.

2. **Add an `awf setup` subcommand** — the canonical one-time, idempotent step run after cloning
   any awf-adopting repo (and after `awf init`), to wire the local clone's hooks. It runs
   `git config core.hooksPath .githooks`. Behaviour:
   - Idempotent: re-running is a no-op (`git config` setting the same value).
   - Errors if `.githooks/` does not exist (message directs the user to run `awf sync` first).
   - If the working directory is not inside a git repository, it **warns and is a no-op** (exit 0)
     rather than failing — so `awf init` chaining (item 3) never breaks in a not-yet-`git init`ed
     project.

3. **`awf init` runs setup at the end**, and **`./x setup` delegates** to `go run ./cmd/awf setup`.
   `runInit` calls `runSetup` after `runSync`; init's trailing setup is best-effort (a setup
   warning never fails init). The dogfood repo's `./x setup` invokes awf from source so it shares
   one implementation of the activation logic (no duplicated `git config` string).

## Invariants

- `templates/hooks/pre-commit.tmpl` contains no hardcoded `awf check`; it renders the check line
  via `checkCmd` with an inline `{{ else }}awf check{{ end }}` default. A pre-commit rendered with
  an empty/unset `checkCmd` yields the line `awf check` and never `<no value>` (verified by a
  golden render assertion in `internal/project/` over the hook template with both empty `checkCmd`
  and `checkCmd: "./x check"`; the `{{ if .vars.checkCmd }}` guard is a condition, not an output
  action, so it does not trip the `<no value>` render error under `missingkey=zero`).
- This repo's `.claude/awf.yaml` sets `checkCmd: "./x check"`, and its rendered `.githooks/pre-commit`
  invokes `./x check` (not a bare `awf check`).
- `awf setup` is idempotent: running it twice in succession leaves `core.hooksPath` equal to
  `.githooks` and exits 0 both times.
- `awf setup` exits non-zero with a directive message when `.githooks/` is absent, and exits 0
  with a warning (making no change) when not inside a git repository.
- `awf init` invokes the same setup logic as `awf setup`; a setup warning does not fail `init`.

## Consequences

Easier:
- Git hooks can finally be activated in this repo without breaking commits, binding ADR-0002's
  "pre-commit blocks lint failures" invariant.
- Adopters and fresh clones have one documented, idempotent command (`awf setup`, or automatic via
  `awf init`) to wire hooks — no undocumented manual `git config`.
- The check command is now symmetric with the gate command; both are project-tunable.

Harder / accepted trade-offs:
- The awf schema grows one var (`checkCmd`); adopters' scaffolded awf.yaml gains an empty
  `checkCmd` line on next `awf init`. Consumers of this var-union addition: (a) `ScaffoldConfig`
  (`internal/project/scaffold.go`) auto-collects `checkCmd` from the template via
  `render.ReferencedVars`, so it is seeded as `checkCmd: ""` with no code change; (b) the
  golden pre-commit assertion in `internal/project/golden_test.go` (which checks the rendered
  hook still contains `awf check`) stays green because its `sampleYAML` omits `checkCmd`, so
  the inline `{{ else }}` default fires; (c) this repo's `.claude/awf.lock` re-renders when
  `.claude/awf.yaml` gains `checkCmd: "./x check"` (the config hash changes), so the
  `.githooks/pre-commit` line and lock entry update on the re-sync in the same commit — no
  separate lock-format change. The lock format itself is unchanged: `checkCmd` is data inside
  the existing config-hash input, not a new lock field.
- `cmd/awf` gains an `os/exec` dependency (to shell `git config`) and the first git-aware
  subcommand; tests must stand up a temp git repo themselves (no existing helper).
- `awf setup` mutates the user's local git config (`core.hooksPath`). This is reversible
  (`git config --unset core.hooksPath`) and local-only, but it is a side effect `init` now performs.

Downstream work unblocked: an implementation plan covering the catalog var + template default,
the `awf setup` subcommand + tests, the `awf init` call, `./x setup`, doc updates (README +
AGENTS layout part), re-sync, and finally activating the hooks in this repo.

Doc-currency obligations the same implementing commit(s) must satisfy:
- `.claude/awf/parts/agents-doc-layout.md` line 15 (the `.githooks/` description) hardcodes the
  manual activation string `git config core.hooksPath .githooks` as literal prose, not a
  `{{ .vars.X }}` interpolation. Since `awf setup` becomes the canonical activation step, that
  line must be hand-edited to direct the reader to `awf setup` in the same change. This is a
  static overlay *part* (a render input), so `awf check` will **not** flag it as drift — the
  identical trap ADR-0002 documented for `agents-doc-conventions.md`. AGENTS.md re-renders from
  the edited part on re-sync.
- `README.md` carries the `go install` delivery convention (the Context's stated assumption) and
  gains the `awf setup` post-clone step.
- When this ADR's status flips to Accepted or Implemented, the same commit must regenerate
  `docs/decisions/ACTIVE.md` via `go test ./internal/adrtools/`. (No `docs/decisions/README.md`
  index row is owed: this repo's README is a how-to guide with no per-ADR rows — `ACTIVE.md` is
  the generated index.)

This ADR is the follow-up ADR-0002 flagged; it does not alter ADR-0002's decision that `./x`
itself stays repo-local (item 5) — `./x setup` is a thin delegator, not a rendered artifact.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Install awf to PATH in setup (`go install ./cmd/awf`) so bare `awf check` works here too | Reintroduces the stale-binary risk ADR-0002 avoided (installed awf rendering against newer templates) and assumes GOBIN on PATH; brittle for a self-rendering tool. |
| Render `./x` into adopters so the hook can call `./x check` everywhere | Large scope; contradicts ADR-0002 item 5 (keep `./x` repo-local) and couples every adopter to a bash runner. |
| Catalog-level default for `checkCmd` | No such mechanism exists; `missingkey=zero` emits a render-failing `<no value>` for unset vars, so the default must be inline in the template. |
| `awf init` auto-activates hooks with no separate command | The user wants `setup` to be the canonical, re-runnable post-clone step (clones don't carry `core.hooksPath`); `init` calling setup is additive, not a replacement. |
| Dedicated `awf setup` that hard-errors outside a git repo | Would make `awf init` fail in a not-yet-`git init`ed project; warn-and-no-op keeps init robust while standalone misuse is still visible. |
