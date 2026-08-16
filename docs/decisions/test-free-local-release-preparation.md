---
format: current-state-v4
slug: test-free-local-release-preparation
status: Implementing
date: 2026-08-16
---
# ADR-test-free-local-release-preparation: Test-free local release preparation


## Context

Release preparation currently changes `project.Version` in
`internal/project/project.go`, promotes the changelog, and renders the new version into the root
lock. ADR-0275 and ADR-0276 make the staged paths select local test suites, so the Go source and
broad `.awf/` classifications run both the profiled Go suite and Pi runtime suite before the release
commit. The tag workflow later runs the complete gate again before publication.

The Go edit is data expressed as code. It also interacts with a test whose equality requirement is
stricter than ADR-0049: that decision and the current `schema-min-version` claim require the binary
version to be at or above the current schema minimum, not equal to it. Consequently an ordinary
release without a schema change should not have to edit the schema minimum table.

The simplification must remain narrow. A release-prep commit still needs compilation and static
analysis, a canonical version and compatible schema floor, staged and rendered-output checks, and
complete tag-time assurance. Unknown paths and neighboring project or configuration inputs must keep
the fail-safe suite selection. The exact root lock exception necessarily applies to any lock-only
transaction, not only a release-shaped tuple; mandatory staged and drift checks remain its structural
and generated-authority guard.

## Decision

1. `decision: embedded-version-authority` Replace the hand-edited Go version constant with
   `internal/project/VERSION` as the one embedded plain-text version authority while retaining
   `project.Version` as the value consumed by
   the CLI, lock stamping, bootstrap pinning, changelog checks, and schema compatibility. No build
   metadata or other input becomes a competing version authority. This revises ADR-0049's constant
   representation without changing its single-authority semantics.
2. `decision: exact-release-input-selection` The staged test selector treats only exact
   `internal/project/VERSION` and exact root `.awf/awf.lock` as inputs that select neither test suite.
   Existing
   documentation-only behavior is unchanged. Neighboring, mixed, unknown, unreadable, malformed,
   and empty selections retain their existing explicit or fail-safe behavior. This refines ADR-0275
   and ADR-0276 without weakening their uncertainty policy.
3. `decision: unconditional-version-validation` Canonical version syntax and the current schema's
   minimum-version relationship are validated by an unconditional lightweight gate step, independent
   of selected test suites. Vet, release-target builds, lint, dead-code analysis, pin checks, staged
   checks, and drift checks remain mandatory under their existing owners.
4. `decision: full-tag-assurance-retained` The tag-triggered release workflow continues to run the
   complete project gate and drift check before publication. Its empty staged selection continues to
   fail safe to both test suites, so local release-prep selection does not reduce publication
   assurance.

## State changes

- update `tooling/cli:single-version-authority`
- update `tooling/quality-gates:staged-test-selection`

## Consequences

- The canonical local release transaction can change the embedded version, changelog, and rendered
  lock without rerunning Go or Pi test suites already covered by the release candidate's development
  history and the later tag gate.
- Version data is no longer a compile-time Go constant. Repository consumers retain the same
  `project.Version` string surface, but new constant-expression use is ruled out.
- A malformed version or version below the current schema floor still fails locally even though the
  suites are not selected.
- Root-lock-only changes skip suites as a general exact-path policy. The staged transition and drift
  oracles, rather than tuple recognition, guard that generated authority; any accompanying `.awf/`
  source change selects the applicable suites normally.
- A release without a schema generation change leaves the minimum-version table untouched. A schema
  change still requires a mapped floor that the binary version meets.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Put the constant in a dedicated allowlisted Go file | A test-free Go file could accumulate executable behavior, requiring brittle structural or diff parsing to keep the exception safe. |
| Recognize and bypass a release-shaped staged tuple | Tuple recognition would couple the gate to release ceremony and could hide arbitrary edits inside an otherwise matching Go file. |
| Keep the current gate behavior | It preserves assurance but repeats both suites locally solely because version data is stored in Go and the generated lock has a broad dependency classification. |

## Status history

- 2026-08-16: Proposed
- 2026-08-16: Accepted; content-sha256: 295dbd2886e0843f32f004681cf118330e1e3ff2e743f840c208495853f58f5b
- 2026-08-16: Implementing; content-sha256: 295dbd2886e0843f32f004681cf118330e1e3ff2e743f840c208495853f58f5b
- 2026-08-16: Applied; operations: update `tooling/cli:single-version-authority`, update `tooling/quality-gates:staged-test-selection`
