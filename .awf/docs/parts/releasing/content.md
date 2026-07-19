How to cut a release of the `awf` binary. The distribution model and its rationale are
[ADR-0030](decisions/0030-prebuilt-binary-distribution-and-release.md); this is the runbook.

A release is a `v*` git tag. Pushing the tag triggers `.github/workflows/release.yml`, which
verifies the tag, `project.Version`, and changelog all pin the same release (see Versioning),
verifies the tagged commit is on `main`, runs the full gate (`./x gate && ./x check`),
extracts the curated release notes for the tagged version (`awf changelog --version`), then runs
GoReleaser (`.goreleaser.yaml`) to build cross-platform binaries (linux/darwin/windows ×
amd64/arm64), package per-OS archives bundling `LICENSE` + `README.md`, write `checksums.txt`,
and create the GitHub Release whose body is those notes, passed as `--release-notes`. GoReleaser's
own commit-derived changelog is disabled: deriving notes from commit subjects leaked internal
commits whose scopes dodged the exclude filters (ADR-0096). Prebuilt binary download
is the canonical install path; `go install` is the source fallback.

## Versioning

awf is pre-1.0; versions are `vMAJOR.MINOR.PATCH` (SemVer). `project.Version`
(`internal/project/project.go`) is the single version authority (ADR-0049): it drives `awf
version`, the lock's `AWFVersion`, the bootstrap pin, and the binary-version gate. The git tag
must equal it; the Release workflow hard-fails on a mismatch before building, so the tag can
never mint a version the binary does not carry. The workflow also hard-fails when the changelog
does not pin the release (`cmd/releasecheck`, ADR-0078): the newest entry must equal
`project.Version` and the standing `[Unreleased]` section must be present and empty, so a tag
can neither ship without its own release notes nor strand late entries outside them.
Schema-generation bumps raise the floor
mechanically: `minVersionBySchema` must contain an entry for the current generation, at or
below `project.Version`, or the gate fails.

## Cut a release

1. **Confirm `main` is green and clean.** On `main`, working tree clean:

   ```
   ./x gate && ./x check && ./x audit <previous-tag>..HEAD
   ```

   All three must pass (`audit` is advisory but should be clean for a release). The audit
   range is required and has no default (ADR-0127): use the previous release tag as the base,
   so the audit covers exactly the commits this release ships.

2. **Verify `project.Version` equals the target version and promote the changelog.** A
   schema-coupled change bumps the const mid-cycle (ADR-0049 Decision 4), so it often already
   matches; bump it only when it does not. A mid-cycle bump touches only the const and the
   lock, never the changelog; the gate holds only ordering (entries strictly descending,
   newest at or below `project.Version`; ADR-0078). Changes accumulate under a standing
   `## [Unreleased]` section at the top of `changelog/CHANGELOG.md` as they land, grouped
   into Breaking changes/Features/Bug fixes/Others by adopter-facing effect (ADR-0041), so
   the changelog is always release-ready. Now, at release, rename that header to
   `## [0.2.0] - YYYY-MM-DD` (the real date you tag) and add a fresh empty `## [Unreleased]`
   above it. `awf changelog` ignores the `[Unreleased]` section (its parser only recognises
   numeric-versioned headers). A changelog entry is required for every tag; rehearse the
   release gate locally so a pin violation is caught before the tag exists rather than by
   the workflow after:

   ```
   go run ./cmd/releasecheck
   ./x gate && ./x check
   git add internal/project/project.go changelog/CHANGELOG.md .awf/awf.lock
   git commit -m "chore(awf): bump version to v0.2.0"
   ```

   (`.awf/awf.lock` co-changes because `AWFVersion` is recorded in the lock; stage it if `./x check`
   reports it.)

3. **Push `main`.**

   ```
   git push origin main
   ```

4. **Tag and push the tag.**

   ```
   git tag v0.2.0
   git push origin v0.2.0
   ```

   The tag push starts the `Release` workflow. It refuses a tag whose commit is not on
   `origin/main` and re-runs the gate before building (ADR-0079), so pushing `main` first
   (step 3) is load-bearing, not just tidy. Watch it in the GitHub Actions tab; on success the
   release appears under Releases with the six archives, `checksums.txt`, and the changelog.

## Preview locally (no publish)

Validate the GoReleaser config and dry-run the full build without tagging or publishing:

```
go run github.com/goreleaser/goreleaser/v2@v2.17.0 check
go run github.com/goreleaser/goreleaser/v2@v2.17.0 release --snapshot --clean
```

The version matches the `version:` input pinned in the workflows; `cmd/pincheck` enforces
the workflow side; keep these two commands in step by hand (ADR-0079).

`--snapshot` writes artifacts to `dist/` (gitignored) and skips the GitHub Release. The same two
commands run on every pull request via the `release-config` job in `.github/workflows/ci.yml`, so a
broken release config fails CI before any tag is pushed.

## Real Pi exploration smoke

On Pi 0.80.9 or newer, run one successful exploration call with a named task, `targeted` breadth, and `paths` detail. Then run a named task with `bounded` breadth that returns not-found and follow
it with a new fresh-context call using a corrected task or `broad` breadth; every call must name
task, breadth, and detail. Confirm intermediate activity stays in tool details and only the final
report enters model-visible content.

## Notes

- **Repo visibility.** The repo is public, so release binaries download without authentication
  (from the Releases page or via `gh release download v0.2.0`) and
  `go install github.com/hypnotox/agentic-workflows/cmd/awf@latest` resolves without a token.
- **Undo a bad tag.** If a tag was pushed in error, delete it locally and remotely, then delete the
  GitHub Release it created:

  ```
  git tag -d v0.2.0
  git push origin :refs/tags/v0.2.0
  gh release delete v0.2.0
  ```

- **Tamper posture (ADR-0079).** `checksums.txt` and the bootstrap's SHA-256 check verify
  download *integrity*, not publisher authenticity: a compromise of the release workflow or
  its token can rewrite binary and checksums together. The accepted mitigations are the
  SHA-pinned actions, dependabot currency, and the gate-and-ancestry checks on tag push;
  artifact attestation and cosign signing are deliberately deferred (revisit at 1.0 or on
  adopter demand).
- **Hand-maintained files.** `.goreleaser.yaml` and the workflow files live outside awf's
  render/lock set (like `.golangci.yml` and `./x`), so `awf check` does not track them; edit them
  directly.
- **Current-state attestation (bridge release).** Attesting a project to current-state authority is
  not part of cutting an awf release; it is an adopter operation the bridge release ships. The
  sequence is: obtain and verify the matching current-state binary, reach a clean HEAD with `./x check`,
  `./x gate`, and `awf upgrade --check` all green, then run `awf upgrade --attest-current-state`. It
  records the clean HEAD, a post-normalization digest, and the ADR cutoff and gaps in an optional
  `bridgeAttestation` lock block, journals the normalization/marker/status/terminal-deletion writes at
  `.awf/current-state-upgrade.journal`, and commits the attested lock last; it runs no project tests
  or gate. The terminal projection prunes `docs/decisions/ACTIVE.md` and the domain ADR indexes without
  regenerating them, so the attested project is deliberately index-pruned and refuses every ordinary
  command until a later current-state release consumes the attestation. If a transaction is interrupted,
  `awf upgrade --recover` rolls it back or cleans it up; if the journal is unusable, restore the working
  tree from Git and reinstall the bridge release before retrying.
