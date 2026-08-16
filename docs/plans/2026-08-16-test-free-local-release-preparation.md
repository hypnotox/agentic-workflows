---
format: plan-v2
date: 2026-08-16
adrs: [test-free-local-release-preparation]
status: Proposed
---
# Plan: Test-free local release preparation

## Goal

Make the canonical local release-prep transaction skip Go and Pi test suites while preserving a
single validated binary version, fail-safe staged selection, all static gates, and the complete
tag-triggered release gate. Do not change release publication, artifact, changelog-note, schema, or
lock formats.

## Architecture summary

`internal/project/VERSION` becomes the embedded data authority behind the existing
`project.Version` string surface. `internal/project` owns canonical version and current-schema-floor
validation; a small `cmd/versioncheck` adapter exposes that validation to the command runner without
acquiring release-only changelog or license policy. The staged selector recognizes only the exact
version and root lock paths as neither-suite inputs, before its existing broad project and `.awf`
categories; uncertainty and neighboring paths continue to fail safe. One coherent implementation
transaction updates the two declared current-state claims and release guidance, renders generated
outputs, and applies both ADR operations. Tag workflow behavior remains unchanged.

## Phase 1: Land the validated test-free release transaction

**Execution mode: inline.**

Completes: ["embedded-version", "local-selection", "tag-assurance"]

### Task 1.1: Embed and unconditionally validate the release version
Kind: batch
Applying: ["test-free-local-release-preparation:embedded-version-authority", "test-free-local-release-preparation:exact-release-input-selection", "test-free-local-release-preparation:unconditional-version-validation", "test-free-local-release-preparation:full-tag-assurance-retained"]
Paths: ["internal/project/VERSION", "internal/project/project.go", "internal/project/version_test.go", "internal/project/drift_test.go", "internal/project/bootstrap_test.go", "cmd/awf/changelog_test.go", "cmd/awf/version_test.go", "cmd/versioncheck/main.go", "cmd/versioncheck/main_test.go", "x", "internal/project/gate_runner_test.go", ".github/workflows/release.yml", "cmd/releasecheck/main.go", "cmd/releasecheck/main_test.go"]
Representative: "A staged `internal/project/VERSION`, `.awf/awf.lock`, and `changelog/CHANGELOG.md` transaction selects neither suite, still invokes `cmd/versioncheck` and every existing static gate, and preserves `project.Version == 0.39.0`."
Edge: "A neighboring `internal/project` or `.awf` path, a mixed change, an unknown path, and an empty, unreadable, or malformed staged snapshot retain their existing applicable or both-suite behavior; invalid canonical version bytes and a version below the current schema floor fail versioncheck."
Post-check: "`go test ./internal/project ./cmd/versioncheck ./cmd/releasecheck` exits zero. The focused gate-runner test reaches terminal success for the release tuple and every neighbor, mixed, unknown, malformed, and empty case; its invocation log contains `run ./cmd/versioncheck` for neither-suite cases. Release-workflow contract tests still prove `./x gate` and `./x check` precede publication."

Before mutation, establish the application baseline: `git status --short` reports only this settled
plan draft before its initial commit and prints no paths when execution begins, `./x check` exits
zero, `go test ./internal/project ./cmd/releasecheck` exits zero, and the focused gate-runner and
release-workflow contract tests exit zero. Record any pre-existing failure rather than changing
implementation scope to absorb it.

Move the current `0.39.0` value, without a `v` prefix, into a newline-terminated
`internal/project/VERSION` embedded by `internal/project`. Retain `project.Version` as the sole
exported string value consumed throughout the repository and keep build provenance display-only.
Add one project-owned validator that rejects noncanonical file bytes, divergence between embedded
and exposed values, invalid non-`v` SemVer, a missing current schema mapping, and an exposed version
below that mapping. Replace the current-schema equality assertion with ADR-0049's existing
at-or-above contract while retaining historical minimum assertions.

Add a focused `cmd/versioncheck` presentation adapter and unit tests; its one-sentence package
comment states that the command exposes project-owned canonical version and schema-floor validation
as an unconditional gate stage. It prints one success line or one actionable stderr diagnostic and
nonzero status. Wire it as an unconditional named gate stage.
In staged selection, place exact `internal/project/VERSION` and exact `.awf/awf.lock` exceptions
before the existing `internal/project/*` and `.awf/*` overlap category. Extend the gate-runner fixture
and timing/failure assertions so versioncheck always runs, short-circuits on failure, and the exact
release tuple skips only the two suites. Do not alter `.github/workflows/release.yml`; retain its empty
staged snapshot fallback to both suites and its pre-publication ordering tests.

### Task 1.2: Apply current authority and publish the simpler runbook
Kind: batch
Latitude: exact
Applying: ["test-free-local-release-preparation:embedded-version-authority", "test-free-local-release-preparation:exact-release-input-selection", "test-free-local-release-preparation:unconditional-version-validation", "test-free-local-release-preparation:full-tag-assurance-retained"]
Paths: [".awf/topics/parts/tooling/cli/current-state.md", ".awf/topics/parts/tooling/quality-gates/current-state.md", ".awf/docs/parts/releasing/content.md", ".awf/docs/parts/architecture/components.md", "changelog/CHANGELOG.md", "docs/decisions/test-free-local-release-preparation.md", "docs/releasing.md", "docs/architecture.md", "glob:docs/topics/**", "glob:docs/domains/**", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Representative: "The release runbook edits `internal/project/VERSION`, promotes the changelog, renders the lock, runs releasecheck, and explains that local suites skip while tag CI remains complete."
Edge: "The CLI claim no longer promises compile-time const semantics; the quality-gate claim names both exact neither-suite inputs, unconditional version validation, unchanged static stages, and uncertainty fallback. Existing Origins and prior Revised-by history remain intact before appending this ADR."
Post-check: "After `./x render`, `./x check` exits zero and `git diff --check` emits no output. `./awf context --show pending docs/decisions/test-free-local-release-preparation.md` reports both declared operations Applied with none Remaining. Generated topic and release prose match their authored sources, and the Unreleased changelog describes the local release-prep improvement without claiming reduced tag assurance."

Update `tooling/cli:single-version-authority` and
`tooling/quality-gates:staged-test-selection` in their authored topic parts, preserving their backing
markers and appending this ADR to `Revised-by`. Before rendering, use the ADR lifecycle in the same
atomic authored transaction to move the reviewed pending ADR to Implementing and append one Applied
event containing both declared updates. Then update the authored release runbook and changelog and
run `./x render`, never editing outputs directly. No claim remains for a later implementation batch;
terminal status stays deferred until implementation assurance settles.

### Phase close

Stage the complete production, tests, command-runner, authored documentation, generated outputs,
lock, and ADR application transaction explicitly. Run the staged check and project gate; both must
exit zero, coverage must remain complete, and dead-code analysis must report no production dead code.
Create one commit:

```commit
feat(tooling): simplify release prep (applies ADR batch)
```

## Definition of done

- `dod: embedded-version` `project.Version` comes only from canonical `internal/project/VERSION`, and unconditional gate validation refuses malformed authority or an unmet current schema floor.
- `dod: local-selection` The exact canonical release-prep transaction skips Go and Pi suites locally while static gates, staged checks, and drift checks remain mandatory; neighboring and uncertain changes retain fail-safe selection.
- `dod: tag-assurance` The unchanged tag workflow runs the complete gate and drift check before publication, and the release runbook accurately distinguishes local prep from tag-time assurance.

## Notes

Inline owners immediately correct stale instructions and record reasoned deviations here. Delegated
owners may report rather than edit; the parent supplies the report to phase review and reconciles it
with findings in one focused post-review settlement commit before checkpointing or later execution.

After implementation assurance settles, `effort-workflow` owns the deferred lifecycle transaction:
reconcile any final plan Notes, append only the ADR's Implemented event, change this plan to
`status: Implemented`, render the decision index and lock, and commit those lifecycle-only changes
together before removing the managed worktree.

- 2026-08-16 implementation deviation: added `cmd/releasecheck/main.go` to Task 1.1 so its stale comment no longer claims release CI runs no tests. This is inside the approved full-tag-assurance boundary, changes no behavior, and is verified by the existing release-workflow contract tests.
- 2026-08-16 implementation deviation: added `.awf/docs/parts/architecture/components.md` and rendered `docs/architecture.md` to Task 1.2 because the architecture command census requires every new `cmd/*` package. This is a mechanically required docs-travel surface, verified by `TestArchitectureDocNamesEveryCmd`.
- 2026-08-16 phase-review settlement: independently proved each exact neither-suite path and mixed-path fail-safe behavior in `TestGateRunnerSelectsTestsFromStagedChanges`; added the `single-version-authority` backing marker to embedded validation and the existing lock, bootstrap, changelog, schema, and provenance consumer proofs. These mechanical additions close both review evidence gaps without changing production behavior.
