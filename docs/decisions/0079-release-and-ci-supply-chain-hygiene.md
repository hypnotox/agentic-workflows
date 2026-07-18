---
status: Implemented
date: 2026-07-08
tags: [sha-pinning, release-pipeline]
related: [30, 40, 49, 65, 78]
domains: [tooling]
---
# ADR-0079: Release and CI supply-chain hygiene

## Context

The 2026-07-08 release/CI audit dive left one open finding cluster: the GitHub
workflows that build, test, and publish awf trust more than they verify.

- **Actions are tag-pinned, not SHA-pinned.** `.github/workflows/ci.yml` and
  `release.yml` reference `actions/checkout@v6`, `actions/setup-go@v6`,
  `goreleaser/goreleaser-action@v7`, and `codecov/codecov-action@v5` by major tag. A
  moved or hijacked tag executes attacker code inside workflows, including the release
  workflow, which holds `contents: write`. The GoReleaser *tool* additionally floats at
  `version: '~> v2'` in both workflows, and `docs/releasing.md` documents local preview
  via `go run github.com/goreleaser/goreleaser/v2@latest`, although ADR-0030 Decision 2
  claims a pinned version is used there, so the runbook has already drifted from its ADR.
- **Nothing counteracts pin rot.** There is no `.github/dependabot.yml`; SHA pins
  without an updater silently stale.
- **The release workflow never runs the test suite.** `release.yml` verifies the tag
  against `project.Version` and runs `cmd/releasecheck` (ADR-0078), then builds and
  publishes. `ci.yml` has no tag trigger. A tag pushed on a commit that never went
  through CI (an unpushed or side-branch commit) ships untested binaries. The gate
  needs nothing exotic: `ci.yml`'s gate job already runs `./x gate` and `./x check` on
  `ubuntu-latest` with only checkout + setup-go.
- **Fork and dependabot PRs cannot pass CI.** Both Codecov upload steps set
  `fail_ci_if_error: true` and use `secrets.CODECOV_TOKEN`; PRs from forks (and from
  dependabot, which draws from a separate secret store) run without repo secrets, so
  the upload step fails their CI. Coverage *enforcement* does not live there: the
  100% floor is `cmd/covercheck` inside `./x gate` (ADR-0012); Codecov is reporting
  only, and both its statuses are informational (ADR-0065 Decision 4).
- **The checksum is integrity, not authenticity.** The rendered bootstrap verifies its
  download's SHA-256 against `checksums.txt` (ADR-0040), but that file is published by
  the same release workflow from the same token. An attacker who can write a release
  can rewrite binary and checksums together. The checksum protects against corrupted
  and truncated downloads, never against a compromised publisher.

Prior art this decision leans on: the repo-local checker-cmd idiom (`cmd/covercheck`,
`cmd/deadcodecheck`, `cmd/releasecheck`: a coverage-ignored `main` wrapping a
unit-tested `run` seam), and the release-workflow wiring test
(`cmd/releasecheck/main_test.go`) that reads `.github/workflows/release.yml` by relative
path and asserts step ordering.

## Decision

1. **Every workflow action is SHA-pinned; the pin is machine-enforced.** All remote
   `uses:` references (`owner/repo@ref`) under `.github/workflows/` pin a full 40-hex
   commit SHA with a trailing `# vX.Y.Z` comment (the format dependabot maintains);
   repo-local `./` action references are exempt (they are repo code), and `docker://`
   references must pin a digest. The GoReleaser tool version is pinned to one exact
   version everywhere it appears: the `version:` input of all three
   `goreleaser-action` invocations (two in `ci.yml`'s release-config job, one in
   `release.yml`) and the two `go run
   github.com/goreleaser/goreleaser/v2@<version>` preview commands in
   `docs/releasing.md` (restoring ADR-0030 Decision 2 conformance). A new repo-local
   `cmd/pincheck` (checker-cmd idiom) parses the workflow files and fails on any
   unpinned remote `uses:` ref, any undigested `docker://` ref, and any
   `goreleaser-action` `version:` input that is not an exact semver version; `./x
   gate` runs it, so an unpinned action or re-floated tool version added later fails
   every commit. The gate-composition mentions in the rendered docs
   (`docs/workflow.md`, `docs/testing.md`, `docs/development.md`) update via their
   `.awf/` parts, and AGENTS.md's Invariants list gains bullets for the two new
   contracts via its `.awf/` part, in the implementing commit.

2. **Dependabot is the staleness counterpart to pinning.** `.github/dependabot.yml`
   configures weekly `github-actions` updates (which bump SHA pins and their version
   comments) and weekly `gomod` updates. Accepted and expected: a gomod bump of a
   pinned tool dependency (golangci-lint, deadcode) may legitimately red the gate on
   the bot's PR: that is signal about the new tool version, not a bug. Dependabot PRs
   run without repository secrets, which Decision 4 makes survivable.

3. **The release workflow refuses untested and off-main commits.** `release.yml` gains,
   between the ADR-0078 release checks and the GoReleaser step: (a) an ancestry check,
   `git fetch origin main` followed by `git merge-base --is-ancestor HEAD origin/main`
   (`HEAD`, not `$GITHUB_SHA`, so an annotated tag's object id cannot bite), refusing
   any tag whose commit is not on `origin/main`; and (b) `./x gate` and `./x check`, so
   the published commit passes the full gate in the same run that builds it. Accepted
   constraint: a hotfix tag on an unmerged branch is refused by design: land it on
   `main` first. The wiring test in `cmd/releasecheck/main_test.go` extends to assert
   the ancestry, gate, and check steps appear before the GoReleaser step.

4. **Codecov uploads are token-optional.** *(Partially amends ADR-0065 Decision 3,
   which reads "`fail_ci_if_error: true` is retained on both (an upload failure fails
   the gate job); each step needs the `CODECOV_TOKEN`.")* The gate job maps the secret
   into a job-level env var (`CODECOV_TOKEN: ${{ secrets.CODECOV_TOKEN }}`) and both
   upload steps carry `if: env.CODECOV_TOKEN != ''`: token present → upload runs with
   `fail_ci_if_error: true` exactly as ADR-0065 prescribes; token absent (fork and
   dependabot PRs) → the steps skip and CI passes on `./x gate` alone, where coverage
   is actually enforced. ADR-0065's invariants (flag-to-profile mapping, informational
   statuses) are untouched. Implementation-time check: no Codecov status may be a
   required branch-protection check, or skipped uploads would wedge fork PRs.

5. **The tamper posture is documented acceptance.** The SHA-256 verification in the
   bootstrap and in `checksums.txt` is documented (in `docs/releasing.md`) as an
   integrity check only: it does not authenticate the publisher, and a compromise of
   the release workflow or its token can rewrite binary and checksums together. The
   accepted mitigations are exactly Decisions 1-3 (pinned actions shrink the code that
   runs with `contents: write`; the gate-on-tag step means that code is at least the
   tested tree). Running the test suite and its module-proxy tool downloads inside the
   write-privileged release workflow is part of this accepted surface. Artifact
   attestation and cosign signing are deliberately deferred, to be revisited at 1.0 or
   on adopter demand, not silently omitted.

## Invariants

- `invariant: workflow-actions-sha-pinned`: `cmd/pincheck`, run by `./x gate`, exits non-zero
  when any remote `uses:` reference under `.github/workflows/` is not pinned to a full
  40-hex commit SHA (repo-local `./` refs exempt, `docker://` refs digest-pinned), or
  when a `goreleaser-action` `version:` input is not an exact semver version.
- `invariant: release-gate-on-tag`: a gate test asserts `release.yml` runs the
  ancestry check, `./x gate`, and `./x check` before the GoReleaser step.
- `.github/dependabot.yml` covers the `github-actions` and `gomod` ecosystems.
- `.github/workflows/ci.yml` maps `secrets.CODECOV_TOKEN` into a job-level env var and
  both Codecov upload steps carry `if: env.CODECOV_TOKEN != ''`, so a missing token
  skips them rather than failing CI.
- `docs/releasing.md` states that checksum verification is integrity-only and names the
  residual publisher-compromise risk.

## Consequences

- The release path gains real preconditions: a tag on an untested or off-main commit
  fails before any artifact builds. Release runs get ~2-3 minutes slower (the gate).
  The undo-a-bad-tag runbook flow is unaffected: the runbook pushes `main` before
  tagging, so conforming tags always pass the ancestry check.
- SHA-pinned `uses:` lines are unreadable without their version comments and would rot
  without dependabot: Decisions 1 and 2 only make sense together, and the pin check
  makes the policy self-enforcing rather than reviewer-enforced.
- Weekly dependabot PRs add merge traffic. The 100% gate and CI make them cheap to
  judge; a red gate on a tool bump is triage signal, not noise.
- External contributors (and dependabot) get green CI without secrets, at the cost of
  no Codecov report on those PRs: acceptable because both Codecov statuses are
  informational and enforcement is local to the gate.
- A new repo-local binary (`cmd/pincheck`) joins the 100%-coverage and dead-code gates;
  marginal cost is one tested `run` function, per the established idiom.
- The two GoReleaser preview commands in `docs/releasing.md` remain convention-pinned:
  `cmd/pincheck` parses only workflow files. Accepted: drift there affects only local
  preview, and the workflow-side tool pin is machine-enforced.
- Adopters get no stronger authenticity guarantee than before; the ADR converts an
  undocumented gap into a documented, deliberately-accepted one with a named
  re-evaluation trigger (1.0 / adopter demand).
- Three GitHub-side behaviors are verified live at implementation rather than in tests
  (step-level `if:` on a secret-mapped env var; `goreleaser-action` accepting an exact
  `version:`; dependabot maintaining SHA pins + comments), all standard, none
  repo-testable.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| GitHub artifact attestation (`attest-build-provenance` + `gh attestation verify`) | No key management and ecosystem-standard, but unverifiable from the POSIX bootstrap script, so it would be an optional side-channel only; deferred to 1.0/demand rather than adopted half-used. |
| Cosign-sign `checksums.txt` via GoReleaser | Permanent key/identity maintenance for a pre-1.0 single-maintainer tool; still unverifiable from the plain bootstrap. |
| Require a green CI status via API instead of re-running the gate in `release.yml` | Cross-workflow coupling and race-prone status lookup; re-running the gate is self-contained and costs minutes. |
| Tokenless Codecov uploads for fork PRs | Rate-limited and flaky for tokenless traffic; skipping is deterministic and loses only an informational report. |
| SHA-pin by convention, no `cmd/pincheck` | A future action addition silently regresses the policy; the repo's standing practice is promoting always-true rules to deterministic checks. |
| Dependabot for `github-actions` only | Leaves Go module and tool updates manual; gomod PRs are cheap to judge under the 100% gate. |
