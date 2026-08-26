---
format: plan-v2
date: 2026-08-26
adrs: [prepare-ordinary-context-without-full-rendering]
status: Proposed
---
# Plan: Speed Up Ordinary Context Queries

## Goal

Make ordinary explicit `awf context <path>...` avoid whole-repository byte capture, full output rendering, and eager impact projection while preserving successful output bytes and the current practical operation-scoped consistency model.

Do not optimize or alter staged, range-selected, or uncovered context; add persistent caching, mutation detection, or retry; transfer semantic policy out of Git, snapshot, Publisher, current-state coordination, or context query owners; or retain unrelated rendering failures as ordinary-context validation.

## Architecture summary

Application coordination selects a focused route only for ordinary explicit context. Git and snapshot ownership capture a type-distinct operation view with complete path and mode inventory plus immutable bytes selected for the answer. Current-state coordination assembles one neutral context input from that view. Publisher supplies its ADR, topic, plan, and output-declaration projections without rendering output bytes or running unrelated output validations. `internal/contextq` remains the classification and projection owner, computes exact impacts on demand, and expands directory descendants through sorted prefix ranges. Staged, range-selected, and uncovered operations retain the existing complete preparation path.

Successful ordinary output is byte-identical to the existing path. Ordinary context validates every input needed for its answer, including configured marker sources, plan links, authority, declarations, and requested artifact evidence, but an unrelated render failure does not block it. Inventory capture and selected reads remain sequential and operation-owned without stronger atomicity machinery.

**Plan flexibility.**

The protected-contract rule in the workflow document governs what a plan may not change. The plan records the best known route at authoring time, not a binding implementation choreography. A commit-capable owner may merge, split, reorder, add, remove, or replace recorded route detail while the protected contract holds. A path omitted from the plan is not alone a reason to stop, and a stale listed path need not be touched. Reapproval is required only when the protected contract would change or an unresolved material decision appears.

Reconcile a Proposed plan only when another phase or reviewer could rely on stale material instructions. Inconsequential and independently local edits require no deviation record. A delegated owner reports material cross-owner revisions for parent reconciliation. A helper remains confined to its assigned paths and gains no scope, commit, review, checkpoint, handoff, or outcome authority from route flexibility.

## Phase 1: Make context projection demand-driven

**Execution mode: subagent-driven.**

Advances: ["ordinary-output-parity", "performance-evidence"]
Completes: ["demand-projection"]

### Task 1.1: Replace eager impact construction with requested-path projection
Applying: ["prepare-ordinary-context-without-full-rendering:demand-driven-context-projection"]
Paths: ["internal/contextq/context.go", "internal/contextq/context_paths.go", "internal/contextq/context_paths_test.go", "internal/contextq/context_projection_test.go", "internal/contextq/context_artifacts_test.go", "internal/contextq/context_adr_test.go", "internal/contextq/render_test.go"]
Post-check: Run the focused contextq tests and comparative exact-file and directory benchmarks; all existing render fixtures must remain byte-identical, exact requests must project only requested impacts, and directory membership must terminate at the same sorted descendant set and grouping as the baseline.

Starting dependency: `internal/contextq` receives the existing complete neutral input and ordinary command routing remains unchanged.

Build one query-local metadata index from the complete path inventory. Use exact lookup for file requests and sorted prefix ranges for directory descendants. Preserve original request spelling and order, classification precedence, nested-adopter boundaries, artifact-sensitive group keys, relationship sources, facet behavior, and deterministic rendering. Do not move classification or projection policy into coordination and do not add a cache beyond one query.

Add focused benchmarks for exact-file and directory projection across representative repository sizes. Record the pre-change benchmark in phase evidence before retaining the optimized implementation. Prefer existing query inputs and real result assembly over a test-only production seam.

### Phase close

The phase is green with demand-driven query work and unchanged output over the existing complete input.

```commit
perf(tooling): project context impacts on demand
```

## Phase 2: Prepare focused Publisher semantics

**Execution mode: subagent-driven.**

Advances: ["ordinary-output-parity", "nonordinary-route-parity", "performance-evidence"]
Completes: ["focused-semantic-preparation"]

### Task 2.1: Establish focused-validation and routing oracles
Applying: ["prepare-ordinary-context-without-full-rendering:context-semantic-declaration-projection", "prepare-ordinary-context-without-full-rendering:focused-ordinary-context-universe"]
Paths: ["internal/contextop/context.go", "internal/contextop/context_test.go", "internal/contextop/context_preparation_test.go", "internal/publisher/inputs.go", "internal/publisher/inputs_test.go", "internal/publisher/output_plan.go", "internal/publisher/output_plan_test.go"]

Starting dependency: Phase 1 has removed eager query work, while ordinary context still captures a complete tree and calls full Publisher preparation.

Before changing production behavior, add an ordinary explicit fixture that fails only because full Publisher preparation reaches an unrelated render or output validation, and observe it fail for that reason. Retain it green with the focused path. Add route-sensitive fixtures proving staged, range-selected, and uncovered operations still invoke complete preparation, and prove malformed authority, marker, plan-link, declaration, and requested-artifact inputs needed by the answer still fail.

### Task 2.2: Add and consume a Publisher-owned context projection
Applying: ["prepare-ordinary-context-without-full-rendering:context-semantic-declaration-projection", "prepare-ordinary-context-without-full-rendering:focused-ordinary-context-universe"]
Paths: ["internal/publisher/inputs.go", "internal/publisher/output_plan.go", "internal/publisher/generated_semantics.go", "internal/publisher/inputs_test.go", "internal/testsupport/publishing_ownership_test.go", "internal/currentstatecoord/context.go", "internal/contextop/context.go", "internal/contextop/context_preparation_test.go"]

Extract shared semantic derivation so Publisher can return defensive ADR, topic, plan, and output-declaration values for context without constructing rendered output nodes, generated-check projections, vocabulary projections, or unrelated validations. Reuse Publisher's existing corpus and declaration policy rather than copying it into coordination. Keep the existing full `Prepare` behavior unchanged for all other consumers.

Route only ordinary explicit operations through focused Publisher preparation, initially over the current complete working tree. Preserve one selected config, lock, authority, marker, plan, and declaration universe. Keep static fallback and delivery unchanged.

### Task 2.3: Apply the single-plan authority update
Applying: ["prepare-ordinary-context-without-full-rendering:context-semantic-declaration-projection"]
Paths: [".awf/topics/parts/rendering/project-output-plan/current-state.md", "docs/decisions/prepare-ordinary-context-without-full-rendering.md", "docs/decisions/INDEX.md", "docs/topics/rendering/project-output-plan.md", ".awf/awf.lock"]

Apply `update rendering/project-output-plan:check-report-single-plan` in the same transaction as the focused Publisher consumer. Preserve one immutable output plan for operations that need it while stating that ordinary explicit context consumes Publisher's focused semantic and declaration projection instead. Retain the claim's existing Origin and append the pending ADR as Revised-by, render through authored sources, regenerate the status index in the same transaction, and keep the ADR Implementing after the application batch.

### Phase close

The phase is green with ordinary context independent of full rendering, required context validations intact, and every nonordinary route unchanged.

```commit
perf(code-design): prepare focused context semantics
```

## Phase 3: Capture only context-required bytes

**Execution mode: subagent-driven.**

Advances: ["ordinary-output-parity", "performance-evidence"]
Completes: ["selective-context-input", "nonordinary-route-parity", "documented-authority"]

### Task 3.1: Introduce the live context inventory and selection model
Applying: ["prepare-ordinary-context-without-full-rendering:focused-ordinary-context-universe"]
Paths: ["internal/git/handle.go", "internal/git/git_test.go", "internal/snapshot/snapshot.go", "internal/snapshot/selection.go", "internal/snapshot/working.go", "internal/snapshot/snapshot_test.go", "internal/snapshot/selection_test.go", "internal/snapshot/working_test.go"]

Starting dependency: Phase 2 supplies focused Publisher semantics but ordinary context still materializes a complete byte-bearing working tree.

Add a type-distinct live context representation whose complete inventory distinguishes regular, executable, and symlink entries from absent paths while selected immutable content remains explicit. Git owns enumeration of tracked-present and visible untracked entries, deletion and ignore behavior, modes, and cancellation; snapshot owns copied sorted inventory and selected bytes. Do not overload `snapshot.Tree` or historical `snapshot.Selection` with ambiguous absence semantics.

Cover tracked, untracked, deleted, ignored, executable, symlink, nested-repository, and canceled enumeration. Symlink targets required for inert lexical classification are selected evidence, not an excuse to read every regular file.

### Task 3.2: Assemble one focused ordinary-context universe
Applying: ["prepare-ordinary-context-without-full-rendering:focused-ordinary-context-universe", "prepare-ordinary-context-without-full-rendering:context-semantic-declaration-projection"]
Paths: ["internal/currentstatecoord/context.go", "internal/currentstatecoord/currentstate.go", "internal/currentstatecoord/currentstate_owner_test.go", "internal/contextinput/input.go", "internal/contextinput/input_test.go", "internal/contextop/context.go", "internal/contextop/context_preparation_test.go", "internal/contextq/context.go", "internal/contextq/context_artifacts.go", "internal/contextq/context_artifacts_test.go"]

Select bytes from the same operation inventory for configuration and lock, ADR and topic authority, domain sidecars, plans and reverse links, configured marker sources outside nested adopters, Publisher declaration inputs, requested exact artifacts, and artifact-relevant descendants of requested directories. Translate that view into neutral context input without making unread content mean path absence. Context coordination owns selection and completion, while Publisher and query owners retain semantic policy.

Add read-spy and differential fixtures that compare focused and complete ordinary output across repeated and mixed exact requests, directories and root, missing, outside, ignored, generated, nested-adopter, symlink, ADR, every facet, marker-heavy inputs, plan references, and artifact drift evidence. Prove unrelated regular payload bytes are not read, required bytes are read, repeated calls observe fresh state, and caller mutation cannot alter captured input. Keep staged, range-selected, and uncovered preparation on complete trees.

### Task 3.3: Apply context authority and architecture updates
Applying: ["prepare-ordinary-context-without-full-rendering:focused-ordinary-context-universe", "prepare-ordinary-context-without-full-rendering:demand-driven-context-projection"]
Paths: [".awf/topics/parts/tooling/context-and-topic/current-state.md", ".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md", "docs/decisions/prepare-ordinary-context-without-full-rendering.md", "docs/topics/tooling/context-and-topic.md", "docs/architecture.md", ".awf/awf.lock"]

Apply `update tooling/context-and-topic:context-query-boundary` and `update tooling/context-and-topic:context-read-only` with the focused capture and query implementation. Preserve their existing Origin, append the pending ADR as Revised-by, describe the distinct inventory and selected-byte neutral boundary, and state that only ordinary explicit context uses it. Keep successful byte compatibility, selected-universe isolation, read-only behavior, and complete nonordinary routes explicit. Reconcile architecture sources with the implemented dependency direction and render all generated outputs.

### Phase close

The phase is green with complete metadata, selected bytes, focused ordinary routing, current authority, and no alternate semantic owner.

```commit
perf(code-design): select ordinary context inputs
```

## Phase 4: Lock end-to-end performance evidence

**Execution mode: subagent-driven.**

Completes: ["ordinary-output-parity", "performance-evidence"]

### Task 4.1: Benchmark and stress the completed ordinary path
Applying: ["prepare-ordinary-context-without-full-rendering:focused-ordinary-context-universe", "prepare-ordinary-context-without-full-rendering:context-semantic-declaration-projection", "prepare-ordinary-context-without-full-rendering:demand-driven-context-projection"]
Paths: ["internal/contextop/context_benchmark_test.go", "internal/contextop/context_test.go", "internal/contextop/context_preparation_test.go", "internal/contextq/render_test.go"]

Starting dependency: Phases 1 through 3 implement the complete focused path and apply every ADR operation.

Add a representative end-to-end benchmark or deterministic harness covering exact files, directories, marker-heavy inputs, artifact-heavy inputs, and increasing repository sizes without a persistent cache. Report latency, allocations, files read, and bytes read for the focused route against the retained complete preparation baseline. Record the comparison in implementation evidence, not as a brittle fixed timing assertion.

Run the differential matrix and focused package tests under ordinary and race-enabled execution where practical. Remove temporary probes and tuning seams. If evidence shows a remaining dominant full-render or unrelated-byte path, correct it only within the approved ownership boundary; return to brainstorming rather than adding a cache, stronger consistency protocol, or changed nonordinary behavior.

### Phase close

The phase is green with reproducible evidence that ordinary context eliminated the diagnosed full-render, unrelated-byte, and eager-projection work while preserving protected behavior.

```commit
test(tooling): verify ordinary context fast path
```

## Definition of done

- `dod: demand-projection` Exact requests construct only requested impacts, directory requests use indexed descendant ranges, and all classification, grouping, relationship, facet, and rendering results remain deterministic.
- `dod: focused-semantic-preparation` Ordinary explicit context obtains Publisher-owned ADR, topic, plan, and declaration projections without rendering output bytes or running unrelated output validation; required answer inputs still validate.
- `dod: selective-context-input` Ordinary explicit context carries complete live path and mode inventory plus only required immutable bytes, and deterministic read-spy evidence proves unrelated regular payloads are skipped.
- `dod: nonordinary-route-parity` Staged, range-selected, and uncovered context retain their complete preparation routes and existing behavior.
- `dod: ordinary-output-parity` Differential fixtures cover exact, directory, mixed, exceptional, authority, marker, plan-reference, artifact, and facet cases with byte-identical successful ordinary output.
- `dod: performance-evidence` Reproducible comparative evidence reports latency, allocations, files read, and bytes read and demonstrates removal of full output rendering, unrelated regular-file reads, and eager whole-tree impact projection from ordinary exact context.
- `dod: documented-authority` All three ADR operations are applied with current-state and architecture sources describing the implemented focused route, generated outputs and lock are clean, and the ADR and plan remain nonterminal for deferred assurance and closure.

## Notes

Apply the plan-flexibility rule above when recording deviations. Delegated owners report material cross-owner revisions rather than editing the plan; the parent supplies the report to phase review and reconciles required plan changes with findings in one focused post-review settlement commit before checkpointing or later execution. Record spike answers, follow-ups, benchmark comparisons, and findings surfaced during implementation.
