---
format: plan-v1
date: 2026-08-02
adrs:
  - capability-planned-execution-for-multi-step-systems
status: Implemented
---
# Plan: Capability-planned execution and repository checks

## Goal

Implement the capability-planned execution pattern from [ADR-capability-planned-execution-for-multi-step-systems](../decisions/capability-planned-execution-for-multi-step-systems.md), first by making one project check construct one output plan, then by moving every direct and aggregate repository check onto one closed, typed preparation and execution path.

The change does not merge the working filesystem, working current-state snapshot, and stage-0 index universes; add registration, plugins, retries, rollback, parallel execution, or persistent caches; change successful command presentation; or broaden staged drift semantics.

## Architecture summary

Phase 1 is a bounded preparatory refactor inside `internal/project`: `Project.CheckReport` creates one operation-owned `OutputPlan` and passes it to both drift and advisory projections, while standalone compatibility entry points continue to derive their own operation inputs. Phase 2 adds the standard-library-only `internal/execution` mechanism and its first consumer in `cmd/awf`: command-owned requirement preparation retains typed config, project, report, current-state, and index values in an operation-local builder, freezes them into typed action closures only after complete preparation, and delegates only selection, closure, ordering, failure policy, and attempted-step outcomes to the shared package.

Dependencies point from `cmd/awf` to `internal/execution` and existing domain packages. `internal/execution` imports no consumer package and transports no consumer value. `internal/project`, `internal/snapshot`, and the prose and memory scanners remain independent of the execution mechanism. Authored ownership, topic claims, architecture, roadmap, and changelog changes live in their `.awf/` or source files; `./x render` produces every managed output.

## Phase 1: Construct one output plan per project check

**Execution mode: inline.**

### Task 1.1: Pin the single-plan project-check boundary
Latitude: exact
Paths: ["internal/project/check_test.go", "internal/project/stateownership_test.go"]

Add a focused source-structure regression in `internal/project/check_test.go` that locates `CheckReport`, `checkWithState`, and `advisoryNotesWithState` through the Go AST rather than unconstrained text counts. Require `CheckReport` to construct exactly one output plan, require both private consumers to receive that plan rather than call `outputPlan`, and require the advisory path not to call `generateDomainDocs` or `generateConfigReference`. Exercise an ordinary check fixture whose notes depend on generated domain and config-reference write nodes so the structural assertion is paired with behavior, not only syntax.

Keep `TestProjectDerivedStateOwnership` in `internal/project/stateownership_test.go` exact: `CheckReport`, `AdvisoryNotes`, `syncReport`, `ConfigReferenceModel`, and `OutputPlan` remain the operation entries that each derive their own state once; no producer moves onto `Project`, package state, or a resettable invocation cache. Do not add the `rendering/project-output-plan:check-report-single-plan` proof marker in this phase; terminal review owns that marker and claim activation.

Run `go test ./internal/project -run 'TestCheckReport.*OutputPlan|TestProjectDerivedStateOwnership' -count=1`. The new single-plan assertion must fail before Task 1.2 and the existing state-ownership test must remain green.

### Task 1.2: Thread one OutputPlan through drift and advisories
Latitude: exact
Paths: ["internal/project/check.go"]

In `Project.CheckReport`, preserve command-wiring validation, one `deriveOperationState`, one plan-directory parse, and diagnostic-to-drift mapping. After operation state and plans are available, call `p.outputPlan(ctx, corpus, topics, eff)` once and pass the returned `*OutputPlan` to both `checkWithState` and `advisoryNotesWithState`.

Change both private helpers to accept the prepared plan. `checkWithState` must continue to use its write files and reservation policies for drift without reconstructing the plan. `advisoryNotesWithState` must use the same plan's write files, including the generated domain documents and config reference already represented by write nodes, for unset-variable, stub, and marker advisories; remove its duplicate `generateDomainDocs` and `generateConfigReference` calls and the coverage exclusions justified only by those repeated producers.

`Project.AdvisoryNotes` remains a standalone compatibility operation: it derives state and parses plans for its own invocation, constructs one output plan, and passes it to the advisory helper. `Project.Check` remains the drift projection of `CheckReport`. Do not cache the plan on `Project`, add a global seam, change output ordering, or change drift and advisory contents.

### Task 1.3: Verify the preparatory transaction
Latitude: exact
Paths: ["internal/project/check.go", "internal/project/check_test.go", "internal/project/stateownership_test.go"]

Run `gofmt -w internal/project/check.go internal/project/check_test.go internal/project/stateownership_test.go`, `go test ./internal/project -count=1`, `go test ./cmd/awf -count=1`, `git diff --check`, `./x render`, `./x check`, and `./x gate`. Every command must succeed; render must introduce no unexplained generated change; the ADR and plan must remain Proposed; and `rg -n 'check-report-single-plan' .awf/topics internal/project` must return no newly activated claim or proof marker.

### Phase close

Stage the complete preparatory transaction, run `./awf check staged` and `./x gate`, and create the commit only after both checks pass:

```commit
refactor(rendering): construct one check output plan
```

## Phase 2: Add capability-planned execution and adopt repository checks

**Execution mode: inline.**

### Task 2.1: Specify the closed execution mechanism
Latitude: exact
Paths: ["internal/execution/execution_test.go"]

Create table-driven tests for a caller-declared system of requirement and step definitions. Cover duplicate requirement and step identities, an unknown requested step, an unknown foundation, an unknown declared dependency, a dependency cycle, and declaration-order tie-breaking at the pre-preparation barrier. Cover foundations preparing before selected-step requirement resolution; a resolver returning an unknown requirement at the post-foundation barrier; dependency closure over the union of selected requirements; declaration-order preparation; and each requirement preparing at most once even when foundations, dependencies, and multiple selected steps share it.

Cover failure at every readiness stage: foundation preparation, conditional resolution, secondary preparation, and action binding must each produce no runnable prepared value and execute zero actions. Binding must reject missing, extra, duplicate, or wrong-identity actions and occur only after the complete selected closure succeeds. Cover requested steps executing in their declaration order, structured outcomes retaining the identity and error of every attempted step, stop-on-failure halting before the next action, and continue-on-failure attempting every selected action.

Cover cancellation before the first action and between actions as execution-level `ctx.Err()` without an invented outcome; cancellation observed after an attempted action must retain that action's outcome and also return `ctx.Err()`. Assert errors by identity or stable typed fields where callers branch on them. Add these proof markers immediately above the named tests that substantively prove their complete claim bodies:

```go
// invariant: code-design/execution-planning:closed-step-selection (TestClosedStepSelection)
// invariant: code-design/execution-planning:requirements-prepared-once (TestRequirementsPreparedOnce)
// invariant: code-design/execution-planning:explicit-step-failure-policy (TestExplicitStepFailurePolicy)
```

The test design must not require a package global, runtime registry, reflection, `any`, a consumer-value bag, panic recovery, parallelism, retry, rollback, or a test-only production option.

### Task 2.2: Implement the standard-library-only execution package
Latitude: exact
Paths: ["internal/execution/execution.go", "internal/execution/execution_test.go"]

Create `internal/execution` with one package comment stating that it selects closed operation steps, prepares their requirement closure once, and executes prepared actions in deterministic order. Implement the concrete exported boundary as `RequirementID string`, `StepID string`, `Requirement{ID RequirementID, Dependencies []RequirementID, Prepare func(context.Context) error}`, `Step{ID StepID, Requirements func(context.Context) ([]RequirementID, error)}`, `Action func(context.Context) error`, `BoundAction{Step StepID, Run Action}`, `Binder func([]StepID) ([]BoundAction, error)`, and `System{Requirements []Requirement, Steps []Step, Foundations []RequirementID, Bind Binder}`. Add `Prepare(context.Context, System, []StepID) (*Prepared, error)`, the `FailurePolicy` constants `StopOnFailure` and `ContinueOnFailure`, `Outcome{Step StepID, Err error}`, and `(*Prepared).Run(context.Context, FailurePolicy) ([]Outcome, error)`. Document every export and map each one to the Phase 2 command consumer; add no constructor, option, or declaration unused by `cmd/awf`.

Use a package-private typed `definitionError` with stable kind, step, requirement, and referenced-identity fields for graph, selection, resolved-identity, and binding-shape failures so package tests assert structure without exporting a speculative caller protocol. Wrap consumer preparation, resolution, binding, and action errors with stage and identity context using `%w`; callers that own those errors must retain `errors.Is` and `errors.As`. Reject an unsupported `FailurePolicy` before running an action.

Validate the complete static graph before any preparation. Use declaration order for selection, topological ties, preparation, action execution, and outcomes; maps may index identities but must never determine observable order. Prepare the foundation dependency closure, resolve selected step requirements, validate every resolved identity, compute the union and dependency closure, and prepare every unique requirement at most once. Invoke `System.Bind` only after the full closure succeeds and require its ordered `[]BoundAction` identities to match selected steps exactly. `Prepared.Run` must return outcomes for attempted actions only and return cancellation separately as specified in Task 2.1.

The package must import only the Go standard library. It must not know about config, projects, Git, snapshots, scanners, writers, command output, or exit codes, and must not expose speculative step dependencies, registration, adapters, hooks, concurrency, retries, or rollback. Run `go test ./internal/execution -count=1`; all Task 2.1 cases must pass.

### Task 2.3: Specify one command-side capability plan
Latitude: exact
Paths: ["cmd/awf/checkrepo_test.go", "cmd/awf/checkgroup_test.go", "cmd/awf/prosegate_test.go", "cmd/awf/memorygate_test.go", "cmd/awf/run_test.go"]

Create `cmd/awf/checkrepo_test.go` around an operation-local dependency set whose functions can count config loads, project opens, report derivations, current-state derivations, and index captures without mutating package globals. Test direct selection of drift, state, prose, and memory and aggregate selection of all four in drift-state-prose-memory declaration order. Prove the aggregate loads working config once, opens at most one Project from that exact prepared config, prepares one complete `Project.CheckReport`, prepares one `CurrentStateReport`, and captures the stage-0 index once when both scanners are enabled. Prove scanner-only selections never open a Project, disabled scanners request no index and still print their established knob note, and a direct enabled scanner captures only the index capability it needs.

Use distinct fixture sentinels for the working project/filesystem input, current-state result, and index tree. Assert none is substituted for another. Make a later preparation fail after earlier requirements are ready and assert that no step or advisory output is written. Make drift return an action error and assert the aggregate still attempts state, prose, and memory, returns the first action error through the existing command mapping, and preserves every attempted action's established output. Assert direct children use stop-on-failure, retain their exact clean and disabled lines, and do not print the aggregate-only version-ahead or project advisory notes. Assert aggregate successful output keeps the version note first when applicable, then project advisory notes, then drift, state, prose, and memory output.

Retain and extend the public dispatch coverage in `checkgroup_test.go`, `prosegate_test.go`, and `memorygate_test.go` for working-config enablement and exemptions, staged bytes, custom `docsDir`, Git-independent disabled behavior, and enabled behavior outside Git. Update the AST structural baseline in `run_test.go`: remove the three direct `project.Open` owners in `checkrepo.go`, require the capability-plan composition path to contain the sole prepared-config project construction, and require no package-global dependency override. Add the marker `// invariant: tooling/cli:repo-check-capability-plan (TestRepoCheckCapabilityPlan)` immediately above the complete command integration test.

Run `go test ./cmd/awf -run 'TestRepoCheckCapabilityPlan|TestCheckDisabledChildDisclosure|TestProseGate|TestMemoryGate|TestProjectOpenCallSites' -count=1`. The new capability tests must fail before Task 2.4 while the retained compatibility tests continue to describe the required endpoint.

### Task 2.4: Route aggregate and direct checks through one prepared operation
Latitude: exact
Paths: ["cmd/awf/checkrepo.go", "cmd/awf/prosegate.go", "cmd/awf/memorygate.go", "cmd/awf/gate.go", "cmd/awf/sync.go"]

Define command-local `execution.StepID` constants `repoStepDrift`, `repoStepState`, `repoStepProse`, and `repoStepMemory` and `execution.RequirementID` constants `repoRequirementConfig`, `repoRequirementProject`, `repoRequirementCheckReport`, `repoRequirementCurrentState`, and `repoRequirementIndex`. Add `repoCheckInputs` with typed config, Project, CheckReport, CurrentStateReport, and index fields. Add `repoCheckDependencies` with exact function fields `loadConfig(string) (*config.Config, error)`, `openProject(context.Context, string, *config.Config) (*project.Project, error)`, `checkReport(context.Context, *project.Project) (project.CheckReport, error)`, `currentState(context.Context, *project.Project) (project.CurrentStateReport, error)`, and `indexTree(context.Context, string) (*snapshot.Tree, error)`. Production wrappers construct that value locally; tests pass a value directly, and no package variable stores it.

Implement `repoCheckSystem(string, io.Writer, bool, *repoCheckInputs, repoCheckDependencies) execution.System` to declare requirements, conditional resolvers, and the binder; the boolean is the aggregate presentation mode. Implement `runRepoCheckSelection(context.Context, string, io.Writer, []execution.StepID, execution.FailurePolicy, bool, repoCheckDependencies) error` as the one prepare-run-outcome adapter. `runCheckRepo` and the four direct child functions are thin production wrappers that supply selected IDs, policy, aggregate mode, and freshly composed dependencies. Keep working config and scanner enablement in the foundation. Resolve Project plus CheckReport only for drift, Project plus current-state report only for state, and the index only for an enabled prose or memory step. A disabled scanner resolves no index requirement and binds its established disabled-note action.

Compose the project requirement with a `project.Loader` whose invocation-local `LoadConfigTree` returns the already prepared config only for `config.RootDir(root)`. Preserve `Loader.Open` as the owner of config validation, target and effective-catalog derivation, catalog conformance, resident-root resolution, and Project construction. Compose the same Git handle and standard catalog dependencies used by ordinary project opening, without re-reading config bytes, caching on `Project`, or creating a second project-opening implementation. Keep `stagedTree` as the one index snapshot mechanism; the shared index requirement calls it once and the existing staged-gate consumers remain valid.

Refactor prose and memory code into typed actions that consume prepared config and, when enabled, the prepared index tree. Their direct entry points must select the same step definitions rather than load config or Git independently. Preserve exemption parsing, staged-file filtering, binary and symlink behavior, output text, and error prefixes.

Preserve existing model-owner presentation boundaries while changing only acquisition and ordering: prose findings continue through `prosegate.Format`, memory findings through `memorycite.Format`, and current-state lines through `project.CurrentStateReport.Notes` and `Findings`. The command retains renderer selection, established drift-line layout, clean and disabled lines, writer routing, ordering, and exit mapping; neither `internal/execution` nor a new command-side result model renders domain output.

Make `runCheckDrift`, `runCheckState`, `runProseGate`, and `runMemoryGate` select one step with stop-on-failure. Make `runCheckRepo` select all four with continue-on-failure. Bind aggregate drift output from the complete `CheckReport`: emit its advisory notes before drift output, while the direct drift action ignores notes. Keep the aggregate version-ahead note in its established leading position and retain first-step-error exit mapping. Preparation and binding errors return before any action or advisory output; no shared package code writes output or chooses an exit result.

### Task 2.5: Close command and structural compatibility coverage
Latitude: exact
Paths: ["cmd/awf/checkrepo_test.go", "cmd/awf/checkgroup_test.go", "cmd/awf/check_test.go", "cmd/awf/prosegate_test.go", "cmd/awf/memorygate_test.go", "cmd/awf/run_test.go", "internal/project/stateownership_test.go"]

Make all Task 2.3 tests pass and update existing exact structural expectations rather than weakening them. `TestProjectDerivedStateOwnership` must continue to show that project operation state is derived by its owning entry and threaded downward. The `cmd/awf/run_test.go` AST census must establish the new single composition path rather than deleting the baseline. No test may swap a package-global function or rely on execution order between tests.

Run `go test ./internal/execution ./internal/project ./cmd/awf -count=1`. The three packages must pass together, including direct command dispatch and aggregate behavior.

### Task 2.6: Apply package ownership and the first four claims
Latitude: exact
Paths: [".awf/domains/code-design.yaml", ".awf/domains/parts/code-design/current-state.md", ".awf/topics/metadata/code-design/execution-planning.yaml", ".awf/topics/parts/code-design/execution-planning/current-state.md", ".awf/topics/parts/tooling/cli/current-state.md", "docs/decisions/capability-planned-execution-for-multi-step-systems.md"]

Create `.awf/domains/code-design.yaml` with exactly:

```yaml
paths:
  - internal/execution/**
```

Revise the code-design domain narrative so it owns `internal/execution/**` as its scoped capability-planning implementation surface while its pre-existing topics remain explicit global guidance. Retain `.awf/topics/metadata/code-design/execution-planning.yaml` as the path-scoped owner of `internal/execution/**`; do not imply that a global topic can own paths.

Replace the execution-planning shell with these exact claims, using the proof names from Task 2.1:

```markdown
Capability-planned execution is the scoped implementation surface for closed multi-step operations. Consumers retain domain values and policy while the shared package owns only closed selection, operation-scoped requirement preparation, binding, ordering, and attempted-step outcomes.

## Claims

### `invariant: closed-step-selection`

internal/execution validates a caller-supplied closed set of step and requirement definitions before preparation, selects requested steps in declaration order, rejects a conditionally resolved unknown requirement at a second barrier, and binds exactly the selected step identities only after their complete requirement closure is prepared. It has no runtime registry, reflection, or universal consumer-value container.
Origin: ADR-capability-planned-execution-for-multi-step-systems
Backing: test

### `invariant: requirements-prepared-once`

One prepared execution completes the foundation dependency closure before selected steps resolve their conditional requirements, then prepares the union of the selected requirement closures in declaration-stable dependency order with each requirement prepared at most once. A validation, preparation, resolution, or binding failure executes zero actions.
Origin: ADR-capability-planned-execution-for-multi-step-systems
Backing: test

### `invariant: explicit-step-failure-policy`

A prepared execution runs selected actions in declaration order under an explicit stop-on-failure or continue-on-failure policy and returns an ordered identity-and-error outcome for every attempted step. Cancellation remains a separate execution-level error: no outcome is invented for an unattempted step, and an attempted action's outcome is retained when cancellation is also observed.
Origin: ADR-capability-planned-execution-for-multi-step-systems
Backing: test
```

Append this exact claim to `.awf/topics/parts/tooling/cli/current-state.md`:

```markdown
### `invariant: repo-check-capability-plan`

The direct drift, state, prose, and memory repository checks and their aggregate select from one closed capability plan. One operation loads working config once, conditionally opens one Project from that prepared config, derives one complete CheckReport and one working CurrentStateReport when selected, and captures one shared stage-0 index for enabled scanners; disabled or scanner-only selections acquire no unrelated capability. The aggregate preserves version, advisory, and step output order, continues after action errors and returns the first, while any preparation failure executes no step; the three working, current-state, and index universes never substitute for one another.
Origin: ADR-capability-planned-execution-for-multi-step-systems
Backing: test
```

Transition the ADR from Proposed to Implementing in this same transaction. Compute its canonical content stamp after every permitted body edit is complete, change frontmatter to `status: Implementing`, append the stamped `Implementing` history event, then append exactly one Applied event naming the first four declared operations in declaration order. Do not apply or mention the fifth operation in that event. The Phase close must stage the ADR lifecycle edit, four claim mutations, their four proof markers, production behavior, and domain ownership as one pair-atomic transaction.

### Task 2.7: Update authored architecture, roadmap, and release notes
Latitude: exact
Paths: [".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md", ".awf/docs/parts/roadmap/ideas.md", "changelog/CHANGELOG.md"]

Add `internal/execution` to the architecture component list with its closed selection, preparation, binding, and outcome ownership and standard-library-only dependency boundary. Update the project and command component text to state that `Project.CheckReport` constructs one operation-owned output plan and that `cmd/awf` composes typed repository-check capabilities through `internal/execution`. Extend the data flow with config foundation, conditional Project and index preparation, typed action binding, and stable direct or aggregate execution while naming the three distinct source universes.

Rewrite the roadmap's explicit snapshot-capability check item to remove the now-implemented orchestration work. Retain only the separate future question about whether staged drift can acquire semantics for directory entries, untracked files, config-tree hygiene, and dead-reference probing; do not imply that this phase broadens staged drift. Add an Unreleased changelog entry for capability-planned repository checks and the successful-output compatibility plus readiness-before-output failure boundary.

### Task 2.8: Render and verify the activation transaction
Kind: batch
Latitude: exact
Paths: [".awf/awf.lock", "docs/architecture.md", "docs/roadmap.md", "docs/domains/code-design.md", "docs/topics/code-design/execution-planning.md", "docs/topics/tooling/cli.md", "docs/decisions/INDEX.md"]
Representative: `.awf/docs/parts/architecture/components.md` renders the updated `internal/execution` ownership into `docs/architecture.md`, and the lifecycle edit renders the ADR's Implementing entry into `docs/decisions/INDEX.md`.
Edge: The domain path sidecar and first claims render `docs/domains/code-design.md` and the two named topic documents; `.awf/awf.lock` records their generated bytes, while no unrelated target or example output changes.
Post-check: After `./x render`, `git status --short` contains only the phase's explicitly named authored, production, test, ADR, changelog, plan, and generated paths; `./x check` finishes clean; any additional generated path is authority drift to resolve before Phase close, not an open-ended staging instruction.

Run `gofmt -w internal/execution/*.go internal/project/check.go internal/project/check_test.go internal/project/stateownership_test.go cmd/awf/checkrepo.go cmd/awf/checkrepo_test.go cmd/awf/checkgroup_test.go cmd/awf/check_test.go cmd/awf/prosegate.go cmd/awf/prosegate_test.go cmd/awf/memorygate.go cmd/awf/memorygate_test.go cmd/awf/gate.go cmd/awf/run_test.go cmd/awf/sync.go`, then `go test ./internal/execution ./internal/project ./cmd/awf -count=1`, `./x render`, `./x check`, `git diff --check`, and `./x gate`. Every command must succeed.

Inspect `git status --short` against the exact batch closure and never hand-edit a generated output. Before staging, require the ADR to be Implementing, the plan to remain Proposed, and `rg -n 'check-report-single-plan' .awf/topics internal/project` to show no activated fifth claim or proof marker. Phase close owns the only staging and staged verdict for this transaction.

### Phase close

Stage the complete implementation, first-application, authored documentation, release-note, and exact generated-output transaction. Run `./awf check staged` and require it to report the four Applied operations with matching claim and proof mutations, the fifth operation Remaining, the code-design production path covered by its scoped topic, and no unbacked marker. Run `./x gate`, and create the commit only after both checks pass:

```commit
refactor(code-design): plan repository check capabilities
```

## Definition of done

- `Project.CheckReport` constructs one `OutputPlan` and threads it to drift and advisories, while direct project compatibility operations remain operation-owned and uncached.
- `internal/execution` is a standard-library-only package with a minimal production-used API that validates both barriers, prepares every selected requirement once, binds after readiness, executes in declaration order, and returns explicit-policy structured outcomes plus separate cancellation.
- Direct and aggregate repository checks use one closed definition set; one aggregate operation loads config once, opens at most one Project, computes one complete check report and one current-state report, and captures one index for both enabled scanners.
- Working filesystem, working current-state, and stage-0 index inputs remain distinct typed capabilities; disabled scanners and scanner-only selections acquire no unrelated Git or Project input.
- Successful direct and aggregate output and first-error routing remain compatible; a preparation failure produces no step or advisory output, and continue-on-failure still attempts later aggregate actions.
- The code-design domain owns `internal/execution/**`, architecture and roadmap sources describe the landed boundary accurately, and generated documentation is drift-clean.
- The linked ADR is Implementing with exactly its first four operations Applied and the fifth Remaining; the plan remains Proposed until settled terminal implementation review.
- `go test ./...`, `./x render`, `./x check`, `./awf check staged`, and `./x gate` reach successful terminal states for their applicable transactions, with 100% statement coverage and no dead production code.

## Notes

Integration deviation: current `main` introduced typed plan-v2 assignment notes in both working and staged check reports after this plan was approved. The merged implementation keeps those values in `CheckReport.PlanNotes` and `CurrentStateReport.PlanNotes`; direct repository and staged aggregates each emit their selected universe, while bare `awf check` shares one command-local sink that coalesces only identical cross-universe plan notes. General project advisories retain their established multiplicity, and distinct working and staged plan notes remain distinct.

The fifth operation is deliberately not part of either implementation phase. After Phase 2 and independent implementation review settle, the main-thread terminal transaction must first record any implementation deviations or follow-ups here while the plan is still Proposed. It must then add `// invariant: rendering/project-output-plan:check-report-single-plan (TestCheckReportBuildsOneOutputPlan)` above the substantive Phase 1 regression test and add the following exact claim to `.awf/topics/parts/rendering/project-output-plan/current-state.md`:

```markdown
### `invariant: check-report-single-plan`

Project.CheckReport constructs one operation-owned OutputPlan after deriving its current state and parsed plans, threads that same plan to both drift and advisory projections, and never regenerates domain documents or the config reference inside either projection. Standalone Check, AdvisoryNotes, OutputPlan, and other direct project operations continue to derive their own operation-scoped inputs without a persistent cache.
Origin: ADR-capability-planned-execution-for-multi-step-systems
Backing: test
```

In that same terminal transaction, append the final Applied event for only `rendering/project-output-plan:check-report-single-plan`, append the stamped Implemented status event, change ADR frontmatter to `status: Implemented`, and change this plan's frontmatter to `status: Implemented`. Run `./x render`, stage the claim, proof marker, ADR and plan lifecycle edits, Notes changes, `docs/decisions/INDEX.md`, `.awf/awf.lock`, and every other render-selected output; require `./awf check staged` and `./x gate` to pass before the lifecycle-freeze commit. If review finds incomplete implementation or a material deviation, leave the fifth operation Remaining and both artifacts nonterminal until corrective implementation and renewed review settle.
