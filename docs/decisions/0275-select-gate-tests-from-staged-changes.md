---
format: current-state-v4
slug: select-gate-tests-from-staged-changes
status: Implemented
date: 2026-08-13
---
# ADR-0275: Select Gate Tests From Staged Changes


## Context

The repository gate always runs its profiled Go suite, coverage check, and Docker-backed Pi
runtime tests. That gives every commit the same assurance, but it also spends test runtime on
staged changes that cannot affect the tested behavior. Documentation-only transactions do not
need any tests, while changes unrelated to the Pi extension do not need its comparatively
expensive runtime lane.

The gate is also run from clean checkouts in CI and pre-push contexts. A missing staged
transaction therefore cannot safely mean that tests are unnecessary. Selection must fail
closed while leaving vet, cross-platform builds, lint, dead-code analysis, and the workflow-pin
check unconditional. Selection is based on staged paths, although the selected commands retain
the existing behavior of testing working-tree content.

## Decision

1. `decision: staged-test-selection` The gate selects its test-related stages from one staged-index snapshot that safely represents additions, deletions, renames, and arbitrary valid filenames. An absent, empty, unreadable, or unclassifiable staged set selects all tests.
2. `decision: docs-only-test-skip` A nonempty staged set containing only `docs/**`, `README.md`, `changelog/CHANGELOG.md`, `.awf/docs/parts/**`, or `templates/docs/**` skips the Go suite, coverage check, and Pi runtime tests.
3. `decision: pi-change-test-selection` A staged set that is not documentation-only runs the Go suite and coverage check. It runs the Pi runtime tests only when a staged path can affect Pi templates or generated extensions, agent runtime inputs, the Pi test harness or its dependencies, rendering or target code, relevant configuration, or `x`. The classification is conservative: uncertainty runs the Pi tests.
4. `decision: non-test-gates-unconditional` Test selection is only a runtime optimization. Vet, released-platform builds, lint, dead-code analysis, and the workflow-pin check continue to run for every gate invocation.
5. `decision: selection-observability` The gate prints explicit notices for skipped test stages, and `./x gate timings` reports timings only for stages that execute.

## State changes

- add `tooling/quality-gates:staged-test-selection`
- update `tooling/quality-gates:pi-extension-container-gate`

## Consequences

Documentation-only commits avoid all test runtime, and ordinary non-Pi changes avoid the Pi
runtime lane. Changes that can affect tested behavior retain the relevant coverage, while
unknown and clean-checkout invocations preserve the complete gate used by CI and pre-push.

The path classification becomes quality-gate policy that must evolve with new documentation
or Pi dependency surfaces. Conservative matching can produce false positives, but avoids the
more serious false negative of skipping relevant tests. Explicit skip output makes the selected
transaction observable, and timing output covers only stages that execute.

Because selection reads the index while commands read the working tree, it does not attest to
an index snapshot. This preserves existing gate behavior and limits the decision to runtime
selection.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Always run every test | Preserves the existing runtime cost even when staged paths cannot affect tests. |
| Skip the entire gate for documentation | Drops inexpensive non-test checks and broadens a runtime optimization into an assurance-policy exception. |
| Select from the branch diff or working tree | Does not match the staged transaction that the local gate is intended to assess. |
| Run Pi tests for every non-documentation change | Misses the available runtime saving for changes unrelated to the Pi extension. |

## Status history

- 2026-08-13: Proposed
- 2026-08-13: Accepted; content-sha256: b666a61ff878cb46b3677e8504e5038e2c6623ba561f1c3f2df0a999c2e2c5b7
- 2026-08-13: Implemented; content-sha256: b666a61ff878cb46b3677e8504e5038e2c6623ba561f1c3f2df0a999c2e2c5b7
