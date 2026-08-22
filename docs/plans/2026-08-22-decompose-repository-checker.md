---
format: plan-v2
date: 2026-08-22
adrs: [0295, 0296, 0299, make-repository-check-results-owner-classified]
status: Proposed
---
# Plan: Decompose Repository Checker

## Goal

Establish a policy-free, deterministically ordered `RepositoryChecker` that consumes explicit results from one semantic owner per check. Every ranked finding names its fixed Error or Warning severity and protected property, while Information remains unranked. Preserve working and staged universes, preparation boundaries, finding contents and category membership, Error-only exit behavior, failure propagation, compatibility projections, and output multiplicity. Keep presentation deterministic without treating relative placement among items in one Warning list as a compatibility boundary.

Do not add a plugin framework, extract RF-005 current-state coordination, begin RF-006 or later work, remove managed compatibility, edit adopters or the audit program, or change observable check policy.

## Architecture summary

A neutral immutable check-result model carries ranked findings with severity and protected property, plus separately unranked information. Focused check owners produce those results from immutable `ProjectState`, Publisher plans and corpora, and the narrow domain values they need. `RepositoryChecker` receives completed owner results and performs only explicit deterministic aggregation. It never selects policy by kind, reparses project inputs, or exposes a registration framework. Boundary adapters preserve existing drift identities, compatibility projections, and owner-rendered presentation.

Working check composition retains distinct report, current-state, and index preparations and the existing drift, state, prose, memory order. Staged composition retains lock, state, and Publisher-drift preparation and order. Publisher preparation remains the single producer of each operation's output plan and corpora; working and staged universes share no mutable derived state. Current-state result adaptation is in scope, but authority preparation and transition coordination remain with the current owner for RF-005.

**Plan flexibility.**

The protected-contract rule in the workflow document governs what a plan may not change. The plan records the best known route at authoring time, not a binding implementation choreography. A commit-capable owner may merge, split, reorder, add, remove, or replace recorded route detail while the protected contract holds. A path omitted from the plan is not alone a reason to stop, and a stale listed path need not be touched. Reapproval is required only when the protected contract would change or an unresolved material decision appears.

Reconcile a Proposed plan only when another phase or reviewer could rely on stale material instructions. Inconsequential and independently local edits require no deviation record. A delegated owner reports material cross-owner revisions for parent reconciliation. A helper remains confined to its assigned paths and gains no scope, commit, review, checkpoint, handoff, or outcome authority from route flexibility.

## Phase 1: Make finding classification explicit

**Execution mode: inline.**

Advances: ["explicit-finding-model", "observable-parity"]
Completes: ["classification-baseline"]

### Task 1.1: Lock the existing collection and presentation contract
Applying: ["0295:errors-protect-validity", "0295:judgement-warns", "0295:optional-notes-inform", "0295:fixed-ranks-preserved", "0295:aggregate-remains-actionable", "0296:boundary-values"]
Paths: ["cmd/awf/check_test.go", "cmd/awf/checkrepo_test.go", "cmd/awf/checkstaged_test.go", "cmd/awf/check_presentation_test.go", "internal/project/check_test.go", "internal/project/staged_drift_test.go", "internal/project/staged_test.go"]

Add the strongest practical durable oracles before structural movement. Cover a bare aggregate containing working and staged Error, Warning, and Information entries; exact category order and membership; Error-only exit mapping; continuation after produced findings versus suppression after operational failure; plan-note deduplication; ordinary cross-universe non-deduplication; direct-child advisory suppression; compatibility-multiplied stub information; working versus staged snapshot isolation; and unchanged staged membership and freshness behavior. Preserve exact finding contents and category fixtures rather than rewriting expected contents to fit the refactor. Ordering assertions may protect category structure and semantic operation sequences, but not relative placement among Warning items.

### Task 1.2: Introduce the neutral ranked finding boundary
Applying: ["0295:errors-protect-validity", "0295:judgement-warns", "0295:optional-notes-inform", "0295:fixed-ranks-preserved", "0296:dependency-direction", "0296:boundary-values", "0296:proportional-operations", "make-repository-check-results-owner-classified:owners-classify-results"]
Paths: ["internal/checkresult/checkresult.go", "internal/checkresult/checkresult_test.go", "internal/project/check.go", "internal/project/check_presentation.go", "internal/project/check_test.go", "internal/project/check_presentation_test.go"]

Create the smallest neutral immutable result model needed by real checker consumers. A ranked finding carries the existing fixed `severity.Rank`, an explicit protected-property identity, and the semantic evidence needed by compatibility and presentation adapters. Information remains a separate unranked value. Give the model one-sentence package ownership, documented earned exports, defensive projections where slices cross boundaries, and no registry, configurable severity, universal checker interface, or alternate renderer.

Adapt the existing project report path as the first production consumer without yet relocating check policy. Eliminate inference of rank or protected property from drift-kind spelling at the converted boundary while keeping `manifest.Drift`, legacy note projections, presentation content and categories, and caller-visible error identity intact.

### Task 1.3: Prove complete explicit classification
Applying: ["0295:errors-protect-validity", "0295:judgement-warns", "0295:fixed-ranks-preserved", "0296:boundary-values", "make-repository-check-results-owner-classified:owners-classify-results"]
Paths: ["internal/checkresult/checkresult_test.go", "internal/project/check_test.go", "internal/testsupport/publishing_ownership_test.go", "internal/testsupport/deps_test.go"]

Add a mutation-sensitive census that fails when a produced ranked finding lacks a valid fixed rank or protected property, when Information enters the ranked model, or when a converted owner path relies on category inference. Extend focused dependency evidence so the neutral model does not import application coordination and no converted check owner can depend on the future aggregator.

### Phase close

The phase is green with existing output and exit fixtures unchanged and the explicit finding model consumed by production.

```commit
refactor(code-design): make check findings explicit
```

## Phase 2: Extract generated, reference, and configuration owners

**Execution mode: subagent-driven.**

Advances: ["semantic-check-owners", "observable-parity", "preparation-reuse"]
Completes: ["generated-reference-configuration-owners"]

### Task 2.1: Move generated-output conformance behind its owner
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0296:proportional-operations", "0299:publisher-constructs-operation-plan", "0299:neutral-plan-values-below-coordination"]
Paths: ["internal/project/check.go", "internal/project/check_output.go", "internal/project/staged_drift.go", "internal/project/check_test.go", "internal/project/staged_drift_test.go", "internal/outputplan/outputplan.go", "internal/manifest/manifest.go", "glob:internal/generatedcheck/**"]
Post-check: Run the generated-check package tests, project working and staged drift tests, the output-plan identity census, and the checker owner/dependency census; all existing working and staged generated-output cases must terminate with the same drift identities and deterministic category placement, and no moved policy implementation may remain in project.

Create the focused GeneratedOutputChecker owner and move tracking availability, lock and output comparison, closed-tree sweep, rendered frontmatter, unused variable and sidecar data classification, and staged rendered-output conformance with their tests. Consume the already prepared immutable Publisher plan and reader for each universe. Do not reopen project state, rebuild plans, merge working and staged inputs, or change required membership, sorted freshness order, tracking Information, direct-child suppression, or failure propagation.

### Task 2.2: Move managed-reference validity behind its owner
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0296:proportional-operations"]
Paths: ["internal/project/check.go", "internal/project/check_test.go", "glob:internal/adr/**", "glob:internal/referencecheck/**"]
Post-check: Run reference-owner tests and the complete existing managed Markdown, skill, ADR-related-link, and ADR-order fixture population; every invalid reference must retain its path, detail, protected property, rank, and category membership under deterministic owner ordering, with no duplicate implementation left in project or a domain package.

Create the focused ReferenceChecker owner for managed Markdown, skill, and ADR-related reference integrity. Preserve the existing direct-child rules, full-profile conditions, link resolution, deterministic owner traversal, and source evidence. Outer composition supplies a working-universe semantic path-existence function or immutable target set; ReferenceChecker owns link-resolution policy and receives no filesystem handle, repository object, mutable project carrier, or application coordinator.

### Task 2.3: Move configuration consistency behind its owner
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0296:proportional-operations"]
Paths: ["internal/project/check.go", "internal/project/check_validation.go", "internal/project/check_test.go", "glob:internal/config/**", "glob:internal/configcheck/**"]
Post-check: Run configuration-owner and project report tests over valid and invalid command wiring, sidecar data, variable usage, and preparation failures; the terminal findings and operational errors must match the baseline, and configuration policy must have one implementation.

Create the focused ConfigurationChecker owner for configuration and command-spec consistency that belongs to RF-004. Keep generated-output vocabulary use with GeneratedOutputChecker when it depends on the prepared output plan. Preserve validation error identity and do not move Loader opening or validation, command parsing, or presentation.

### Phase close

The phase is green with generated, reference, and configuration policy owned outside aggregation and with working and staged universe behavior unchanged.

```commit
refactor(code-design): extract foundational check owners
```

## Phase 3: Extract plan, pitfall, and vocabulary owners

**Execution mode: subagent-driven.**

Advances: ["semantic-check-owners", "observable-parity", "preparation-reuse", "compatibility-preservation"]
Completes: ["domain-check-owners"]

### Task 3.1: Move plan validity and plan advisories behind their owner
Applying: ["0295:judgement-warns", "0295:optional-notes-inform", "0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0296:proportional-operations", "0299:publisher-constructs-operation-plan"]
Paths: ["internal/project/check.go", "internal/project/check_test.go", "glob:internal/plan/**", "glob:internal/audit/**", "glob:internal/plancheck/**"]
Post-check: Run plan-owner, project report, audit planned-subject, and command aggregate tests across parse failures, ADR references, plan-v2 structure, assignment warnings, commit-subject checks, scope Information, and plan-note deduplication; finding contents, category membership, deterministic presentation, and failure identity must equal the baseline except for unprotected relative Warning-item placement, and policy must have one implementation.

Create the focused PlanChecker owner for plan diagnostics, structure, references, plan-specific invocation of commit-subject checks, and plan warnings and Information. Reuse the existing shared Conventional Commit evaluator in `internal/audit`; PlanChecker adapts its result and does not duplicate or relocate that policy. Consume the Publisher-prepared plan corpus and parse error exactly once. Preserve full-profile conditions, unknown-scope Information, assignment-warning membership, deterministic owner output, and aggregate-only plan-note deduplication. Do not move commit-time authorization from `internal/commitpolicy` or advisory repository analysis from `internal/audit`.

### Task 3.2: Move pitfall validity behind its owner
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0296:proportional-operations", "0299:publisher-constructs-operation-plan"]
Paths: ["internal/project/check.go", "internal/project/check_test.go", "glob:internal/pitfall/**", "glob:internal/pitfallcheck/**"]
Post-check: Run pitfall-owner and project report tests across strict corpus errors, domain and ADR references, compatibility callers, and omitted versus supplied corpora; production must consume the Publisher-prepared corpus once, existing compatibility projections must remain supported, and no output multiplicity may change.

Create the focused PitfallChecker owner using the prepared pitfall and ADR corpora. Remove `compatPitfallCorpus` only after a caller census proves its omitted-corpus reopen path is internal unsupported residue and every real production caller supplies the prepared corpus. Otherwise retain a confined compatibility adapter without letting it become the production path. Preserve all exported legacy report and operation projections unless repository support authority independently proves removal legal.

### Task 3.3: Move glossary and tag health behind their owner
Applying: ["0295:judgement-warns", "0295:optional-notes-inform", "0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0296:proportional-operations", "0299:publisher-constructs-operation-plan"]
Paths: ["internal/project/check.go", "internal/project/check_test.go", "internal/publisher/glossary.go", "internal/publisher/topics.go", "glob:internal/pitfall/**", "glob:internal/vocabularycheck/**"]
Post-check: Run vocabulary-owner and project report tests over glossary domains, glossary terseness, tag vocabulary, tag frequency, untagged artifacts, strict corpus failures, and full/core profile conditions; every rank, property, note, category membership, deterministic owner result, and error identity must match the baseline with one implementation per policy.

Create the focused VocabularyChecker owner for glossary validity and heuristic notes plus tag vocabulary and health. Consume immutable prepared render inputs and pitfall corpus projections without reparsing or importing Publisher coordination. Keep rendering and glossary generation in Publisher, and keep tag/config source ownership in their existing domain packages.

### Phase close

The phase is green with all named leaf policies in semantic owners, their tests moved with behavior, and compatibility or multiplicity unchanged.

```commit
refactor(code-design): extract domain check owners
```

## Phase 4: Establish policy-free repository aggregation

**Execution mode: inline.**

Completes: ["explicit-finding-model", "semantic-check-owners", "policy-free-aggregation", "observable-parity", "preparation-reuse", "compatibility-preservation", "documented-boundary"]

### Task 4.1: Compose owner results in RepositoryChecker
Applying: ["0295:aggregate-remains-actionable", "0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0296:proportional-operations", "make-repository-check-results-owner-classified:owners-classify-results", "make-repository-check-results-owner-classified:repository-checker-aggregates-results"]
Paths: ["internal/project/check.go", "internal/project/check_presentation.go", "internal/project/operations.go", "glob:internal/repositorycheck/**", "glob:internal/checkresult/**", "glob:internal/currentstate/**", "glob:internal/prosegate/**", "glob:internal/memorycite/**", "internal/testsupport/deps_test.go", "internal/testsupport/publishing_ownership_test.go"]
Post-check: Run the owner census, import-direction tests, result projection tests, direct and aggregate presentation fixtures, and package dead-code tests; every policy must have exactly one owner, RepositoryChecker must contain only explicit ordered aggregation and projection, and no owner may import the aggregator or application coordination.

Create the focused RepositoryChecker as the sole policy-free aggregator. It consumes owner-produced immutable Results, including adapted current-state, prose, and memory results, in the established semantic order. It may preserve explicit order in direct code but may not classify by kind, contain a giant policy switch, register plugins, discover dependencies, prepare inputs, or implement leaf policy. Move result-model presentation into its owner while leaving command code responsible for parse, compose, selection, streams, central rendering invocation, and exit mapping.

Retain `Notes`, `TrackingNotes`, `PlanNotes`, `AdvisoryNotes`, compatibility-multiplied files, and other caller-visible projections unless a complete caller and support-policy census proves an internal field obsolete without changing projected contents, category membership, or multiplicity. Remove only replaced private selection residue such as `classified` after all production and test callers use explicit result fields.

### Task 4.2: Wire the working universe without changing coordination
Applying: ["0295:aggregate-remains-actionable", "0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0296:proportional-operations", "0299:publisher-constructs-operation-plan", "0299:neutral-plan-values-below-coordination"]
Paths: ["cmd/awf/check.go", "cmd/awf/checkrepo.go", "cmd/awf/check_presentation.go", "cmd/awf/checkrepo_test.go", "cmd/awf/check_presentation_test.go", "internal/project/contextstate.go", "internal/project/currentstate.go", "internal/project/operations.go", "glob:internal/repositorycheck/**"]
Post-check: Run direct drift, direct state, direct prose, direct memory, complete working aggregate, and exact finding-content and category-presentation tests; working report, current-state, and index preparations must remain distinct, the semantic operation order must remain drift then state then prose then memory, produced findings must preserve continuation, operational failures must preserve suppression, and aggregate-only advisories must remain absent from direct children.

Compose the working RepositoryChecker from the existing distinct preparations. Reuse one Publisher preparation for the report plan and corpora, adapt but do not relocate CurrentStateCoordinator policy, and preserve the separate stage-0 index requirement for prose and memory. Keep command collection free of check policy while preserving complete report versus operational-error behavior and category order.

### Task 4.3: Wire the staged universe without changing coordination
Applying: ["0295:aggregate-remains-actionable", "0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "0296:proportional-operations", "0299:publisher-constructs-operation-plan", "0299:neutral-plan-values-below-coordination"]
Paths: ["cmd/awf/check.go", "cmd/awf/checkstaged.go", "cmd/awf/checkstaged_test.go", "internal/project/staged_drift.go", "internal/project/staged_test.go", "glob:internal/repositorycheck/**", "glob:internal/generatedcheck/**"]
Post-check: Run direct staged state, direct staged drift, complete staged aggregate, dirty-working-tree isolation, lock-transition, membership, freshness, and combined bare-check fixtures; staged lock then state then Publisher drift semantics must remain unchanged, working and staged universes must remain distinct, category presentation must remain deterministic without protecting their relative Warning-item placement, and only plan notes may deduplicate across the aggregate.

Compose staged owner results from the existing staged lock, current-state, and Publisher preparations. Leave staged current-state authority coordination and its separate plan parsing for RF-005. Do not share mutable prepared state with the working universe or collapse state and drift snapshots.

### Task 4.4: Publish the implemented ownership boundary
Applying: ["0296:dependency-direction", "0296:extraction-owners", "0296:boundary-values", "make-repository-check-results-owner-classified:owners-classify-results", "make-repository-check-results-owner-classified:repository-checker-aggregates-results"]
Paths: ["glob:.awf/domains/**", ".awf/topics/metadata/code-design/dependency-composition.yaml", ".awf/topics/parts/code-design/dependency-composition/current-state.md", ".awf/topics/metadata/tooling/cli.yaml", ".awf/topics/parts/tooling/cli/current-state.md", ".awf/topics/metadata/rendering/project-output-plan.yaml", ".awf/topics/parts/rendering/project-output-plan/current-state.md", ".awf/topics/metadata/rendering/sync-and-drift.yaml", ".awf/topics/parts/rendering/sync-and-drift/current-state.md", ".awf/docs/parts/architecture/components.md", "docs/architecture.md", "docs/decisions/make-repository-check-results-owner-classified.md", "docs/decisions/INDEX.md", "glob:docs/topics/**", ".awf/awf.lock"]
Post-check: Run `./awf context` over every new checker package and changed command path, render, then run `./awf check`; every new production and test path must resolve to a narrow owning selector, generated output must be clean, and prose must still name CurrentStateCoordinator as an RF-005 future boundary.

Transition ADR-make-repository-check-results-owner-classified from Proposed to Implementing and apply all four declared claim updates atomically with their exact mutations: `tooling/cli:repo-check-capability-plan`, `tooling/cli:check-severity-by-protected-property`, `rendering/project-output-plan:check-report-single-plan`, and `rendering/sync-and-drift:agent-guide-size-advisory`. The capability-plan claim preserves Error source order plus semantic operation and category order while requiring deterministic Warning presentation without protecting relative legacy Warning-item placement. Update authored architecture and current-state authority to describe the implemented policy-free RepositoryChecker, explicit owner results, preserved single Publisher plan, and unchanged working/staged preparation boundaries. Add narrow selector coverage for every new package and move invariant proof paths with their owner tests. Render the decision index, generated topics, architecture, and lock with their sources. Leave the ADR Implementing and this plan Proposed for effort-workflow's deferred post-integration terminal closure. Do not edit the audit program or add adopter-facing changelog text when observable behavior remains unchanged.

### Phase close

The phase is green with a policy-free RepositoryChecker; preserved finding contents, rank and protected property, category membership, multiplicity, plan-note deduplication, Error-only exit behavior, operational failure behavior, distinct universes, deterministic presentation, and semantic operation and category order; generated authority current; and no RF-005 coordination moved. Relative placement among Warning items is not protected.

```commit
refactor(code-design): establish repository checker aggregation
```

## Definition of done

- `dod: classification-baseline` Durable fixtures prove the pre-refactor working, staged, combined, direct-child, compatibility, presentation, failure, and exit contracts before policy movement.
- `dod: explicit-finding-model` Every ranked owner finding explicitly carries fixed Error or Warning severity and a protected property; Information remains unranked and no aggregate infers classification from drift kind or presentation category.
- `dod: semantic-check-owners` Generated output, managed references, plans, pitfalls, vocabulary, configuration, current state, prose, memory, commit policy, and audit checks each have one semantic owner with behavior tests in that owner.
- `dod: generated-reference-configuration-owners` Generated-output, reference, and configuration policy has moved from project aggregation without changing working or staged results.
- `dod: domain-check-owners` Plan, pitfall, glossary, and tag policy has moved from project aggregation without reparsing prepared corpora or changing profile behavior.
- `dod: policy-free-aggregation` RepositoryChecker only consumes and orders owner Results; it contains no leaf policy, kind-to-severity switch, plugin registry, preparation, or reverse owner dependency, and `internal/project/check.go` is no longer a god file.
- `dod: observable-parity` Direct and aggregate finding contents, category membership, deterministic presentation, Error-only exit, produced-finding continuation, operational-failure suppression, plan-note deduplication, ordinary evidence multiplicity, and working/staged universe semantics match the starting contract; relative placement among items in one Warning list is not protected.
- `dod: preparation-reuse` Each working or staged operation reuses its RF-003 Publisher plan and corpora without reopening, reparsing, rebuilding, or sharing mutable derived state across universes; RF-005-owned current-state preparations remain distinct.
- `dod: compatibility-preservation` Caller-visible note projections and compatibility output multiplicity remain unless complete support-policy evidence proves an internal residue removable; ADR-0297-managed compatibility is unchanged.
- `dod: documented-boundary` Authored selectors, architecture, and current-state topics describe and cover the implemented owners while retaining RF-005 as future work; rendered output and lock are clean.

## Notes

Apply the plan-flexibility rule above when recording deviations. Delegated owners report material cross-owner revisions rather than editing the plan; the parent supplies the report to phase review and reconciles required plan changes with findings in one focused post-review settlement commit before checkpointing or later execution. Record spike answers, follow-ups, and findings surfaced during implementation.

Grounding confirmed the existing exact working order, staged order, failure-versus-finding behavior, compatibility multiplicity, RF-003 preparation reuse route, and dependency constraints. ADR-0295 and ADR-0296 authorize fixed classification and semantic ownership, while ADR-0299 supplies the prepared plan boundary. Plan review exposed that terminal ADRs could not authorize the exact current-state claim mutations needed to make that target current. ADR-make-repository-check-results-owner-classified now carries those update operations without changing the delegated RF-004 boundary.

Initial plan review produced four findings. The lifecycle-freeze request was rejected because the authorized integration-ready stop and effort-workflow require this plan to remain Proposed until deferred post-integration closure. The active-claim finding was accepted; the successor ADR was proposed, corrected after ADR review, and verified with no residual findings. ReferenceChecker now receives only a semantic working-universe existence capability, preventing filesystem or project representation leakage. PlanChecker now reuses `internal/audit` as the single Conventional Commit evaluator and owns only plan-specific invocation and result adaptation.

Phase 1 moved `internal/checkresult/**` selector ownership from Phase 4 into the defining transaction. The package must be current-state owned for every phase to close green; later phases retain selector responsibility only for the owner packages they introduce.

After all phases completed, D5 narrowed the compatibility boundary for relative Warning-item placement. Freshness review found Task 4.4 and the applied `tooling/cli:repo-check-capability-plan` claim still protected source order within each severity. Commit `9d7535dbf` corrected that stale authority through an atomic Reapplied event, authored claim mutation, render, and lock update; exact Warning-item ordering machinery remains excluded. Completed Phases 1 through 4 are affected: Phase 1 established the old ordering oracle, Phases 2 and 3 moved Warning owners, and Phase 4 composed and presented their results. Before progression, renew implementation assurance once over the complete landed RF-004 range `e146598f0..9d7535dbf`, emphasizing the typed staged PlanChecker route, deterministic category and semantic-operation presentation, finding contents, rank and property, category membership, multiplicity, plan-note deduplication, Error-only exit, operational failures, universe separation, compatibility projections, absence of legacy-order compatibility machinery, and the no-integration and no-terminal-closure boundary.
