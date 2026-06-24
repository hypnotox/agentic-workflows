---
status: Proposed
date: 2026-06-24
supersedes: []
superseded_by: ""
tags: [tooling]
related: []
---
# ADR-0002: Static Linting via golangci-lint and the ./x Command Runner

## Context

The `awf` repo has no static linting. Its gate is `go test ./... && go vet ./...`,
enforced only by local git hooks under `.githooks/` (this repo has no CI). `go vet`
catches a narrow class of issues; there is no errcheck, staticcheck, errorlint,
bodyclose, revive, formatting enforcement, or any of the higher-signal analyses that
keep a Go codebase clean.

Three sibling repos already lint (`internal reference projects`). They show two
shapes: a lean curated set (a reference project: ~9 linters) and a comprehensive set (a reference project: ~40
linters with golines and heavy per-package exclusions tuned for a large transpiler).
`a reference project` declares golangci-lint as a Go `tool` dependency and runs `go tool
golangci-lint run`, pinning the version in `go.mod` with no separate install. All of
the user's repos drive repo interactions through a single `./x` command-runner script;
this is a proven per-repo idiom.

Two couplings shaped the decision:

1. **The gate is parameterised.** `.claude/awf.yaml` holds `gateCmd` / `gateCmdFull`,
   interpolated into the rendered hook templates and skill/AGENTS.md prose. Changing
   the gate is a config edit plus a re-sync, not a hand-edit of rendered files.

2. **The pre-push hook is currently broken.** `templates/hooks/pre-push.tmpl` renders
   `{{ .vars.gateCmd }} full` — it assumes `gateCmd` is a *script that accepts a `full`
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
   `check`, `adr`, `build`, `install`, `fmt`.

4. **Route the gate through `./x gate`** = `go test ./... && go vet ./... && go tool
   golangci-lint run` (`full` accepted as a synonym; awf has no slower tier). Set
   `gateCmd: "./x gate"` and `gateCmdFull: "./x gate full"` in `.claude/awf.yaml`, bump
   `gateDuration`, then re-sync so the rendered hooks, skill files, and AGENTS.md adopt the
   new gate command and `.claude/awf.lock` updates. This repairs the previously-broken
   pre-push hook.

5. **`.golangci.yml` and `./x` are hand-maintained repo files outside the awf render/lock
   set** — they are not rendered from templates and not tracked in `.claude/awf.lock`, so
   they do not affect `awf check`. Linting is adopted for this repo only; promoting it into
   the awf standard (a rendered template + catalog entry) is explicitly deferred to a
   future ADR.

## Invariants

- The gate invoked by `./x gate` runs golangci-lint; the pre-commit hook blocks any commit
  with lint failures.
- The golangci-lint version is pinned in `go.mod` via the `tool` directive; running the gate
  requires no separate manual install.
- `.golangci.yml` and `./x` are absent from `.claude/awf.lock` and never cause `awf check`
  to report drift.
- `gateCmd` / `gateCmdFull` in `.claude/awf.yaml` reference `./x gate` (and `./x gate full`);
  the rendered `.githooks/pre-push` invokes a valid command — never a bare `go vet ./... full`.
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

Ruled out (for now):
- Per-package exclusions and complexity gates in the baseline config.
- Linting as part of the rendered awf standard (deferred to a future ADR).

Downstream work unblocked: an implementation plan covering — add the tool dep + config,
write `./x`, fix any lint findings the new config surfaces, flip the gate vars and re-sync,
and update the gate docs.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Edit `pre-push.tmpl` to use `gateCmdFull` instead of introducing `./x` | Changes the shared, published hook template — alters the standard for every adopter; out of scope. A repo-local `./x` conforms to the existing template convention without touching the standard. |
| golangci-lint via separate install / CI action (a reference project) | This repo has no CI; local hooks are the only gate. A `go tool` dep guarantees every contributor runs the same pinned version with zero setup. |
| Lean curated linter set (a reference project, ~9 linters) | Lower signal; the codebase is clean enough to pass a stricter set, and the goal is highest practical quality. |
| Comprehensive set with complexity gates + per-package exclusions (a reference project) | Requires tuning and exclusions that a small, clean codebase does not yet warrant; ratchet on later if needed. |
| Make linting part of the awf standard now (rendered config + catalog entry) | Larger, separately load-bearing change to the published standard; defer until the in-repo baseline is proven. |
