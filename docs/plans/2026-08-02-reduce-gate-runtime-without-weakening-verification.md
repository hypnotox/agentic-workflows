---
format: plan-v1
date: 2026-08-02
adrs: []
status: Proposed
---
# Plan: Reduce gate runtime without weakening verification

## Goal

Reduce `./x gate` wall-clock time by removing duplicate Pi-container execution and repeated test setup, while preserving every verification claim, sequential failure behavior, and the generic awf extended-tier capability; `awf check` performance and gate-stage parallelism are non-goals.

## Architecture summary

The repository runner remains the sequential owner of gate orchestration. Ordinary Go tests skip the Docker-backed Pi runtime smoke, while one explicit uncached gate stage enables and runs that same Go proving unit exactly once. An opt-in timing argument observes the existing stage boundaries without changing their commands or order. Test-only analysis and helper setup stay parent-operation-owned and are shared only where immutable; isolated fixture roots remain isolated. Authored `.awf/` sources own documentation and Sundial configuration changes, and rendering produces their managed fan-out.

## Phase 1: Deduplicate and instrument the sequential gate

**Execution mode: inline.**

### Task 1.1: Add runner regression coverage before changing orchestration
Latitude: exact
Paths: ["internal/project/gate_runner_test.go", "internal/project/example_wiring_test.go"]

Add a focused Go fixture that copies the root `x` into an isolated repository-shaped temporary directory and supplies fake `go` and Pi-lane commands that append their invocations to a log. The fixture must first fail against the current runner and then prove the final contracts without invoking Docker or real analysis tools:

- `./x gate` and `./x gate timings` execute identical stage commands in identical order, including one profiled Go suite, coverage check, one explicitly enabled uncached `TestPiRealRuntimeSmoke` invocation, vet, every non-host released-platform build, lint, deadcode with pipeline failure propagation, and pincheck;
- the timing form emits one stable `gate timing: <label> <whole-seconds>s` line per attempted stage, while ordinary gate output emits none;
- a configured fake-command failure returns that exact status, emits the failing stage timing in timing mode, and leaves every later stage absent from the invocation log;
- `./x gate full`, unknown gate arguments, and extra gate arguments fail with the gate usage text and invoke no stage;
- `./x test` prints exactly `test: Pi container skipped; run './x pi-test run' alone or './x gate' to include it` to stderr before forwarding its arguments to `go test ./...`.

Keep `TestPiExtensionContainerGateWiring` as the static ownership proof. Strengthen it to assert the explicit gate command selects only `TestPiRealRuntimeSmoke`, disables Go test caching with `-count=1`, supplies the opt-in environment value, and does not directly add a second `container.sh run` inside the gate arm. Extend the existing Sundial runner assertions to prove its `gate` and `test` arms reject arguments rather than silently accepting the retired `full` spelling.

### Task 1.2: Make the Pi runtime proof explicit, singular, and uncached
Latitude: exact
Paths: ["x", "internal/project/target_test.go", "internal/project/example_wiring_test.go", "internal/project/gate_runner_test.go"]

In `TestPiRealRuntimeSmoke`, retain all existing invariant markers and the real `./x pi-test run` subprocess. Before that subprocess, require `AWF_PI_RUNTIME_SMOKE=1`; otherwise call `t.Skip` with the same actionable guidance as the runner notice: `Pi container skipped; run './x pi-test run' alone or './x gate' to include it`. Do not move runtime-claim proof markers to rendered-source string tests.

Refactor the root `gate` arm so its ordinary profiled `go test ./...` leaves the smoke skipped, then replace the direct `tools/pi-extension-test/container.sh run` stage with exactly:

`env AWF_PI_RUNTIME_SMOKE=1 go test ./internal/project -run '^TestPiRealRuntimeSmoke$' -count=1`

This named stage is the gate's sole Pi runtime execution. Preserve the durable `coverage.out`, the coverage checker, vet, host-derived released-platform build matrix, lint, deadcode pipeline, pincheck, and their current order. Keep `./x pi-test run|reset` as the direct standalone container interface.

Add the exact stderr notice from Task 1.1 to the `test` arm before its forwarded Go command. The gate arm must not print the skip notice because it runs the enabled smoke later.

### Task 1.3: Add opt-in sequential stage timing and retire the repository full alias
Latitude: exact
Paths: ["x", "internal/project/gate_runner_test.go"]

Accept exactly zero gate arguments or the single argument `timings`. Reject `full`, unknown values, and extra arguments with `usage: ./x gate [timings]` and status 2. Remove the legacy `full` comment and usage spelling.

Introduce one shell-owned `run_gate_step <label> <command...>` mechanism used by both ordinary and timed modes. Keep commands synchronous. When timings are enabled, sample Bash `SECONDS` immediately around the command, print `gate timing: <label> <elapsed>s` to stderr after success or failure, and return the command's exact status. When timings are disabled, add no timing output. Structure the call sites so `set -e` stops after a failed wrapper return and the deadcode producer/consumer retains `pipefail`. Give each cross-build target a distinct label. Add no concurrency, timing cache, new host dependency, or persistent timing file.

### Task 1.4: Consolidate state-ownership mutation package loads
Latitude: exact
Paths: ["internal/project/stateownership_test.go"]

Keep the normal production `packages.Load` and every existing production assertion unchanged in meaning. Replace the separate mutation overlay loads with one combined overlay map and one `loadProjectPackage` call:

- one synthetic `internal/project/state_ownership_mutation_fixture.go` contains distinct functions for receiver field assignment, locally constructed conforming assignment, address escape, nested `deriveOperationState`, direct `adr.LoadCorpus`, and whole-value replacement;
- one synthetic `internal/contextq/state_ownership_mutation_fixture.go` contains the widened `Query.state` mutation;
- the combined result must positively identify each forbidden function through the detector that owns that behavior and must explicitly prove the locally constructed function produces no finding.

Do not introduce a package-global cache, mutable seam, weakened prefix assertion, or change to the production detector. Do not modify `internal/adr/corpus_test.go`; grounding verified that its normal production graph is already cached with `sync.Once`.

### Task 1.5: Build the context-spill helper once while retaining fixture isolation
Latitude: exact
Paths: ["internal/project/context_wrapper_test.go"]

Have `TestContextSpillObservabilityContract` build `cmd/contextspilllog` once into a parent-owned temporary path, then pass that immutable executable path to each focused subtest and its `contextRunnerFixture`. Remove the helper build from per-fixture setup. Each subtest must still receive a distinct temporary repository root, copied runner, fake awf script, local log state, environment, and assertions; only the helper executable is shared. Add no global cache or swappable production dependency.

### Task 1.6: Remove active no-op full-tier surfaces and document the new gate boundary
Kind: batch
Latitude: exact
Paths: [".awf/docs/parts/development/command-runner.md", ".awf/docs/parts/testing/gate.md", ".awf/docs/parts/testing/layout.md", ".awf/docs/parts/testing/tiers.md", ".awf/parts/workflow/composing-the-gate.md", "changelog/CHANGELOG.md", "examples/sundial/.awf/config.yaml", "examples/sundial/.awf/docs/parts/development/command-runner.md", "examples/sundial/x", "docs/development.md", "docs/testing.md", "docs/workflow.md", "examples/sundial/.awf/hooks/pre-push.sh", "examples/sundial/.claude/skills/sundial-bugfix/SKILL.md", "examples/sundial/.claude/skills/sundial-debugging/SKILL.md", "examples/sundial/.pi/skills/sundial-bugfix/SKILL.md", "examples/sundial/.pi/skills/sundial-debugging/SKILL.md", "examples/sundial/docs/config-reference.md", "examples/sundial/docs/development.md", "examples/sundial/docs/workflow.md"]
Representative: In the root testing/development/workflow authoring parts, replace the no-op `./x gate full` compatibility prose with the single sequential gate plus opt-in `./x gate timings`; state that `./x test` omits Docker with an actionable notice and that the gate explicitly runs the uncached Pi smoke once.
Edge: In Sundial, remove its configured `gateCmdFull`, remove the no-op argument acceptance and authored command-runner promise, and let publication-safe generic templates render coherent unset-extended-tier prose; do not remove or narrow the standard catalog's generic `gateCmdFull` capability, its generic tests, or historical ADR/plan/research text.
Post-check: Run `./x render`, require `./x check` to report clean with zero example notes, and run targeted `git grep` checks over `x`, the five root authored parts, the three root rendered docs, `examples/sundial/x`, and the listed Sundial active authored/rendered surfaces; those active surfaces must contain no `./x gate full`, no `gate full` no-op promise, and no no-op/full legacy wording, while generic catalog support remains covered by its existing tests.

Edit only authored `.awf/` sources, the hand-written runners/config, and `changelog/CHANGELOG.md`; obtain every managed output by running `./x render`. Add an Unreleased changelog entry describing the observable repo-development change: timed gate diagnostics, Docker-free ordinary tests with explicit guidance, one uncached Pi gate lane, and removal of the awf/Sundial no-op alias. Do not rewrite terminal ADRs, completed plans, research records, generic template behavior, or generic `gateCmdFull` fixtures.

### Task 1.7: Verify semantic equivalence and record the measured improvement
Paths: ["coverage.out", "docs/plans/2026-08-02-reduce-gate-runtime-without-weakening-verification.md"]

Run focused regression commands first:

- `go test ./internal/project -run 'Test(GateRunner|PiExtensionContainerGateWiring|PiRealRuntimeSmoke|ProjectDerivedStateOwnership|ContextSpillObservabilityContract)'` exits zero without Docker, reports the smoke skipped under verbose execution, and exercises every non-Docker contract;
- `go test ./internal/adr` exits zero, confirming the intentionally unchanged cached structural analysis;
- `./x test ./internal/project` prints the exact Pi omission guidance and exits zero without Docker;
- `./x render && ./x check` exits zero with clean root and Sundial state.

Then run `./x gate timings` once. It must exit zero, write the valid durable `coverage.out`, execute the Pi container once through the enabled runtime smoke, and emit every stable stage label. Compare its stage output with the brainstorm baseline recorded in the effort memory (profiled Go suite about 121.5s, explicit Pi lane about 9.5s, total measured components about 151s); treat those figures as indicative only and record the new observed timings under this plan's Notes. Do not add a machine-dependent duration assertion or fail the change merely because host load causes variance. Use the invocation log regression, invariant checks, coverage floor, and clean gate as the deterministic equivalence criteria.

### Phase close

Stage the complete transaction explicitly. Confirm the wired hook posture with `git config core.hooksPath`; if hooks are not wired, run `awf check staged` and `./x gate` manually. Create the one closing commit only after the staged check and full sequential gate pass.

```commit
refactor(tooling): deduplicate and instrument the gate
```

## Definition of done

- `./x gate` preserves every required sequential verification stage and executes the Docker-backed `TestPiRealRuntimeSmoke` exactly once through an explicit `-count=1` stage.
- `./x gate timings` runs the identical transaction and reports each attempted stage without changing output, order, or failure status.
- `./x test` is Docker-free and prints exact guidance for running the Pi lane alone or through the gate; verbose direct Go testing exposes the same skipped-test guidance.
- Root and Sundial runners reject the retired no-op `full` argument, and no active awf/Sundial project-specific surface advertises that alias; generic extended-tier support and historical records remain intact.
- State-ownership mutation analysis uses one combined overlay load, and context-wrapper tests build one shared immutable helper without losing any negative case or fixture isolation.
- Focused tests, render/check, staged authority, 100% coverage, TypeScript strict/full coverage, static analysis, cross-builds, deadcode, pincheck, and the final timed gate are green.

## Notes

Record the final `./x gate timings` stage output and any implementation deviation here. The pre-change measurements are indicative: the warm gate components totaled about 151 seconds, including about 121.5 seconds for the profiled Go suite and 9.5 seconds for the explicit Pi lane; an earlier whole-gate run took about 302 seconds, so no hard wall-time target is contractual.
