---
format: plan-v2
date: 2026-08-28
adrs: [performance-budgeted-parallel-verification]
status: Proposed
---
# Plan: Accelerate testing and verification

## Goal

Reduce the ordinary full gate by at least 4x, pursue the stronger local and hosted targets, and provide 10 to 15 second common affected-package feedback while preserving exact behavioral, coverage, platform, mutation, and release assurance. Broad mutation, weakened oracles, a speculative test taxonomy, and unrelated production cleanup are non-goals.

## Architecture summary

The fast commit gate remains unchanged. A separate affected-package feedback path selects changed Go packages, reverse dependents, and declared meta-suites with conservative widening. Terminal full assurance moves from one serial profiled Go invocation to deterministic contention-qualified shards whose coverage is merged by one canonical owner before the existing exact policy runs. Mutable `cmd/awf` process seams move into an unexported instance-owned runner that continues to derive command policy from `internal/clispec`; expensive package fixtures use immutable seeds and explicit mutation clones. CI fans native Go, Pi, analysis, platform, coverage, and release work into exact-revision dependencies of the stable required conclusions. GoReleaser snapshot construction and production archive validation occur once against the same CI artifact bytes.

A versioned qualification record owns workload identities, reference environments, cache preparation, sample method, baselines, budgets, and achieved evidence. The current coverage recipe remains authoritative until a prototype proves complete identity-level equivalence; deterministic whole-`coverpkg` shard merging is the fallback if no more compact candidate qualifies. Remove accidental work before relying on concurrency, preserve representative caller boundaries around owner-level matrices, and keep selected mutation outside the ordinary full-gate budget.

## Phase 1: Establish qualified performance evidence

**Execution mode: inline.**

Completes: ["qualified-performance-evidence"]

### Task 1.1: Add the qualification record and timing owner
Applying: ["performance-budgeted-parallel-verification:qualified-performance-contract"]
Paths: ["test-performance.json", "internal/testperformance", "cmd/testperformance", "x", "internal/project/gate_runner_test.go", ".awf/domains/tooling.yaml", "docs/domains/tooling.md", "docs/topics/tooling", ".awf/awf.lock"]
Post-check: "Run the testperformance unit and command suites against canonical and malformed records, then execute the fast and ordinary-full timing reporters on the declared stable toolchain. Require the emitted artifact to identify every declared workload and environment field, reject an environment mismatch instead of comparing unlike samples, preserve stage/package/test observations, and validate the tracked record without rewriting it."

Create one internal owner for the versioned qualification schema, canonical loading and rendering, environment identity, sample aggregation, and budget evaluation, with a thin command entry point and runner integration. Add `internal/testperformance/**` to `.awf/domains/tooling.yaml`, render its generated domain and topic outputs with the lock, and require domain coverage to classify the new package as tooling-owned. Track the approved local and hosted workload definitions, exact stable toolchain, CPU/OS/architecture/filesystem/memory and runner-image identity, cache preparation, warm and cold sample method, ordinary-versus-exceptional mutation classification, landed baseline, minimum threshold, and stronger targets. Emit both human timings and a machine-readable artifact from the same observations. Treat a single noisy wall-clock sample as evidence, never as a correctness failure; deterministic schema, environment, workload, and component-regression violations remain blocking.

### Task 1.2: Prototype equivalent compact coverage recipes
Kind: spike
Question: "Among deterministic whole-`coverpkg` text shards, dependency-scoped text shards, and Go binary covdata, which fastest recipe reproduces the authoritative profile's complete instrumented universe, exact covered and uncovered identities, all six selector projections, directive execution evidence, and filtered output on the declared reference environment? Record commands, canonical hashes, elapsed components, profile sizes, and the selected recipe in Notes; select whole-`coverpkg` text shards as the exact fallback if neither compact candidate proves every equivalence condition."
Applying: ["performance-budgeted-parallel-verification:equivalent-coverage-collection"]

### Task 1.3: Apply the qualification rule and documentation
Applying: ["performance-budgeted-parallel-verification:qualified-performance-contract"]
Kind: batch
Paths: ["docs/decisions/0314-performance-budgeted-parallel-verification.md", "docs/decisions/INDEX.md", ".awf/topics/parts/tooling/quality-gates/current-state.md", ".awf/docs/parts/testing", "docs/topics/tooling/quality-gates.md", "docs/testing.md", ".awf/awf.lock"]
Representative: ["tooling/quality-gates:verification-performance-contract"]
Edge: ["the performance claim is a rule verified by qualified evidence, not a flaky per-run timing invariant", "the targeted mutation workload remains separately reported", "the current authoritative coverage recipe remains the fallback"]
Post-check: "Use the ADR lifecycle command to enter Implementing and apply exactly the add operation for `tooling/quality-gates:verification-performance-contract` with its qualification procedure. Render from `.awf/`, require clean drift, and require pending context to report the other ADR operations as remaining while this operation is applied."

Transition ADR-0314 to Implementing with its first Applied batch only after the record, reporter, focused tests, and landed qualification evidence exist. Author the rule from the topic source and update testing documentation with reproducible measurement commands and workload distinctions.

### Phase close

```commit
feat(tooling): add verification evidence (applies 0314 batch)
```

## Phase 2: Remove accidental test work

**Execution mode: inline.**

Completes: ["redundant-test-work-removed"]

### Task 2.1: Separate context correctness from performance measurement
Applying: ["performance-budgeted-parallel-verification:oracle-preserving-consolidation"]
Paths: ["internal/contextop/context_benchmark_test.go"]

Retain a cheap correctness test proving complete and focused context routes return the required equivalent evidence and that the focused route captures the intended smaller dependency surface. Move repeated latency and allocation loops into ordinary `Benchmark...` functions excluded from correctness runs. Run the focused test red against a deliberately broken parity assertion, restore it green, and record benchmark output only as performance evidence.

### Task 2.2: Eliminate duplicate exported-test invocation
Applying: ["performance-budgeted-parallel-verification:oracle-preserving-consolidation"]
Kind: batch
Paths: ["internal/project/effort_workflow_template_test.go", "internal/testsupport"]
Representative: ["TestEffortWorkflowSkillContract directly invoking TestEffortWorkflowTemplate"]
Edge: ["an exported test remains Go-discoverable exactly once", "a genuine shared scenario becomes an unexported helper", "an invariant proof marker stays on a retained complete oracle"]
Post-check: "Run a repository-wide AST-based test over every Go test file and require no exported top-level Test function to call another exported top-level Test function. Require the project package tests green and verify every invariant proof marker still resolves to a discoverable retained test."

Replace the current duplicate call with one discoverable owner and an unexported helper only where shared setup is genuinely required. Put the non-recurrence scan with repository test-support policy rather than a shell grep.

### Task 2.3: Remove redundant evaluation rendering
Applying: ["performance-budgeted-parallel-verification:oracle-preserving-consolidation", "performance-budgeted-parallel-verification:immutable-fixture-seeds"]
Paths: ["internal/evals/fixture_test.go", "internal/evals"]
Post-check: "Run the evaluation package uncached with JSON timing output. Require each target's full-catalog seed to be constructed once per immutable input identity, each test to receive an isolated read-only view or explicit clone, the exhaustive target assertions to remain green, and the package timing artifact to contain no nested target rerender path."

Build one immutable full-catalog seed per target, remove the loop that renders both targets inside an already target-specific case, and make mutating consumers clone explicitly. Keep target coverage and rendered-output assertions unchanged.

### Task 2.4: Publish the first before-and-after evidence
Applying: ["performance-budgeted-parallel-verification:qualified-performance-contract"]
Paths: ["test-performance.json", "cmd/testperformance", "internal/contextop", "internal/evals", "internal/project"]
Post-check: "Capture stable-toolchain uncached package and top-test artifacts for every changed package plus one ordinary full sample. Require the qualification command to compare like environments, retain the landed baseline, publish achieved deltas without claiming the final budget, and report any widened or exceptional workload separately."

Update only observed evidence fields owned by the qualification record. Do not weaken a later budget or delete a slow oracle merely because this phase does not yet reach the final target.

### Phase close

```commit
test(tooling): remove redundant verification work
```

## Phase 3: Make dominant test ownership parallel-safe

**Execution mode: inline.**

Completes: ["parallel-safe-dominant-packages"]

### Task 3.1: Introduce the instance-owned command runner
Applying: ["performance-budgeted-parallel-verification:instance-owned-test-seams"]
Kind: batch
Paths: ["cmd/awf", "internal/clispec", "internal/testsupport/thin_command_composition_test.go", "internal/project/gatedcommands.go", "internal/project/gatedcommands_test.go", "internal/publisher/gatedcommands.go"]
Representative: ["cmd/awf/main.go", "cmd/awf/dispatch.go", "cmd/awf/main_test.go", "cmd/awf/dispatch_test.go", "cmd/awf/init_test.go", "cmd/awf/gate_test.go"]
Edge: ["getwd, stdin, interactivity, and handler composition are instance-owned", "the runner stays unexported and accepts explicit one-operation dependencies", "internal/clispec remains the only command membership and policy source", "help, parse, guard, gate, lease, stdout, stderr, and exit behavior stay at their current boundaries"]
Post-check: "Run the command, clispec, project gated-command, publisher, and thin-composition suites plus a repository AST scan. Require no mutable package-global command-process seam converted under `cmd/awf`, no new global test seam anywhere, no second command registry, exact clispec-to-handler coverage, and retained representative tests for help, parsing, working-directory failure, unknown commands, state guards, capability selection, binary gates, initialization interaction, nested groups, commit-message input, presentation, lease release, and mutation rollback."

Move process dependencies into one cohesive runner assembled by `main`, convert handler closures or methods only as required by that owner, and migrate seam-swapping tests to fresh runner instances. Keep command implementation packages and result presentation unchanged unless direct injection is necessary to remove a global swap.

### Task 3.2: Collapse scaffold-per-command policy matrices
Applying: ["performance-budgeted-parallel-verification:oracle-preserving-consolidation", "performance-budgeted-parallel-verification:instance-owned-test-seams"]
Paths: ["cmd/awf/gate_test.go", "cmd/awf/dispatch_test.go", "cmd/awf/main_test.go", "cmd/awf/test_helpers_test.go", "internal/clispec", "internal/project/gatedcommands_test.go", "internal/testsupport/thin_command_composition_test.go"]
Post-check: "Derive the complete command, capability, and gating inventories from clispec and require direct metadata/route tests to cover every member. Run representative scaffolded CLI tests for each distinct guard, capability, and dispatch behavior, then mutation-test changed policy predicates and require survivors to be either killed or exact reviewed equivalents. Require no test whose sole purpose is executing every switch arm or rebuilding a project for metadata parity."

Test shared policy once at its owner. Retain a representative end-to-end case for each distinct observable refusal, pre-mutation guard, profile capability, nested dispatch, and output/exit contract, while deleting command-by-profile cells that differ only in clispec data already checked directly.

### Task 3.3: Introduce immutable package fixture seeds
Applying: ["performance-budgeted-parallel-verification:immutable-fixture-seeds"]
Kind: batch
Paths: ["internal/testsupport", "internal/project/project_test_helpers_test.go", "internal/project/testmain_test.go", "internal/publisher/test_helpers_test.go", "internal/evals/fixture_test.go", "cmd/awf/test_helpers_test.go", "cmd/awf/testmain_test.go"]
Representative: ["project scaffold seed", "publisher catalog seed", "evaluation target seed", "CLI minimal project seed"]
Edge: ["testsupport remains a dependency leaf and imports no repository internal package", "package-specific builders own internal-package construction", "parallel tests never share a live mutable root", "mode, symlink, Git identity, hooks, and cleanup semantics survive cloning"]
Post-check: "Run dependency-boundary tests, seed canonicalization tests, and each consumer package under repeated parallel stress with isolated TMPDIR. Require immutable seed hashes to remain unchanged, every mutating consumer to receive a distinct clone, current-home cleanup to succeed, and race or filesystem-ownership findings to be zero."

Cache bytes, manifests, or read-only seed trees at the narrowest package owner. Measure clone strategies on the declared filesystem and use the least expensive one that preserves file modes, symlinks, Git topology, and ownership evidence. Do not introduce reflink-only correctness or production imports of test support.

### Task 3.4: Apply command and fixture ownership claims
Applying: ["performance-budgeted-parallel-verification:instance-owned-test-seams", "performance-budgeted-parallel-verification:immutable-fixture-seeds"]
Kind: batch
Paths: ["docs/decisions/0314-performance-budgeted-parallel-verification.md", ".awf/topics/parts/tooling/cli/current-state.md", ".awf/topics/parts/tooling/test-infrastructure/current-state.md", "docs/topics/tooling/cli.md", "docs/topics/tooling/test-infrastructure.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Representative: ["tooling/cli:cli-runner-instance-ownership", "tooling/test-infrastructure:immutable-fixture-seeds"]
Edge: ["both new claims carry Backing: test and valid proof annotations", "existing dependency-composition and clispec single-source claims remain unchanged"]
Post-check: "Apply one ADR lifecycle batch containing exactly the two add operations with their matching source claims and proof markers. Render and require clean drift, then require pending context to show both operations applied and every unimplemented quality-gate operation remaining."

### Phase close

```commit
refactor(code-design): make command tests instance-owned
```

## Phase 4: Parallelize equivalent full assurance

**Execution mode: inline.**

Completes: ["equivalent-parallel-full-gate"]

### Task 4.1: Add deterministic profile aggregation
Applying: ["performance-budgeted-parallel-verification:equivalent-coverage-collection"]
Paths: ["internal/coverage/coverage.go", "internal/coverage/policy.go", "internal/coverage/coverage_test.go", "internal/coverage/policy_test.go", "internal/coverage/policy_edge_test.go", "cmd/covercheck/main.go", "cmd/covercheck/main_test.go", "cmd/covercheck/policy_edge_test.go"]
Post-check: "Observe tests fail before merger implementation for mixed modes, malformed later headers, duplicate identities, conflicting statement counts, noncanonical paths, missing shard universes, and reordered inputs. After implementation, require deterministic set-mode output independent of input order, OR-merged execution counts, one canonical identity per block, and exact rejection diagnostics for every invalid case."

Give `internal/coverage` the single multi-profile parsing and canonical merge boundary, exposed through the existing thin covercheck command. Do not concatenate text profiles or let the runner reinterpret block identities.

### Task 4.2: Execute the selected equivalent shard recipe
Applying: ["performance-budgeted-parallel-verification:bounded-parallel-assurance", "performance-budgeted-parallel-verification:equivalent-coverage-collection"]
Paths: ["x", "internal/project/gate_runner_test.go", "internal/coverage", "cmd/covercheck", "test-performance.json"]
Post-check: "On the declared stable environment, run the authoritative serial recipe and the Phase 1 selected recipe from the same clean revision. Require identical instrumented-universe digest, exact raw covered and uncovered sets, all critical selector projections, directive inventories and executed-ignore results, filtered profile, and coverage-policy outcome. Repeat shard input ordering and bounded-concurrency stress, and record the ordinary full timing without treating this intermediate task as the phase's final budget qualification."

Replace the serial Go lane only after exact equivalence passes. Use deterministic timing-balanced shards and bounded process/GOMAXPROCS settings recorded by qualification, isolate mutable homes and temporary roots, merge coverage once, and run the current policy on the canonical result. Keep the mutation blocker's qualified recipe and budget separate; its preflight may consume the new full recipe only after its own trust contract is renewed.

### Task 4.3: Parallelize only proven-independent full stages
Applying: ["performance-budgeted-parallel-verification:bounded-parallel-assurance"]
Paths: ["x", "internal/project/gate_runner_test.go", "test-performance.json"]
Post-check: "Measure the declared stage dependency candidates both sequentially and concurrently with identical warm-cache preparation. Retain only combinations whose repeated wall time improves without changing output, exit, cleanup, or resource ceilings. Require a failed prerequisite to prevent its dependent policy step, all started stages to terminate, and the final diagnostic to identify every failed stage."

Run independent Pi, analysis, platform-build, and Go-shard work concurrently only where qualified evidence shows a net gain on the reference host. Keep version and schema authority before consumers, coverage policy after profile aggregation, and mutation after its exact selector. Executor log buffering, stable replay, cancellation, and cleanup are plan-owned mechanics rather than new policy claims.

### Task 4.4: Apply the equivalent coverage claim and report targets
Applying: ["performance-budgeted-parallel-verification:equivalent-coverage-collection", "performance-budgeted-parallel-verification:qualified-performance-contract"]
Kind: batch
Paths: ["docs/decisions/0314-performance-budgeted-parallel-verification.md", ".awf/topics/parts/tooling/quality-gates/current-state.md", ".awf/docs/parts/testing", "docs/topics/tooling/quality-gates.md", "docs/testing.md", "test-performance.json", "coverage-baseline.json", "coverage-review.json", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Representative: ["tooling/quality-gates:coverage-raw-identity-ratchet"]
Edge: ["the merged profile remains one whole-derived exact policy input", "no baseline miss or ignore reason changes merely because collection representation changed", "the 55-second stronger target is reported separately from the mandatory 4x threshold"]
Post-check: "Apply exactly the coverage-ratchet update with matching claim prose and retained test backing. Regenerate only through canonical owners, render and require clean drift, then require pending context to show the coverage operation applied. Run qualified warm and cold samples and require the minimum full-gate threshold while reporting the stronger target honestly."

### Phase close

```commit
feat(tooling): parallelize equivalent full verification
```

## Phase 5: Add common affected-package feedback

**Execution mode: inline.**

Completes: ["affected-package-feedback"]

### Task 5.1: Own affected-package and meta-suite selection
Applying: ["performance-budgeted-parallel-verification:affected-package-feedback"]
Paths: ["test-selection.json", "internal/testselection", "cmd/testselection", "internal/git", "go.mod", ".awf/domains/tooling.yaml", "docs/domains/tooling.md", "docs/topics/tooling", ".awf/awf.lock"]
Post-check: "Run table and real-repository tests for changed package ownership, test-package changes, reverse dependents, deleted and renamed paths, untracked Go files, build tags, generated assets, templates, configuration, runner/tooling files, declared meta-suites, empty changes, malformed Git output, and unavailable dependency graphs. Require deterministic package and reason output, no omitted reverse dependent, conservative widening or explicit refusal for every uncertain case, and backend-agnostic contract-suite registration for any new `internal/git` entrypoint."

Create one versioned selector policy and one Go owner for path normalization, package discovery, reverse dependency closure, meta-suite declarations, and fail-closed outcomes. Add `internal/testselection/**` to `.awf/domains/tooling.yaml`, render its generated domain and topic outputs with the lock, and require domain coverage to classify the new package as tooling-owned. `internal/testselection` consumes normalized changed-path evidence from `internal/git`; if the existing seam cannot supply the complete set from `HEAD` to the working tree, add the smallest cohesive `internal/git` entrypoint rather than parsing Git output again. Default local selection includes staged, unstaged, deleted, renamed, and untracked relevant files; explicit staged and range inputs remain available for reproducible candidate checks. The selector reports every package or meta-suite with its reason and never silently converts uncertainty into an empty result.

### Task 5.2: Compose the separate feedback command
Applying: ["performance-budgeted-parallel-verification:affected-package-feedback"]
Paths: ["x", "internal/project/gate_runner_test.go", "cmd/testselection", "test-selection.json", "test-performance.json"]
Post-check: "Execute fast gate and affected feedback against representative leaf-package, reverse-dependent, CLI, template, generator, configuration, tooling, deletion, empty, and uncertain fixtures. Require the fast gate composition unchanged, selected behavioral commands to run exactly once, shared or uncertain inputs to widen visibly, and common qualified workloads to report against the 10 to 15 second target without treating widened workloads as common samples."

Expose affected behavior as a separate test-feedback command rather than adding tests back to `./x gate`. Reuse the bounded shard executor for multi-package selections, omit coverage from common feedback, and preserve exhaustive full verification at terminal boundaries.

### Task 5.3: Apply feedback and gate-cadence claims
Applying: ["performance-budgeted-parallel-verification:affected-package-feedback"]
Kind: batch
Paths: ["docs/decisions/0314-performance-budgeted-parallel-verification.md", ".awf/topics/parts/tooling/quality-gates/current-state.md", ".awf/docs/parts/testing", ".awf/docs/parts/development/command-runner.md", "templates/partials/gate-cadence.md", ".awf/parts/agents-doc/commands.md", "docs/topics/tooling/quality-gates.md", "docs/testing.md", "docs/development.md", "docs/workflow.md", "docs/config-reference.md", "AGENTS.md", ".pi/agents", ".pi/skills", ".claude/agents", ".claude/skills", "test-selection.json", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Representative: ["tooling/quality-gates:affected-package-feedback", "tooling/quality-gates:gate-tier-cadence"]
Edge: ["the commit gate remains static-analysis-only", "focused feedback is separate and fail-closed", "full terminal cadence and exact mutation qualification remain unchanged"]
Post-check: "Apply one ADR lifecycle batch containing exactly the affected-feedback add and gate-tier-cadence update, with test backing and matching claim mutations. Render and require clean drift, review generated command/testing prose for the three distinct iteration, commit, and terminal boundaries, and require pending context to leave only exact-revision repository acceptance unapplied."

### Phase close

```commit
feat(tooling): add affected-package test feedback
```

## Phase 6: Parallelize hosted assurance and relocate release validation

**Execution mode: inline.**

Completes: ["parallel-ci-release-assurance", "final-performance-evidence"]

### Task 6.1: Extract the production archive validator
Applying: ["performance-budgeted-parallel-verification:ci-owned-release-artifact-validation"]
Paths: [".goreleaser.yaml", "cmd/releasecheck/main.go", "cmd/releasecheck/main_test.go"]
Post-check: "Run synthetic tar, zip, checksum, mode, owner, path-membership, and restricted-rootless fixtures through the production validator without invoking GoReleaser. Require malformed, missing, extra, unsafe, wrong-owner, wrong-mode, and checksum-mismatched artifacts to fail with stable diagnostics, while the validator accepts a separately constructed canonical snapshot directory."

Move archive membership, checksum, ownership, mode, and rootless-extraction validation from the heavyweight test body into a production releasecheck operation that accepts an explicit `dist` root. Retain small synthetic tests for validator behavior and remove every ordinary Go-test path that invokes GoReleaser.

### Task 6.2: Split CI into exact-revision parallel lanes
Applying: ["performance-budgeted-parallel-verification:bounded-parallel-assurance", "performance-budgeted-parallel-verification:equivalent-coverage-collection", "performance-budgeted-parallel-verification:ci-owned-release-artifact-validation"]
Kind: batch
Paths: [".github/workflows/ci.yml", ".github/workflows/release.yml", "x", "internal/project/gate_runner_test.go", "internal/project/remote_policy_docs_test.go", "cmd/releasecheck/main.go", "cmd/releasecheck/main_test.go", "test-performance.json"]
Representative: ["Linux Go coverage shards", "coverage aggregation and policy", "Pi behavior", "analysis and cross-builds", "macOS native Go shards", "release-config snapshot and validator", "stable CI / gate aggregator"]
Edge: ["all jobs use the same exact revision", "coverage policy and Codecov consume only the canonical merged profile", "range-qualified mutation runs once on Linux and remains separately timed", "release-config validates the same snapshot bytes it constructs", "stable required gate and release-config identities remain unchanged"]
Post-check: "Validate workflow syntax and run structural workflow tests that remove each required dependency in turn and must fail. On hosted CI, require every shard artifact to identify the candidate SHA and workload, aggregation to reject missing or foreign artifacts, both stable required conclusions to bind the exact revision, Codecov to receive only canonical raw/filtered outputs, and production archive validation to consume the release-config snapshot."

Use fixed, timing-balanced matrix shards derived from the tracked qualification record, not runner-order discovery. Keep Linux-only coverage and mutation, native macOS behavior, exact platform assertions, and exact-SHA release acceptance. Parallel compute cost may rise; wall-clock critical path and complete dependency closure are the governing evidence.

### Task 6.3: Apply exact-revision acceptance and release documentation
Applying: ["performance-budgeted-parallel-verification:bounded-parallel-assurance", "performance-budgeted-parallel-verification:ci-owned-release-artifact-validation"]
Kind: batch
Paths: ["docs/decisions/0314-performance-budgeted-parallel-verification.md", ".awf/topics/parts/tooling/quality-gates/current-state.md", ".awf/topics/parts/tooling/changelog-and-release/current-state.md", ".awf/docs/parts/testing", ".awf/docs/parts/releasing/content.md", "docs/topics/tooling/quality-gates.md", "docs/topics/tooling/changelog-and-release.md", "docs/testing.md", "docs/releasing.md", "test-performance.json", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Representative: ["tooling/quality-gates:exact-revision-repository-acceptance"]
Edge: ["the required conclusion remains stable while its dependency fan-in changes", "release artifact construction may fail in CI after local tests and lint pass", "the release workflow still verifies exact-SHA required conclusions before credentials"]
Post-check: "Apply the final exact-revision acceptance update with retained test backing and matching claim prose. Render and require clean drift, then require ADR pending context to report no remaining operation while status stays Implementing. Meaning-review testing and release docs for exact artifact ownership, stable job identities, and absence of local GoReleaser construction claims."

### Task 6.4: Qualify the completed system
Applying: ["performance-budgeted-parallel-verification:qualified-performance-contract"]
Paths: ["test-performance.json", "cmd/testperformance", "x", ".github/workflows/ci.yml", "coverage-baseline.json"]
Post-check: "On the declared local environment before the closing commit, collect repeatable warm and cold fast, common affected, ordinary full, and selected-mutation samples; require ordinary full at or below the 4x threshold and classify widened common workloads separately. After the immutable Phase 6 closing commit exists, run complete CI and release-config assurance against that exact SHA on the declared hosted runner class, require both stable conclusions green, and retain SHA-keyed emitted timing artifacts for the hosted critical path without rewriting the tracked record. Report whether the 10 to 15 second common, 55 second stronger local-full, 60 second hosted, and 16x feedback targets were achieved without changing their thresholds."

Run focused package, selector, runner, coverage, release, workflow, current-state, render, and drift checks before the one terminal full verification for the complete candidate. Record all local qualification evidence, preserve the artifacts needed for implementation review, and create the immutable Phase 6 closing commit with the subject below. Push and qualify that exact SHA in hosted CI; no implementation mutation may follow hosted qualification. Audit the immutable implementation range, and do not close ADR or plan lifecycle here; effort finalization owns the deferred terminal transaction after assurance settles.

### Phase close

```commit
feat(tooling): parallelize hosted verification
```

## Definition of done

- `dod: qualified-performance-evidence` A canonical versioned record identifies workloads, environments, baselines, budgets, sample method, and component evidence; unlike environments are never compared as one qualification.
- `dod: redundant-test-work-removed` Correctness runs contain no embedded benchmark loops, direct exported-Test invocation, nested target rendering, or deleted oracle whose distinct behavior lacks a retained owner.
- `dod: parallel-safe-dominant-packages` Mutable command seams are instance-owned, clispec remains the only command-policy source, expensive reusable fixtures are immutable or explicitly cloned, and representative CLI, safety, recovery, topology, rollback, and invariant oracles remain green under stress.
- `dod: equivalent-parallel-full-gate` Deterministic shards and the canonical merger reproduce the authoritative coverage universe and every exact projection, the full gate remains exhaustive, and ordinary local full verification meets the approved 4x minimum without selected mutation.
- `dod: affected-package-feedback` The separate common-feedback path selects changed packages, reverse dependents, and declared meta-suites with visible fail-closed widening while the fast commit gate remains unchanged.
- `dod: parallel-ci-release-assurance` Stable exact-revision conclusions depend on every required parallel native, policy, coverage, and release lane; CI validates the exact snapshot it builds and ordinary local Go tests never invoke GoReleaser.
- `dod: final-performance-evidence` Qualified evidence reports the achieved common, strong local-full, hosted critical-path, cold-cache, and exceptional mutation results against unchanged approved targets, with any conservative widened workload classified separately and no assurance weakened to improve a number.

## Notes

The landed stable-toolchain baseline is 1.204 seconds for the fast gate and 625.426 seconds for an ordinary full gate with an empty mutation universe; the profiled serial Go lane accounts for 605 seconds. The ordinary full minimum is therefore at or below 156 seconds, while 55 seconds remains the stronger target. These are qualification inputs, not expected corpus counts.

Apply ADR-0314 operations in this exact sequence: Phase 1 adds `tooling/quality-gates:verification-performance-contract`; Phase 3 adds `tooling/cli:cli-runner-instance-ownership` and `tooling/test-infrastructure:immutable-fixture-seeds`; Phase 4 updates `tooling/quality-gates:coverage-raw-identity-ratchet`; Phase 5 adds `tooling/quality-gates:affected-package-feedback` and updates `tooling/quality-gates:gate-tier-cadence`; Phase 6 updates `tooling/quality-gates:exact-revision-repository-acceptance`. Keep ADR-0314 Implementing and this plan Proposed after the final application batch; effort finalization performs their status-only Implemented transitions after review and terminal reconciliation.

Phase 1 Task 1.2 ran on the dirty Task 1.1 candidate at `d53a6c172ce177c8461bb82be7032101c217657f` with Go 1.26.4. The authoritative serial command was `go test -p=1 -timeout=20m -count=1 ./... -coverpkg=./... -coverprofile=<serial>`; it took 566 seconds and emitted 114,525,690 bytes, but the then-unfinished candidate had two focused repository-test failures. Four deterministic whole-`coverpkg` text shards used isolated homes and temporary roots, ran concurrently in about 510, 363, 284, and 133 seconds, and emitted 11,571,922, 6,144,398, 15,262,531, and 10,598,247 bytes. Four dependency-scoped shards derived instrumentation with `go list -deps -test`, took 518, 372, 294, and 141 seconds, and emitted the same respective sizes. A deterministic temporary merger deduplicated identities and OR-merged counts; it never concatenated profile headers.

Both text-shard candidates reproduced the complete 17,050-identity universe with digest `375111319368b7b96b41d3f295d07e0a4422d260b6775a5c80b9c53ee7bc6848` and produced byte-identical 1,431,581-byte canonical merges with SHA-256 `2938eb9d7ab9dd311d9e48a66ad8d88e57b9078265a2abdd9068d3fccf4d97b6`. They did not qualify: relative to the serial canonical profile, `internal/project/tree_reader.go:42.17,44.4` and `:73.17,75.4` moved from covered to uncovered, changing the `publication-application` selector and the filtered digest from `a01a1a4bdc35471b2fdaaaef5bac0142d91b3cfd392303622b276ee62db2326d` to `f3b19567ef4be57719ea1bc5a5cd2dd510236ea474a75148050d38ff62bb14bf`; the other five selector projections matched. Binary covdata emitted a 97,266-byte text profile for one compiled package test but provided no supported complete `go test ./...` test-binary universe without a new runner and merger. No compact candidate proved every required condition. Deterministic whole-`coverpkg` text shards remain the exact fallback, and Phase 4 must resolve the two shard-sensitive blocks and re-prove directive execution and every exact projection from a green candidate before replacing serial collection.

The spike used package prefix `github.com/hypnotox/agentic-workflows/` with these exact deterministic groups: G0 `changelog cmd/awf cmd/deadcodecheck cmd/repoaudit internal/audit internal/checkresult internal/commitpolicy internal/configspec internal/contextq internal/currentstatecoord internal/frontmatter internal/initop internal/memorycite internal/pitfall internal/presentation internal/prosegate internal/render internal/snapshot internal/testsupport/fsfixture internal/upgrade tools/pi-extension-test/lockrun`; G1 `cmd/mutants cmd/testperformance internal/catalog internal/clispec internal/config internal/contextdelivery internal/contextspill internal/domainop internal/execution internal/generatedcheck internal/initspec internal/migrate internal/pitfallcheck internal/project internal/repositorycheck internal/testperformance internal/testsupport/gitfixture internal/vocabularycheck`; G2 `cmd/contextspilllog cmd/pincheck cmd/versioncheck internal/changelog internal/commitgateop internal/configcheck internal/contextinput internal/coverage internal/effort internal/filepublication internal/git internal/localdocop internal/outputplan internal/plan internal/projectlicense internal/publisher internal/referencecheck internal/resident internal/testsupport internal/topic internal/worktree`; G3 `cmd/covercheck cmd/releasecheck internal/adr internal/checkop internal/commitmsg internal/configop internal/contextop internal/currentstate internal/effortop internal/evals internal/filesystem internal/glossary internal/manifest internal/pathglob internal/plancheck internal/projectstate internal/refs internal/severity internal/testsupport/cmd/testtmpclean internal/topicop templates`.

For each `N`, the whole command was `env HOME=<tmp>/homeN TMPDIR=<tmp>/tmpN GOTMPDIR=<tmp>/gotmpN GOTOOLCHAIN=go1.26.4 go test -p=1 -timeout=20m -count=1 $(cat groupN.pkgs) -coverpkg=./... -coverprofile=wholeN.out`. The dependency command first ran `go list -deps -test $(cat groupN.pkgs) | grep '^github.com/hypnotox/agentic-workflows/' | sort -u | paste -sd, -`, then passed that result as `-coverpkg` to the same isolated `go test` command. The temporary merger invocation was `python3 merge.py whole-merged.out whole0.out whole1.out whole2.out whole3.out`; `merge.py` required identical modes, parsed every non-header line with `rsplit(' ', 2)`, rejected conflicting statement counts, retained the maximum execution count for each exact block, and wrote sorted identities. The covdata attempt ran `go test -c -coverpkg=./... -o covercheck.test ./cmd/covercheck`, `covercheck.test -test.gocoverdir=covdata -test.run '^TestNoSuchTest$'`, and `go tool covdata textfmt -i=covdata -o=binary-text.out`. These commands and memberships reproduce the reported evidence without relying on package discovery order.

The ordinary-full component maxima are the landed baseline values for the serial profiled lane, the five dominant packages, coverage policy, and Pi smoke. This pins the deterministic comparison boundary to observed evidence rather than inventing new thresholds; a qualified observation missing one of those components or exceeding its median limit blocks qualification, while wall-clock targets remain reported evidence.

Phase 1's closing subject carries the required explicit ADR application qualifier because the transaction enters ADR-0314 Implementing and applies its first batch.

The completed Phase 1 candidate recorded three green warm fast-gate samples at 1.210, 1.229, and 1.231 seconds, for a 1.229-second median. Its timed ordinary-full reporter completed every Go test in 602 seconds and reached coverage policy in 603.802 seconds total; coverage policy then refused the expected new `cmd/testperformance` and `internal/testperformance` identity universe before the focused coverage expansion. This policy-refused sample is diagnostic evidence rather than an achieved full-gate observation, so it does not replace the landed 625.426-second baseline or qualify a budget.

Plan-review dispositions: scope the command-seam scan to the converted `cmd/awf` ownership boundary so unrelated historical globals cannot widen Phase 3; qualify the mandatory 4x result after Phase 4's measured stage concurrency rather than at its intermediate collector task; consume normalized change evidence through `internal/git` so test selection cannot create a second Git-status policy; and qualify hosted assurance only after the immutable Phase 6 commit, retaining SHA-keyed emitted evidence so the qualified revision is not mutated.

Phase 5 distinguishes production imports from test-only imports so a test edge selects its importing package without falsely affecting that package's production consumers. A full `cmd/awf` reverse-dependent run exceeded the common budget, so its repository-declared exact-name composition suite retains the representative registry, help, architecture, runner, and process-boundary oracles for indirect changes; a direct `cmd/awf` change still runs the complete owner package, and shared or uncertain evidence still widens to complete Go behavior. This applies the ADR's owner-matrix and representative caller-boundary rule without changing terminal assurance.
