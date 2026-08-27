The changelog package parses and validates the changelog and release-notes pipeline. The claims below capture the current changelog and release contracts.

## Claims

### `invariant: changelog-embed-decodes`

The changelog embedded in the binary decodes at load time into at least one parsed entry, with no network access. Loading the embedded changelog file system returns entries without error.
Origin: ADR-0041
Backing: test

### `invariant: changelog-flags-exclusive`

The changelog command rejects a version selector and a since selector supplied together, returning a usage error rather than running. The two range flags are mutually exclusive.
Origin: ADR-0041
Backing: test

### `invariant: changelog-monotonic`

At every commit the embedded changelog entries must be strictly descending by semantic version, and the newest entry must not exceed the binary's version constant. Exact release-notes matching is deferred to release time.
Origin: ADR-0078
Backing: test

### `invariant: changelog-range-chronological`

Extracting a changelog range requires the from and to endpoints to be in chronological order. Passing a newer starting point and an older ending point returns an error.
Origin: ADR-0041
Backing: test

### `invariant: changelog-rule-advisory`

The repo-audit changelog-conformance verdict (an adopter-facing change in the range while the [Unreleased] section is unchanged) is reported as a Warning with a zero exit code, while a git or read failure inside the same rule remains an Error.
Origin: ADR-0107
Backing: test

### `invariant: release-changelog-pin`

The release check refuses to pass unless the newest versioned changelog entry matches the version being released. A changelog whose newest entry is older than the release version fails with a message to promote the unreleased section before tagging.
Origin: ADR-0078
Backing: test

### `invariant: release-gate-on-tag`

The release workflow verifies successful exact-SHA CI conclusions, checkout and tag identity, prior-release mutation selection, origin/main ancestry, ./x gate, and ./x check before its needs-bound credential-bearing GoReleaser publish step.
Origin: ADR-0079
Revised-by: ADR-bind-repository-and-release-acceptance-to-exact-revisions
Backing: test

### `rule: hosted-release-protection`

The live GitHub `release tags` ruleset applies to `refs/tags/v*` with no bypass actor and requires GitHub Actions checks `CI / gate` and `CI / release-config` on the exact tagged revision before the ref can be created or updated.
Origin: ADR-bind-repository-and-release-acceptance-to-exact-revisions

### `invariant: release-notes-from-changelog`

The GitHub Release body is sourced from the curated changelog: the release workflow extracts the tagged version's section via `awf changelog --version` and passes it to GoReleaser through --release-notes before the GoReleaser step runs, and .goreleaser.yaml disables GoReleaser's own commit-derived changelog, so a commit subject can never reach the release notes.
Origin: ADR-0096
Backing: test
