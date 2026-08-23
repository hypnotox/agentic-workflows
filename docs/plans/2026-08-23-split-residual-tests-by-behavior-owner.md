---
format: plan-v2
date: 2026-08-23
adrs: []
status: Proposed
---
# Plan: Split Residual Tests by Behavior Owner

## Goal

Split the five residual mixed-ownership test aggregates into direct behavior-owner homes while preserving every top-level test name, observable behavior, package-local fixture ownership, package runtime and isolation, and 100% statement coverage. Do not change production behavior or architecture, coverage policy, compatibility behavior, historical comments, or cohesive large tests outside the five named aggregates. Current guidance, generated copies, and proof pointers that name a deleted aggregate remain currency work inside the same transaction.

## Architecture summary

This is a file-only test reorganization over `internal/project/spine_test.go`, `internal/project/project_test.go`, `internal/project/check_test.go`, `internal/project/staged_test.go`, and `cmd/awf/run_test.go`. Tests move within their existing Go packages to files named for the semantic owner that a maintainer would change with the behavior. Package-internal fixtures used by multiple behavior owners move to one explicit package-local `_test.go` helper home; fixtures used by one owner remain local. Nothing moves to `internal/testsupport` merely to shrink a file.

The protected owner map is:

- rendered agents, workflow templates, publication-safe fallbacks, agents-guide templates, and documentation templates own the former spine oracles;
- project opening, rendering, drift, resident outputs, publication, initialization, repository checks, staged checks, audit inputs, and current-state lifecycle behavior own the former project aggregates;
- command help, dispatch, sync composition, initialization, versioning, listing, checking, publishing, upgrade, exit, and gate behavior own the former command aggregate.

Exact move choreography may merge or split the proposed destination files when ownership stays equally direct, the helper home remains singular and package-local, and the protected census and exclusions hold. The change must not add or alter a production export, seam, global, implementation, or dependency. It must not edit coverage configuration or `// coverage-ignore` policy. It excludes cohesive large files, especially `internal/testsupport/thin_command_composition_test.go`, audit history, upgrade journal, worktree manager, publisher target, Git walk, effort memory operations, and already focused command effort/check tests.

The immutable baseline is exact base `822d04dfb37baa9d892cc4c454fc9970d3fd6bcf`. Sorted `go test -list '^Test'` package census hashes are `e0d928e4a66646ec018040ca8a8b95ae486b71218031fba31b7c73ffeb2c84de` for `./internal/project`, `7a812c25f0a2c1b618d52bf1924e036532d671de7301daa9ca213c36343a8465` for `./cmd/awf`, and `2a51927101300dac58c30700faf1423b3ce1bcbb6f3313cef7e71e2eab2536e9` for `./internal/testsupport`. These hashes are exact preservation oracles, not target counts. For each package, write `go test -list '^Test' "$pkg"` to a temporary raw file and require that command to succeed before `LC_ALL=C grep '^Test' "$raw" | LC_ALL=C sort > "$names"`; hash only the names file and remove both temporary files. Reuse that exact procedure in every census check.

A second temporary oracle protects content that a name hash cannot see. Use Go's parser and formatter to inventory every top-level `FuncDecl` and non-import `GenDecl` from the five aggregate files at the immutable base, key each declaration by its declared identifiers, and compare its canonical formatted bytes with exactly one working-tree declaration. This comparison must cover test bodies, subtests, helper bodies, fixture types, constants, and variables while intentionally ignoring only file placement and import declarations. Review ownership and import-only changes separately, and do not retain generalized test machinery.

Execution runs mutable package tests sequentially, compares ordinary runtime to the recorded baseline range, and treats shuffled duration as diagnostic while requiring shuffled correctness and isolation. Terminal independent implementation assurance must settle over the exact final candidate tip before integration or lifecycle closure.

**Plan flexibility.**

The protected-contract rule in the workflow document governs what a plan may not change. The plan records the best known route at authoring time, not a binding implementation choreography. A commit-capable owner may merge, split, reorder, add, remove, or replace recorded route detail while the protected contract holds. A path omitted from the plan is not alone a reason to stop, and a stale listed path need not be touched. Reapproval is required only when the protected contract would change or an unresolved material decision appears.

Reconcile a Proposed plan only when another phase or reviewer could rely on stale material instructions. Inconsequential and independently local edits require no deviation record. A delegated owner reports material cross-owner revisions for parent reconciliation. A helper remains confined to its assigned paths and gains no scope, commit, review, checkpoint, handoff, or outcome authority from route flexibility.


## Phase 1: Give rendered-template tests direct owners

**Execution mode: inline.**

Advances: ["exact-census", "boundary-preservation", "runtime-isolation"]
Completes: ["template-owner-oracles"]

### Task 1.1: Split the template spine by semantic owner
Kind: batch
Paths: ["internal/project/spine_test.go", "internal/project/template_test_helpers_test.go", "internal/project/agent_template_test.go", "internal/project/maintainable_workflow_template_test.go", "internal/project/plan_execution_workflow_template_test.go", "internal/project/diagnostic_workflow_template_test.go", "internal/project/authoring_workflow_template_test.go", "internal/project/review_workflow_template_test.go", "internal/project/effort_workflow_template_test.go", "internal/project/publication_safe_template_test.go", "internal/project/agents_doc_template_test.go", "internal/project/documentation_template_test.go", "internal/publisher/catalog_sweep_test.go", ".awf/skills/tdd.yaml", ".awf/skills/parts/debugging/debugging-surfaces.md", ".pi/skills/awf-tdd/SKILL.md", ".pi/skills/awf-debugging/SKILL.md", ".claude/skills/awf-tdd/SKILL.md", ".claude/skills/awf-debugging/SKILL.md", ".awf/awf.lock"]
Post-check: Apply the deterministic declaration and package-census procedures from the Architecture summary to the original spine, then prove the working tree assigns each declaration exactly once to a direct owner, leaves no emptied aggregate, retains exact declaration content and package names, and passes sequential ordinary and fixed-seed shuffled `./internal/project` plus focused `./internal/publisher` tests. Run render and drift checks after the authored guidance changes; the generated copies and lock must be current.

Move shared rendering, layout-default, leak, and ordering fixtures to one package-local helper home. Partition the oracles into rendered agent contracts; maintainability, durable-oracle, clean-integration, direct-execution, context, and verification workflow contracts; planning and execution workflows; diagnostic workflows; authoring and ADR lifecycle workflows; review and retrospective workflows; effort and memory workflows; publication-safe fallbacks; agents-guide templates; and documentation templates. Keep fixture types or helpers local when only one destination consumes them. Preserve every assertion, test name, subtest, environment control, and golden input without strengthening or weakening behavior.

Update the publisher catalog-golden completeness test to scan the new behavior-owner files while preserving its forward, reverse, orphan, and non-artifact checks. Replace current TDD and debugging guidance references to the deleted spine through their authored sources and render the Pi and Claude outputs.

### Phase close

The template spine is absent, each former oracle has one direct behavior-owner home, the internal/project census is byte-for-byte equivalent to the immutable baseline, and focused sequential ordinary and shuffled tests are green.

```commit
test(code-design): split template oracles by owner
```

## Phase 2: Give project, check, and staged tests direct owners

**Execution mode: inline.**

Advances: ["exact-census", "boundary-preservation", "runtime-isolation"]
Completes: ["project-owner-oracles"]

### Task 2.1: Split project lifecycle and publication tests
Kind: batch
Paths: ["internal/project/project_test.go", "internal/project/project_test_helpers_test.go", "internal/project/commitpolicy_test.go", "internal/project/adr_project_test.go", "internal/project/resident_migration_sync_test.go", "internal/project/project_open_test.go", "internal/project/sync_render_test.go", "internal/project/layout_test.go", "internal/project/drift_test.go", "internal/project/resident_output_sync_test.go", "internal/project/publication_sync_test.go", "internal/project/scaffold_test.go", "internal/publisher/sync.go"]
Post-check: Enumerate the original project aggregate at the immutable base and prove each top-level test and helper has exactly one owner, the aggregate is absent, the internal/project census matches the immutable hash, and focused sequential ordinary and shuffled package runs pass.

Move broadly reused scaffold, layout, Git, lock, config-path, and raw-file fixtures to one explicit package-local helper home. Assign the tests to commit-policy projection; project ADR creation and branch behavior; resident migration and retired-plan resync; project opening and section-override validation; render and output-plan sync; fixed layout; drift; resident-root pruning and confinement; publication reporting, symlink, classification, and backup behavior; and initialization refusal. In the same transaction that removes `project_test.go`, update only the stale proof-location comment in `internal/publisher/sync.go` to the direct publication owner; this is a current proof pointer, not historical-comment cleanup or a production behavior change. Preserve local divergent setup instead of manufacturing tables or shared helpers.

### Task 2.2: Split repository-check tests by checker owner
Kind: batch
Paths: ["internal/project/check_test.go", "internal/project/check_test_helper_test.go", "internal/project/check_pitfalls_test.go", "internal/project/check_glossary_test.go", "internal/project/operation_state_check_test.go", "internal/project/tag_vocabulary_check_test.go", "internal/project/adr_link_check_test.go", "internal/project/pending_adr_check_test.go", "internal/project/generated_tracking_check_test.go", "internal/project/plan_validation_check_test.go", "internal/project/plan_commit_check_test.go", "internal/project/plan_artifact_check_test.go", "internal/project/check_result_test.go", "internal/project/notes_test.go", "internal/project/initialization_check_test.go", "internal/project/agents_doc_budget_test.go"]
Post-check: Enumerate the original check aggregate at the immutable base and prove each test, helper, and fixture constant is assigned exactly once, the aggregate is absent, no checker result or diagnostic assertion changes, the internal/project census matches the immutable hash, and focused sequential ordinary and shuffled package runs pass.

Put shared corpus, topic, skill, and plan derivation fixtures in the existing project check helper home. Assign tests to pitfall checks, glossary checks, operation-state derivation, tag vocabulary, ADR links, pending ADRs, generated tracking, prepared-plan validation, commit diagnostics, V2 plan artifacts, check aggregation, advisory notes, initialization reporting, and guide-size boundaries. Keep owner-specific configuration fixtures beside their tests.

### Task 2.3: Split staged and audit tests by immutable-universe owner
Kind: batch
Paths: ["internal/project/staged_test.go", "internal/project/staged_test_helpers_test.go", "internal/project/staged_plan_test.go", "internal/project/staged_drift_test.go", "internal/project/staged_drift_compat_test.go", "internal/project/commit_authorization_test.go", "internal/project/audit_inputs_test.go", "internal/project/incremental_adr_lifecycle_test.go", "internal/project/version_test.go"]
Post-check: Enumerate the original staged aggregate at the immutable base and prove each test and fixture has exactly one owner, the aggregate is absent, staged and historical universes retain their assertions and isolation, the internal/project census matches the immutable hash, and focused sequential ordinary and fixed-seed shuffled package runs pass.

Move only genuinely shared staged snapshot, open, and lock fixtures to one package-local staged helper home. Assign tests to staged plan parsing; staged drift, index, HEAD, lock, transition, and working-tree isolation; historical compatibility; commit authorization; range and audit transitions; incremental ADR lifecycle pairs; and initialized-version immutability.

### Phase close

All three internal/project aggregates are absent, their tests and fixtures have direct semantic homes, the package census is exactly preserved, and focused sequential ordinary and shuffled tests remain green without production or coverage-policy changes.

```commit
test(code-design): split project oracles by owner
```

## Phase 3: Give command tests direct owners and prove whole-change preservation

**Execution mode: inline.**

Completes: ["command-owner-oracles", "exact-census", "boundary-preservation", "runtime-isolation", "terminal-assurance"]

### Task 3.1: Split command dispatch and operation tests
Kind: batch
Paths: ["cmd/awf/run_test.go", "cmd/awf/test_helpers_test.go", "cmd/awf/projectstate_test.go", "cmd/awf/command_stage_test.go", "cmd/awf/sync_composition_test.go", "cmd/awf/init_test.go", "cmd/awf/version_test.go", "cmd/awf/global_help_test.go", "cmd/awf/dispatch_test.go", "cmd/awf/list_add_test.go", "cmd/awf/failure_paths_test.go", "cmd/awf/check_test.go", "cmd/awf/publishing_test.go", "cmd/awf/initrender_test.go", "cmd/awf/upgrade_test.go", "cmd/awf/main_test.go", "cmd/awf/gate_test.go"]
Post-check: Enumerate the original command aggregate at the immutable base and prove each top-level test and helper has exactly one owner, the aggregate is absent, command behavior, help, streams, bypasses, exit mapping, and operation boundaries are unchanged, the cmd/awf census matches the immutable hash, and focused sequential ordinary and fixed-seed shuffled command tests pass.

Move the genuinely package-shared minimal-config, initialization, and scaffold fixtures to one command-local helper home. Assign tests to resident-root resolution; command-stage deadlines; sync loader, composition, and reporting; initialization; initial-version immutability; help and usage; profile gating and dispatch; add and list; bare-directory failures; check failures; publishing; init-render authority; upgrade; the single `os.Exit` contract; and the schema gate. Keep AST composition helpers local to sync composition and the Git opener local to resident-root tests.

### Task 3.2: Prove exact census, runtime, isolation, and scope preservation
Kind: batch
Paths: ["internal/project", "cmd/awf", "internal/testsupport", "docs/plans/2026-08-23-split-residual-tests-by-behavior-owner.md"]
Post-check: From a candidate tree containing only the intended phase changes, run the exact successful census and declaration-content procedures from the Architecture summary for `./internal/project`, `./cmd/awf`, and `./internal/testsupport`; each package digest must equal its immutable baseline hash and every moved declaration must match exactly. Run each package sequentially through ordinary uncached tests and the fixed seed `1771736407` shuffled tests, then run `go test ./...` and the unmodified `./x gate`; all commands must succeed, the gate must report 100% statement coverage, ordinary package runtime must remain consistent with the recorded baseline range absent a reasoned environmental variance, and shuffled runs must show no isolation failure. Inspect the full implementation diff and source probes to prove it contains only test reorganization, required current guidance/render/lock currency, the one proof-pointer comment, and mutable plan Notes; it contains no production behavior, coverage configuration or `// coverage-ignore` change, new package-level seam or global, new export, generalized testsupport API, excluded cohesive aggregate, compatibility cleanup, or historical-comment cleanup. Record the exact commands, outcomes, runtime comparison, and any authority-permitted route adjustment in Notes before the closing commit.

Preserve the package test census exactly rather than compensating for a missing oracle with a renamed or replacement test. Review tests whose names contain conjunctions in context; retain an inseparable transactional oracle, or split it only if exact census preservation is explicitly reapproved. Do not optimize for file length. A large focused destination is valid when one semantic owner explains it.

### Phase close

The command aggregate is absent, all five source aggregates have direct behavior-owner homes, exact declaration content, package censuses, and behavior boundaries are preserved, sequential ordinary and shuffled evidence is green, and the full Go suite and 100% gate pass. Create the closing candidate commit, then obtain independent implementation review over that exact tip. If review settlement changes the tip, land the focused settlement, rerun affected evidence and the gate, and renew assurance once over the final tip. This phase and `terminal-assurance` complete only when that exact final tip is settled; integration and lifecycle mutation remain later authority.

```commit
test(code-design): split command oracles by owner
```

## Definition of done

- `dod: template-owner-oracles` Every former spine test is directly findable under its rendered agent, workflow, publication-safety, agents-guide, or documentation template owner, with one clear package-local fixture home.
- `dod: project-owner-oracles` Every former project, check, and staged aggregate test is directly findable under its project operation, checker, immutable-universe, audit, lifecycle, or initialization owner, with shared helpers housed once and owner-local setup kept local.
- `dod: command-owner-oracles` Every former command aggregate test is directly findable under its help, dispatch, operation, initialization, version, list, check, publishing, upgrade, exit, or gate owner.
- `dod: exact-census` The exact successful package test-list procedure produces the immutable baseline digests for internal/project, cmd/awf, and internal/testsupport; the temporary canonical declaration comparison matches every moved test, subtest, assertion, helper, fixture, constant, and variable; and no observable oracle or coverage is lost.
- `dod: boundary-preservation` The implementation diff changes test organization, required current guidance and rendered currency, one proof-pointer comment, and mutable plan Notes only; it changes no production behavior or architecture, production export/seam/global, generalized testsupport API, coverage policy, compatibility behavior, historical-comment scope, or excluded cohesive large test.
- `dod: runtime-isolation` Sequential ordinary and fixed-seed shuffled focused runs, followed by `go test ./...`, remain green; ordinary runtime stays consistent with the recorded baseline absent explained environmental variance, and shuffled execution exposes no isolation failure.
- `dod: terminal-assurance` The unmodified gate reports 100% statement coverage and an independent implementation review settles against the owner map, exclusions, census, behavior, runtime, isolation, and verification evidence before integration or terminal lifecycle mutation.

## Notes

Apply the plan-flexibility rule above when recording deviations. Delegated owners report material cross-owner revisions rather than editing the plan; the parent supplies the report to phase review and reconciles required plan changes with findings in one focused post-review settlement commit before checkpointing or later execution. Record spike answers, follow-ups, and findings surfaced during implementation.

Authoring baseline on Go 1.26.6 linux/amd64 with 12 CPUs: three isolated sequential ordinary passes reported project `51.888s`, `50.154s`, `49.424s`; command `49.689s`, `50.266s`, `52.132s`; and testsupport `2.381s`, `1.798s`, `1.708s`. Fixed-seed shuffled runs at seed `1771736407` reported `78.481s`, `171.096s`, and `9.167s` respectively and all passed. Shuffled duration is order-sensitive diagnostic evidence, not a strict performance threshold.

Phase 3 candidate evidence on Go 1.26.6 linux/amd64 with 12 CPUs:

- The successful raw-list and C-locale sort procedure produced 438 project tests at `e0d928e4a66646ec018040ca8a8b95ae486b71218031fba31b7c73ffeb2c84de`, 317 command tests at `7a812c25f0a2c1b618d52bf1924e036532d671de7301daa9ca213c36343a8465`, and 62 testsupport tests at `2a51927101300dac58c30700faf1423b3ce1bcbb6f3313cef7e71e2eab2536e9`.
- The temporary Go parser and formatter oracle matched all 75 spine, 54 project, 49 check, 39 staged, and 56 command top-level non-import declarations exactly once with identical canonical bytes. All five aggregates are absent.
- Sequential uncached ordinary runs passed for project at `55.452s` package and `55.797s` wall, command at `51.067s` package and `51.606s` wall, and testsupport at `2.037s` package and `2.343s` wall. The modest project variance from the authoring range is consistent with the loaded Phase 1 and Phase 2 runs and has no correctness or isolation symptom.
- Fixed-seed `1771736407` shuffled runs passed for project at `53.529s` package and `53.910s` wall, command at `49.018s` package and `49.543s` wall, and testsupport at `2.013s` package and `2.249s` wall. `go test -count=1 ./...` passed in `78.932s` wall.
- The full implementation-range path and source probes found only test reorganization, the authorized current TDD and debugging reference currency and generated lock transaction, the catalog scanner adjustment, this Notes evidence, and the authorized publisher proof-pointer comment. They found no production behavior, export, seam, global, dependency, coverage-policy or `// coverage-ignore` change, generalized testsupport API, compatibility or historical cleanup, or excluded cohesive test change.
- The pre-candidate unmodified gate passed at 100% statement coverage, `21358/21358`, with deadcode and pincheck clean and only the three pre-existing `Uid` advisories. No material route deviation occurred.

Plan review dispositions:

- Reasoned, phase gateability: add the publisher catalog-golden census to Phase 1 and preserve its forward, reverse, orphan, and non-artifact completeness checks across the new owner files.
- Reasoned, terminal assurance: make the final phase close produce a candidate commit, review that exact tip independently, and require renewed evidence and assurance after any settlement change before the phase completes.
- Reasoned, documentation currency: include only the current TDD/debugging references, their rendered outputs and lock, and the publisher proof-pointer comment that would otherwise name deleted aggregates; historical records remain untouched.
- Reasoned, oracle strength: pair the exact package-name hashes with a temporary canonical Go declaration comparison so unchanged names and coverage cannot conceal weakened bodies, subtests, fixtures, or helpers.
