---
status: Proposed
date: 2026-06-28
supersedes: []
superseded_by: ""
tags: [tooling, release, distribution]
related: [0003]
domains: [tooling]
---
# ADR-0030: Prebuilt Binary Distribution and Release Pipeline

## Context

awf is distributed today via `go install github.com/hypnotox/agentic-workflows/cmd/awf@latest`,
which requires a Go toolchain on the adopter's machine. [ADR-0003](0003-binary-delivery-and-setup.md)
stated this as the delivery convention — adopters "obtain `awf` via `go install <module>/cmd/awf@latest`,
putting `awf` on PATH" — and explicitly deferred the concrete publish mechanics: "The concrete
go-gettable module path is resolved at the Phase-4 publish step." The default pre-commit hook check
command relies only on a bare `awf` resolving on PATH, not on *how* it got there.

The standard awf renders is language-agnostic by design (ADR-0008): an adopter project in any
language should be able to run `awf` without installing Go. The `go install`-only path contradicts
that positioning — a Python or Rust repo's contributors should not need a Go toolchain to install the
tool. There is no first release: `git tag -l` is empty, so adopters cannot pin a version, and there
are no downloadable artifacts.

Grounding discoveries that shape the design:

- `cmd/awf/version.go` `awfVersion()` returns `debug.ReadBuildInfo().Main.Version` when set (only by
  `go install module@version`), else falls back to the `project.Version` constant
  (`internal/project/project.go:18`, currently `"0.1.0"`). A `go build`/GoReleaser binary leaves
  `Main.Version` as `(devel)`, so it would fall back to the constant — correct for `v0.1.0` by
  coincidence, but not tracking future tags. `project.Version` is *also* the lock's `AWFVersion`
  (`project.go:87`), so it cannot itself be ldflags-injected without making renders non-reproducible.
- awf's drift oracle (`awf check`) is lock-driven: it inspects only lock-tracked files plus the
  regenerated `ACTIVE.md`/domain docs. It does not scan `.github/`, repo-root dotfiles, or
  `.goreleaser.yaml`. `.golangci.yml` and `./x` are the existing precedent for hand-maintained files
  that live outside the render/lock set (ADR-0002). The root `README.md` is hand-authored (not in the
  lock) and outside the ADR-0020 dead-reference scope.
- `audit.allowedScopes` in `.awf/config.yaml` is `[adr, awf, plans]` — no scope fits release/CI files.
  `awf audit` is advisory (wired into no gate), so a mis-scoped commit is flagged, not blocked.
- Repo visibility (public vs private) is intentionally deferred and is **out of scope** for this ADR;
  the release pipeline produces a GitHub Release either way.

## Decision

1. **Prebuilt binaries are the canonical install path.** awf is distributed as prebuilt
   cross-platform binaries; `go install <module>/cmd/awf@latest` is demoted to a secondary
   "install from source (Go users)" route. The `README.md` install section is rewritten to lead with
   the binary download and move the "Requires Go 1.26+" caveat to the source path. This partially
   supersedes ADR-0003's stated delivery convention: the on-PATH assumption the hook default relies on
   still holds; only the *acquisition method* changes. That convention lived in ADR-0003's Context as a
   **stated assumption** it explicitly disclaimed introducing ("does not introduce or alter it") — not a
   numbered Decision item or Invariant — so none of ADR-0003's Decision items or Invariants change here.
   ADR-0003 therefore stays `Implemented` and the linkage is `related: [0003]`, not `supersedes:`.

2. **GoReleaser (v2) is the release tool.** It runs in CI via the pinned `goreleaser-action` on
   `v*` tag pushes, and `go run github.com/goreleaser/goreleaser/v2@<pinned>` is used for local
   snapshots. GoReleaser is deliberately **not** a `go.mod` `tool` dependency (its dependency tree is
   large and it is release-only). `.goreleaser.yaml` is hand-maintained and lives outside awf's
   render/lock set, following the `.golangci.yml` / `./x` precedent.

3. **Release artifacts.** The build matrix is linux/darwin/windows × amd64/arm64. Each target ships a
   per-OS archive (`tar.gz` on unix, `zip` on windows) bundling `LICENSE` + `README.md`; the release
   includes a `checksums.txt`, a changelog grouped from Conventional Commits, and a GitHub Release
   created from the tag. A new `.github/workflows/release.yml` triggers on `push` tags matching `v*`
   with `permissions: contents: write`.

4. **Version stamping via a dedicated ldflags var.** A package-level `var version string` is added in
   `cmd/awf`; GoReleaser injects `-ldflags "-s -w -X main.version={{.Version}}"`. `awfVersion()`
   precedence becomes injected `version` → `debug.ReadBuildInfo().Main.Version` → `project.Version`.
   `project.Version` remains the source of truth for the lock's `AWFVersion` and the dev/test
   fallback, and is bumped per release.

5. **CI release-config guard.** `.github/workflows/ci.yml` gains a pull-request job that runs
   `goreleaser check` + `goreleaser release --snapshot --clean` (no publish), so a broken release
   config fails before a tag is ever pushed.

6. **Audit scope for release commits.** `audit.allowedScopes` in `.awf/config.yaml` gains `ci`, so
   release/CI commits (`.github/`, `.goreleaser.yaml`, `.gitignore`) carry a fitting
   Conventional-Commits scope instead of overloading `awf`.

## Invariants

- `inv: version-ldflags-precedence` — `awfVersion()` returns the ldflags-injected package var when it
  is non-empty, in preference to the `runtime/debug` BuildInfo version and the `project.Version`
  constant. (Backed by a `cmd/awf` test that sets the package var and asserts it is returned.)
- `.goreleaser.yaml` builds `./cmd/awf` into a binary named `awf` and injects `main.version`; the
  ldflags `-X` target matches the declared package var name. (Textual contract — GoReleaser config is
  not in an `invariants.sources` glob; the CI snapshot job exercises it.)
- `.goreleaser.yaml`, `.github/workflows/release.yml`, and `.github/workflows/ci.yml` stay outside
  awf's render/lock set and produce no `awf check` drift entries. (Textual contract.)
- `project.Version` remains the lock's `AWFVersion` source and is never ldflags-driven, so renders
  stay reproducible across `go run`, `go build`, and released binaries. (Textual contract.)
- The release workflow creates a GitHub Release only on a `v*` tag push. (Textual contract.)

## Consequences

Easier:
- Non-Go adopters install awf as a binary with no toolchain — fulfilling the language-agnostic
  positioning (ADR-0008).
- Tagging `v0.1.0` produces reproducible, checksummed artifacts plus a Conventional-Commits changelog
  from a single tag push.
- Released and snapshot binaries report an exact version while renders stay reproducible (the lock
  version stays source-controlled, decoupled from the ldflags display value).

Harder / accepted trade-offs:
- A new hand-maintained surface (`.goreleaser.yaml` + the release workflow) lives outside the drift
  oracle — like `.golangci.yml` / `./x`, it can rot silently. The PR-time `goreleaser check` +
  `--snapshot` job is the mitigation.
- GoReleaser via `go run` / the action (not a pinned `go.mod` tool dep) means the tool version floats
  unless the action tag and the `goreleaser/v2@<version>` `go run` target are pinned — both are
  pinned for reproducibility.
- `project.Version` must still be bumped per release to keep the lock's `AWFVersion` and the dev/test
  fallback honest; the ldflags var only fixes the released-binary *display*.

Downstream work unblocked: an implementation plan covering the `cmd/awf` version var + test,
`.goreleaser.yaml`, the release workflow, the CI snapshot guard, the `.gitignore` `/dist` entry, the
README install rewrite, the `allowedScopes` bump + re-sync, and finally tagging `v0.1.0`.

Doc-currency obligations the implementing commit(s) must satisfy:
- `README.md` install section rewritten (binary primary, `go install` demoted to source).
- `cmd/awf/version.go`'s doc comment reworded — it currently names `go install module@version` as the
  only BuildInfo source; it must also describe the ldflags var.
- The tooling domain narrative (`.awf/domains/parts/tooling/current-state.md` → `docs/domains/tooling.md`)
  gains a release/distribution sentence when this ADR flips to `Implemented` (ADR-0019's
  `domain-doc-staleness` rule watches this co-change).
- `docs/decisions/ACTIVE.md` regenerated via `./x sync` on the propose commit and on every status flip.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Hand-rolled GitHub Actions build matrix | Reimplements archives, checksums, and changelog by hand; per-OS zip-vs-tar and the arch matrix are fiddly to get right and easy to drift. |
| `./x release` bash cross-compile loop | Reinvents GoReleaser in bash and grows the deliberately-minimal `./x` runner substantially; loses the field-standard packaging/changelog behaviour. |
| Keep `go install` as the only path, no binaries | Fails the language-agnostic goal — every adopter would still need a Go toolchain. |
| GoReleaser as a pinned `go.mod` `tool` dep | Drags a very large dependency tree into `go.sum` for a release-only tool; the action + pinned `go run` keeps the module lean. |
| Bump `project.Version` const only, no ldflags var | Couples the lock version and CLI display, relies on a tag==const guard, and gives no `git describe` detail for snapshot builds. |
| Overload the `awf` audit scope for release/CI commits | Hides genuinely different concerns under one scope; a dedicated `ci` scope keeps Conventional-Commits meaningful. |
