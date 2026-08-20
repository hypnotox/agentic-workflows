---
format: plan-v2
date: 2026-08-20
adrs: [0296]
status: Proposed
---
# Plan: Separate Project State from Operations

## Goal

Replace the broad `internal/project.Project` state-and-operation receiver with Loader-constructed,
immutable `ProjectState` facts and operation-owned functions whose repository, tree-reader, and
filesystem dependencies are explicit. Preserve behavior and CLI output, retain `project.Open` only
as the authorized compatibility opener, and leave Publisher, RepositoryChecker,
CurrentStateCoordinator, command-operation extraction, and support-floor cleanup to RF-003 through
RF-006, RF-008B, and RF-014B.

## Architecture summary

ADR-0296 fixes the direction and owners. `Loader` constructs a private, defensively copied
`ProjectState`; configuration facts are separated from the concrete tree reader that supplies
sidecars and parts; catalog and target access returns immutable snapshots; Git and
`ProjectTreeReader` values are passed to the operation that uses them. During migration, one bounded exported
`Project` compatibility facade with private immutable state and a frozen caller and method allowlist
may forward existing receivers to private operation functions. The final phase exports
`ProjectState` and the operation functions with their first command consumers, then removes that
facade and every operation method. Package functions remain temporary `internal/project` owners
until the later issue assigned to each semantic extraction. No new
Publisher, RepositoryChecker, CurrentStateCoordinator, all-purpose application object, service
locator, provider-owned interface, or test-only seam is introduced.

**Plan flexibility.**

The protected-contract rule in the workflow document governs what a plan may not change. The plan records the best known route at authoring time, not a binding implementation choreography. A commit-capable owner may merge, split, reorder, add, remove, or replace recorded route detail while the protected contract holds. A path omitted from the plan is not alone a reason to stop, and a stale listed path need not be touched. Reapproval is required only when the protected contract would change or an unresolved material decision appears.

Reconcile a Proposed plan only when another phase or reviewer could rely on stale material instructions. Inconsequential and independently local edits require no deviation record. A delegated owner reports material cross-owner revisions for parent reconciliation. A helper remains confined to its assigned paths and gains no scope, commit, review, checkpoint, handoff, or outcome authority from route flexibility.

## Phase 1: Establish immutable loaded project facts

**Execution mode: subagent-driven.**

Advances: ["state-operation-separation", "behavior-preserved"]
Completes: ["immutable-project-state"]

### Task 1.1: Separate configuration facts from tree access
Kind: batch
Applying: ["0296:dependency-direction", "0296:boundary-values", "0296:proportional-operations"]
Paths: ["internal/config/config.go", "glob:internal/config/*_test.go"]
Representative: A loaded configuration snapshot deep-copies every map, slice, pointer block, local document, and arbitrary data value while an operation-scoped concrete tree value reuses the existing `TreeReader` for sidecars and parts.
Edge: A staged snapshot must continue reading all config, sidecar, part, and metadata bytes from its selected tree, while ordinary filesystem loading preserves current path and error behavior.
Post-check: Run the focused config tests, including mutation-after-construction cases for every reference-shaped field and reader parity cases for filesystem and snapshot trees; require all tests to pass and the state snapshot to retain no root, raw bytes, reader, or filesystem-mode dependency.

Introduce the smallest config-owned immutable facts value and concrete operation tree binding needed by
`ProjectState`. Preserve the existing config grammar, validation, serialization, migration, and
`TreeReader` contract. Do not add an interface, default dependency, or second parsing policy.

### Task 1.2: Construct immutable state behind the bounded facade
Kind: batch
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values"]
Paths: ["internal/project/project.go", "internal/project/target.go", "internal/project/loader_test.go", "internal/project/project_test.go", "internal/project/stateownership_test.go", "internal/project/target_test.go"]
Representative: A private `projectState` privately owns invoking and resident roots, nested status, validated immutable config facts, selected and complete catalog snapshots, and resolved target snapshots; it owns no Git handle or project-tree reader. The existing exported `Project` name becomes only a forwarding facade with a frozen production caller and method allowlist.
Edge: Accessors return scalars or defensive deep copies. Mutating Loader inputs or returned catalog, config, target, map, slice, or nested data values cannot change later state observations. No new caller or receiver method may attach to the facade.
Post-check: Run focused Loader, project-state, target, and state-ownership tests; require construction and validation behavior to remain green, mutation probes to leave state unchanged, and AST contracts to prove no repository handle, tree reader, exported reference-shaped field, or post-construction write exists on the private state and that the facade matches its frozen allowlists.

Keep `Loader` as the only validated constructor. Preserve repository and non-repository opening errors,
resident-root selection, profile selection, target resolution, and catalog validation. The bounded
exported facade keeps existing production signatures green, gains no caller or method, and is
removed in Phase 3 when the state and functions gain their first outside-package consumers.

### Phase close

Close with immutable state construction in production use and all pre-existing operations still
behaviorally green through the explicitly temporary bridge.

```commit
refactor(code-design): introduce immutable project state
```

## Phase 2: Give operations explicit inputs

**Execution mode: subagent-driven.**

Advances: ["state-operation-separation", "explicit-operation-dependencies", "behavior-preserved"]
Completes: ["operation-functions"]

### Task 2.1: Convert state-independent and tree-reading receivers
Kind: batch
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0296:proportional-operations"]
Paths: ["glob:internal/project/*.go"]
Representative: Layout, rendering, output planning, config-reference, topic, plan, ADR-numbering, pitfall, list, scaffold, and advisory behavior becomes private package functions that receive the private project state plus the existing concrete tree input required by that operation.
Edge: Derive the ADR corpus, topic corpus, pitfall corpus, effective skills, plans, and output plan exactly once per operation and thread them to consumers; do not create a universal operation context or pre-empt Publisher, RepositoryChecker, CurrentStateCoordinator, or command-operation types.
Post-check: Run the complete `internal/project` test package and the project state-ownership contract; require byte-identical rendered fixtures and diagnostics, exactly one derivation producer per operation, and a source contract whose only temporary `Project` receiver methods are an explicit forwarding allowlist scheduled for deletion in Phase 3.

Convert cohesive helpers and operations to private direct functions or narrowly private operation
helpers behind the frozen facade. Export no replacement function before its first outside-package
production consumer in Phase 3. Reuse `ProjectTreeReader`; add no provider-owned or test-only
interface. Keep tests with the behavior they prove and replace field mutation fixtures with
constructor or explicit input fixtures.

### Task 2.2: Extract repository and staged-universe dependencies
Kind: batch
Applying: ["0296:dependency-direction", "0296:boundary-values", "0296:proportional-operations"]
Paths: ["internal/project/project.go", "internal/project/currentstate.go", "internal/project/contextstate.go", "internal/project/commitpolicy.go", "internal/project/staged_drift.go", "internal/project/output_plan.go", "internal/project/render.go", "internal/project/topics.go", "glob:internal/project/*_test.go"]
Representative: Working, index, HEAD, branch, commit-policy, context, and staged-drift operations receive the concrete semantic Git handle; render and snapshot operations receive the selected `ProjectTreeReader` and config tree explicitly.
Edge: Replace `openRootProject` with operation-local staged composition that never invokes Loader or reads working-tree configuration. Preserve the `ContextState` value seam into `internal/contextq` and preserve the forbidden `internal/project` to `internal/contextq` edge.
Post-check: Run focused staged, current-state, context, commit-policy, render, and output-plan tests plus `TestRepositoryLayerDirection`; require staged fixtures to remain independent of working-tree bytes, all Git absence/refusal diagnostics to remain exact, and no `ProjectState` field or accessor to expose a repository or reader.

Move mechanism selection to the operation boundary without changing current-state or repository-check
ownership. Compatibility formats, migration readers, and ADR-0297 keep/defer paths are untouched.

### Phase close

Close with every operation implemented through explicit state and mechanism inputs, with the old
receiver surface only as a bounded forwarding bridge and no changed command output.

```commit
refactor(code-design): make project operation inputs explicit
```

## Phase 3: Remove the broad receiver from production composition

**Execution mode: subagent-driven.**

Completes: ["state-operation-separation", "explicit-operation-dependencies", "behavior-preserved", "compatibility-boundary-preserved"]

### Task 3.1: Migrate command and context composition
Kind: batch
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:proportional-operations"]
Paths: ["glob:cmd/awf/*.go", "glob:internal/contextq/*.go", "internal/evals/fixture_test.go", "internal/evals/plan_flexibility_test.go"]
Representative: The state type and selected operation functions are exported with these first command consumers; command handlers compose Loader, `ProjectState`, the concrete Git repository, and the one selected operation function, while injected test dependencies model the operation result rather than a god receiver.
Edge: Preserve command parsing, result rendering, stream choice, error identity, exit mapping, capability-plan ordering, single-load behavior, and all exact human output. Do not introduce RF-006 command-operation structs or move policy into `cmd/awf`.
Post-check: Run `go test ./cmd/awf ./internal/contextq ./internal/evals`; require all command and context fixtures to pass and a production-source search to find no `*project.Project`, direct mutable state field access, or new caller of `project.Open`.

Migrate every production caller to the state and operation APIs. `internal/contextq` continues to
consume only `project.ContextState`; project never imports contextq.

### Task 3.2: Delete the facade and prove the final boundary
Kind: batch
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values"]
Paths: ["glob:internal/project/*.go", "internal/config/config.go", "glob:internal/config/*_test.go", "glob:internal/project/*_test.go", ".awf/docs/glossary.yaml", ".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md", "docs/architecture.md", "docs/glossary.md", ".awf/awf.lock"]
Representative: `project.Open` remains a function returning validated `ProjectState` for authorized compatibility callers, but no `Project` type alias, compatibility facade, operation receiver, or mutable public state field remains.
Edge: Future Publisher, RepositoryChecker, CurrentStateCoordinator, and focused command-operation extraction stays recorded as residual RF-003 through RF-006 work. Do not rename those future owners into existence or relocate their policies now.
Post-check: Run the full package tests, `./x render`, and `./x check`; require source contracts to prove that `ProjectState` has fact accessors only, production contains no `type Project`, no operation method on `ProjectState`, no `*project.Project`, no hidden Git/tree-reader state, and no forbidden import edge, while generated architecture and glossary prose describe the implemented boundary without claiming RF-003 through RF-006 complete.

Remove every forwarding receiver and migrate tests to the real production seams. Update authored
architecture and glossary sources, render their outputs, and inspect the Loader, state, operation,
and future-owner descriptions for semantic fidelity.

### Phase close

Close with a reader able to distinguish immutable loaded state from operation functions, every
operation dependency explicit, behavior and CLI output unchanged, and future extraction debt named
without parallel implementation.

```commit
refactor(code-design): separate project state from operations
```

## Definition of done

- `dod: immutable-project-state` Loader constructs validated `ProjectState` facts that remain immutable after construction and expose no mutable config, catalog, map, slice, repository, reader, or filesystem alias.
- `dod: operation-functions` Project behavior is invoked through operation-owned functions with explicit concrete dependencies rather than one broad receiver or universal dependency bag.
- `dod: state-operation-separation` Production contains no broad `Project` type or operation methods on `ProjectState`; the state type says where and what the project is, while function ownership says what an operation does.
- `dod: explicit-operation-dependencies` Git, snapshot, config-tree, rendering-tree, and filesystem inputs are selected at composition and passed only to operations that use them; Loader remains the validated state constructor and project never imports contextq.
- `dod: behavior-preserved` Existing tests, generated bytes, CLI output, error identity, ordering, working/staged universe isolation, `./x check`, and the full 100% coverage gate remain green.
- `dod: compatibility-boundary-preserved` `project.Open` remains only as the authorized no-new-caller compatibility opener, and RF-008B/RF-014B migration, parser, audit-history, punctuation, lock-provenance, effort, bridge, and cutover compatibility is unchanged.

## Notes

Apply the plan-flexibility rule above when recording deviations. Delegated owners report material cross-owner revisions rather than editing the plan; the parent supplies the report to phase review and reconciles required plan changes with findings in one focused post-review settlement commit before checkpointing or later execution. Record spike answers, follow-ups, and findings surfaced during implementation.

Grounding confirmed that ADR-0296 resolves the architecture and no new ADR is required. `ContextState`
still exposes reference-shaped values and remains separate pre-existing boundary debt unless RF-002
would create a new alias through it. RF-003 retains publication and output orchestration; RF-004
retains repository-check aggregation and checker ownership; RF-005 retains current-state
coordination; RF-006 retains command-use-case extraction and final command-handler reduction.

Plan review required the temporary bridge to remain exported because current command signatures use
`*project.Project`; the plan now freezes its callers and methods, keeps replacement state and
functions private until their first outside-package production consumers, and deletes the bridge in
Phase 3. The review requests to restate the workflow-owned phase gate and terminally freeze the plan
inside implementation were not applied: phase execution already mandates the full gate, and the
plan remains Proposed until the deferred post-assurance integration transaction.

Phase 1 review closed config facts to YAML-decoded semantic data shapes. `NewFacts` rejects an
injected non-semantic value instead of retaining an arbitrary Go reference or operation mechanism;
ordinary parsed configuration and its grammar are unchanged.
