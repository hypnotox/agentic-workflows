---
date: 2026-07-21
adrs: [144]
status: Proposed
---
# Plan: Agent-Oriented Current-State Context and CLI Navigation

## Goal

Implement [ADR-0144](../decisions/0144-agent-oriented-current-state-context-and-cli-navigation.md) so `awf context` is a concise, path-attributed orientation query with a lossless `--full` projection, known-artifact and ADR navigation, honest applicability evidence, a configurable topic-size advisory, and metadata-derived runner command parity. Non-goals are symbolic glob intersection, recursive claim-reference expansion, a persisted artifact ledger, removal of `contextIgnore`, and making the claim budget a failing gate.

## Architecture summary

The implementation first freezes ADR-0144 after creating its two empty destination-topic shells. It then applies the frozen operations in five independently checked V2 batches: runner metadata, topic-budget schema, context and applicability foundations, concise/full plus ADR projections, and repository ownership. `internal/clispec` becomes the runner-forwarding authority. `internal/topic` owns one applicability model shared by topic and context. `internal/project` replaces the union-shaped `ContextResult` with request/effective-path results assembled from a single working or index universe, and builds artifact records from layout/catalog/config/topic/ADR data plus the output-plan declaration authority. `cmd/awf` only validates options and renders that semantic model. The final ownership batch creates the two test-backed claims, updates repository scopes, completes authored and rendered documentation, and co-flips ADR-0144 and this plan.

## File structure

- **Created:** `internal/migrate/maxclaimspertopic.go`, `internal/migrate/maxclaimspertopic_test.go`, `internal/project/context_paths.go`, `internal/project/context_artifacts.go`, `internal/project/context_adr.go`, `internal/project/context_paths_test.go`, `internal/project/context_artifacts_test.go`, `internal/project/context_adr_test.go`, `.awf/topics/metadata/rendering/adapter-outputs.yaml`, `.awf/topics/parts/rendering/adapter-outputs/current-state.md`, `.awf/topics/metadata/tooling/test-infrastructure.yaml`, `.awf/topics/parts/tooling/test-infrastructure/current-state.md`.
- **Modified:** `internal/clispec/clispec.go`, `internal/clispec/clispec_test.go`, `templates/runner/x.tmpl`, `internal/project/runner_test.go`, `internal/project/example_wiring_test.go`, `x`, `examples/sundial/x`, `internal/config/config.go`, `internal/config/config_test.go`, `internal/config/edit.go`, `internal/config/edit_test.go`, `internal/configspec/spec.go`, `internal/configspec/spec_test.go`, `internal/migrate/migrate.go`, `internal/migrate/migrate_test.go`, `internal/project/version.go`, `internal/project/version_test.go`, `internal/project/scaffold.go`, `internal/project/scaffold_test.go`, `internal/project/confighash.go`, `internal/project/configreference.go`, `internal/project/configreference_test.go`, `internal/project/check.go`, `internal/project/check_test.go`, `internal/topic/coverage.go`, `internal/topic/coverage_test.go`, `internal/topic/query.go`, `internal/topic/query_test.go`, `internal/topic/render.go`, `internal/topic/render_test.go`, `cmd/awf/topic.go`, `cmd/awf/topic_test.go`, `internal/project/context.go`, `internal/project/context_test.go`, `cmd/awf/context.go`, `cmd/awf/context_test.go`, `internal/project/currentstate.go`, `internal/project/output_plan.go`, `internal/project/output_plan_test.go`, `internal/adr/corpus.go`, `internal/adr/corpus_test.go`, `.awf/config.yaml`, `.awf/domains/rendering.yaml`, `.awf/domains/tooling.yaml`, `.awf/topics/parts/tooling/cli/current-state.md`, `.awf/topics/parts/config/configuration/current-state.md`, `.awf/topics/parts/rendering/project-output-plan/current-state.md`, `.awf/parts/agents-doc/commands.md`, `templates/docs/working-with-awf.md.tmpl`, `.awf/docs/parts/architecture/overview.md`, `.awf/docs/parts/architecture/components.md`, `.awf/docs/parts/architecture/data-flow.md`, `.awf/docs/parts/glossary/prepend.md`, `.awf/docs/parts/testing/gate.md`, `.awf/parts/working-with-awf/commands.md`, `templates/skills/adr-lifecycle/SKILL.md.tmpl`, `templates/skills/brainstorming/SKILL.md.tmpl`, `templates/skills/reviewing-impl/SKILL.md.tmpl`, `templates/skills/reviewing-plan/SKILL.md.tmpl`, `templates/docs/agents-md-standard.md.tmpl`, `docs/decisions/0144-agent-oriented-current-state-context-and-cli-navigation.md`, and generated outputs from `./x sync`, including `AGENTS.md`, target skill copies, `docs/config-reference.md`, domain/topic docs, `docs/decisions/INDEX.md`, `examples/sundial/x`, and both locks.
- **Deleted:** none.

## Phase 1: Freeze the decision with destination topic shells

- [ ] **Task 1.1: Create the empty destination topics.** Add exactly these metadata files before ADR-0144 becomes Accepted:

  `.awf/topics/metadata/rendering/adapter-outputs.yaml`:
  ```yaml
  title: Adapter outputs
  summary: Generated executable adapter-runtime outputs and their ownership boundary.
  paths:
    - .pi/extensions/**
  ```

  `.awf/topics/metadata/tooling/test-infrastructure.yaml`:
  ```yaml
  title: Test infrastructure
  summary: Shared internal test-support infrastructure and its dependency boundary.
  paths:
    - internal/testsupport/**
  ```

  Create both matching `current-state.md` parts with one publication-safe introductory sentence and an empty `## Claims` section. Do not add either pending claim, alter domain paths, or remove `contextIgnore` yet. Run `./x sync`; generated topic docs must render, and `./x check` must be clean.

- [ ] **Task 1.2: Accept ADR-0144 without applying an operation.** Change ADR-0144 frontmatter from `Proposed` to `Accepted` and append an Accepted history event with the canonical digest reported by the staged checker. Do not append an Applied event. Run `./x sync`, explicitly stage the ADR, four topic inputs, `docs/topics/**`, `docs/decisions/INDEX.md`, `.awf/awf.lock`, and any other sync output. Run `go run ./cmd/awf check --staged`; it must report clean with every State changes operation Remaining. Run `./x gate`, then commit:

  ```commit
  docs(adr): accept 0144 agent-oriented context navigation
  ```

## Phase 2: Derive runner availability from CLI metadata

- [ ] **Task 2.1: Make runner forwarding a closed command attribute.** In `internal/clispec/clispec.go`, add this contract-bearing declaration:

  ```go
  type RunnerDisposition struct {
      Forward bool
      Reason  string
  }
  ```

  Add `Runner RunnerDisposition` to every top-level `Command`. Set `Forward: false` with these exact nonempty reasons for `init`, `upgrade`, and `uninstall`: `requires a pre-adoption invocation`, `must cross the pinned bootstrap boundary`, and `runner-mediated self-removal is unsafe`. Set `Forward: true` and an empty reason for every other current top-level command. Add helpers that return forwarded commands in `Commands` order and validate that exactly one disposition is meaningful: forwarded entries have no reason; excluded entries have a reason. Children inherit the owning top-level disposition and must not declare another one. Extend `internal/clispec/clispec_test.go` to fail on an unclassified command, either invalid reason state, changed initial exclusions, order drift, or a child-level declaration.

- [ ] **Task 2.2: Generate the managed runner from the metadata.** Replace the hard-coded awf verb cases and usage list in `templates/runner/x.tmpl` with render data derived from `clispec` forwarding metadata. Keep `runner-project-verbs` in-place content project-owned, but validate during output planning that no project case label collides with a forwarded awf name; return a deterministic error naming the verb and its metadata owner. In `internal/project/runner_test.go`, prove every forwarded command appears once in dispatch and usage, every excluded command appears in neither, project verbs remain editable, and a collision refuses before write. Update `internal/project/output_plan.go` only at the validation seam needed to perform this check; do not parse arbitrary shell beyond case labels in the owned project-verb section.

- [ ] **Task 2.3: Enforce repository and example parity.** Update repository `x` so every forwarded command not already special-cased delegates to `go run ./cmd/awf <verb> "$@"`; retain repository-specific behavior for `sync` and `check`, and retain project-only verbs. Update `examples/sundial/x` through the template render, never by hand. Replace the command-list assertions in `internal/project/example_wiring_test.go` with parity assertions driven by `clispec.Forwarded`: repository `x` must contain a delegating branch for every forwarded verb or an explicitly tested repository-special branch, while the example must equal rendered output. Ensure excluded commands are absent. Update runner usage documentation from the same ordered metadata.

- [ ] **Task 2.4: Apply the runner State changes batch.** Update `cli-command-spec-single-source` in `.awf/topics/parts/tooling/cli/current-state.md`, preserving Origin, its complete Revised-by prefix, and test backing, then add `managed-runner-command-parity` with `Origin: ADR-0144` and `Backing: test`. Add proof markers to the metadata and runner parity tests. Append ADR-0144's first history batch in declaration order:

  `update tooling/cli:cli-command-spec-single-source`, `add tooling/cli:managed-runner-command-parity`.

  The history order is Implementing status first, then Applied with the next checker-reported state sequence. Run `./x sync`; stage the ADR, the two exact claim mutations and proofs, implementation, tests, template, repository/example runners, and generated docs/locks. Run `go run ./cmd/awf check --staged` and `./x gate`; both must pass, with only this batch Applied. Commit:

  ```commit
  feat(tooling): derive runner commands (applies 0144 batch)
  ```

## Phase 3: Add the configurable topic claim-budget advisory

- [ ] **Task 3.1: Add and validate `maxClaimsPerTopic`.** In `internal/config/config.go`, add `MaxClaimsPerTopic *int` with YAML key `maxClaimsPerTopic`, decode only an integer scalar, reject zero and negative values during config validation, and add `EffectiveMaxClaimsPerTopic() int` returning 20 for nil. Extend `internal/config/edit.go` with a typed nested integer setter rather than general `any` serialization; it must preserve comments and unrelated keys and must be used by the migration. Cover omitted/default, explicit positive, zero, negative, wrong YAML kind, duplicates, canonical encode, and comment preservation in `internal/config/config_test.go` and `internal/config/edit_test.go`.

- [ ] **Task 3.2: Advance schema generation and serialize the default.** Add schema-16 migration `topic-claim-budget` in `internal/migrate/migrate.go` and implement it in `internal/migrate/maxclaimspertopic.go`. It inserts `currentState.maxClaimsPerTopic: 20` only when absent, creates `currentState` when absent without discarding config content, preserves an explicit positive value, is idempotent, and reports exactly one migration line when it writes. Update `internal/project/scaffold.go` so new init output includes the explicit value. Update the schema-version expectations and minimum version mapping in `internal/project/version.go`, `internal/project/version_test.go`, upgrade tests, migration registry tests, example fixtures, and locks. Do not weaken the existing V2-cutoff migration ownership. Tests must prove a schema-15 tree upgrades to 16 with its ADR cutoff unchanged and a second upgrade is a no-op.

- [ ] **Task 3.3: Wire every configuration consumer.** Add `currentState.maxClaimsPerTopic` to `internal/configspec/spec.go` with type `positive integer`, default `20`, and advisory semantics. Include the effective value in config reference live-state output and in the render/config hash inputs used by generated guidance. Update configspec reflection parity, `internal/project/configreference_test.go`, `internal/project/confighash.go` tests, manifest/lock fixture expectations, and `.awf/config.yaml` plus the Sundial config to carry `maxClaimsPerTopic: 20`. `awf config currentState.maxClaimsPerTopic` must report the explicit or default value consistently.

- [ ] **Task 3.4: Emit one non-failing note per oversized topic.** Add a pure evaluator beside `internal/topic/coverage.go` that accepts the corpus and effective threshold and returns topic-ID-sorted notes only when `len(topic.Claims) > threshold`. Each note names the claim count, limit, `.awf/topics/metadata/<domain>/<topic>.yaml`, and `.awf/topics/parts/<domain>/<topic>/current-state.md`. Call it from `internal/project/check.go` after current-state loading in working check; staged checking retains its transition/coverage duties and must not turn the note into an error. Tests cover equality (no note), strictly above (one note), multiple sorted topics, explicit override, default 20, and unchanged success status. Update the model adopter so `./x check` emits no advisory note.

- [ ] **Task 3.5: Apply the budget State changes batch.** Add `tooling/cli:topic-claim-budget-advisory` and `config/configuration:topic-claim-budget-configured` as test-backed invariants with `Origin: ADR-0144`; place proof markers on the evaluator/check tests and config/schema tests. Append one Applied event containing those two operations in ADR declaration order with the next state sequence. Update authored configuration, architecture, configuration-reference, and working-with-awf sources for the new key; run `./x sync`. Stage the complete schema, migration, behavior, claim, proof, docs, example, and lock transaction. Run `go run ./cmd/awf check --staged` and `./x gate`; both must pass. Commit:

  ```commit
  feat(config): add topic claim-budget advisory (applies 0144 batch)
  ```

## Phase 4: Build path-attributed context, applicability, and artifact foundations

- [ ] **Task 4.1: Replace symbolic effective pairs with one applicability model.** In `internal/topic/coverage.go`, replace the Cartesian-product `EffectiveSelectors` projection with a model containing non-null sorted `DomainPaths`, `TopicPaths`, `MatchedPaths`, and `MarkerSites`. `MatchedPaths` contains current snapshot paths that match both selector families; it is evidence, not a symbolic intersection proof. Add an explicit human/JSON statement that both domain and topic selectors must match. Update `internal/topic/query.go`, `internal/topic/render.go`, and `cmd/awf/topic.go` to consume this model for `awf topic --coverage`. Tests must cover global topics, disjoint globs with no matched paths, overlapping globs, state/touches/proof marker sites, empty arrays rather than null, deterministic ordering, and removal of every `Effective: domain ... + topic ...` Cartesian assertion.

- [ ] **Task 4.2: Introduce the request/effective-path result contract.** In `internal/project/context_paths.go`, define closed string enums and JSON structs for `ContextProjection` (`concise`, `full`), request expansion status, and primary path classification (`covered`, `eligible-unowned`, `context-ignored`, `generated-output`, `nested-adopter`, `symlink`, `not-found`, `outside-repository`). Replace `ContextResult.Paths/Domains/Topics/Pending/Unowned` with non-null `Requests []ContextRequest` and `Paths []ContextPath`; each request keeps normalized input and sorted effective-path IDs, and each unique path keeps sorted request IDs. Preserve duplicates only as attribution, never duplicate path records.

  Implement classification in this exact precedence: outside repository; beneath a nested `.awf/config.yaml` boundary; present as a non-reservation output-plan node; symlink; `contextIgnore`; absent in the selected tree; covered; eligible-unowned. Use lexical cleaning plus root-bounded `Lstat`/snapshot metadata; never follow a symlink for expansion or authority. A symlink record reports `targetInsideRepository` without reading through the target. Directory requests receive expansion status and eligible sorted descendants, not an aggregate primary classification. Explicit files remain one effective path even when ineligible or absent. Tests in `internal/project/context_paths_test.go` cover every class, every precedence collision, absent planned output, ignored nested adopter, escaping absolute/`..`/symlink input, mixed directory descendants, overlapping requests, staged-only directories, deletion, and deterministic non-null JSON slices.

- [ ] **Task 4.3: Resolve authority independently of eligibility.** Refactor `internal/project/context.go` so working `ContextFor` and `StagedContextRoot` create one `contextUniverse` containing the selected tree, config, lock, topic/ADR corpus, marker index, nested boundaries, and output declarations. For every safely matchable effective path, compute owning domains and applicable topics even when its class is generated-output or context-ignored. Do not resolve through nested adopters, outside paths, or symlinks. Keep `Uncovered` and `StagedUncoveredRoot` on their existing eligible-only model. `--range` continues to provide request paths only; `cmd/awf/context.go` must assemble those paths against one working universe. Dirty working config, lock, topic, manifest, or path bytes must not affect `--staged` results.

- [ ] **Task 4.4: Add derived artifact attribution.** In `internal/project/context_artifacts.go`, define the ordered closed `ArtifactRole` values `config`, `lock`, `manifest`, `template`, `convention-part`, `authored-data`, `topic-metadata`, `claim-part`, `decision-record`, and `managed-output`, plus non-null source/output/navigation slices. Build attribution only from loaded layout/config/catalog/topic/ADR inputs and a declaration-only projection shared with `OutputPlan`; the manifest contributes snapshot identity and drift metadata but never labels a path generated. Extract from `internal/project/output_plan.go` the minimum pure declaration seam needed so working and staged universes can derive identical planned paths and dependencies from their own file reader. The staged builder must read sidecars, parts, config, lock, and corpus only from `snapshot.Tree`, never materialize into the repository or consult dirty working files.

  Recognition rules are exact: config/lock/manifest use layout singleton paths; template uses embedded template IDs; convention parts use declared section paths; authored-data uses enabled sidecar/data inputs not claimed by a more specific topic role; topic metadata and claim part use the canonical topic layout; decision-record uses parsed `NNNN-*.md`; managed-output requires a non-reservation output-plan node. Multiple roles are allowed and role-sorted. Source-to-output and output-to-source edges come from declaration dependencies. Local reservations and unmanaged lookalikes receive no generated role. Tests cover disabled artifacts, local reservations, in-place source/output multiplicity, shared target outputs, topic/domain/index outputs, absent outputs, staged sidecar/part divergence, manifest drift metadata, and stable ordering.

- [ ] **Task 4.5: Share applicability evidence with context.** Add the `internal/topic` applicability block to every applicable topic under each `ContextPath`, using the selected universe's eligible/current paths and marker index. Include domain and topic selectors separately, the both-must-match statement, concrete matched paths, and marker sites. Do not calculate or imply symbolic intersection. Human and JSON topic/context tests must assert semantic equality of the shared block.

- [ ] **Task 4.6: Apply the foundations State changes batch.** Update `context-read-only` to describe single working/index universe reads while preserving Origin/Revised-by/backing. Add test-backed `context-path-attribution`, `context-path-classification`, `context-known-artifact-navigation`, and `context-applicability-navigation` under tooling/cli, and `rendering/project-output-plan:managed-output-attribution`, all with `Origin: ADR-0144` and proof markers on the corresponding tests. Append one Applied event containing these six operations in their ADR declaration order. Update architecture and glossary authored sources for request, effective path, classification, artifact role, and applicability evidence. Run `./x sync`, stage the exact claims/proofs and all behavior/generated outputs, then run `go run ./cmd/awf check --staged` and `./x gate`; both must pass. Commit:

  ```commit
  feat(tooling): add attributed context foundations (applies 0144 batch)
  ```

## Phase 5: Deliver concise/full and explicit ADR projections

- [ ] **Task 5.1: Assemble concise and full claim projections from one model.** Extend `ContextPath` topic entries with `DirectClaims`, `OmittedClaimCount`, and `TopicCommand`; full entries additionally expose all current claims, backing, Verify text, proof/direct/touches sites, direct incoming/outgoing reference IDs, scopes, matched paths, and pending operations. Concise direct selection is the union of exact-path `state:`, `touches-state:`, and proof markers; state narrows only its topic, touches stays advisory, and proof selects only its backed invariant. Omitted count is the applicable topic's current claims minus unique direct claims and is never negative. Full expands all current claims but only direct reference IDs, never referenced bodies or unrelated ADR history. Ensure every semantically meaningful collection encodes as `[]`, not `null`, and sort by qualified ID/path/line.

- [ ] **Task 5.2: Add first-class ADR artifact projection.** In `internal/project/context_adr.go`, recognize only explicit `docs/decisions/NNNN-*.md` effective paths that parsed in the selected ADR corpus. Produce number, title, status, `mutable` for Proposed and `frozen` otherwise, and authority role text stating that prose is pending intent or decision history, never current authority. Reuse `adr.Corpus.OperationProgress` and operation batch records; do not infer lifecycle state from claim presence. Project Proposed operations as Proposed, Accepted as Remaining, Implementing as Applied plus Remaining, Implemented as Applied, and partially Abandoned as Applied plus Canceled. Applied entries include state sequence. Each operation links its topic and classifies its claim as active-current, historically-removed, or not-yet-current. Full mode adds only operation-linked provenance, backing, marker sites, and removal history by reusing topic query/history helpers; it must not add plans, tags, relations, unrelated ADRs, or invented tombstones.

  Tests in `internal/project/context_adr_test.go` cover every V2 lifecycle, direct V2 implementation, partially Abandoned progress, add/update/remove, removed claim history, malformed/non-ADR lookalikes, staged-vs-working ADR divergence, concise/full boundaries, and the guarantee that Proposed/Accepted prose is not emitted as authority.

- [ ] **Task 5.3: Update CLI flags and renderers.** Add `--full` to the context clispec bool flags and help. In `cmd/awf/context.go`, reject `--full --uncovered` with a usage error before project loading; allow it with explicit paths, `--staged`, and `--range`. Outside an adopted tree, both projections return the same successful static reference and explicitly say live classification/authority requires adoption. Human rendering groups requests then unique effective paths, prints classification before authority, calls omitted claims out with `awf topic <id>`, and labels artifact/ADR navigation. JSON serializes exactly the selected projection from the same result model; it must not hide full-only data in concise JSON. Preserve write-error context and no-write behavior.

  Replace union-oriented cases in `cmd/awf/context_test.go` and `internal/project/context_test.go` with table/golden assertions for concise/full human and JSON parity, multi-request attribution, omitted counts, direct proof/touches/state selection, complete full claims/references/proofs/pending data, ignored/generated ownership, ADR paths, range working-snapshot semantics, static fallback, flag conflicts, open/gate/read/write failures, and staged isolation. Keep every `--uncovered` behavior and fixture intact except the new explicit `--full` rejection.

- [ ] **Task 5.4: Update every complete-authority caller.** Search with `rg -n 'awf context|./x context' .awf templates docs examples AGENTS.md --glob '*.md' --glob '*.tmpl'`. For each managed workflow skill or agent instruction that requires all applicable authority, edit its authored `.awf/**/parts/**` or template source to invoke `awf context --full`; leave explicitly initial/orientation examples concise. Update `.awf/parts/agents-doc/commands.md` and `templates/docs/working-with-awf.md.tmpl` with the concise/full contract and `--full --uncovered` refusal. Run `./x sync`; the post-check is the same `rg`, manually classified so no complete-authority prose still promises all claims from bare context, plus `./x check` clean. Never hand-edit generated skill copies or `AGENTS.md`.

- [ ] **Task 5.5: Apply the projection State changes batch.** Update `context-default-excludes-history`, `context-output-parity`, and `context-static-fallback`, preserving each claim's provenance and backing. Add test-backed `context-adr-operation-projection` and `context-full-authority-packet` with `Origin: ADR-0144`. Append one Applied event containing those five operations in ADR declaration order and add/update proof markers with the behavior. Run `./x sync`, stage the implementation, claims, proofs, authored guidance and generated copies, and run `go run ./cmd/awf check --staged` plus `./x gate`; both must pass. Commit:

  ```commit
  feat(tooling): deliver context projections (applies 0144 batch)
  ```

## Phase 6: Establish repository ownership and complete the decision

- [ ] **Task 6.1: Apply executable-surface ownership atomically.** Extend `.awf/domains/rendering.yaml` with `.pi/extensions/**`. In `.awf/topics/parts/rendering/adapter-outputs/current-state.md`, add `invariant: generated-adapter-runtime-ownership`, `Origin: ADR-0144`, `Backing: test`, stating that enabled target extension outputs are owned by this topic even though generated classification excludes them from whole-tree coverage. Put its proof marker on `internal/project/output_plan_test.go` or the target-output parity test that asserts `.pi/extensions/**` declaration and attribution.

  Extend `.awf/domains/tooling.yaml` with `internal/testsupport/**`, delete the exact `internal/testsupport/**` entry from `.awf/config.yaml` `contextIgnore`, and add `invariant: test-support-leaf-boundary`, `Origin: ADR-0144`, `Backing: test` to the test-infrastructure part. Move or add its proof marker to `internal/testsupport/deps_test.go`, whose assertion must continue to reject imports from other repository internal packages while permitting the standard library and its own subpackages. Ensure both topic paths remain bounded by their owning domain paths and no generated extension becomes coverage-eligible.

- [ ] **Task 6.2: Finish documentation and examples from authored sources.** Update architecture, glossary, testing, configuration reference, working-with-awf, workflow/agent guidance, CLI help, and relevant examples for the final behavior. Document classification precedence, request/effective attribution, artifact roles, concise/full semantics, proof/touches relevance, ADR authority labels, topic budget, runner exclusions, and snapshot guarantees without duplicating ADR rationale. Ensure template prose remains coherent with missing optional values. Run `./x sync`; stage all authored inputs and generated outputs together. `./x check` must be clean, the Sundial example must have no advisory notes, and `./x gate` must report `prose-gate: clean`; do not embed the prohibited punctuation glyphs in a search command or documentation example.

- [ ] **Task 6.3: Apply the final operations and freeze ADR and plan.** Append the final Applied event with the next state sequence containing, in declaration order, `add rendering/adapter-outputs:generated-adapter-runtime-ownership` and `add tooling/test-infrastructure:test-support-leaf-boundary`, then append the Implemented status event with ADR-0144's frozen digest. Change ADR frontmatter to `Implemented`. Under this plan's Notes, record concrete deviations or the exact line `- No implementation deviations from ADR-0144 or this plan.`; change plan frontmatter to `Implemented`. Run `./x sync` again and stage the ADR, plan, ownership metadata/claims/proofs/config, docs, `docs/decisions/INDEX.md`, rendered topic/domain docs, runners, and locks. Run `go run ./cmd/awf check --staged`; it must report every ADR-0144 operation Applied, no Remaining operations, valid provenance/backing, and clean staged coverage. Run `./x gate`; it must pass. Commit:

  ```commit
  feat(awf): complete agent-oriented context navigation (implements 0144)
  ```

## Verification

- Run `git status --short`; it must be empty after the final commit.
- Run `./x check`; it must report clean and the example adopter must emit no advisory notes.
- Run `./x gate`; all Go tests, 100% statement coverage, Pi extension tests, vet, lint, dead-code, pin, and prose gates must pass.
- Run `go run ./cmd/awf context internal/project/context.go`; it must show one concise request/path group, its classification, directly relevant claims, and explicit omitted-claim drilldowns.
- Run `go run ./cmd/awf context --full internal/project/context.go`; it must show the complete applicable claims, backing/proof sites, references, scopes, matched paths, pending operations, and artifact navigation without truncation.
- Run `go run ./cmd/awf context docs/decisions/0144-agent-oriented-current-state-context-and-cli-navigation.md`; it must label ADR prose as history rather than current authority and show every operation Applied with a state sequence.
- Run `go run ./cmd/awf topic tooling/cli --coverage`; it must show domain paths and topic paths separately, state that both match, and show concrete matched paths/marker sites without Cartesian effective pairs.
- Run `go run ./cmd/awf context --full --uncovered`; it must exit with a usage error and must not open or mutate the project.
- Run `go run ./cmd/awf context --staged --json internal/project/context.go` with a deliberately dirty but unstaged config/topic edit, then discard only that deliberate edit; output must remain derived from the index universe.

## Notes

- ADR-0144 remains the design authority; this plan fixes implementation sequencing and verification only.
- State-sequence numbers and the frozen content digest are intentionally obtained from the staged checker at execution time rather than hard-coded measurements.
