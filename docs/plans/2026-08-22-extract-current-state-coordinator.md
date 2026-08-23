---
format: plan-v2
date: 2026-08-22
adrs: [0296, 0299, 0300, move-context-query-input-below-application-coordination]
status: Proposed
---
# Plan: Extract Current-State Coordinator

## Goal

Establish a focused application-level current-state coordinator that selects immutable operation universes, prepares ADR, topic, plan, and current-state inputs, coordinates transition checks and authority queries, and returns semantic results while the existing domain, snapshot, Git, Publisher, RepositoryChecker, and command owners retain their policies. Make `internal/project` orchestration smaller, reuse same-tree Publisher semantics where one operation already needs Publisher, and preserve current-state authority, history separation, error identity, compatibility projections, presentation, and operation behavior.

Do not replace `Project` with another dependency bag or service locator, collapse distinct working, staged, merge-parent, numbering-before/after, or history universes, transfer Publisher or domain policy, remove compatibility, perform RF-006 command-use-case cleanup or RF-007 test cleanup, edit the audit program or adopters, or perform numbering, integration, terminal artifact closure, topology removal, retrospective, or effort finish.

## Architecture summary

`internal/currentstatecoord` is a focused application package whose direct operation functions and small immutable operation-specific values coordinate ADR, topic, plan, and current-state authority. It selects or receives the exact semantic snapshot and Git capabilities required by one operation, invokes the existing `internal/adr`, `internal/currentstate`, `internal/topic`, `internal/plan`, and `internal/plancheck` owners, and returns their semantic results without implementing their parsing or lifecycle rules. It does not expose a long-lived coordinator object, universal dependency interface, mutable cache, or service-locator carrier.

A neutral immutable context-input owner below application coordination carries the state needed by `internal/contextq`; this removes the `contextq` dependency on `internal/project` without making a lower package import the coordinator. Publisher remains the sole output-plan and publication-preparation owner. Where a context operation already prepares Publisher from one selected tree, coordinator completion consumes Publisher's defensive ADR, topic, plan, and plan projections instead of reparsing the same tree. Independent working report, working current-state, stage-0 index, staged state, staged Publisher drift, first-parent, merge-parent, numbering-before/after, and historical operations remain independently selected and never share persistent derived state.

Commands may compose and invoke coordinator functions directly where RF-005 ownership requires rewiring, but command parsing, rendering, streams, and exit mapping stay in `cmd/awf`; broader command use-case extraction remains RF-006. `internal/project` retains Loader and unrelated operations plus only caller-supported bounded compatibility adapters. ADR-0296 authorizes this ownership and direction. ADR-0299 constrains Publisher preparation, and ADR-0300 constrains typed current-state result partitions and RepositoryChecker consumption; neither owner is reversed.

**Plan flexibility.**

The protected-contract rule in the workflow document governs what a plan may not change. The plan records the best known route at authoring time, not a binding implementation choreography. A commit-capable owner may merge, split, reorder, add, remove, or replace recorded route detail while the protected contract holds. A path omitted from the plan is not alone a reason to stop, and a stale listed path need not be touched. Reapproval is required only when the protected contract would change or an unresolved material decision appears.

Reconcile a Proposed plan only when another phase or reviewer could rely on stale material instructions. Inconsequential and independently local edits require no deviation record. A delegated owner reports material cross-owner revisions for parent reconciliation. A helper remains confined to its assigned paths and gains no scope, commit, review, checkpoint, handoff, or outcome authority from route flexibility.

## Phase 1: Establish current-state coordination ownership

**Execution mode: inline.**

Advances: ["coordinator-owner", "universe-parity", "result-parity", "ownership-proofs"]
Completes: ["core-transition-coordination"]

### Task 1.1: Lock transition, universe, and result behavior before movement
Applying: ["0300:owners-classify-results", "0300:repository-checker-aggregates-results"]
Context: ["0296:boundary-values"]
Paths: ["internal/project/currentstate_test.go", "internal/project/currentstate_plan_result_test.go", "internal/project/currentstate_compat_test.go", "internal/project/staged_test.go", "cmd/awf/checkrepo_test.go", "cmd/awf/check_test.go", "cmd/awf/commitgate_test.go"]

Strengthen the existing oracles before moving coordination. Cover working snapshot and filesystem fallback identity; staged HEAD-before and index-after separation; authored versus merge-aggregate selection; dirty-working-byte isolation; distinct first-parent and every `MERGE_HEAD` parent; configuration and lock selection; static, pair, merge, qualification, coverage, fan-out, and plan-artifact results; `CurrentResult`, `PlanArtifactResult`, `OwnerResult`, and legacy projections; provisional Information; typed plan-warning deduplication; operational-error wrapping; Error-only exit behavior; partial output suppression; and deterministic semantic operation and category order. Preserve exact presentation bytes, contents, category membership, and multiplicity while treating relative item order inside one Warning list as unprotected.

Add mutation-sensitive fixtures for universe swapping, accidental working-tree reuse during staged operations, compatibility-slice routing, and result-partition loss. Observe each new oracle fail for the intended mutation before retaining it green.

### Task 1.2: Create the focused coordinator and move core current-state checks
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0296:proportional-operations", "0300:owners-classify-results", "0300:repository-checker-aggregates-results"]
Paths: ["glob:internal/currentstatecoord/**", "internal/project/currentstate.go", "internal/project/currentstate_test.go", "internal/project/currentstate_plan_result_test.go", "internal/project/currentstate_compat_test.go", "internal/project/staged_plan.go", "internal/project/staged_test.go"]
Post-check: Run the coordinator, current-state domain, project compatibility, and staged transition tests; all working and staged fixtures must terminate with unchanged semantic findings, typed partitions, compatibility projections, error identities, and universe selection, and no moved transition-coordination implementation may remain in project.

Create `internal/currentstatecoord` with a one-sentence package owner and direct functions or small operation-specific immutable values. Move working current-state loading and checking, staged before/after transition preparation, plan-artifact adaptation, and aggregate result construction. Reserve stale-merge authority loading and qualification for Phase 3 so this phase does not create a path that a later phase must replace. Keep parsing, corpus identity, static and pair checks, merge rules, qualification rules, topic semantics, plan diagnostics, snapshot tree construction, and Git semantics in their existing lower owners.

Every operation selects or receives its own immutable tree, config, lock, corpus, and result values. No persistent cache, reset protocol, provider-owned universal interface, test-only production seam, or all-purpose coordinator struct is allowed. Exports must be documented and earned by the production callers introduced in the same transaction.

### Task 1.3: Rewire real consumers and confine project compatibility
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:proportional-operations", "0300:repository-checker-aggregates-results"]
Paths: ["cmd/awf/checkrepo.go", "cmd/awf/checkstaged.go", "cmd/awf/checkrepo_test.go", "cmd/awf/check_test.go", "internal/project/export_test.go", "internal/testsupport/deps_test.go", "internal/testsupport/check_result_ownership_test.go"]

Compose the new coordinator at the existing command composition seams without moving parsing, presentation, stream choice, or exit mapping. Preserve RepositoryChecker as a consumer of completed typed results only. Retain project wrappers only where a complete caller and support census proves a live compatibility need; wrappers delegate directly and contain no policy or alternate preparation path. Remove private dead orchestration after callers move.

Extend dependency and result-route censuses so lower domain, state, snapshot, Git, Publisher, RepositoryChecker, and project-state packages cannot import the coordinator; the coordinator cannot import commands or context-query presentation; individual check owners remain independent; and every production current-state route reaches the coordinator exactly once.

### Phase close

The phase is green with core working, staged, and merge current-state coordination owned by the focused package, real consumers rewired, and all protected universes and result contracts unchanged.

```commit
refactor(code-design): establish current-state coordinator
```

## Phase 2: Extract context and authority-query preparation

**Execution mode: inline.**

Advances: ["coordinator-owner", "universe-parity", "ownership-proofs"]
Completes: ["context-boundary", "context-query-coordination", "parse-reuse"]

### Task 2.1: Introduce a neutral immutable context input
Applying: ["0296:dependency-direction", "0296:boundary-values", "0296:proportional-operations"]
Paths: ["glob:internal/contextinput/**", "internal/project/contextstate.go", "internal/contextq/context.go", "internal/contextq/context_test.go", "internal/contextq/context_projection.go", "internal/testsupport/deps_test.go"]
Post-check: Run context-input, context-query, project compatibility, and dependency-direction tests; `internal/contextq` must consume only the neutral immutable input, import neither project nor the coordinator, and mutation of caller-owned maps or slices must not alter a constructed input or query result.

Create the smallest neutral owner for immutable context-query input below application coordination. Move only the value representation and defensive construction needed by both the coordinator and `internal/contextq`; keep query selection, relationship projection, authority rendering inputs, and coverage semantics with `contextq` and `topic`. Remove the `contextq` dependency on project. Do not move a Project carrier, Publisher preparation, snapshot selection, or coordinator behavior into the neutral package.

### Task 2.2: Complete context operations from Publisher semantics without same-tree reparsing
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0299:publisher-constructs-operation-plan", "0299:neutral-plan-values-below-coordination"]
Paths: ["internal/currentstatecoord/context.go", "internal/currentstatecoord/context_test.go", "internal/project/contextstate.go", "internal/project/plan_context.go", "cmd/awf/publishing.go", "cmd/awf/context.go", "cmd/awf/context_test.go", "cmd/awf/publishing_test.go", "internal/publisher/inputs.go", "internal/publisher/inputs_test.go"]

Split context coordination into operation-specific selection and completion values. Working and staged context each select one immutable tree and its own config and lock, Publisher prepares its output plan and defensive ADR, topic, and plan semantics from that exact tree, and coordinator completion derives current-state and context inputs from those semantics without reparsing the same corpus. Publisher remains the derivation owner for Publisher-participating operations and the coordinator never reconstructs output planning policy.

Keep `--staged` authority distinct from working authority and keep range selection limited to queried paths. A working context operation, staged context operation, working repository check, staged state check, and staged drift operation remain independent even when they point at equal bytes. Do not introduce cross-call caches or share mutable preparation across those operations.

### Task 2.3: Move topic query and plan-context derivation behind focused operations
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0296:proportional-operations"]
Paths: ["internal/currentstatecoord/topic.go", "internal/currentstatecoord/topic_test.go", "internal/currentstatecoord/context.go", "internal/project/topic_query.go", "internal/project/topic_query_test.go", "internal/project/plan_context.go", "cmd/awf/topic.go", "cmd/awf/topic_test.go"]

Move application preparation for topic and claim queries plus plan-to-authority context into direct coordinator operations. The coordinator loads the relevant working authority universe, invokes current-state validation, and supplies semantic corpora and safe path projections to `internal/topic`; topic parsing, marker, domain, history, references, applicability, and coverage rules stay in `internal/topic`. Commands continue to validate flags, invoke, render, and map errors.

### Task 2.4: Prove parse cardinality, freshness, and universe isolation
Applying: ["0299:publisher-constructs-operation-plan", "0299:neutral-plan-values-below-coordination"]
Context: ["0296:boundary-values"]
Paths: ["internal/currentstatecoord/context_test.go", "internal/contextq/context_test.go", "internal/publisher/inputs_test.go", "internal/testsupport/deps_test.go"]

Add mutation-sensitive production-path evidence that each Publisher-participating context operation parses each selected ADR, topic, and plan corpus once and threads the resulting immutable semantics to all participants. Instrument the real tree-reader or parser boundary rather than adding a test-only production seam. Prove repeated operations observe fresh repository state; after construction, caller mutation cannot alias config, maps, slices, corpora, paths, or results; Core never gains Full-only authority; and working, index, HEAD, merge-parent, and history inputs cannot contaminate one another.

Do not assert one parse across intentionally distinct operations or before/after universes. The oracle names its input tree and participating consumers so a future optimization cannot collapse capability universes to satisfy a count.

### Phase close

The phase is green with neutral context input ownership, focused context and topic operations, same-tree Publisher semantic reuse where applicable, and all working, staged, range, and history behavior preserved.

```commit
refactor(code-design): extract current-state context operations
```

## Phase 3: Extract authority transition application operations

**Execution mode: inline.**

Advances: ["coordinator-owner", "universe-parity", "result-parity", "ownership-proofs"]
Completes: ["authority-application-operations"]

### Task 3.1: Move ADR numbering coordination without moving ADR or Publisher policy
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0296:proportional-operations"]
Context: ["0299:publisher-constructs-operation-plan"]
Paths: ["internal/currentstatecoord/number.go", "internal/currentstatecoord/number_test.go", "internal/project/adrnumber.go", "internal/project/adrnumber_test.go", "cmd/awf/adr.go", "cmd/awf/adr_test.go"]

Move the application operation that loads pre-mutation authority, validates pending slugs and declared add-before-revise operations, coordinates ADR renumbering and exact topic provenance substitution, then invokes the existing post-mutation Publisher sync. Keep ADR parsing, corpus identity, lifecycle, pairing, renumbering primitives, topic validation and substitution rules, atomic filesystem behavior, and Publisher plan and publication ownership with their existing owners.

Preserve highest-plus-one and assignment order, canonical provenance ordering, anchored replacement, untouched plan bytes, collision and invalid-operation errors, no-assignment failures before the first rename, accumulated partial assignments after later substitution or Publisher failure, and intentionally distinct pre-mutation and post-mutation universes. Never coalesce those two parses.

### Task 3.2: Move plan-read authority preparation without moving plan semantics
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0296:proportional-operations"]
Paths: ["internal/currentstatecoord/plan.go", "internal/currentstatecoord/plan_test.go", "internal/project/plan_read.go", "internal/project/plan_read_test.go", "cmd/awf/read.go", "cmd/awf/read_test.go", "glob:internal/plan/**"]
Post-check: Run coordinator plan-operation, plan-domain, project compatibility, and read-command tests; every plan-v2 scope, linked-ADR validation, task projection, typed error, and output byte must match the baseline, and no plan parsing or projection policy may be duplicated in the coordinator.

Move only application preparation for selecting one working-filesystem plan and loading the current ADR corpus required by plan-v2 projection. `internal/plan` retains parsing, scope validation, closure, projection, and presentation semantics. Preserve clean stdout on failure, exact projection bytes, path and phase/task errors, and the independent operation universe.

### Task 3.3: Confine commit-authorization coordination to authority loading and qualification
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0296:proportional-operations"]
Paths: ["internal/currentstatecoord/commit.go", "internal/currentstatecoord/commit_test.go", "internal/project/currentstate.go", "cmd/awf/commitgate.go", "cmd/awf/commitgate_test.go", "glob:internal/commitpolicy/**"]
Post-check: Run coordinator commit-authority, commitpolicy, project compatibility, and commitgate command tests across ordinary, initial, merge, stale, malformed-message, and operational-failure cases; commitpolicy must remain the sole authorization-policy owner, every parent universe must remain distinct, and command result bytes and exit behavior must be unchanged.

Move stale-merge authority snapshot loading and current-state qualification coordination, not commit-message policy. Preserve result-index selection, first-parent HEAD or empty-tree selection, every `MERGE_HEAD` parent, cleaned-message qualification, `commitmsg.SyntaxError` identity through `errors.As`, wrapped Git and snapshot causes, condition/category fields, the three changed axes, ordered next actions, and command-owned rendering and exit mapping. `internal/commitpolicy` remains the sole owner of authorization decisions.

### Task 3.4: Remove duplicate project orchestration and prove the final application boundary
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0296:proportional-operations"]
Paths: ["internal/project/contextstate.go", "internal/project/currentstate.go", "internal/project/staged_plan.go", "internal/project/topic_query.go", "internal/project/adrnumber.go", "internal/project/plan_context.go", "internal/project/plan_read.go", "internal/project/export_test.go", "internal/project/stateownership_test.go", "internal/testsupport/deps_test.go", "internal/testsupport/check_result_ownership_test.go"]

Run a complete production caller census and remove replaced private helpers, duplicate parsers, and alternate preparation paths. Keep only bounded supported project adapters that delegate directly to coordinator or lower owners and cannot select a different universe, reparse inputs, mutate results, or retain caches. Do not perform unrelated Project decomposition, compatibility deletion, command cleanup, or residual test reorganization.

Strengthen exact ownership and dependency-direction proofs for production callers, coordinator imports, remaining project adapters, operation-owned derivation, exported first consumers, and absence of lower-owner reversals. Temporarily mutate each route, reverse import, cache/alias, and parse-cardinality case to prove the relevant census or behavior oracle fails, then restore it before the gate.

### Phase close

The phase is green with numbering, plan-read preparation, and stale-merge authority coordination behind focused application operations, domain and commit-policy owners intact, and project retaining no competing coordinator.

```commit
refactor(code-design): extract current-state authority operations
```

## Phase 4: Publish and verify the implemented boundary

**Execution mode: inline.**

Completes: ["coordinator-owner", "universe-parity", "result-parity", "ownership-proofs", "documented-boundary"]

### Task 4.1: Reconcile current-state claims and architecture through authored sources
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0296:proportional-operations", "0299:publisher-constructs-operation-plan", "0299:neutral-plan-values-below-coordination", "0300:owners-classify-results", "0300:repository-checker-aggregates-results", "move-context-query-input-below-application-coordination:neutral-context-query-input"]
Paths: [".awf/docs/parts/architecture/components.md", ".awf/docs/parts/architecture/data-flow.md", ".awf/topics/metadata/rendering/project-output-plan.yaml", ".awf/topics/parts/rendering/project-output-plan/current-state.md", ".awf/topics/metadata/tooling/context-and-topic.yaml", ".awf/topics/parts/tooling/context-and-topic/current-state.md", ".awf/domains/tooling.yaml", ".awf/domains/parts/tooling/current-state.md", "docs/decisions/move-context-query-input-below-application-coordination.md", "docs/decisions/INDEX.md", "docs/architecture.md", "docs/domains/tooling.md", "glob:docs/topics/**", ".awf/awf.lock"]
Post-check: Run managed context over each new package and changed command path, render, and run `./awf check`; every production and test path must resolve to its narrow owner, generated explanatory documentation and proof locations must name the implemented coordinator and neutral context input while every unchanged active claim remains true, generated outputs must match authored sources, and no rendered file may be edited without its source.

Update the project-output-plan explanatory introduction to describe Publisher semantic reuse by coordinator operations without transferring Publisher ownership or collapsing capability universes. Reconcile authored architecture components, data flow, and the tooling-domain narrative so CurrentStateCoordinator is current rather than future and commit authorization retains its policy owner. Add narrow domain and topic selectors for new production and owner-test paths, and move or extend backing proof markers without changing active claim bodies other than the successor-authorized `context-query-boundary` update. Preserve the supported project context aliases and direct delegation adapters needed to keep the existing context-query, state-ownership, and single-plan claims true, then render generated topics, domains, architecture, selectors, and lock.

Phase review proved the old `context-query-boundary` claim materially false because it still required the superseded project context-state seam. Apply ADR-move-context-query-input-below-application-coordination explicitly by appending its Implementing and Applied events atomically with the claim update, preserving its Origin and Revised-by prefix before appending this ADR. Keep Publisher as corpus owner, retain distinct universes and the bounded staged project adapter, and leave the ADR Implementing for the orchestrator-owned deferred terminal transaction.

### Task 4.2: Complete RF-005 behavioral and structural assurance evidence
Applying: ["0296:dependency-direction", "0296:boundary-values", "0299:publisher-constructs-operation-plan", "0299:neutral-plan-values-below-coordination", "0300:owners-classify-results", "0300:repository-checker-aggregates-results", "move-context-query-input-below-application-coordination:neutral-context-query-input"]
Paths: ["glob:internal/currentstatecoord/**", "glob:internal/contextinput/**", "internal/contextq/boundary_test.go", "internal/project/stateownership_test.go", "internal/testsupport/deps_test.go", "internal/testsupport/check_result_ownership_test.go", "internal/testsupport/publishing_ownership_test.go", "cmd/awf/presentation_boundary_test.go"]
Post-check: Run focused coordinator, context-input, ADR, currentstate, topic, plan, contextq, Publisher, project, commitpolicy, command, and structural-census tests; run `git diff --check`, `./x render`, `./x check`, and the project gate; every check must reach its authorized terminal state with no production dead code, drift, unowned path, reverse dependency, weakened oracle, or unexpected lint finding.

Verify the complete behavior matrix at the final implementation tip: distinct working report, working current-state, stage-0 index, staged state, staged Publisher drift, first-parent, result-index, every merge-parent, numbering-before/after, context range, and history universes; same-tree context parse reuse only where Publisher already participates; fresh repeated operations and defensive ownership; typed current-state results and compatibility projections; exact error identity, partial numbering, presentation bytes, category membership, multiplicity, semantic operation and category order; Error-only exits; and unprotected intra-Warning relative order.

Strengthen the complete-package contextq reverse-import proof and bind `CompleteContext` to the ADR, topic, plan, and declaration projections from the same local Publisher preparation, including mutation-sensitive negative fixtures. Record any authority-determined route deviation and residual debt in Notes before renewed phase review. Leave ADR-0296, ADR-0299, and ADR-0300 Implemented, leave ADR-move-context-query-input-below-application-coordination Implementing after its explicit application, and leave this plan Proposed for the orchestrator-owned deferred terminal transaction. Do not edit the audit program, add changelog text when observable behavior is unchanged, number or integrate anything, remove topology, run retrospective, or finish the effort.

### Phase close

The phase is green with generated authority current, the final coordinator boundary mechanically protected, observable behavior unchanged, and RF-005 ready for terminal implementation assurance and complete-range audits.

```commit
refactor(code-design): publish current-state coordinator boundary
```

## Definition of done

- `dod: coordinator-owner` `internal/currentstatecoord` is the single application owner for ADR, topic, plan, and current-state coordination through direct operation functions and small immutable values, with no service locator, universal dependency bag, persistent cache, or alternate project coordinator.
- `dod: core-transition-coordination` Working current-state checks, staged before/after checks, and plan-artifact classification are coordinated outside project while lower domain and mechanism rules stay with their existing owners.
- `dod: context-boundary` A neutral immutable context input below application coordination replaces project as the context-query carrier, and `internal/contextq` imports neither project nor the coordinator.
- `dod: context-query-coordination` Working and staged context preparation, topic and claim query preparation, and plan-to-authority context derivation are focused coordinator operations with unchanged range, history, coverage, relationship, and presentation behavior.
- `dod: parse-reuse` Within each Publisher-participating context operation, one selected immutable tree yields one Publisher preparation whose defensive ADR, topic, plan, and output-plan semantics reach every participant; intentionally distinct operations and universes remain independently prepared and no persistent cache exists.
- `dod: authority-application-operations` ADR numbering, plan-read authority loading, and commit-authorization authority loading and qualification use focused application operations while ADR, topic, plan, Publisher, and commitpolicy semantics retain one owner.
- `dod: universe-parity` Working report, working current-state, stage-0 index, staged state, staged drift, result index, first parent, every merge parent, numbering before and after, context range, and history universes retain their exact selection and isolation semantics; dirty working bytes never affect staged behavior.
- `dod: result-parity` Typed result partitions, compatibility projections, findings, protected properties, category membership, multiplicity, plan-warning deduplication, Error-only exits, operational failures, error identities, partial numbering results, presentation bytes, and semantic operation and category order match the starting contract; relative item order inside one Warning list is not protected.
- `dod: ownership-proofs` Mutation-sensitive caller, dependency, parse-cardinality, universe, freshness, no-cache, alias, exported-consumer, and result-route proofs fail for representative regressions and pass at the final clean tip.
- `dod: documented-boundary` Authored selectors, architecture, project-output-plan, state-ownership, and context-query authority describe and cover the implemented coordinator and neutral values; generated outputs and lock are clean under the linked successor ADR without any further ADR or audit-program edit.

## Notes

Phase 1 advanced the narrow tooling-domain and `tooling/context-and-topic` selectors for `internal/currentstatecoord/**` from Phase 4. Phase 2 likewise advanced those selectors for `internal/contextinput/**`. The repository gate requires every new production package and owner test to be domain-owned and claim-covered in the same phase that creates it; these ordering changes apply the already planned selectors without changing an active claim body. Phase 4 still owns the explanatory current-state and architecture reconciliation.

Apply the plan-flexibility rule above when recording deviations. Delegated owners report material cross-owner revisions rather than editing the plan; the parent supplies the report to phase review and reconciles required plan changes with findings in one focused post-review settlement commit before checkpointing or later execution. Record spike answers, follow-ups, and findings surfaced during implementation.

Grounding verified the existing operation universes, result and error contracts, same-tree parse duplication, package direction, and current-state documentation pressure at starting commit `cc0e17ed4550dfbf3a3a2bf95027f6062e9df687`. ADR-0296 fully authorizes the extraction. The plan preserves Publisher as the producer of semantic corpora for Publisher-participating context operations, so ADR-0299 needs no amendment. Numbering, plan-read preparation, and stale-merge authority loading are hosted as focused application operations in the same package but do not become domain, Publisher, or commit-authorization policy; this keeps the package's one concern at application current-state coordination rather than creating an all-purpose object.

Initial plan review found two reasoned boundary issues. Stale-merge authority loading and qualification are now reserved for Phase 3 so Phase 1 cannot create overlapping coordinator paths. Phase 4 review later proved that `tooling/context-and-topic:context-query-boundary` still required the superseded project context-state seam despite the bounded staged adapter, so execution followed the required ADR chain. ADR-move-context-query-input-below-application-coordination now authorizes the narrow claim update; its application and the review-requested proof strengthening complete Phase 4 without changing behavior or the RF-005 boundary.

The successor ADR affects the landed context boundary from Phase 1 (`e8ea6379a`, settlement `770113f58`), Phase 2 (`ddc16b3cb`, settlements `143bbc71f` and `65bac5a45`), and unsettled Phase 4 (`3d8878056`); Phase 3 (`9e8a01a61`, settlement `26dabfb62`) is unaffected. Renewed implementation assurance covers the complete range `cc0e17ed4550dfbf3a3a2bf95027f6062e9df687..HEAD` through the Phase 4 settlement tip, focused on the atomic claim application and two strengthened proofs without reopening unrelated settled behavior. Plan freshness review corrected the stale `documented-boundary` outcome to acknowledge the approved successor ADR; this is a semantics-preserving disposition of the review's reasoned finding.
