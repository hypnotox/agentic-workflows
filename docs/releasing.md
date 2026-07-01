# Releasing awf

How to cut a release of the `awf` binary. The distribution model and its rationale are
[ADR-0030](decisions/0030-prebuilt-binary-distribution-and-release.md); this is the runbook.

A release is a `v*` git tag. Pushing the tag triggers `.github/workflows/release.yml`, which runs
GoReleaser (`.goreleaser.yaml`) to build cross-platform binaries (linux/darwin/windows ×
amd64/arm64), package per-OS archives bundling `LICENSE` + `README.md`, write `checksums.txt`,
generate a Conventional-Commits changelog, and create the GitHub Release. Prebuilt binary download
is the canonical install path; `go install` is the source fallback.

## Versioning

awf is pre-1.0; versions are `vMAJOR.MINOR.PATCH` (SemVer). Two version surfaces exist and must
agree at release time:

- **The git tag** (e.g. `v0.1.0`) — GoReleaser derives the release version from it and stamps it
  into the binary via `-ldflags "-X main.version=…"`. This drives `awf version` on released builds.
- **`project.Version`** (`internal/project/project.go`) — the lock's `AWFVersion` and the dev/test
  fallback for `awf version`. It is **not** ldflags-driven, so renders stay reproducible. Bump it to
  match the tag in the release-prep commit.

## Cut a release

1. **Confirm `main` is green and clean.** On `main`, working tree clean:

   ```
   ./x gate && ./x check && ./x audit
   ```

   All three must pass (`audit` is advisory but should be clean for a release).

2. **Set `project.Version` to the target version and add its changelog entry.** Edit `const
   Version` in `internal/project/project.go` to the version you are about to tag (without the `v`
   prefix — e.g. `"0.2.0"`). Add a matching `## [0.2.0] - YYYY-MM-DD` entry to the top of
   `changelog/CHANGELOG.md` (newest first), grouped into Breaking changes/Features/Bug
   fixes/Others by adopter-facing effect (ADR-0041). If `project.Version` already matches (as it
   did for `v0.1.0`), skip the version-const edit, but the changelog entry is still required for
   every tag.

   ```
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

   The tag push starts the `Release` workflow. Watch it in the GitHub Actions tab; on success the
   release appears under Releases with the six archives, `checksums.txt`, and the changelog.

## Preview locally (no publish)

Validate the GoReleaser config and dry-run the full build without tagging or publishing:

```
go run github.com/goreleaser/goreleaser/v2@latest check
go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean
```

`--snapshot` writes artifacts to `dist/` (gitignored) and skips the GitHub Release. The same two
commands run on every pull request via the `release-config` job in `.github/workflows/ci.yml`, so a
broken release config fails CI before any tag is pushed.

## Notes

- **Repo visibility.** The repo is public, so release binaries download without authentication —
  from the Releases page or via `gh release download v0.2.0` — and
  `go install github.com/hypnotox/agentic-workflows/cmd/awf@latest` resolves without a token.
- **Undo a bad tag.** If a tag was pushed in error, delete it locally and remotely, then delete the
  GitHub Release it created:

  ```
  git tag -d v0.2.0
  git push origin :refs/tags/v0.2.0
  gh release delete v0.2.0
  ```

- **Hand-maintained files.** `.goreleaser.yaml` and the workflow files live outside awf's
  render/lock set (like `.golangci.yml` and `./x`), so `awf check` does not track them — edit them
  directly.
