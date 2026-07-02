---
status: Proposed
date: 2026-07-02
supersedes: []
retires_invariants: [version-ldflags-precedence]
superseded_by: ""
tags: [versioning, release, bootstrap]
related: [30, 39, 40, 48]
domains: [tooling]
---
# ADR-0049: Single version authority

## Context

The binary's version identity currently has three sources, resolved by precedence in
`awfVersion()` (ADR-0030 Decision 4): the ldflags-injected `main.version` (releases), the module
version from `debug.ReadBuildInfo()` (`go install` builds), and the `project.Version` const
(dev/test fallback). Meanwhile `Sync` stamps the **const** into the lock's `AWFVersion`
(`internal/project/project.go:128`), and the rendered bootstrap pins the **const** (ADR-0040).

A 2026-07-02 analysis reproduced three failures born from this split:

1. **A permanent, un-clearable version note.** Go now stamps a VCS pseudo-version into plain
   `go build` binaries (verified: `go build` at HEAD reports `v0.5.2-0.20260702…`), so the
   `coverage-ignore` comment on the build-info branch ("Main.Version is set only by
   `go install module@version`") is stale. For any source-built binary, `check`'s ahead-note
   compares pseudo-version vs the stamped const and always fires — and its remediation text
   ("run awf sync to re-pin") is false, because sync restamps the const, not the running
   version. ADR-0039 Decision 3's parenthetical ("`sync` will restamp `awfVersion` to the
   running version") describes behaviour that was never built.
2. **Wired hooks dead on arrival.** Config-schema generation 6 shipped after tag `v0.5.1` while
   `project.Version` stayed `0.5.1`, so a fresh render pins a *released* binary that hard-fails
   on the tree it is pinned into (`schema generation 6 > 5`; `commit-gate` cannot even parse the
   gen-6 config under strict fields). Nothing ties a schema-generation bump to a version bump.
3. **Bootstrap stdout pollution.** On every cache miss, `sha256sum -c` prints
   `<asset>: OK` to stdout before the script echoes the binary path, so the documented
   invocation `"$(bash .awf/bootstrap.sh)" check` captures two lines and execs garbage — every
   fresh CI run and every first wired commit per machine fails, then succeeds on the warm
   cache. Additionally `sha256sum` is not stock on macOS although the script accepts `darwin`,
   and when the pinned version has no published release yet, direct bootstrap callers hard-fail
   (the hook shim's fetch-failure fallback to a PATH `awf` is the only escape, per ADR-0048).

## Decision

1. **`project.Version` is the sole version authority.** `awfVersion()` returns `project.Version`
   unconditionally. The `main.version` ldflags var and the `debug.ReadBuildInfo()` branch are
   removed as identity sources, and `.goreleaser.yaml` stops injecting `-X main.version`. Gate
   errors, the check ahead-note, the lock stamp, and the bootstrap pin all read the same value;
   `awf sync` therefore genuinely clears an ahead-note, making ADR-0039 Decision 3's
   parenthetical true as written. Amends ADR-0030 Decision 4 (the precedence chain is retired
   with its invariant).

2. **Build provenance is display-only.** `awf version` prints `project.Version` and may append
   build metadata from `debug.ReadBuildInfo()` (module pseudo-version, VCS revision) as a
   parenthetical suffix. Provenance never feeds gating, stamping, or pinning.

3. **A release tag must equal `project.Version`.** The release pipeline fails when the tag's
   version differs from the const. `docs/releasing.md` drops the "skip the version-const edit if
   it already matches" allowance's optional tone: the release-prep commit is where the const and
   changelog move together, and the pipeline guard makes divergence impossible rather than
   documented-against.

4. **Schema-generation bumps force version bumps, mechanically.** A `minVersionBySchema`
   table in `internal/project` maps each config-schema generation to the minimum
   `project.Version` allowed to render it. A test asserts (a) an entry exists for
   `migrate.Current()` and (b) `semver.Compare(project.Version, min) >= 0`. Adding a migration
   without a table entry, or with an unbumped const, fails the gate. (`internal/project` already
   imports `internal/migrate`; the reverse placement would cycle.)

5. **Bump `project.Version` to `0.6.0` now**, seeding the table with `{6: "0.6.0"}` — schema
   generation 6 is already live past `v0.5.1`. Until `v0.6.0` is tagged, renders pin a not-yet-
   published version: download fails, the ADR-0048 hook-shim fallback covers hooks, and
   Decision 6 covers machines with a source-installed awf. The next release should follow
   promptly once the fix batch lands.

6. **Bootstrap resolves a matching local binary before downloading.** If an `awf` on PATH
   reports exactly the pinned version, the bootstrap prints that binary's path and exits without
   fetching. Pin-exactness is preserved; the checksum step applies to downloads only (a PATH
   binary is already in the user's trust domain). This closes the unpublished-pin window for
   source-channel adopters and CI images with awf preinstalled.

7. **Bootstrap output and portability hardening.** All diagnostics — including checksum
   verification output — go to stderr; stdout carries exactly one line, the resolved binary
   path. Checksum verification falls back to `shasum -a 256` when `sha256sum` is absent
   (stock macOS); both branches verify before install, and ADR-0040's `bootstrap-checksum`
   backing test is extended to cover both.

## Invariants

- `inv: single-version-authority` — `awfVersion()` returns `project.Version`; no ldflags var or
  module build info feeds version gating, lock stamping, or bootstrap pinning.
- `inv: schema-min-version` — `minVersionBySchema` contains an entry for `migrate.Current()`,
  and `project.Version` is semver-at-or-above it.
- `inv: bootstrap-stdout-path-only` — the rendered bootstrap writes exactly one line to stdout:
  the resolved binary path; all diagnostics go to stderr.
- `inv: bootstrap-local-first` — the rendered bootstrap uses a PATH `awf` reporting exactly the
  pinned version before attempting any download.
- The release pipeline refuses a tag that does not equal `project.Version` (textual contract;
  lives in CI, outside the invariant scanner's source globs).

## Consequences

- Source-built binaries report the const (with provenance suffix); the permanent ahead-note
  disappears, and when a genuine ahead-note fires, `awf sync` clears it as the message promises.
- Every future schema bump carries a version bump in the same change or the gate fails — the
  class of failure 2 cannot recur. `docs/releasing.md` and ADR-0030's release flow lose the
  ldflags stamping step and gain the tag guard.
- Until `v0.6.0` ships, direct `bash .awf/bootstrap.sh` invocations on machines with no matching
  local awf fail with a download error. Accepted: strictly better than today's state, where the
  download *succeeds* and delivers a binary that cannot operate on the tree.
- `awf version` output changes shape for release builds (const + provenance instead of the
  injected tag — identical value under the Decision 3 guard).
- Retires `version-ldflags-precedence` (ADR-0030) once Implemented; ADR-0039's
  `version-compat-gate` invariant is untouched — the comparison logic stands, only its binary-side
  operand becomes single-sourced.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep ldflags precedence, only demote the build-info branch | Retains a second identity source that the tag guard must then police anyway; `awf version` and the lock would still disagree on source builds. |
| Fix only the comparison (`lockVsBinary` reads the const, `awfVersion()` unchanged) | Clears the note but gate errors and `awf version` keep reporting an identity that is never stamped or pinned — the split that caused all three failures survives. |
| Derive the version purely from VCS build info (drop the const) | Pseudo-versions are not downloadable release assets; breaks self-pinning (ADR-0040) and reproducible renders. |
| Detect released-ness and render a PATH-fallback bootstrap for unreleased versions | Requires remote knowledge at render time and weakens the self-pinning invariant; local-first resolution achieves the same relief deterministically. |
| Runtime compatibility probe in the hook shim (fall back when the pinned binary rejects the tree) | Treats the symptom in every rendered payload instead of removing the version split at its source. |
