---
date: 2026-07-28
adrs: [167]
status: Proposed
---
# Plan: Remove workflow telemetry and mandatory Pi routing

## Goal

Implement [ADR-0167](../decisions/0167-remove-workflow-telemetry-and-mandatory-pi-routing.md): remove telemetry, session assignment, and Pi workflow routing; make the skill catalog advisory; and preserve only effort, memory, worktree, native Pi skills, and effort commands. It does not alter historical ADRs or implemented plans, recover destroyed local telemetry/assignment bytes, or introduce replacement metrics.

## Architecture summary

First, ship generation 21 as a narrow, destructive, retryable resident-root migration. Next, replace the split workflow metadata with one complete catalog profile and keep the pre-cutover rendered topology working. The final change is one atomic product cutover: it removes the telemetry/assignment/router implementation and outputs, changes handoff and all workflow prose, updates generated/documented current state, and applies the remaining ADR operations together. It cannot be sliced without temporary compatibility machinery: the existing telemetry extension is simultaneously the dashboard, assignment-selection host, and hidden-body router, while its descriptor, resident roots, CLI joins, template claims, generated root and Sundial outputs, and Pi tests currently assert that one installed surface.

## File structure

- **Created:** `internal/migrate/remove_workflow_residents.go` and `internal/migrate/remove_workflow_residents_test.go`; focused catalog, project, effort, CLI, and Pi test files only where a deleted test cannot be repurposed.
- **Modified:** `internal/migrate/migrate.go`, `internal/project/project.go`, `internal/project/render.go`, `internal/project/local.go`, `internal/project/target.go`, `internal/catalog/catalog.go`, `internal/catalog/workflow.go`, `internal/catalog/standard.go`, their tests, `internal/effort/{paths,service,store,types}.go`, `cmd/awf/{dispatch,effort,main,uninstall}.go`, `internal/clispec/clispec.go`, templates and authored `.awf/` sources named below, `changelog/CHANGELOG.md`, ADR-0167, this plan, and regenerated root/Sundial outputs and locks.
- **Deleted:** `internal/telemetry/`; `cmd/awf/metrics.go` and telemetry tests; `internal/effort/assignment.go`, `internal/effort/assignment_test.go`; `templates/pi/awf-telemetry/`; `templates/pi/awf-workflow/SKILL.md.tmpl`; `tools/pi-extension-test/tests/{protocol,session-v1,telemetry-registration,telemetry-writer}.test.ts`; active telemetry topic metadata and part; generated `.awf/{metrics,assignments}/.gitignore`, `.pi/extensions/awf-telemetry/`, `.pi/awf-workflows/`, `.pi/skills/awf-workflow/`, and matching `examples/sundial/` paths.

## Phase 1: Ship destructive generation-21 migration support

- [ ] **Task 1.1: Register generation 21 and remove only obsolete resident roots.** In `internal/migrate/remove_workflow_residents.go`, implement `applyRemoveWorkflowResidents(root string, out io.Writer) error`; register it after generation 20 as `{To: 21, Name: "remove-workflow-residents", Apply: applyRemoveWorkflowResidents}` in `internal/migrate/migrate.go`; update `ConfigForCurrentSchema` only if its config-byte dispatch requires an explicit generation-21 arm. Set the release floor in the single project-version authority: `project.Version = 0.25.0` and `minVersionBySchema[21] = 0.25.0` in `internal/project/project.go`, with associated version/schema tests.

  Resolve the confined primary control root through the existing `internal/git` control-root API, never from an invoking linked worktree. Implement a private dependency-injected helper taking `lstat` and `removeAll` functions; `applyRemoveWorkflowResidents` passes `os.Lstat` and `os.RemoveAll`, while tests inject a deterministic second-root failure without permission assumptions. For each literal resident name, in this order, `metrics`, then `assignments`: inspect `<primary>/.awf/<name>` with `Lstat`; absent means print `remove-workflow-residents: <name> already absent` and continue; a symlink or non-directory root returns a visible error before recursive deletion and does not follow it; a real directory is removed recursively and, only after successful removal, prints `remove-workflow-residents: <name> removed`. Removal failure returns the underlying error, preserves the other root's actual state, and a later `awf upgrade` retries the remaining root and reports its current result. Do not preserve `.gitignore`, descendants, legacy ledgers, or assignments: all bytes below these roots are intentionally disposable. Do not create either root, mutate effort/memory/worktree residents, or make normal rendering retain the removed roots.

  Add table-driven tests in `internal/migrate/remove_workflow_residents_test.go` for both roots absent, each root present with nested files and `.gitignore`, metrics removed before assignments, partial failure then retry, unsafe symlink root refusal without touching its target, non-directory refusal, output order, current-generation no-op, lock stamping, ordinary schema/binary gate refusal for an older binary, and primary-root behavior from a linked worktree. Extend `internal/migrate/migrate_test.go`, `internal/project/version_test.go`, and the existing control-root safety tests for registry order, release floor, and no stale resident constants. Build an upgrade binary and run:

  ```sh
  gofmt -w internal/migrate/remove_workflow_residents.go internal/migrate/remove_workflow_residents_test.go internal/migrate/migrate.go internal/migrate/migrate_test.go internal/project/project.go internal/project/version_test.go
  go test ./internal/migrate ./internal/project ./internal/git
  go test ./internal/migrate -run 'TestRemoveWorkflowResidentsMigration|TestRun'
  ```

  All commands exit zero. Exercise the migration registry directly against temporary fixture roots, not the top-level `awf upgrade` command whose mandatory post-migration render still owns the pre-cutover topology in this phase. Each fixture lock records schema 21 and `0.25.0`, output contains one removed/already-absent line for each root in the stated order, and a registry retry exits zero without recreating either root. Do not run upgrade against this repository root or `examples/sundial` in Phase 1: before the Phase 3 cutover, rendering would recreate the retired sentinels. Both adopted trees deliberately remain at schema 20 until Phase 3.

- [ ] **Task 1.2: Apply the first ADR operation and commit the migration.** Update only the retained migration claim in `.awf/topics/parts/config/migrations-and-locks/current-state.md` so it describes generation 21 destructive cleanup and its safe, retryable refusal branch; retain the claim slug `workflow-telemetry-config-migration` as an update, not a new claim. Put `// invariant: config/migrations-and-locks:workflow-telemetry-config-migration` on `TestRemoveWorkflowResidentsMigration` in `internal/migrate/remove_workflow_residents_test.go`. Update `.awf/domains/parts/config/current-state.md`, `.awf/docs/parts/development/dependencies.md` if it names schema generations, and the release/config-reference authored source only where it names telemetry residents or generation 20 as current.

  Change ADR-0167 from Accepted to Implementing, append the checker-derived digest status event, then append one Applied batch with checker-derived sequence and exactly this operation: `update config/migrations-and-locks:workflow-telemetry-config-migration`. Run `./x render` so root and `examples/sundial/` locks and all affected generated references settle, then run `./x check`. Explicitly stage only:

  ```sh
  git add internal/migrate/remove_workflow_residents.go internal/migrate/remove_workflow_residents_test.go internal/migrate/migrate.go internal/migrate/migrate_test.go internal/project/project.go internal/project/version_test.go internal/git/controlroot.go internal/git/controlroot_test.go .awf/topics/parts/config/migrations-and-locks/current-state.md .awf/domains/parts/config/current-state.md .awf/docs/parts/development/dependencies.md docs/development.md docs/config-reference.md .awf/awf.lock examples/sundial/.awf/awf.lock docs/decisions/0167-remove-workflow-telemetry-and-mandatory-pi-routing.md docs/decisions/INDEX.md
  ./awf check --staged
  ./x gate
  ```

  Before staging, run `git diff --name-only` and require its complete set to be exactly the listed paths plus the known generated lock/index paths already listed; refuse to stage if any other path appears. Do not use a catch-all generated-path rule. `./awf check --staged` must report the first operation Applied and every later ADR-0167 operation Remaining; both checks exit zero. Commit:

  ```commit
  feat(config): remove obsolete workflow residents (applies 0167 batch)
  ```

## Phase 2: Make the metadata advisory while retaining the old topology

- [ ] **Task 2.1: Replace split workflow metadata with `WorkflowProfile`.** In `internal/catalog/catalog.go`, delete `WorkflowMapping`, `SkillSpec.Chain`, and `SkillSpec.Trigger`; add `WorkflowProfile` with `Kind WorkflowKind`, `Purpose string`, `UsuallyFollows []string`, and `CommonFollowUps []string`, and add `Profile WorkflowProfile` to `SkillSpec`. Keep `SkillSpec.RequiresSkills` structurally available, but set it to empty for every standard skill. Leave `AgentSpec.RequiresSkills` unchanged because it is an artifact dependency, not a workflow relationship.

  Replace `ValidateWorkflowMappings` and `WorkflowMappingsForSkills` in `internal/catalog/workflow.go` with validation and enabled-profile projection helpers. Every standard skill must have a valid `chain`, `task`, or `support` kind and a concise nonblank purpose; each advisory list must be declaration-order preserving, contain only existing catalog skill names, reject self references and duplicates within its own list, and never require that an advisory neighbor is enabled. Do not build closure edges from either advisory list. Update `internal/catalog/standard.go` so every current workflow mapping, chain flag, trigger, and workflow-to-workflow requirement becomes exactly one profile: preserve useful ordering as `UsuallyFollows` and `CommonFollowUps`, and use the former trigger text as the task purpose where applicable. Chain and support skills also receive a purpose. No standard `RequiresSkills` entry remains.

  In `internal/project/local.go`, synthesize local skills with `Profile.Kind: catalog.WorkflowTask`, `UsuallyFollows:nil`, `CommonFollowUps:nil`, and purpose equal to `sidecar.data.description` after strict string validation and `strings.TrimSpace`; use `A project-local skill.` when the value is non-string, empty, or whitespace-only. Non-string, empty, and whitespace-only descriptions take that fallback; no local skill gains structural requirements or advisory neighbors. Update `internal/project/project.go`, `internal/project/confighash.go`, `internal/project/render.go`, `internal/project/drift_test.go`, `internal/evals/chain_test.go`, and catalog/project tests to consume profiles instead of mappings/chain/trigger fields while retaining the current router and hidden-body behavior until Phase 3.

  Keep the existing task-only guide-row model and its rendered output byte-equivalent in this phase. Adapt its router/guide consumers to read `WorkflowProfile`, but do not introduce full catalog rows, profile prose, or advisory text until Phase 3. Test the old task-only rows remain byte-equivalent alongside profile validation, disabled advisory neighbor, local valid description, and local fallback purpose.

- [ ] **Task 2.2: Preserve old rendering behavior with the new authority and apply operations 2-4.** Do not change `templates/agents-doc/AGENTS.md.tmpl`, `templates/docs/workflow.md.tmpl`, guide/workflow transition prose, or their current-state parts/claims in this phase. Keep the installed Pi router contract and rendered topology byte-equivalent, adapting only its old router/guide consumers to `WorkflowProfile`. Do not add full catalog rows or advisory prose until Phase 3.

  Replace proof markers with: `// invariant: rendering/catalog-and-targets:enabled-set-closed` on a catalog/project test proving only enabled skill outputs are rendered; remove the proof for `exploration-skill-closure`; and put `// invariant: rendering/catalog-and-targets:requires-skills-exact` on a test proving standard skills have empty `RequiresSkills`, agent requirements remain validated, and advisory metadata, including standard-skill prose references, does not create a structural dependency or enablement closure; `AgentSpec.RequiresSkills` remains exact. The updated claims must match those markers and retain their existing slugs.

  Run:

  ```sh
  gofmt -w $(find internal/catalog internal/project internal/evals -type f -name '*.go')
  go test ./internal/catalog ./internal/project ./internal/evals
  ./x render
  ./x check
  ```

  Append one ADR Applied batch, in declaration order, with exactly:

  ```text
  update rendering/catalog-and-targets:enabled-set-closed
  remove rendering/catalog-and-targets:exploration-skill-closure
  update rendering/catalog-and-targets:requires-skills-exact
  ```

  Before staging, require `git diff --name-only` to contain only these exact categories: `internal/catalog/*.go`, `internal/project/{local,project,confighash,render,drift_test}.go`, `internal/evals/chain_test.go`, any existing test file changed under those three directories, templates needed solely by profile consumers, ADR-0167, and `docs/decisions/INDEX.md`. It must not contain guide/workflow templates, `.awf/` guide/workflow/current-state sources, or root/Sundial rendered topology outputs. Stage that verified set by its explicit paths; refuse if an unexpected path appears. Run `./awf check --staged`, requiring precisely operations 1-4 Applied in declaration order and the remainder Remaining, then `./x gate`; both exit zero. Commit:

  ```commit
  refactor(rendering): make workflow catalog advisory (applies 0167 batch)
  ```

## Phase 3: Atomic telemetry, assignment, and Pi routing cutover

All tasks in this phase are one coupled transaction and one commit. Do not commit an intermediate subset. A compatibility extension would itself have to retain telemetry descriptor/writer semantics, assignment storage and CLI joins, selected-session entries, router hidden bodies, and contradictory generated claims while the target outputs and docs state their removal. That temporary product machinery is explicitly rejected by ADR-0167.

- [ ] **Task 3.1: Delete telemetry and assignment product authority.** Delete `internal/telemetry/**`, `cmd/awf/metrics.go`, `cmd/awf/metrics_session_test.go`, all other `cmd/awf/*metrics*_test.go`, `internal/effort/assignment.go`, and `internal/effort/assignment_test.go`. Remove their imports, dispatch handlers, CLI grammar/help, main-command forwarding, config-reference entries, output declarations, resident constants, ownership/sweep/uninstall exceptions, and tests from `cmd/awf/{dispatch,effort,main,uninstall}.go`, `internal/clispec/{clispec,clispec_test}.go`, `internal/git/{controlroot,controlroot_test}.go`, `internal/project/{project,render,output_plan,sweep,install,currentstate,context_artifacts,example_wiring}_test.go`, `internal/snapshot/working_test.go`, and `internal/effort/{paths,service,store,types,paths_test,service_test,store_test,safety_test}.go` as applicable.

  `internal/effort.Service` must retain new/list/show/rename/memory/worktree/integrate/integrated/complete/abandon/reopen/repair behavior without reading assignments or returning `AssignedSessionIDs`; `awf effort` must reject `assign`, `unassign`, and `assignments` as unknown grammar. Resident-root planning, manifest, drift, discovery, sweep, uninstall, control-root names, and generated ignores must contain exactly efforts, memory, and worktrees. Add `resident-output-preservation` in `.awf/topics/parts/rendering/singletons-and-payloads/current-state.md` with a proof marker on a deterministic project test that establishes exactly those roots and verifies the generation-21 migration removes only metrics/assignments. Delete `.awf/topics/metadata/tooling/workflow-telemetry.yaml` and `.awf/topics/parts/tooling/workflow-telemetry/current-state.md`; remove the telemetry topic completely, never leave an empty topic. Remove `internal/telemetry/**` from `.awf/domains/tooling.yaml`.

- [ ] **Task 3.2: Render native Pi skills and simplify handoff.** Delete `templates/pi/awf-telemetry/`, `templates/pi/awf-workflow/SKILL.md.tmpl`, and `templates/pi/awf-workflow/`; remove telemetry protocol producers and outputs from `internal/project/target.go`; remove `routesWorkflows`, `HiddenWorkflowPath`, `WorkflowRouterPath`, `workflowRouterData`, routed-skill splitting, hidden rendering, and router data from `internal/project/{target,render,project}.go`. Pi's ordinary `Target.SkillPath` must render every enabled standard and synthesized local skill at `.pi/skills/<prefix>-<name>/SKILL.md`, with native progressive disclosure and the same effective enabled-set behavior as other targets. Disabled skills and disabling Pi must prune those paths; no router, `.pi/awf-workflows/`, or telemetry extension output remains.

  In `templates/pi/awf-handoff/index.ts.tmpl`, retain the single-tool preflight, queue, countdown/cancel, parent link, optional confined regular-file `.awf/memory/` path validation, kickoff, cleanup, and editor fallback. Delete `validateMemoryEffort`, selection request/restoration, `runAwf`, child assignment, custom selection entries, and all `Effort:` parsing/comparison. `buildKickoffWrapper` must say only to read the optional memory path before the immediate action. Add/adjust `tools/pi-extension-test/tests/handoff.test.ts` so regular confined memory succeeds, absent memory succeeds, symlink/directory/traversal/absolute paths fail, no awf process is invoked, and no selection entry is appended. Put native skill discovery assertions in the appropriate Go project rendering tests. Delete `tools/pi-extension-test/tests/{protocol,session-v1,telemetry-registration,telemetry-writer}.test.ts` and their fixtures/imports; retain `runner.test.ts`. Update TypeScript test discovery so the remaining generated root and Sundial extension files type-check and reach the required coverage floor.

- [ ] **Task 3.3: Rewrite workflow guidance and active documentation.** In every governed `templates/skills/*/SKILL.md.tmpl`, especially terminal sections, replace mandatory predecessor/successor/only-entry language with a recommendation grounded in `WorkflowProfile` purpose and advisory neighbors. Preserve mandatory controls inside a selected skill: approval check-ins, staged checks, `./x gate`, and scoped procedure requirements are not workflow transitions. Update `templates/{agents-doc/AGENTS.md.tmpl,docs/workflow.md.tmpl,docs/working-with-awf.md.tmpl,docs/architecture.md.tmpl,docs/testing.md.tmpl,docs/releasing.md.tmpl,partials/pi-minimum-runtime.md}` and their authored `.awf/parts/**`, `.awf/docs/parts/**`, `.awf/agents-doc.yaml`, `.awf/domains/parts/**`, `.awf/topics/parts/**`, and `docs/pitfalls.yaml` counterparts to remove active dashboard, metrics, assignment, selected effort, router, hidden-body, telemetry, and lifecycle-selection terminology while retaining legitimate effort state transitions and ADR lifecycle terms.

  In Phase 3, replace the task-only guide with catalog-derived ordered rows for every enabled standard and synthesized local skill: `- \`<prefix>-<name>\` (<kind>): <purpose>. Usually follows: ... Common follow-ups: ...`, omitting each empty optional clause. Update `changelog/CHANGELOG.md` under Unreleased with the shipped breaking change: generation-21 upgrade deletes local `.awf/metrics` and `.awf/assignments`; Pi uses native skills; `/awf-effort`, `awf_workflow`, metrics, and session assignment are gone. Do not edit historical ADRs or implemented plans. The guide must state that any enabled skill may be used when its purpose fits and never imply advisory relations are mandatory.

  Add proof markers to deterministic tests: `rendering/guide-and-doc-templates:guide-entry-point-routing`, `rendering/guide-and-doc-templates:working-memory-single-home`, `rendering/workflow-skill-templates:mandatory-approval-boundaries`, `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`, `rendering/workflow-skill-templates:workflow-transitions-advisory`, `rendering/pi-workflows:pi-session-handoff-lifecycle`, `rendering/pi-workflows:pi-session-handoff-public-contract`, `rendering/pi-workflows:pi-session-handoff-workflow`, `rendering/pi-workflows:pi-native-workflow-skills`, `rendering/pi-runtime:pi-extension-target-render`, `rendering/pi-runtime:pi-minimum-runtime`, and `rendering/pi-runtime:pi-real-runtime-smoke`. The advisory test must scan rendered guide/skills for profile-derived relationship text and absence of mandatory predecessor/successor wording. The native-Pi test must prove normal paths, enabled/disabled cleanup, router/hidden-output absence, and parity with other targets. Remove markers for every removed claim rather than leaving orphaned proof comments.

- [ ] **Task 3.4: Apply remaining operations, regenerate both adopters, and make the final commit.** Update current-state claims and append one checker-derived ADR Applied event containing every remaining operation in this exact declaration order:

  ```text
  update tooling/effort-management:effort-record-authority
  remove tooling/effort-management:session-effort-assignment
  remove tooling/workflow-telemetry:event-protocol-and-ledger
  remove tooling/workflow-telemetry:privacy-integrity-and-retention
  remove tooling/workflow-telemetry:canonical-projections-and-diagnostics
  update tooling/cli:effort-command-contract
  remove tooling/cli:metrics-command-contract
  update rendering/singletons-and-payloads:memory-gitignore-always-on
  remove rendering/singletons-and-payloads:workflow-telemetry-governed-outputs-and-resident-data
  add rendering/singletons-and-payloads:resident-output-preservation
  update rendering/project-output-plan:output-plan-complete
  update rendering/guide-and-doc-templates:guide-entry-point-routing
  update rendering/guide-and-doc-templates:working-memory-single-home
  update rendering/workflow-skill-templates:mandatory-approval-boundaries
  update rendering/workflow-skill-templates:memory-checkpoint-chain-coverage
  add rendering/workflow-skill-templates:workflow-transitions-advisory
  remove rendering/adapter-outputs:pi-workflow-telemetry-runtime
  update rendering/pi-workflows:pi-session-handoff-lifecycle
  update rendering/pi-workflows:pi-session-handoff-public-contract
  update rendering/pi-workflows:pi-session-handoff-workflow
  remove rendering/pi-workflows:pi-lifecycle-enforcing-workflow-router
  add rendering/pi-workflows:pi-native-workflow-skills
  remove rendering/pi-workflows:pi-workflow-telemetry-public-contract
  update rendering/pi-runtime:pi-extension-target-render
  update rendering/pi-runtime:pi-minimum-runtime
  update rendering/pi-runtime:pi-real-runtime-smoke
  ```

  Run the focused deletion/cutover checks, then render both the repository and Sundial and verify generated deletions explicitly:

  ```sh
  gofmt -w $(find cmd/awf internal/effort internal/catalog internal/clispec internal/git internal/migrate internal/project -type f -name '*.go')
  go test ./cmd/awf ./internal/effort ./internal/catalog ./internal/clispec ./internal/git ./internal/migrate ./internal/project ./internal/snapshot
  ./x pi-test run
  go build -o /tmp/awf-schema-21 ./cmd/awf
  /tmp/awf-schema-21 upgrade
  (cd examples/sundial && /tmp/awf-schema-21 upgrade)
  ./x render
  test ! -e .awf/metrics && test ! -e .awf/assignments
  test ! -e .pi/extensions/awf-telemetry && test ! -e .pi/awf-workflows && test ! -e .pi/skills/awf-workflow
  test ! -e examples/sundial/.awf/metrics && test ! -e examples/sundial/.awf/assignments
  test ! -e examples/sundial/.pi/extensions/awf-telemetry && test ! -e examples/sundial/.pi/awf-workflows && test ! -e examples/sundial/.pi/skills/awf-workflow
  ./x check
  ```

  All commands exit zero. The cutover binary upgrades both adopted schema-20 trees before either render, so the migration deletes the two resident roots and the subsequent cutover render proves neither root is recreated. Run `git diff --name-only`; its verified affected set must contain only: deleted `internal/telemetry/`, `internal/effort/assignment.go`, `internal/effort/assignment_test.go`, `cmd/awf/metrics.go`, metrics tests, Pi telemetry/router templates, and the four named TypeScript tests; modified `cmd/awf/{dispatch,effort,main,uninstall}.go` and existing tests, `internal/effort/`, `internal/{catalog,clispec,git,migrate,project,snapshot}/` source/tests, `tools/pi-extension-test/tests/{handoff,runner}.test.ts`, templates, `.awf/` domain/topic/part sources, generated root and `examples/sundial/` outputs/locks, `changelog/CHANGELOG.md`, ADR-0167, this plan, and `docs/decisions/INDEX.md`; and deleted telemetry metadata/part and generated metrics, assignments, telemetry-extension, router, and hidden-workflow paths. Refuse to stage if any path outside that set appears. Stage each verified modification by its explicit path and each verified deletion with `git rm`; never use `git add -A`.

  Do not stage or run a staged check with ADR-0167 still Implementing after all operations are Applied. Instead append the final checker-derived Applied event and final status event, change ADR-0167 and this plan frontmatter to `status: Implemented`, then run `./x render`. Stage the status, claims, and generated files from the same verified set, run `./awf check --staged`, then run `./x gate` exactly once. The staged check must report all ADR-0167 operations Applied in declaration order and no stale telemetry/assignment topic/marker. Commit:

  ```commit
  feat(rendering): remove telemetry and Pi router (implements 0167)
  ```

## Verification

From a clean checkout at the final commit, `go test ./...`, `./x pi-test run`, `./x check`, and `./x gate` exit zero. `awf metrics`, `awf effort assign`, `awf effort unassign`, and `awf effort assignments` are rejected by CLI grammar; remaining effort commands work without assignment fields. A generation-20 checkout upgrades to schema 21, deletes only the two obsolete resident roots with one ordered report per root, refuses a symlinked root, and succeeds on retry after partial deletion. Root and Sundial contain ordinary Pi skill paths for each enabled skill and contain no telemetry extension, router skill, hidden workflow bodies, metrics resident root, or assignments resident root. The guide and rendered skills expose purpose/kind/advisory neighbors without mandatory transition language.

## Notes

The final status flip is intentionally in the atomic cutover commit. Historical records retain retired terminology as append-only history; only active authority and generated present-tense outputs change.
