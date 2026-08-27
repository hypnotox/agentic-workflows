---
format: current-state-v4
slug: load-curated-release-notes-through-goreleaser
status: Proposed
date: 2026-08-27
---
# ADR-load-curated-release-notes-through-goreleaser: Load curated release notes through GoReleaser

## Context

ADR-0096 made `changelog/CHANGELOG.md` the authority for adopter-facing GitHub Release
notes. Its implementation both supplied the tagged version's curated section through
GoReleaser's `--release-notes` flag and set `.goreleaser.yaml`'s `changelog.disable` to
true. The invariant test enforced both facts but did not exercise their combined runtime
semantics.

GoReleaser v2.17.0 evaluates `changelog.disable` before its changelog pipe runs. A disabled
pipe therefore never loads the custom release-notes file. Production evidence confirms the
result: every published release from v0.18.0 through v0.40.0 has a one-byte blank body.
v0.41.0 was also published blank and required manual repair. Releases before v0.18.0
predate the curated-notes wiring and contain commit-derived bodies.

When the changelog pipe is enabled and `--release-notes` names a file, GoReleaser loads that
file and returns without deriving a changelog from commits. The custom file, rather than a
configuration-level pipe disable, is therefore the boundary that excludes commit subjects
from publication. Snapshot builds do not publish and GoReleaser skips the changelog pipe for
snapshots independently.

## Decision

1. `decision: custom-notes-own-publication` Published GitHub Release notes remain sourced
   exclusively from the tagged version's curated changelog section supplied as GoReleaser's
   custom release-notes file. GoReleaser's changelog pipe remains enabled so it can load that
   file, while commit-derived `use`, groups, and filters remain absent.
2. `decision: prove-composed-semantics` Release verification must include both the corrected
   configuration-and-workflow contract and regression evidence that exercises their composed
   behavior, proving the supplied curated file becomes the exact release body without
   commit-derived output.

## State changes

- update `tooling/changelog-and-release:release-notes-from-changelog`

## Consequences

- GoReleaser can load and publish the curated notes file, restoring the outcome ADR-0096
  intended without reintroducing commit-derived release notes.
- The configuration no longer uses `changelog.disable: true`; absence of commit-derived
  `use`, groups, and filters plus the custom notes input preserves one adopter-facing source
  of truth.
- Verification must catch the exact incompatible configuration that produced blank bodies
  and exercise the composed release-note behavior rather than checking its parts alone.
- Existing blank historical release bodies require an operational backfill from their exact
  curated changelog sections. That repair changes hosted presentation only; it does not move
  tags or replace assets.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the changelog pipe disabled and edit every release after GoReleaser publishes | This adds a second credential-bearing publication mechanism, creates an avoidable partial-success state, and preserves the configuration that prevents the documented custom-notes path from working. |
| Re-enable commit-derived GitHub changelog configuration with improved filters | This restores competing sources of truth and recreates the leak class ADR-0096 eliminated. |
| Leave future releases dependent on manual repair | Manual correction is not deterministic and already failed silently across 18 releases. |

## Status history

- 2026-08-27: Proposed
