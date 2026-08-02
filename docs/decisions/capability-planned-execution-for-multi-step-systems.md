---
format: current-state-v3
slug: capability-planned-execution-for-multi-step-systems
status: Proposed
date: 2026-08-02
---
# ADR-capability-planned-execution-for-multi-step-systems: Capability-planned execution for multi-step systems

## Context

`awf check repo` is a closed multi-step operation whose aggregate and direct-child paths have drifted apart internally. The aggregate in `cmd/awf/checkrepo.go` opens a project for advisory notes, calls a drift child that opens another project, and calls a state child that opens a third. The drift child invokes `Project.Check`, which projects only drift from `Project.CheckReport` even though that report also computes notes. In one aggregate run, the command therefore computes advisories once directly and computes then discards them a second time through the drift child.

The duplication continues inside the project operation. `Project.CheckReport` derives its ADR corpus, topic corpus, effective skill set, and parsed plans once, but its drift and advisory branches each call `outputPlan`. The advisory branch then regenerates domain documents and the config reference even though those files are already nodes in that output plan. Reading the production call tree at the current branch gives three output-plan constructions for one `awf check repo`: one from the aggregate's direct `AdvisoryNotes` call and two from the drift child's `CheckReport`. This is operation-state duplication, not a throughput-only concern: each pass can observe mutable filesystem inputs at a different instant.

The other repo-check children demonstrate why one undifferentiated repository snapshot is not the answer. ADR-0210 assigns working-filesystem semantics to repo drift, a working snapshot to repo current-state evaluation, and the stage-0 index to the prose and memory whole-corpus scans. `runProseGate` and `runMemoryGate` each load working config, return before Git access when disabled, and otherwise call the same `stagedTree` mechanism separately. A shared operation may reuse one index capture across enabled scanners, but it must not supply index bytes to drift or working bytes to the scanners. It must also preserve direct scanner behavior: a disabled scanner reads config and reports its knob without opening Git or validating unrelated catalog state.

ADR-0180 already supplies the state-lifetime rule: an operation derives a value once and threads it to every consumer rather than caching it on a longer-lived value. ADR-0195 leaves rendering, output planning, drift, and current-state loading in `internal/project`. ADR-0210 leaves presentation and exit routing in `cmd/awf` and explicitly preserves the distinct check universes. The missing piece is reusable orchestration mechanics for a closed set of independently runnable steps: select one or many steps, prepare their shared prerequisites once, bind typed actions only after preparation succeeds, and execute in stable order.

The repository has adjacent but narrower models. `internal/catalog` computes catalog-specific dependency closure. `internal/project.ResolveEnable` and `ResolveDisable` produce config-mutation plans. `internal/project.OutputPlan` owns rendered artifact recipes and persistence policy. `internal/plan` owns authored Markdown plans. None owns operation preparation and execution, and extending any would merge unrelated meanings. The names `plan`, `node`, `workflow`, and `capability` are already domain vocabulary, so the shared mechanism needs its own narrowly named package and uses `step` and `requirement` vocabulary.

A new shared package also changes ownership authority. The code-design domain is currently documented as pathless because it owns global guidance only. `internal/execution/**` has no honest existing product domain: assigning a command-independent mechanism to tooling or rendering would misstate its purpose. The code-design domain therefore needs an authored path sidecar and a scoped execution-planning topic. This does not solve the roadmap's separate question of whether a global topic may own paths; the new topic is path-scoped.

### Coupling census

This decision introduces a package rather than moving an existing type, so there is no original-package caller or sibling-test migration set. The dependency direction is one-way: `internal/execution` imports only the standard library, and `cmd/awf` is its first production importer. `internal/project`, the scanners, and snapshot packages remain independent of the shared mechanism. Check-specific requirement meanings and typed values stay in `cmd/awf`, while the project APIs keep their existing package ownership. No interface inversion, cross-package method relocation, or `init` ordering is involved.

## Decision

1. Add a standard-library-only `internal/execution` package. Its one-sentence ownership is selecting closed operation steps, preparing their requirement closure once, and executing prepared actions in deterministic order. It owns orchestration mechanics only; consumers own requirement meanings, typed values, preparation mechanisms, applicability, business policy, presentation, and exit mapping.

2. Model one execution from a closed, caller-supplied set of step and requirement definitions. A requirement has an identity, dependency identities, and an operation-local preparation function. A step has an identity and resolves its conditional requirement identities after foundations exist. A system names foundation requirements, its closed definitions, and one operation-local binder. There is no package-global registry, runtime registration, reflection, `any`-valued dependency bag, service locator, persistent cache, or generic plugin protocol.

3. Validate in two barriers. Before any preparation, reject duplicate definition identities, unknown requested steps, unknown declared dependencies, and dependency cycles. After foundations prepare and selected steps resolve their conditional requirements, reject any resolved identity outside the prevalidated requirement set before preparing a secondary requirement. Selection and topological tie-breaking follow declaration order so maps never control observable order.

4. Preparation is derivation-only and operation-scoped. Prepare the foundation closure first, resolve requirements for the selected steps, compute their union and dependency closure, and prepare each unique requirement at most once. Any foundation, resolution, secondary preparation, or binding failure executes zero actions. No business mutation belongs in preparation; a mutating consumer puts mutation in its action phase.

5. Bind only after the complete selected requirement closure succeeds. The consumer converts its temporary preparation builder into immutable typed action closures, and the shared package validates that the bound action identities exactly match the selected steps. The shared package never transports consumer values itself. This keeps type assertions and universal input containers out of the boundary while letting each action capture only its domain inputs.

6. Execute prepared actions in selected declaration order and return one structured outcome per attempted step, retaining step identity and error. The caller explicitly chooses stop-on-failure or continue-on-failure. Context cancellation always stops further execution. The package does not render text, recover panics, choose an error aggregation policy, or assign an exit code. Direct one-step check children consume stop-on-failure; the repo aggregate consumes continue-on-failure, so both exported policies have concrete production consumers.

7. Make `awf check repo` the first adopter. Working config and scanner enablement are its foundation. Drift conditionally requires one opened `Project` and one complete `CheckReport`; state requires the same opened project and one working `CurrentStateReport`; enabled prose and memory steps require one shared stage-0 index snapshot; disabled scanners request no index and still execute their disabled-note action. Direct children and the aggregate select from the same definitions and use the same prepare-bind-execute path.

8. Preserve the three source capabilities as distinct typed values: working project/filesystem inputs for drift and advisories, the working snapshot owned by current-state evaluation, and the stage-0 index for prose and memory. No common tree bag or fallback crosses them. Scanner-only selections continue to use config-only foundation validation rather than opening a Project, and disabled scanners continue to avoid Git entirely.

9. Preserve successful command presentation and exit behavior. The aggregate emits its version and project advisory notes in their established positions, runs drift, state, prose, and memory in declaration order, continues after action-level findings, and maps the first step error to its existing exit behavior. Direct children retain their child-specific clean and disabled lines. Preparation now happens before any action: if any selected requirement cannot prepare, the aggregate intentionally emits no step output, replacing the current failure path in which an earlier child may print before a later child fails.

10. Refactor `Project.CheckReport` to construct one `OutputPlan` after deriving operation state and plans, then thread that plan into both drift and advisory projections. The advisory projection consumes the plan's write files directly instead of regenerating domain documents and the config reference. Keep `Check`, `AdvisoryNotes`, and direct project operations as compatibility projections, each deriving its own operation-owned inputs when called independently.

11. Give the pattern current-state ownership. Add `.awf/domains/code-design.yaml` with `internal/execution/**`, revise the code-design domain narrative from pathless to owning that implementation surface while its existing topics remain global, and add the path-scoped `code-design/execution-planning` topic. Update architecture and the roadmap's explicit snapshot-capability check item without claiming that global topics gained path selectors.

12. Back the generic selection, validation, exactly-once preparation, ordering, binding, failure-policy, cancellation, and result contracts with focused `internal/execution` tests. Back the first adopter with command integration tests proving direct and aggregate selection, one project open, one output-plan construction, one index capture, conditional no-Git behavior, universe isolation, stable output, zero-action preparation failure, and continued execution after action errors. Update the exact project-open and operation-state structural baselines, and add a structural check preventing `CheckReport` consumers from reconstructing the output plan. Introduce no package-global test seam.

13. Treat this as the first implementation of a common pattern, not evidence for anticipated features. Every exported declaration lands with a concrete use by `cmd/awf`; no option, hook, adapter, transitive step dependency, parallel executor, retry policy, rollback protocol, or registration surface is added without a concrete later consumer and its own semantics.

## State changes

- add `code-design/execution-planning:closed-step-selection`
- add `code-design/execution-planning:requirements-prepared-once`
- add `code-design/execution-planning:explicit-step-failure-policy`
- add `tooling/cli:repo-check-capability-plan`
- add `rendering/project-output-plan:check-report-single-plan`

## Consequences

Single-child and aggregate execution stop being parallel orchestration implementations. A selected operation has one validated definition set, one foundation pass, one requirement union, and one action order. Shared expensive inputs become operation-owned facts rather than repeated reads, while distinct filesystem and Git universes remain visibly different types.

The common package creates a reusable pattern for non-command systems without making command output, project loading, or Git part of its API. Its closure-based boundary is deliberately less automatic than a universal typed dependency container: each consumer writes a small binder, but the compiler keeps domain values typed and the shared package cannot become a service locator.

Preparation failure behavior changes for the repo aggregate. A corrupt later requirement now prevents all child output rather than allowing earlier child output before failure. This makes execution atomic with respect to readiness, not with respect to business mutation; actions may still use continue-on-failure when they are independent. Successful output and action-failure continuation remain compatible.

The code-design domain gains its first owned production path and therefore ceases to be wholly pathless. Existing global code-design topics remain global and do not acquire path ownership. The new scoped topic owns only the execution mechanism. The production-package domain-ownership gate will fail if this migration is omitted.

Costs include a new package, a command-side adapter and builder, new structural baselines, and a larger ADR/plan surface than the minimal duplicate-call fix. The gain is a mechanically reusable lifecycle with the exact first consumer the package-composition authority requires. Future consumers must still justify each new execution feature rather than treating this decision as a blank check.

A written implementation plan is required: package authority, generic mechanism, project single-plan refactor, command adoption, scanner input separation, tests, and generated documentation form interdependent transactions.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Only make `runCheckRepo` call `CheckReport` once | Removes immediate duplication but leaves direct and aggregate children on separate orchestration paths and establishes no reusable preparation model. |
| Convert every repo check to one snapshot tree | Working drift, working current-state, and tracked-corpus scans have intentionally different source semantics; one tree would erase ADR-0210's boundary. |
| Put the runner in `internal/project` | The mechanism is not project-specific and is intended for non-command consumers; project ownership would invert the generic-to-domain dependency. |
| Extend `internal/plan` or `OutputPlan` | Those packages own authored plans and rendered artifact recipes respectively, not runtime capability preparation. |
| Use an interface registry or `map[Requirement]any` | Creates a universal dependency bag, runtime assertions, and speculative registration rather than typed consumer-owned binding. |
| Fully generic typed step inputs | Heterogeneous step input types force more type machinery than the first consumer needs; operation-local typed closures preserve compile-time domain types directly. |
| Execute each step immediately after its own preparation | Preserves partial failure output but permits a later preparation failure after earlier actions have run, defeating readiness-before-execution. |
| Hard-code continue-on-failure in the runner | Fits checks but makes the common mechanism unsafe for dependent or mutating consumers; an explicit policy states the caller's semantics. |

## Status history

- 2026-08-02: Proposed
