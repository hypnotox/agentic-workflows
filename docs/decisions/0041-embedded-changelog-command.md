---
status: Implemented
date: 2026-07-01
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [tooling, changelog, rendering]
related: [17, 30]
domains: [tooling]
---
# ADR-0041: Embedded Changelog and the `awf changelog` Command

## Context

Adopters install the `awf` binary into their own, unrelated project (prebuilt binary or `go
install`, ADR-0030) and currently have no offline way to see what changed between awf releases.
GoReleaser already auto-generates GitHub Release notes per tag from Conventional Commits (grouped
Features / Bug fixes / Others, excluding `docs:`/`test:`/`chore:`/`ci:` — see the `changelog:` block
in `.goreleaser.yaml`), but reading them requires visiting GitHub and trusts a commit's type prefix
to say whether it was user-facing.

That trust is misplaced at least once already: `v0.3.1`'s only commit besides the routine
version-bump commit is typed `docs:` ("instruct adopters to wire awf check into their gate"), yet it
edited `templates/docs/workflow.md.tmpl` — a rendered template every adopter's `workflow.md` is built
from. GoReleaser's own filter would have (and did) treat that release as content-free.

We want a command, `awf changelog`, that works with zero network dependency (matching every other
awf command) and is curated by actual adopter-facing effect, not by mirroring an automated
type-prefix proxy.

## Decision

1. New top-level `changelog/` package (sibling of the existing top-level `templates/` package):
   `changelog/CHANGELOG.md` is the hand-maintained, newest-first source of truth, with headers
   `## [X.Y.Z] - YYYY-MM-DD`. `changelog/embed.go` exposes `var FS embed.FS` via
   `//go:embed CHANGELOG.md`, mirroring `templates/embed.go` exactly. A new top-level package is
   required because `go:embed` cannot embed a file outside its own package directory — CHANGELOG.md
   cannot live at the true repo root and still be embedded by a package under `internal/`.
2. New `internal/changelog` package parses the embedded raw markdown into an ordered `[]Entry`
   (Version, Date, raw section body), splitting on a package-level anchored header regex
   (`^## \[(\d+\.\d+\.\d+)\] - (\d{4}-\d{2}-\d{2})$`), mirroring the `ccRe`/`adrNameRe` idiom already
   used in `internal/audit`. It exposes three filters: `Version(entries, v)`, `Since(entries, v)`
   (exclusive of `v` — everything released after it), and `Range(entries, from, to)` (inclusive of
   both ends; `from` must be the chronologically older version and `to` the newer, git
   range convention, no silent reordering). All three normalize a leading `v` for comparison via
   `golang.org/x/mod/semver` (already a dependency) rather than reusing `cmd/awf`'s private,
   unexported `normalizeSemver` — a small, deliberate local duplication of a four-line idiom instead
   of a cross-package move.
3. New `awf changelog` command (`cmd/awf/changelog.go`), wired into `main.go`'s existing
   `argSpecs`/dispatch pattern. No flags prints the whole embedded file verbatim. `--version <v>`
   prints one version's section. `--since <v>` prints every version strictly newer than `v`.
   `--range <from>..<to>` (single flag value, not two positionals) prints every version in
   `[from, to]`. The three flags are mutually exclusive: 2+ given is a usage error (exit 2, the
   existing `usageErr` convention). An unmatched version in any flag is a runtime error (exit 1),
   distinct from a usage error.
4. `changelog/CHANGELOG.md` entries are grouped per version into up to four sections — Breaking
   changes, Features, Bug fixes, Others — chosen by actual adopter-facing effect (does it change
   rendered template output, CLI behavior, or config/lock schema), not by mechanically mirroring a
   commit's Conventional-Commits type or `.goreleaser.yaml`'s type-prefix exclude filter. A release
   with no adopter-facing effect still gets a version header, with an explicit "_No user-facing
   changes in this release._" body, so every real tag resolves under `--version`/`--range` rather
   than silently disappearing.
5. `docs/releasing.md`'s cut-a-release runbook gains a required step to update
   `changelog/CHANGELOG.md`, folded into the existing step that bumps `internal/project/project.go`'s
   `Version` const, so both land in the same release-prep commit.
6. `changelog/CHANGELOG.md` is backfilled now for every existing tag, v0.1.0 through the release
   current at authoring time, using real tag dates (`git tag --format='%(creatordate:short)'`), so
   `awf changelog` is useful immediately regardless of which version an adopter is currently on.

## Invariants

- `invariant: changelog-embed-decodes` — the embedded `changelog/CHANGELOG.md` always parses without
  error; enforced by a test that parses the real embedded `changelog.FS` content directly (not a
  synthetic fixture), mirroring how `internal/catalog`'s tests load the real `templates.FS`. Any
  downstream call site may then treat a parse failure as unreachable.
- `invariant: changelog-range-chronological` — `internal/changelog.Range` rejects a `from` version that
  is chronologically newer than `to` rather than silently reordering them.
- `invariant: changelog-flags-exclusive` — `awf changelog` rejects a command line that sets 2 or more of
  `--version`/`--since`/`--range` at once.
- Category placement in `changelog/CHANGELOG.md` (Breaking changes / Features / Bug fixes / Others)
  is judged by adopter-facing effect, not by a commit's Conventional-Commits type or
  `.goreleaser.yaml`'s filter — a textual contract for whoever authors the next release's entry, not
  a machine-checkable one.

## Consequences

Adopters get an always-available, offline `awf changelog`, consistent with every other awf command
having zero network dependency. Every future release-prep commit gains one more required
hand-authored artifact — real, ongoing maintenance cost, relying on the runbook step in Decision 5
and reviewer discipline alone, with no automated backstop if it is missed.

The curated changelog will diverge from GoReleaser's auto-generated GitHub Release notes for the
same tag: different category rules, different inclusion criteria. That is accepted duplication, not
a bug — GoReleaser's notes are a mechanical, always-generated safety net; `changelog/CHANGELOG.md`
is a curated, adopter-facing narrative with a different purpose.

A malformed embedded `changelog/CHANGELOG.md` is a hard, unrecoverable error at parse time. That is
acceptable because it is a build-time-controlled asset validated by `inv: changelog-embed-decodes`
against the real embedded content, not user input — the same posture `internal/catalog` already
takes toward `templates.FS`.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Fetch the changelog from the GitHub Releases API at runtime | Introduces awf's first network dependency and a new failure mode (offline, rate-limited, no token) — rejected in favor of full-offline consistency with every other command. |
| Generate the changelog from local git tags + Conventional Commits, mirroring `internal/audit`'s commit walk | Only works if the adopter has awf's own source repo cloned locally; most adopters run the installed binary inside an unrelated project and have no such history to walk. |
| Mirror `.goreleaser.yaml`'s exact type-prefix exclude filter for backfill content | `v0.3.1`'s only user-facing effect landed in a `docs:`-typed commit that touched a rendered template; a hand-curated file has no excuse to inherit an automated proxy's blind spot, so effect-based curation was chosen instead. |
| Fold Breaking changes into the Features/Bug fixes categories, matching GoReleaser's own grouping | Adopters upgrading need an explicit, scannable signal when a config migration or CLI behavior change requires action; worth diverging from GoReleaser's taxonomy for that visibility. |
| Add an `internal/audit` rule flagging a `Version`-const change uncoupled from a `changelog/CHANGELOG.md` change | Every existing audit rule sources its paths from configurable `Inputs` fields populated from `.awf/config.yaml`; hardcoding awf's own repo paths would bake a permanent no-op into the shared engine every adopter's `awf audit` runs, and a version-file/changelog-file pairing isn't a shape most adopters are expected to have — deferred rather than generalized or hardcoded. |
