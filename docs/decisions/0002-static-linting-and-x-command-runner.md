---
status: Implemented
date: 2026-06-24
supersedes: []
superseded_by: ""
tags: [static-linting, command-runner]
related: [101]
domains: [tooling]
---
# ADR-0002: Static Linting via golangci-lint and the ./x Command Runner

## Context

The `awf` repo has no static linting. Its gate is `go test ./... && go vet ./...`,
enforced only by local git hooks under `.githooks/` (this repo has no CI). `go vet`
catches a narrow class of issues; there is no errcheck, staticcheck, errorlint,
bodyclose, revive, formatting enforcement, or any of the higher-signal analyses that
keep a Go codebase clean.

Three internal sibling Go repos already lint. They show two
shapes: a lean curated set (~9 linters) and a comprehensive set (~40
linters with golines and heavy per-package exclusions tuned for a large transpiler).
One declares golangci-lint as a Go `tool` dependency and runs `go tool
golangci-lint run`, pinning the version in `go.mod` with no separate install. All of
the user's repos drive repo interactions through a single `./x` command-runner script;
this is a proven per-repo idiom.

Two couplings shaped the decision:

1. **The gate is parameterised.** `.claude/awf.yaml` holds `gateCmd` / `gateCmdFull`,
   interpolated into the rendered hook templates and skill/AGENTS.md prose. Changing
   the gate is a config edit plus a re-sync, not a hand-edit of rendered files.

2. **The pre-push hook is currently broken.** `templates/hooks/pre-push.tmpl` renders
   `{{ .vars.gateCmd }} full`: it assumes `gateCmd` is a *script that accepts a `full`
   subcommand* (the golden-test fixtures in `internal/project/spine_test.go` already
   expect `gateCmd: "./x gate"` / `gateCmdFull: "./x gate full"`). But awf set `gateCmd`
   to a raw command, so the on-disk `.githooks/pre-push` is
   `go test ./... && go vet ./... full`, where `go vet ./... full` errors
   ("package full is not in std"). It is dormant only because the branch was never pushed.
   Conforming to the template's script convention repairs this without editing the shared
   (published) template.

Scope was deliberately constrained: lint this repo only. Promoting linting into the awf
*standard* (a rendered config template + catalog entry shipped to every adopter) is a
larger, separately load-bearing change.

## Decision

1. **Adopt golangci-lint v2** as the project's static-analysis tool, declared as a Go
   `tool` dependency in `go.mod` (`tool github.com/golangci/golangci-lint/v2/cmd/golangci-lint`)
   and run via `go tool golangci-lint run`. The version is pinned in `go.mod`; no separate
   install is required to run the gate.

2. **Configure linting in `.golangci.yml`** (v2 schema, `version: "2"`), tuned
   "strict-but-clean": a broad bug + quality + style linter set (errcheck, staticcheck,
   govet with nilness, errorlint, bodyclose, nilerr, ineffassign, unused, unconvert,
   unparam, wastedassign, predeclared, gocritic, revive, perfsprint, misspell, usetesting,
   intrange, usestdlibvars, dupword, and similar) plus `gofmt` and `goimports` formatters.
   The baseline carries **no per-package exclusions** and **no complexity gates**
   (gocyclo / gocognit / goconst), on the basis that this small, clean codebase passes a
   stricter set without tuning. Gates can be ratcheted on later once the baseline is green.

3. **Introduce `./x`**, an executable bash command-runner at the repo root, as the single
   entry point for repo interactions. Subcommands: `gate [full]`, `lint`, `test`, `sync`,
   `check`, `adr`, `build`, `install`, `fmt`. The awf-invoking subcommands run the CLI
   **from source** (`./x sync` → `go run ./cmd/awf sync`, `./x check` → `go run ./cmd/awf
   check`), so the dogfooded render always matches the current source tree and can never
   use a stale binary (awf renders its own `.claude/` files; a stale binary would emit
   wrong output). `./x build` (`go build`) and `./x install` (`go install ./cmd/awf`) are
   separate convenience wrappers and are **not** on the sync/check path. `./x` is not on
   PATH and depends on no installed `awf`.

4. **Route the gate through `./x gate`** = `go test ./... && go vet ./... && go tool
   golangci-lint run` (`full` accepted as a synonym; awf has no slower tier). Set
   `gateCmd: "./x gate"` and `gateCmdFull: "./x gate full"` in `.claude/awf.yaml`, bump
   `gateDuration`, then re-sync so the rendered hooks, skill files, and the **var-driven**
   portions of AGENTS.md adopt the new gate command and `.claude/awf.lock` updates. This
   repairs the previously-broken pre-push hook. **The re-sync does not cover the static
   AGENTS.md overlay parts:** `.claude/awf/parts/agents-doc-conventions.md` line 4 hardcodes
   the gate string (`go test ./... && go vet ./... (≈10s)`) as literal prose, not a
   `{{ .vars.gateCmd }}` interpolation, so it must be hand-edited in the same change to read
   `./x gate` and the new duration; otherwise the rendered AGENTS.md will contain two
   contradictory gate commands, and `awf check` will not flag it (the overlay part is a
   render *input*, so the lock still matches after re-sync).

5. **`.golangci.yml` and `./x` are hand-maintained repo files outside the awf render/lock
   set**: they are not rendered from templates and not tracked in `.claude/awf.lock`, so
   they do not affect `awf check`. Linting is adopted for this repo only; promoting it into
   the awf standard (a rendered template + catalog entry) is explicitly deferred to a
   future ADR.

## Invariants

- The gate invoked by `./x gate` runs golangci-lint; the pre-commit hook blocks any commit
  with lint failures **when hooks are active** (`git config core.hooksPath .githooks`). The
  hooks are currently dormant in this repo (`core.hooksPath` is unset), so activating them
  is a prerequisite for this invariant to bind: the implementation plan must either set
  `core.hooksPath` or document the manual `./x gate` discipline; otherwise the gate is
  advisory only.
- The golangci-lint version is pinned in `go.mod` via the `tool` directive; running the gate
  requires no separate manual install.
- `.golangci.yml` and `./x` are absent from `.claude/awf.lock` and never cause `awf check`
  to report drift.
- `gateCmd` / `gateCmdFull` in `.claude/awf.yaml` reference `./x gate` (and `./x gate full`);
  the rendered `.githooks/pre-push` invokes a valid command, never a bare `go vet ./... full`.
- No file under `.claude/awf/parts/` and no rendered file contains the literal string
  `go test ./... && go vet ./...` as a *gate* command after the flip (grep is the test);
  the only gate command that survives the change is `./x gate`. (`go vet ./...` may still
  appear as a tool *description*, e.g. in `debugging-surfaces.md`, but not as the gate.)
- The baseline `.golangci.yml` enables no per-package exclusions; introducing one is a
  signal that a package needs attention or the rule needs reconsideration.

## Consequences

Easier:
- Uniform, high-signal static analysis on every commit, far beyond `go vet`.
- A single memorable entry point (`./x`) for every repo interaction, matching the user's
  other repos.
- The long-broken pre-push hook is fixed as a side effect of conforming to the template's
  script convention.

Harder / accepted trade-offs:
- `go.mod` / `go.sum` gain a large set of `// indirect` dependencies pulled in by
  golangci-lint. This is noise in the dependency tree and is relevant to the planned
  Phase-4 publish step (`go mod tidy` will retain them as tool deps).
- The first `./x gate` run is slower while golangci-lint's analysis cache warms.
- Editing `.claude/awf.yaml` (the gate vars) requires an immediate `./x sync` before
  committing, or the pre-commit `awf check` will fail on drift in the re-rendered files.
- The gate string is hardcoded in `.claude/awf/parts/agents-doc-conventions.md` (a static
  overlay part), so a gate change is *not* fully captured by re-sync; the part file must be
  hand-edited in lockstep. `awf check` cannot catch a stale gate string there because the
  part is a render input, not a rendered output. This is a pre-existing coupling the gate
  change surfaces, not one this ADR introduces.

Ruled out (for now):
- Per-package exclusions and complexity gates in the baseline config.
- Linting as part of the rendered awf standard (deferred to a future ADR).

Downstream work unblocked: an implementation plan covering: add the tool dep + config,
write `./x`, fix any lint findings the new config surfaces, flip the gate vars and re-sync,
hand-edit the static gate string in `.claude/awf/parts/agents-doc-conventions.md`, and
update the gate docs. When this ADR's status flips to Accepted or Implemented, the same
commit must regenerate `docs/decisions/ACTIVE.md` via `go test ./internal/adrtools/`.
(No `docs/decisions/README.md` index row is owed: this repo's README is a how-to guide with
no per-ADR rows; `ACTIVE.md` is the generated index.)

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Edit `pre-push.tmpl` to use `gateCmdFull` instead of introducing `./x` | Changes the shared, published hook template: alters the standard for every adopter; out of scope. A repo-local `./x` conforms to the existing template convention without touching the standard. |
| golangci-lint via separate install / CI action | This repo has no CI; local hooks are the only gate. A `go tool` dep guarantees every contributor runs the same pinned version with zero setup. |
| Lean curated linter set (~9 linters) | Lower signal; the codebase is clean enough to pass a stricter set, and the goal is highest practical quality. |
| Comprehensive set with complexity gates + per-package exclusions | Requires tuning and exclusions that a small, clean codebase does not yet warrant; ratchet on later if needed. |
| Make linting part of the awf standard now (rendered config + catalog entry) | Larger, separately load-bearing change to the published standard; defer until the in-repo baseline is proven. |
