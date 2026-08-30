The changelog package parses and validates the changelog and release-notes pipeline. The claims below capture the current changelog and release contracts.

## Claims

### `invariant: changelog-embed-decodes`

The changelog embedded in the binary decodes at load time into at least one parsed entry, with no network access. Loading the embedded changelog file system returns entries without error.
Backing: test

### `invariant: changelog-flags-exclusive`

The changelog command rejects a version selector and a since selector supplied together, returning a usage error rather than running. The two range flags are mutually exclusive.
Backing: test

### `invariant: changelog-monotonic`

At every commit the embedded changelog entries must be strictly descending by semantic version, and the newest entry must not exceed the binary's version constant. Exact release-notes matching is deferred to release time.
Backing: test

### `invariant: changelog-range-chronological`

Extracting a changelog range requires the from and to endpoints to be in chronological order. Passing a newer starting point and an older ending point returns an error.
Backing: test

### `invariant: changelog-rule-advisory`

The repo-audit changelog-conformance verdict (an adopter-facing change in the range while the [Unreleased] section is unchanged) is reported as a Warning with a zero exit code, while a git or read failure inside the same rule remains an Error.
Backing: test

### `invariant: release-changelog-pin`

The release check refuses to pass unless the newest versioned changelog entry matches the version being released. A changelog whose newest entry is older than the release version fails with a message to promote the unreleased section before tagging.
Backing: test

### `invariant: release-gate-on-tag`

The release workflow verifies the successful exact-SHA `CI / gate` conclusion, checkout and tag identity, origin/main ancestry, the release version, curated notes, and production snapshot archive integrity before its needs-bound credential-bearing GoReleaser publish step. It does not repeat repository test or lint assurance.
Backing: test

### `invariant: release-platforms`

Release archives contain exactly Linux/amd64, Linux/arm64, Darwin/amd64, and Darwin/arm64 artifacts. Windows is unsupported and has no production, test, or release implementation.
Backing: test

### `rule: hosted-release-protection`

The live GitHub `release tags` ruleset requires the app-bound `CI / gate` conclusion on the exact tagged revision and no retired release-configuration status.

### `invariant: release-notes-from-changelog`

The GitHub Release body is sourced from the curated changelog: the release workflow extracts the tagged version's section via `awf changelog --version`, passes it to GoReleaser through `--release-notes` while commit-derived `use`, groups, and filters remain absent, and verifies the published body against that exact file. A commit subject cannot reach the release notes.
Backing: test
