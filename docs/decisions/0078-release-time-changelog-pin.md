---
status: Implemented
date: 2026-07-08
tags: [changelog, release-pipeline]
related: [41, 49, 73]
domains: [tooling]
---
# ADR-0078: Release-time changelog pin

## Context

`TestChangelogLatestMatchesVersion` (`cmd/awf/changelog_test.go`, added in `62bff64` with no
ADR mandating it) pins the newest embedded changelog entry to `project.Version` at every
commit. ADR-0049 Decision 4 ties schema-generation bumps to version bumps, so the const bumps
mid-cycle, and the pin then forces promoting `[Unreleased]` to a versioned, dated section
long before any tag exists. The schema-7 cycle demonstrated every downstream cost:

- The promotion stamps a **provisional date** that goes stale if the tag slips (the
  `[0.11.0] - 2026-07-08` section sat released-looking but untagged).
- The standing `[Unreleased]` section had to be restored by hand afterwards (`f384fdf`).
- The forced promotion **conflicts with ADR-0073's `changelog-unreleased` rule**: after a
  promotion, the next adopter-facing change must still land under `[Unreleased]` (the rule
  errors otherwise), and nothing enforces folding those entries into the already-promoted
  section at release: content can silently ship outside the release notes.

The pin's underlying guarantee is still wanted: a released binary must embed its own release
notes as its newest entry, and notes must never exist for a version the binary does not
carry. But the guarantee is only *needed* at release time, and release CI currently enforces
none of it: `ci.yml` has no tag trigger and `release.yml` runs no tests, so once the
every-commit pin is relaxed, the release workflow is the sole place the exact match can be
enforced.

Two further facts constrain the design. `TestRunChangelogSinceLatest` also couples to the
exact pin (it calls `changelog.Since(entries, project.Version)`, which errors when
`project.Version` has no entry, exactly the mid-cycle window this ADR legalises).
And `internal/changelog.Parse` deliberately ignores `[Unreleased]` (its header regex accepts
only dated numeric headers, per ADR-0073) and validates no ordering: `entries[0]` is merely
file order, so nothing today proves the changelog is sorted newest-first.

## Decision

1. **The gate test checks ordering, not identity.** `TestChangelogLatestMatchesVersion` is
   replaced by a monotonicity check: every adjacent entry pair is strictly descending
   (`semver.Compare` with the `"v"` prefix over the bare `X.Y.Z` entry versions), and the
   newest entry is at or below `project.Version`: it fails **iff**
   `semver.Compare("v"+entries[0].Version, "v"+project.Version) > 0` or any pair is
   non-descending. Mid-cycle, `project.Version` may run ahead of the changelog; the changelog
   may never run ahead of the binary, and the file may never be mis-sorted.
   `TestRunChangelogSinceLatest` derives its `--since` argument from `entries[0].Version`
   instead of `project.Version`, decoupling it from the pin.

2. **The exact pin moves to the release gate.** A repo-local `cmd/releasecheck` (the
   `covercheck`/`repoaudit` pattern: logic behind a unit-tested `run` seam, `main` a
   coverage-ignored `os.Exit` wrapper) loads the embedded changelog and fails unless
   (a) the newest entry's version equals `project.Version`, (b) the standing
   `## [Unreleased]` header is present (its absence is a failure, since it is the anchor
   ADR-0073's `changelog-unreleased` rule keys on) and (c) that section's body is empty
   after `strings.TrimSpace` (the standing header with surrounding blank lines counts as
   empty). Since the parser discards `[Unreleased]` by design, the header/emptiness half
   reads the raw embedded bytes with its own section walk (prior art: `repoaudit`'s
   `unreleasedSection`, which is `gitFunc`-bound and not reusable).
   `release.yml` runs it immediately after the existing tag-equals-`project.Version` step, so
   a tag can neither ship without its own release notes nor strand late entries under
   `[Unreleased]`. A companion test backs the wiring: it reads
   `.github/workflows/release.yml` and asserts a `releasecheck` invocation step appears
   before the GoReleaser step, so unwiring the check fails the gate.

3. **Changelog promotion becomes a release-time act.** `[Unreleased]` accumulates entries all
   cycle regardless of version bumps; renaming it to `## [X.Y.Z] - <date>` (real date) and
   adding a fresh empty `[Unreleased]` happens in the release-prep commit, immediately before
   tagging, and that step runs `go run ./cmd/releasecheck` locally as a pre-tag rehearsal,
   so a pin violation is caught before the tag exists rather than by the workflow after.
   `docs/releasing.md` is updated throughout (the intro's tag-push description, the
   Versioning section's hard-fail list, and step 2), the pitfalls entry describing the forced
   mid-cycle promotion retires, the "Growing a pinned set breaks exact-assertion tests"
   pitfalls entry drops its stale exact-pin citation, and AGENTS.md's Invariants list gains a
   bullet for the new contracts; a mid-cycle `project.Version` bump (ADR-0049 Decision 4 is
   unchanged) now touches only the const and the lock.

## Invariants

- `invariant: changelog-monotonic`: the gate fails when the embedded changelog's entries are not
  strictly descending by semver or the newest entry exceeds `project.Version`; it does not
  fail merely because `project.Version` has no entry yet.
- `invariant: release-changelog-pin`: `cmd/releasecheck` exits non-zero unless the newest embedded
  changelog entry equals `project.Version` and a standing `[Unreleased]` section is present
  and empty modulo whitespace; a gate test asserts `release.yml` invokes it before the
  GoReleaser step.

## Consequences

- Mid-cycle version bumps stop cascading into the changelog: no provisional dates, no
  hand-restored `[Unreleased]`, no released-looking-but-untagged sections. ADR-0073's
  `changelog-unreleased` rule and the gate test stop pulling in opposite directions:
  `[Unreleased]` is always the one home for in-cycle entries.
- The exact-match guarantee narrows from every-commit to release-time. A stale or missing
  release section is now caught by the release workflow rather than the local gate, later
  (at tag push) but at the only moment it matters, and it fails before any artifact builds.
  A workflow-side failure still costs the full undo-a-bad-tag cleanup (delete the tag
  locally and remotely, delete the auto-created GitHub Release); the pre-tag rehearsal in
  the release runbook exists precisely to make that path unlikely, with `release.yml` as
  the backstop.
- A new repo-local binary joins the 100%-coverage and dead-code gates; its `main` follows the
  established coverage-ignored wrapper pattern, so the marginal cost is one tested `run`
  function.
- The `[Unreleased]`-section walk in `releasecheck` duplicates ~15 lines of parsing beside
  `repoaudit`'s: accepted, consistent with ADR-0041's small-duplicate stance, since the two
  read from different sources (embedded FS vs `git show`).
- The release-prep commit gains one step (promote + stamp the real date), guarded
  mechanically by `releasecheck` instead of prose.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the every-commit exact pin | Forces premature promotion on every mid-cycle bump; conflicts with ADR-0073's `[Unreleased]` rule; caused the 0.11.0 provisional-date confusion. |
| Shell step in `release.yml` (grep/awk over the changelog) | Duplicates the header grammar outside Go, untested; the repo's deterministic checks live in tested Go mains. |
| `awf changelog --verify-release` flag on the shipped binary | Repo-internal release plumbing on an adopter-facing CLI surface: wrong audience. |
| Keep the exact pin as a build-tag/env-guarded test run only by `release.yml` | Tag-guarded files sit outside the default `./...` compile and complicate the 100%-coverage and dead-code gates; the repo's convention is deterministic checks as tested Go mains. |
| Relax to `entries[0] ≤ Version` without the descending-order assertion | Leaves file order unvalidated; a mis-sorted entry would silently become "newest" and dodge both pins. |
