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

Add `TestGateRunnerModes`, a focused Go fixture that copies the root `x` into an isolated repository-shaped temporary directory and supplies fake `go` and Pi-lane commands that append their invocations to a log. Before Task 1.2 changes production behavior, run `go test ./internal/project -run '^TestGateRunnerModes$'` and require a nonzero result caused by the missing `timings` contract, retired-alias rejection, or singular explicit Pi-smoke stage. After the implementation, the same command must exit zero without invoking Docker or real analysis tools and prove:

- `./x gate` and `./x gate timings` execute identical stage commands in identical order, including one profiled Go suite, coverage check, one explicitly enabled uncached `TestPiRealRuntimeSmoke` invocation, vet, every non-host released-platform build, lint, deadcode with pipeline failure propagation, and pincheck;
- the timing form emits one stable `gate timing: <label> <whole-seconds>s` line per attempted stage, while ordinary gate output emits none;
- a configured fake-command failure returns that exact status, emits the failing stage timing in timing mode, and leaves every later stage absent from the invocation log;
- `./x gate full`, unknown gate arguments, and extra gate arguments fail with the gate usage text and invoke no stage;
- `./x test` prints exactly `test: Pi container skipped; run './x pi-test run' alone or './x gate' to include it` to stderr before forwarding its arguments to `go test ./...`.

Keep `TestPiExtensionContainerGateWiring` as the static ownership proof. Strengthen it to assert the explicit gate command selects only `TestPiRealRuntimeSmoke`, disables Go test caching with `-count=1`, supplies the opt-in environment value, and does not directly add a second `container.sh run` inside the gate arm. Extend the existing Sundial runner assertions to prove its `gate` arm rejects `full`, unknown values, and extra arguments rather than silently accepting them; preserve the `test` arm's existing forwarding of arguments to `go test ./...`.

### Task 1.2: Make the Pi runtime proof explicit, singular, and uncached
Latitude: exact
Paths: ["x", "internal/project/target_test.go", "internal/project/example_wiring_test.go", "internal/project/gate_runner_test.go"]

In `TestPiRealRuntimeSmoke`, retain all existing invariant markers and the real `./x pi-test run` subprocess. Before that subprocess, require `AWF_PI_RUNTIME_SMOKE=1`; otherwise call `t.Skip` with the same actionable guidance as the runner notice: `Pi container skipped; run './x pi-test run' alone or './x gate' to include it`. Do not move runtime-claim proof markers to rendered-source string tests.

Refactor the root `gate` arm so it clears ambient `AWF_PI_RUNTIME_SMOKE` and its ordinary profiled `go test ./...` leaves the smoke skipped. Replace the direct `tools/pi-extension-test/container.sh run` stage with `run_gate_step pi-runtime-smoke run_pi_runtime_smoke`. That helper must execute `env AWF_PI_RUNTIME_SMOKE=1 go test -json ./internal/project -run '^TestPiRealRuntimeSmoke$' -count=1`, preserve a nonzero test status, and refuse a zero-status result unless the structured output contains the passing `TestPiRealRuntimeSmoke` event. This named stage is the gate's sole Pi runtime execution. Preserve the durable `coverage.out`, the coverage checker, vet, host-derived released-platform build matrix, lint, deadcode pipeline, pincheck, and their current order. Keep `./x pi-test run|reset` as the direct standalone container interface.

Add the exact stderr notice from Task 1.1 to the `test` arm before its forwarded Go command. The gate arm must not print the skip notice because it runs the enabled smoke later. The runner notice is the guaranteed non-verbose surface; the `t.Skip` reason exposes the same guidance under direct `go test -v`, while direct non-verbose `go test` remains silent because the Go driver suppresses successful skipped-test output.

### Task 1.3: Add opt-in sequential stage timing and retire the repository full alias
Latitude: exact
Paths: ["x", "internal/project/gate_runner_test.go"]

Accept exactly zero gate arguments or the single argument `timings`. Reject `full`, unknown values, and extra arguments with `usage: ./x gate [timings]` and status 2. Remove the legacy `full` comment and usage spelling.

Introduce one shell-owned `run_gate_step <label> <command...>` mechanism used by both ordinary and timed modes. Keep commands synchronous. When timings are enabled, sample Bash `SECONDS` immediately around the command, print `gate timing: <label> <elapsed>s` to stderr after success or failure, and return the command's exact status. When timings are disabled, add no timing output. Structure the call sites so `set -e` stops after a failed wrapper return and the deadcode producer/consumer retains `pipefail`. Use these exact ordered labels: `go-test`, `covercheck`, `pi-runtime-smoke`, `vet`, then the applicable non-host members of `build-linux-amd64`, `build-linux-arm64`, `build-darwin-amd64`, `build-darwin-arm64`, `build-windows-amd64`, and `build-windows-arm64` in released-matrix order, followed by `lint`, `deadcode`, and `pincheck`. Add no concurrency, timing cache, new host dependency, or persistent timing file.

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
Paths: [".awf/awf.lock", ".awf/docs/parts/development/command-runner.md", ".awf/docs/parts/testing/gate.md", ".awf/docs/parts/testing/layout.md", ".awf/docs/parts/testing/tiers.md", ".awf/parts/workflow/composing-the-gate.md", "changelog/CHANGELOG.md", "examples/sundial/.awf/awf.lock", "examples/sundial/.awf/config.yaml", "examples/sundial/.awf/docs/parts/development/command-runner.md", "examples/sundial/x", "docs/development.md", "docs/testing.md", "docs/workflow.md", "examples/sundial/.awf/hooks/pre-push.sh", "examples/sundial/.claude/skills/sundial-bugfix/SKILL.md", "examples/sundial/.claude/skills/sundial-debugging/SKILL.md", "examples/sundial/.pi/skills/sundial-bugfix/SKILL.md", "examples/sundial/.pi/skills/sundial-debugging/SKILL.md", "examples/sundial/docs/config-reference.md", "examples/sundial/docs/development.md", "examples/sundial/docs/workflow.md"]
Representative: In the root testing/development/workflow authoring parts, replace the no-op `./x gate full` compatibility prose with the single sequential gate plus opt-in `./x gate timings`; state that `./x test` omits Docker with an actionable notice and that the gate explicitly runs the uncached Pi smoke once.
Edge: In Sundial, remove its configured `gateCmdFull`, remove the no-op gate-argument acceptance and authored command-runner promise while preserving test-argument forwarding, and let publication-safe generic templates render coherent unset-extended-tier prose; do not remove or narrow the standard catalog's generic `gateCmdFull` capability, its generic tests, or historical ADR/plan/research text.
Post-check: Run `./x render && ./x check` and require zero exit with clean root and Sundial state and no example notes. Then require `! git grep -n -E '(\./x gate full|gate full[^.]*identical|full[^.]*no-op legacy|no-op legacy[^.]*full)' -- x .awf/docs/parts/development/command-runner.md .awf/docs/parts/testing/gate.md .awf/docs/parts/testing/layout.md .awf/docs/parts/testing/tiers.md .awf/parts/workflow/composing-the-gate.md docs/development.md docs/testing.md docs/workflow.md examples/sundial/x examples/sundial/.awf/config.yaml examples/sundial/.awf/docs/parts/development/command-runner.md examples/sundial/.awf/hooks/pre-push.sh examples/sundial/.claude/skills/sundial-bugfix/SKILL.md examples/sundial/.claude/skills/sundial-debugging/SKILL.md examples/sundial/.pi/skills/sundial-bugfix/SKILL.md examples/sundial/.pi/skills/sundial-debugging/SKILL.md examples/sundial/docs/config-reference.md examples/sundial/docs/development.md examples/sundial/docs/workflow.md` to exit zero because the confined search has no matches. Require `git grep -q 'Key: "gateCmdFull"' -- internal/catalog/standard.go` and `git grep -q 'gateCmdFull' -- internal/project/hooks_test.go internal/project/spine_test.go` to exit zero, proving the generic declaration and its generic tests remain.

Edit only authored `.awf/` sources, the hand-written runners/config, and `changelog/CHANGELOG.md`; obtain every managed output by running `./x render`. Add an Unreleased changelog entry describing the observable repo-development change: timed gate diagnostics, Docker-free ordinary tests with explicit guidance, one uncached Pi gate lane, and removal of the awf/Sundial no-op alias. Do not rewrite terminal ADRs, completed plans, research records, generic template behavior, or generic `gateCmdFull` fixtures.

### Task 1.7: Verify semantic equivalence and record the measured improvement
Latitude: exact
Paths: ["coverage.out", "docs/plans/2026-08-02-reduce-gate-runtime-without-weakening-verification.md"]

Run focused regression commands first:

- `go test -v ./internal/project -run 'Test(GateRunner|PiExtensionContainerGateWiring|PiRealRuntimeSmoke|ProjectDerivedStateOwnership|ContextSpillObservabilityContract)'` exits zero without Docker, reports the smoke skipped with actionable guidance, and exercises every non-Docker contract;
- `go test ./internal/adr` exits zero, confirming the intentionally unchanged cached structural analysis;
- `./x test ./internal/project` prints the exact Pi omission guidance and exits zero without Docker;
- `./x render && ./x check` exits zero with clean root and Sundial state.

Then run `./x gate timings` once. It must exit zero, write the valid durable `coverage.out`, execute the Pi container once through the enabled runtime smoke, and emit every stable stage label. Compare its stage output with the brainstorm baseline recorded in the effort memory (profiled Go suite about 121.5s, explicit Pi lane about 9.5s, total measured components about 151s); treat those figures as indicative only and record the new observed timings under this plan's Notes. Do not add a machine-dependent duration assertion or fail the change merely because host load causes variance. Use the invocation log regression, invariant checks, coverage floor, and clean gate as the deterministic equivalence criteria. Record the observed timings and any deviations, but keep this plan `status: Proposed` through implementation and terminal review; the post-review deferred flip transaction changes it to `status: Implemented` and freezes it.

### Phase close

Stage the complete transaction explicitly with `git add x internal/project/gate_runner_test.go internal/project/example_wiring_test.go internal/project/target_test.go internal/project/stateownership_test.go internal/project/context_wrapper_test.go .awf/awf.lock .awf/docs/parts/development/command-runner.md .awf/docs/parts/testing/gate.md .awf/docs/parts/testing/layout.md .awf/docs/parts/testing/tiers.md .awf/parts/workflow/composing-the-gate.md changelog/CHANGELOG.md docs/development.md docs/testing.md docs/workflow.md examples/sundial/.awf/awf.lock examples/sundial/.awf/config.yaml examples/sundial/.awf/docs/parts/development/command-runner.md examples/sundial/.awf/hooks/pre-push.sh examples/sundial/x examples/sundial/.claude/skills/sundial-bugfix/SKILL.md examples/sundial/.claude/skills/sundial-debugging/SKILL.md examples/sundial/.pi/skills/sundial-bugfix/SKILL.md examples/sundial/.pi/skills/sundial-debugging/SKILL.md examples/sundial/docs/config-reference.md examples/sundial/docs/development.md examples/sundial/docs/workflow.md docs/plans/2026-08-02-reduce-gate-runtime-without-weakening-verification.md`. Do not stage ignored `coverage.out`. Require `git diff --cached --name-only` to contain the complete expected transaction and `git diff --name-only` to print nothing. Confirm the wired hook posture with `git config core.hooksPath`; if hooks are not wired, run `awf check staged` and `./x gate` manually. Create the one closing commit only after the staged check and full sequential gate pass.

```commit
refactor(tooling): deduplicate and instrument the gate
```

## Definition of done

- `./x gate` preserves every required sequential verification stage and executes the Docker-backed `TestPiRealRuntimeSmoke` exactly once through an explicit `-count=1` stage.
- `./x gate timings` runs the identical transaction and reports each attempted stage without changing output, order, or failure status.
- `./x test` is Docker-free and prints exact guidance for running the Pi lane alone or through the gate; verbose direct Go testing exposes the same skipped-test guidance, while direct non-verbose Go testing remains silent by Go-driver design.
- Root and Sundial gate arms reject the retired no-op `full` argument, Sundial's test arm still forwards Go-test arguments, and no active awf/Sundial project-specific surface advertises that alias; generic extended-tier support and historical records remain intact.
- State-ownership mutation analysis uses one combined overlay load, and context-wrapper tests build one shared immutable helper without losing any negative case or fixture isolation.
- Focused tests, render/check, staged authority, 100% coverage, TypeScript strict/full coverage, static analysis, cross-builds, deadcode, pincheck, and the final timed gate are green.

## Notes

The final `./x gate timings` run passed with these whole-second stage measurements: `go-test` 72s, `covercheck` 1s, `pi-runtime-smoke` 14s, `vet` 1s, `build-linux-arm64` 1s, `build-darwin-amd64` 1s, `build-darwin-arm64` 1s, `build-windows-amd64` 1s, `build-windows-arm64` 2s, `lint` 8s, `deadcode` 3s, and `pincheck` 0s. The measured stage sum was about 105 seconds, compared with the indicative pre-change warm component sum of about 151 seconds. Host load and cache state remain variable, so no hard wall-time target is contractual.

Implementation followed the approved design. The initial timed gate exposed errorlint and unparam findings in the new runner tests; those test assertions were corrected before the recorded passing run.

The plan review had instructed the Phase close to freeze the plan, but current `docs/plans/README.md` requires plans to remain Proposed through implementation and terminal review. Repository authority prevailed: the implementation transaction keeps this plan Proposed, and the post-review deferred flip freezes it.

Terminal review found that an ambient `AWF_PI_RUNTIME_SMOKE=1` could reach ordinary tests and that a skipped or unmatched targeted test could still return zero. The user approved clearing the ambient value at ordinary boundaries. The fix clears it for the gate transaction and `./x test`, enables it only inside the targeted helper, requires a structured pass event, strengthens exact cross-target and timing assertions, and exercises failures from both sides of the deadcode pipeline.
