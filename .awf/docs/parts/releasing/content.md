How to cut a release of the `awf` binary. [ADR-0030](decisions/0030-prebuilt-binary-distribution-and-release.md) owns the distribution model.

## Release runbook

1. On a clean `main`, audit the release range:

   ```
   ./awf audit <previous-tag>..HEAD
   ```

   The audit is advisory but must be clean. The required range starts at the previous tag and includes every commit being shipped. Local hooks are optional preflight. Before tagging, require the pushed `main` commit's complete CI run to succeed; the tag workflow consumes those exact-revision conclusions and verifies publication identity.

2. Set `internal/project/VERSION` to the target `MAJOR.MINOR.PATCH` with its standing newline. Promote `changelog/CHANGELOG.md`'s entries: rename `## [Unreleased]` to `## [0.2.0] - YYYY-MM-DD`, then add a new empty `## [Unreleased]` above it. Entries are grouped by adopter-facing effect: Breaking changes, Features, Bug fixes, or Others.

   ```
   printf '0.2.0\n' > internal/project/VERSION
   ./x render
   go run ./cmd/releasecheck
   git add internal/project/VERSION changelog/CHANGELOG.md .awf/awf.lock
   git commit -m "chore(awf): bump version to v0.2.0"
   ```

   A schema-coupled bump often already changed the version mid-cycle. It changes the version file and lock, not the changelog. During development the gate requires descending changelog entries with the newest at or below `project.Version`; releasecheck requires an exact newest-version match and an empty `[Unreleased]`. The canonical three-file release-prep transaction skips Go and Pi test suites locally while versioncheck and every static gate still run. The pushed commit's aggregate `CI / gate` conclusion supplies repository assurance before tagging.

3. Push `main` and wait for that commit's `CI / gate` check to succeed. Then tag and push the matching version:

   ```
   git push origin main
   # Wait for the pushed commit's CI run to succeed.
   git tag v0.2.0
   git push origin v0.2.0
   ```

4. Watch the `Release` workflow. On success, Releases contains four archives, `checksums.txt`, and curated changelog notes. Linux tarballs record portable `root:root` ownership, executable mode for `awf`, and regular-file modes for `LICENSE` and `README.md`, so they extract under a restricted rootless user namespace as well as an ordinary user.

The live GitHub `release tags` ruleset requires successful `CI / gate` before tag creation or update. The release workflow verifies `CI / gate`, checkout and tag identity, ancestry on `main`, `project.Version`, and curated notes, then constructs and validates one production-equivalent snapshot before its needs-bound credential-bearing GoReleaser job. It does not repeat repository tests or lint. GoReleaser builds linux and darwin archives for amd64/arm64, bundles `LICENSE` and `README.md`, writes `checksums.txt`, and uses curated notes through `--release-notes`. It refuses a dirty checkout, including untracked files; workflow artifacts therefore belong under `$RUNNER_TEMP` or a deliberately ignored path. Release notes use `"$RUNNER_TEMP/release-notes.md"`. Commit-derived GoReleaser notes are disabled (ADR-0096).

## Versioning

awf is pre-1.0 and uses `vMAJOR.MINOR.PATCH` SemVer. The newline-terminated `internal/project/VERSION`, embedded as `project.Version`, is the single authority (ADR-0049): it drives `awf version`, lock `AWFVersion`, the bootstrap pin, and the binary-version gate. The tag must match it. `cmd/releasecheck` requires the canonical AGPL-3.0-only `LICENSE`, matching README badge and footer, archive inclusion, a newest changelog entry equal to `project.Version`, and an empty standing `[Unreleased]` section (ADR-0078). `minVersionBySchema` must cover the current schema generation at or below `project.Version`.

## Validate configuration locally

```
go run github.com/goreleaser/goreleaser/v2@v2.17.0 check
```

Keep the command version aligned with workflow `version:`; `cmd/pincheck` enforces the workflow pin (ADR-0079). Production snapshot construction and portability validation belong to the read-only release verification job. Ordinary local Go tests use synthetic archive fixtures and never invoke GoReleaser.

## Rollback

Delete a bad tag locally and remotely, then delete its GitHub Release:

```
git tag -d v0.2.0
git push origin :refs/tags/v0.2.0
gh release delete v0.2.0
```

## Security and distribution

Prebuilt downloads are canonical; `go install` is the source fallback. The public repository permits unauthenticated downloads, including `gh release download v0.2.0` and `go install github.com/hypnotox/agentic-workflows/cmd/awf@latest`.

The archive and `checksums.txt` travel through the same GitHub Release channel. Comparing them detects transfer corruption, but does not independently establish publisher authenticity: a compromised release workflow or token can replace both. SHA-pinned actions, exact-revision main and tag rulesets, Dependabot currency, and tag gate-and-ancestry checks are the accepted mitigations; immutable releases, artifact attestation, and cosign remain deferred (ADR-0079).

`.goreleaser.yaml` and workflow files are hand-maintained outside the awf render/lock set, like `.golangci.yml` and `./x`.

## Adopter upgrade recovery

Adopter projects use permanent lock authority. A supported schema upgrade journals its mutations and publishes the replacement lock last. If `.awf/current-state-upgrade.journal` remains after interruption, run only `awf upgrade --recover`; precommit recovery restores prior bytes and residents, while postcommit recovery cleans transaction residue without rolling authority back. If the journal is unusable, restore the project from Git and retry from a supported source.

### Exact revision acceptance

Release verification reads the successful `CI / gate` conclusion for the exact tag SHA before publication. The live tag ruleset requires that same conclusion before accepting the tag. The workflow checks out and rechecks the tag SHA, constructs and validates snapshot archives with read-only credentials, and only then enables the credential-bearing publish job. Repository behavior remains owned by the exact-revision CI run. These controls constrain publication; they are not an independent authenticity system for artifacts and checksums distributed through the same channel.
