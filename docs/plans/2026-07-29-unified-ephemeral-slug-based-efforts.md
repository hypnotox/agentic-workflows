---
date: 2026-07-29
adrs: [175]
status: Proposed
---
# Plan: Unified ephemeral slug-based efforts

## Goal

Implement [ADR-0175](../decisions/0175-unified-ephemeral-slug-based-efforts.md): replace UUID
lifecycle records and standalone memory with one immutable slugged, memory-owning effort; make
managed-worktree operations derive from Git topology; migrate legacy residents safely; and make the
complete generated agent workflow create, carry, integrate, review, retrospect, and finish that unit.

Non-goals are retained effort history, standalone memory, automatic integration or cleanup,
force-discard behavior, coordination locks, protocol-1 compatibility after upgrade, or changing the
deliberate subagent model-selection contract established by ADR-0173.

## Architecture summary

The implementation has two execution tranches separated by the current legacy managed worktree.
The first tranche resyncs onto the completed ADR-0173 implementation, accepts ADR-0175, replaces the
effort/worktree/CLI authority without advancing the project schema, moves citation and Pi handoff to
the owned memory path, and publishes complete generated workflow guidance. Those changes remain
compatible with the existing schema gate and close green in the legacy worktree.

After governed review of that tranche, the executor integrates it into the intended clean target
checkout, settles any divergent merge through staged check, gate, merge commit, and renewed review,
records integration using the still-current legacy command only for this transition, and removes the
legacy worktree/path/branch. The second tranche then runs in the target checkout: it adds the
journaled legacy-resident reset, advances the schema, removes the standalone memory resident root,
updates render/discovery/uninstall ownership, and closes ADR-0175. This ordering avoids making the
new schema gate require an upgrade that must refuse the very legacy worktree performing it.

`internal/effort` owns protocol-2 static residents and crash-durable publication/finish;
`internal/worktree` owns stateless native-Git topology mutations; `internal/upgrade` owns journaled
resident-tree quarantine and recovery; the project renderer owns only the efforts and worktrees
resident roots. Generated instructions come only from templates and `.awf/` parts. ADR-0175 claim
operations travel with the code and proof markers that make each claim true; the migration operation
is intentionally the final applied batch because its activation is the cutover boundary.

## File structure

- **Created:** `internal/migrate/unified_effort_residents.go`,
  `internal/migrate/unified_effort_residents_test.go`. An additional focused `_test.go` file under
  `internal/effort`, `internal/worktree`, `internal/upgrade`, `internal/project`, or `cmd/awf` is
  permitted only when a named fault matrix cannot remain cohesive in the existing same-package
  suites and only after this Proposed plan names the exact path and adds it to the phase staging
  command before the file is created.
- **Modified - effort and Git authority:** `internal/effort/{types,paths,store,memory,service,safeio}.go`,
  platform-specific safe-publication files and all affected `internal/effort/*_test.go`;
  `internal/worktree/{git,topology,manager}.go` and all affected `internal/worktree/*_test.go`;
  `internal/git/controlroot.go` and focused tests; `cmd/awf/{effort.go,effort_test.go,
  effort_worktree_test.go}`; `internal/clispec/{clispec.go,clispec_test.go}`; and
  `internal/project/topics_test.go`.
- **Modified - migration and resident ownership:** `internal/migrate/{migrate.go,
  remove_workflow_residents_test.go}` and focused tests;
  `internal/upgrade/{journal.go,journal_test.go,upgrade.go,upgrade_test.go}`;
  `cmd/awf/{upgrade.go,upgrade_test.go,main.go,run_test.go,check_test.go,checkgroup_test.go}`;
  `internal/project/{project.go,render.go,output_plan.go,install.go,currentstate.go,context.go,
  sweep.go,banner.go}` and their focused tests, including `banner_test.go`, `coverage_test.go`,
  `context_artifacts_test.go`, and `memory_test.go`; `templates/embed.go`; `.awf/config.yaml`, `.awf/awf.lock`,
  `examples/sundial/.awf/config.yaml`, and `examples/sundial/.awf/awf.lock`.
- **Modified - citation and handoff:** `internal/memorycite/{memorycite.go,memorycite_test.go}`;
  `cmd/awf/{memorygate.go,memorygate_test.go,commitgate.go,commitgate_test.go,checkgroup_test.go}`;
  `templates/pi/awf-handoff/index.ts.tmpl`; `tools/pi-extension-test/tests/handoff.test.ts`;
  `internal/project/target_test.go`; and generated root/Sundial Pi handoff outputs.
- **Modified - first-class agent guidance:** `templates/partials/{checkpoint-routine.md,
  checkpoint-approval.md}`; all applicable `templates/skills/{brainstorming,proposing-adr,
  adr-lifecycle,writing-plans,reviewing-plan,reviewing-plan-resync,reviewing-adr,executing-direct,
  executing-plans,subagent-driven-development,reviewing-impl,retrospective,debugging,bugfix,tdd,
  refactor-coupling-audit,exploring,roadmap-graduation}/SKILL.md.tmpl`;
  `.awf/skills/parts/retrospective/procedure.md`; `templates/agents-doc/AGENTS.md.tmpl`;
  `templates/docs/{workflow.md.tmpl,working-with-awf.md.tmpl}`; `internal/catalog/standard.go`;
  `internal/evals/chain_test.go`; and focused `internal/project/{spine,target,project}_test.go`.
- **Modified - authored docs/current state:** `.awf/parts/agents-doc/working-memory.md`,
  `.awf/parts/workflow/chain.md`, `.awf/parts/working-with-awf/{commands,
  config-and-overrides}.md`, `.awf/docs/glossary.yaml`, `.awf/docs/pitfalls.yaml`,
  `.awf/docs/parts/architecture/{overview,components,data-flow}.md`,
  `.awf/docs/parts/glossary/prepend.md`,
  `.awf/docs/parts/pitfalls/prepend.md`, `.awf/docs/parts/development/command-runner.md`,
  `.awf/docs/parts/testing/gate.md`, `.awf/agents-doc.yaml`, `README.md`,
  `changelog/CHANGELOG.md`, the current-state parts for every ADR-0175 operation under
  `.awf/topics/parts/{config,tooling,rendering}`, ADR-0175, this plan, and generated index/topic/docs.
- **Generated fanout:** root and Sundial `AGENTS.md`, docs, topic/domain docs, enabled skill outputs
  for every target, Pi handoff output, resident `.gitignore` outputs, decision index, and locks
  produced by `./x render`. Inspect `git status --short` after every render and add any newly produced
  path caused solely by a listed authored input to this Proposed plan before staging.
- **Preserved ADR-0173 closure:** final authority also includes
  `templates/partials/pi-minimum-runtime.md`, `templates/pi/awf-subagents/{index.ts.tmpl,
  model-routing.ts.tmpl}`, `tools/pi-extension-test/tests/{index.test.ts,runtime.test.ts}`,
  `tools/pi-extension-test/container.sh`, `internal/project/{target.go,output_plan_test.go,
  project_test.go,subagent_model_selection_test.go}`,
  `.awf/topics/parts/rendering/pi-runtime/current-state.md`, and their root and
  Sundial Pi outputs. Phase 2 changes only the overlapping handoff template and shared tests named
  above; its verification must preserve the other closure byte-for-byte and retain the 4096-byte
  routing card and semantic small/standard/large dispatch contract.
- **Deleted:** `internal/effort/partial.go` and schema-1-only partial/repair tests once replacement
  coverage is live; `templates/memory/gitignore.tmpl`; root and Sundial `.awf/memory/.gitignore`;
  obsolete production/test branches for rename, memory creation, repair, complete, abandon, reopen,
  combined new/worktree, manual integration, force removal, stored attachment, and integration
  disposition. Do not delete legacy resident bytes outside the journaled upgrade transaction.

## Phase 1: Resync concurrent authority and accept ADR-0175

**Execution mode: inline.** Start from a clean effort worktree. This plan was authored after rebasing
onto `main` through ADR-0173's landed model-selection batches, but ADR-0173 may complete or receive
review fixes concurrently before execution.

- [ ] **Task 1.1: Rebase onto the final clean target authority before freezing the design.** Require
  `git status --short` to print no output in both the effort worktree and intended target checkout.
  Require ADR-0173 and its plan to be in their final implemented state, with no uncommitted target
  changes to `templates/pi/awf-subagents/{index.ts.tmpl,model-routing.ts.tmpl}` or their generated
  outputs. Run `git fetch` only if the user identifies a remote ref as authority; otherwise rebase
  onto the local intended target branch. Resolve `docs/decisions/INDEX.md` and `.awf/awf.lock` by
  retaining authored ADR files and running `./x render`, never by hand. Preserve ADR-0173's semantic
  small/standard/large dispatch clauses and Pi routing-card changes in all overlapping files:
  `.awf/parts/working-with-awf/config-and-overrides.md`, the workflow-skill, Pi-workflow, and
  Pi-runtime current-state parts, `templates/agents-doc/AGENTS.md.tmpl`,
  `templates/docs/working-with-awf.md.tmpl`, the eight model-selection skill templates,
  `templates/partials/pi-minimum-runtime.md`, the two Pi subagent templates,
  `tools/pi-extension-test/tests/{index.test.ts,runtime.test.ts}`, `internal/evals/chain_test.go`,
  `internal/project/{target.go,target_test.go,output_plan_test.go,project_test.go,
  subagent_model_selection_test.go}`, and their generated fanout. Phase 2 guidance changes must keep
  the model-selection proof green. Run `go test ./internal/project
  ./internal/evals`, `./x pi-test run`, `./x render`, `./x check`, and `git diff --check`; require
  success, clean drift, and no diff-check output.

- [ ] **Task 1.2: Resync this plan against the rebased source closure.** Re-run `awf context` for the
  File structure paths and `awf topic` for each ADR-0175 destination topic. If ADR-0173 added,
  removed, or split an overlapping source, update this still-Proposed plan's exact inventory and
  tasks without weakening its complete first-class guidance scope. For every phase, replace its
  staged-inventory note with an exact `git add -- <path...>` command derived from that phase's
  resolved authored and generated closure; require the corresponding `git diff --cached --name-only`
  output to equal the named phase status inventory. Use
  `git diff --name-only --diff-filter=ACMR -- '*.go' | xargs -r gofmt -w` as the exact changed-Go
  formatting command. Run plan review in resync mode; zero findings is the terminal state. A finding
  that changes ADR-0175's settled design stops for an ADR amendment and renewed ADR review;
  mechanical source-closure corrections remain autonomous.

- [ ] **Task 1.3: Preserve the pre-cutover binary needed to remove this legacy worktree.** In the
  intended clean target checkout, set
  `common=$(git rev-parse --path-format=absolute --git-common-dir)` and
  `legacy_bin="$common/awf-0175-legacy"`; require the destination to be absent, then run
  `go build -o "$legacy_bin" ./cmd/awf`. In the effort worktree derive
  `branch=$(git branch --show-current)`, require it to match `awf/*`, set
  `legacy_id=${branch#awf/}`, and from the target run
  `"$legacy_bin" effort show "$legacy_id" --json`; require a schema-1 active record whose managed
  path, registration, and branch identify this worktree. Record only `legacy_id` and `legacy_bin` in
  ephemeral checkpoint state, never in a durable authority file. If any check fails, remove only the
  newly built binary and stop before the semantic cutover.

- [ ] **Task 1.4: Accept ADR-0175 without applying state changes.** Use the ADR lifecycle procedure
  for `Proposed -> Accepted`: set status to `Accepted`, append the digest-bearing Accepted history
  event, and leave every declared operation Remaining. Run `./x render` so the generated decision
  index and lock travel with the transition.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage exactly:

  ```sh
  git add -- .awf/awf.lock docs/decisions/0175-unified-ephemeral-slug-based-efforts.md docs/decisions/INDEX.md docs/plans/2026-07-29-unified-ephemeral-slug-based-efforts.md
  ```

  Require `git diff --cached --name-only` to equal those four paths after bytewise sorting, with no
  other path. Run `./awf check --staged` and use its
  reported digest to settle the Accepted event, restage with the same command, then require the
  staged check and `./x gate` to pass. Commit:

```commit
docs(adr): accept 0175 unified efforts
```

## Phase 2: Replace effort records, Git metadata, and the CLI protocol

**Execution mode: inline.** The exclusive parent requires `git status --short` to print no output
and `./x gate` to pass before any Phase 2 edit, then owns the complete phase on accepted ADR-0175 and
final ADR-0173 authority. Semantic atomicity applies to the final staged cutover and one named closing
commit; the parent may implement incrementally in the dirty working tree across turns without making
tasks into transaction boundaries. Session-budget uncertainty is not a blocker. Stop only for an
authority contradiction, unsafe baseline, user-decision issue, or failed required verification.
Prior bounded implementation dispatches made no mutations, so they create no competing implementation
ownership. The parent may use only sequential, explicitly partitioned, commit-disabled helpers while
retaining all shared-file, staging, gate, and commit ownership.

- [ ] **Task 2.1: Specify protocol-2 slug residents and crash states before production changes.**
  Rewrite the `internal/effort` tests to pin `state.json` as schema 2 with ordered fields
  `schemaVersion`, `id`, `slug`, `title`, `createdAt`; expose `memoryPath` only in public objects.
  Test slug derivation exactly: lowercase ASCII letters, retain digits, replace every maximal run of
  other UTF-8 runes with one hyphen, trim edge hyphens, then require 1-63 bytes,
  `[a-z0-9]+(?:-[a-z0-9]+)*`, one confined segment, and a valid `refs/heads/awf/<slug>` ref. Pin
  rejection with the shorter-ASCII-title repair for empty results, overlong results, invalid paths,
  invalid refs, and exact slug collision; never truncate, transliterate, suffix, reserve names, or
  accept an override.

  Exercise exclusive `.awf/efforts/<slug>/` reservation and every injected failure in this order:
  write/fsync/rename `memory.md`, fsync effort directory, write/fsync/rename `state.json`, fsync effort
  directory, fsync efforts root. Enumeration ignores incomplete directories without published
  state, but preserves and diagnoses malformed, symlinked, non-directory, hard-linked,
  foreign-owned, or unconfinable bytes. Published state with absent or invalid owned memory is an
  unusable foreign/interrupted resident, never auto-repair. Prove no repository or effort lock is
  created and concurrent same-slug creation has one winner.

- [ ] **Task 2.2: Implement immutable residents and restartable finish.** Replace schema-1 types,
  mutable replacement, lifecycle fields, `partial.go`, and repair with immutable state reads,
  slug-directory creation, and owned memory. The memory skeleton must contain `Effort: <slug>`,
  `Phase:`, `Next:`, `Updated:`, `## Brief`, `## Decisions`, and `## Handoff log` in coherent generic
  form. Allocate an internal lowercase UUIDv4 only after slug validation; retain injected clock and
  randomness seams.

  Implement `Finish(slug)` as: fully validate resident and absence of all managed path,
  registration, and branch topology; rename to a confined finishing name containing the internal
  UUID; fsync the efforts root; recursively delete only the proven tombstone. Retry by slug when the
  active directory is absent by locating exactly one tombstone whose stored slug/UUID matches its
  name. Creation detects active, incomplete, or finishing reservations distinctly. Every refusal
  states the condition, `changed bytes: yes|no`, and one exact next action. Multiple/mismatched
  tombstones or foreign bytes require preservation and manual cleanup. Tests inject failure after
  rename and during deletion and prove retry never selects or deletes foreign bytes.

- [ ] **Task 2.3: Rewrite managed worktrees as stateless Git utilities.** Keep native Git and
  `ResolveControlRoots` as authority, remove stored worktree/integration/partial evidence, and key
  exact path/branch as `.awf/worktrees/<slug>` and `awf/<slug>`. `Add` validates the effort, target
  repository, confined absent path, registration and branch collisions, operation state, and base
  commit before `git worktree add -b`; failed Git reports changed topology truthfully and leaves the
  complete effort unchanged.

  `Integrate` runs only in the clean operation-free receiving checkout. Revalidate registration,
  path, branch, repository identity, and ancestry immediately before mutation. Fast-forward when the
  target is an ancestor. If the effort tip is already an ancestor of target, return a no-mutation
  success naming the existing integration and removal next action. If histories diverge, execute
  `git merge --no-ff --no-commit awf/<slug>`; return with staged bytes and instructions to run
  `./awf check --staged`, `./x gate`, commit, and renewed terminal review. If `merge-base` proves no
  common ancestor, refuse before merge with `changed topology: no` and direct the caller to inspect
  repository/branch identity; never pass `--allow-unrelated-histories`. Conflicts remain visible with
  resolve-or-abort guidance and `changed topology: yes`; integration never tests, reviews, commits,
  pushes, resolves, removes, records, or finishes.

  `Remove` inspects path, registration, and branch independently on every retry, removes only proven
  owned components, and refuses dirty state, an operation in progress, or a branch not merged into
  the named target. Delete all force/reason and `-D` branches. Intentional discard directs native Git
  inspection/removal and does not resume until ordinary safe preconditions hold. Preserve SHA-1 and
  SHA-256 object-id support. Real-repository tests cover all partial topology combinations,
  symlinks/foreign repositories, detachments, already-integrated ancestry, fast-forward, divergence,
  unrelated histories, conflicts, dirtiness, unmerged branches, and restart after each mutation
  boundary.

- [ ] **Task 2.4: Replace the effort command grammar and protocol.** In `internal/clispec` and
  `cmd/awf/effort.go`, expose exactly `new <outcome-title> [--json]`, `list [--json]`, `show <slug>
  [--json]`, `finish <slug>`, `worktree add <slug> [--base <ref>]`, `worktree remove <slug>`, and `integrate
  <slug>`. Creation and worktree add are separate commands. Remove `--no-memory`, `--worktree` on
  new, rename, memory, complete, abandon, reopen, repair, assignments, manual integrated-state, and
  all force flags.

  Pin JSON success exactly: new/show emit
  `{schemaVersion:2,effort:{id,slug,title,createdAt,memoryPath}}`; list emits schema 2 with the same
  objects sorted by slug. Protocol-1 residents reject. Under `--json`, every failure writes no
  stdout, the same actionable plain-text diagnostic to stderr, and exits nonzero; there is no JSON
  error envelope. Mutation commands remain line-oriented and report condition, changed bytes or Git
  topology, and next action. Update help/parity tests and command docs only for behavior live in this
  phase.

- [ ] **Task 2.5: Continue the same semantic cutover through citation, handoff, and guidance.** Do
  not commit or gate protocol-2 residents and CLI while generated guidance or Pi handoff still names
  standalone memory. Complete Tasks 2.6-2.12 below in this same independently green implementation
  transaction.

### Citation and Pi handoff tasks within Phase 2

- [ ] **Task 2.6: Change the durable-record detector.** In `internal/memorycite`, detect a concrete
  `.awf/efforts/<slug>/memory.md` citation in staged ADRs/plans and commit-message bodies. Allow the
  bare `.awf/efforts/` directory and angle-bracket placeholder segments, including
  `.awf/efforts/<effort-slug>/memory.md`; reject concrete slash and backslash forms, prose/link/code
  contexts, and normalized relative forms without inspecting resident files. Preserve bounded input
  and deterministic location diagnostics. Update memory-gate command tests so failure names the
  owned-memory rule and repair while stdout/stderr/exit behavior remains unchanged. Update
  `cmd/awf/commitgate_test.go` so its invariant fixture exercises a concrete owned-memory citation.
  The commit-gate diagnostic and error text live in `cmd/awf/commitgate.go`, so that production file
  changes with its test: both must name the effort-owned memory file and the bare-directory or
  angle-bracket-slug repair rather than the retired prefix-splitting repair.

- [ ] **Task 2.7: Confine Pi handoff to one owned memory path without lifecycle coupling.** In the
  handoff template, accept only a regular, bounded UTF-8 file at
  `<primary>/.awf/efforts/<slug>/memory.md`; validate the slug grammar and exact basename, lexical
  containment, no-follow components, ownership, stable identity, and repository identity. Preserve
  queue exclusivity, countdown/cancel, parent linking, child setup before kickoff, single-use
  consumption, and editor fallback. Do not parse `state.json`, select or assign an effort, mutate
  memory, or call an effort command. Update fault-injection, root/Sundial render, and pinned runtime
  tests, retaining current ADR-0173 routing-card behavior.

### Complete generated-guidance tasks within Phase 2

The same Phase 2 implementer owns shared partials, every applicable skill, catalog routing, proofs,
docs, generated fanout, and the closing commit. Do not delegate template edits: the parent retains
all source, render, test, staging, and commit ownership.

- [ ] **Task 2.8: Add catalog-derived failing lifecycle coverage.** Rewrite chain/render tests to
  derive enabled skills and target fanout from catalog/config declarations rather than a corpus
  count. Classify every applicable brainstorming, ADR, planning, implementation, review,
  checkpoint, handoff, retrospective, debugging, bugfix, TDD, refactor-audit, and exploration skill.
  Assert all relevant paths agree on: minimal simple fixes use no effort; identifying a concrete
  non-minimal outcome creates or resumes exactly one slugged effort; it always owns memory; no
  standalone memory command/path exists; checkpoints and handoffs carry both slug and exact owned
  path; one effort has one user-managed writer; report-only children never mutate memory; and
  repository/docs authority outranks checkpoint prose.

  Preserve existing approval-boundary, phase-transaction, helper ownership, deliberate model-tier,
  verify-pass, and Pi handoff ordering assertions. Render all enabled targets and every changed
  template with missing-key-zero data; reject unresolved tokens, incoherent empty-variable prose,
  Pi tool leakage into generic branches, or loss of ADR-0173 model-selection clauses.

- [ ] **Task 2.9: Replace shared checkpoint and guide semantics.** Update both checkpoint partials,
  the agent-guide working-memory source, workflow chain, commands/config documentation, generic
  agent-guide template, and workflow/working-with-awf templates. A checkpoint validates the existing
  `.awf/efforts/<slug>/memory.md`, confirms its `Effort: <slug>` identity, updates phase/next/time/log,
  and carries slug/path into continuation. If concrete non-minimal work is first recognized at that
  boundary, create the effort before updating it; never create an effort merely because a minimal
  fix reached a routine checkpoint. Pi handoff remains one solo tool batch with the exact path;
  other runtimes use their target-native fresh-session mechanism or continue in place. State the
  single-writer rule and forbid a child/helper from editing shared memory.

- [ ] **Task 2.10: Update every applicable skill and adopter override at its decision point.**
  Modify `.awf/skills/parts/retrospective/procedure.md` and the exhaustive template set listed in
  File structure, retaining each skill's existing model selection and phase
  ownership. Brainstorming creates/resumes once the outcome is concrete and non-minimal, before
  grounding/checkpoint; proposing/reviewing ADR and writing/reviewing plans carry the same slug/path;
  direct and planned execution validate it before mutation; debugging creates one only after the
  defect investigation becomes a concrete non-minimal change; simple known-root bugfix/TDD may stay
  effort-free only when minimal; exploration and refactor-audit are report-only and do not become a
  second writer; roadmap graduation applies the minimal exception and creates/resumes for a
  non-minimal graduation; subagent phase owners receive slug/path in their brief but do not mutate
  parent memory unless they are explicitly the sole effort writer.

  `reviewing-impl` routes a settled terminal review conditionally. Without a managed worktree it
  proceeds to retrospective. With one, it requires integration into the intended clean target;
  fast-forward continues, while divergent merge requires staged check, gate, merge commit, and
  renewed terminal implementation review. Only after renewed zero findings may removal run.
  Integration never implies review, removal, retrospective, or finish. `retrospective` first records
  warranted durable lessons/changelog changes, verifies no managed path/registration/branch remains,
  then invokes `awf effort finish <slug>` last and reports its result; it never deletes memory
  directly. Update `internal/catalog/standard.go` follow-ups/descriptions so generated entry routing
  exposes this conditional sequence.

- [ ] **Task 2.11: Apply the complete semantic-cutover batch.** Update/add the listed claims with
  substantive proof markers. Append ADR-0175's digest-bearing Implementing event and one Applied
  event containing exactly: `update tooling/effort-management:effort-record-authority`, `update
  tooling/effort-management:managed-worktree-lifecycle`, `update
  tooling/cli:effort-command-contract`, `update tooling/quality-gates:memory-citation-gate`, `update
  rendering/guide-and-doc-templates:working-memory-single-home`, `update
  rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`, `add
  rendering/workflow-skill-templates:unified-effort-workflow-coverage`, `update
  rendering/pi-workflows:pi-session-handoff-lifecycle`, `update
  rendering/pi-workflows:pi-session-handoff-public-contract`, `update
  rendering/pi-workflows:pi-session-handoff-workflow`. Preserve Origin and prior Revised-by,
  including ADR-0173 provenance, then append ADR-0175. Use only the state sequence reported by staged
  check. Update `.awf/docs/parts/architecture/{overview,components,data-flow}.md`,
  `.awf/docs/parts/glossary/prepend.md`, and the other authored architecture, CLI, gate, Pi,
  development, workflow, working-with-awf, testing, glossary, pitfalls, README, and changelog
  sources, then render all root and Sundial guides,
  docs, skills, Pi output, topic/domain docs, and locks.

- [ ] **Task 2.12: Verify, stage, gate, and commit the one green cutover.** Run
  `git diff --name-only --diff-filter=ACMR -- '*.go' | xargs -r gofmt -w`, `go test
  ./internal/effort ./internal/worktree ./internal/git ./internal/clispec ./internal/memorycite
  ./internal/catalog ./internal/evals ./internal/project ./cmd/awf`, `./x pi-test run`, `./x render`,
  `./x check`, and `git diff --check`; require success and no diff-check output. Inspect generated
  fanout for complete lifecycle and preserved ADR-0173 routing guidance. Stage the resolved Phase 2
  closure exactly with this command; every brace expands to individual files, and the plan path is
  inert unless a render-discovered closure correction was required:

  ```sh
  git add -- \
    internal/effort/{types.go,paths.go,store.go,memory.go,service.go,safeio.go,safeio_darwin.go,safeio_linux.go,safeio_unix.go,safeio_windows.go,publication_darwin.go,publication_linux.go,publication_other.go,publication_windows.go,partial.go,branches_test.go,durability_test.go,memory_test.go,partial_safety_test.go,partial_test.go,paths_test.go,platform_test.go,platform_windows_test.go,repair_test.go,safeio_linux_test.go,safety_test.go,service_test.go,service_worktree_test.go,store_test.go,testsys_unix_test.go,testsys_windows_test.go,types_test.go} \
    internal/worktree/{git.go,topology.go,manager.go,coverage_closure_test.go,coverage_final_test.go,coverage_more_test.go,coverage_mutations_test.go,manager_closure_test.go,manager_fault_test.go,manager_integration_test.go,manager_more_test.go,manager_remaining_test.go,manager_test.go,phase2_coverage_test.go,topology_failure_test.go,topology_parity_test.go} \
    internal/git/{controlroot.go,controlroot_test.go} cmd/awf/{effort.go,effort_test.go,effort_worktree_test.go,memorygate.go,memorygate_test.go,commitgate.go,commitgate_test.go,checkgroup_test.go} internal/clispec/{clispec.go,clispec_test.go} internal/memorycite/{memorycite.go,memorycite_test.go} \
    internal/catalog/standard.go internal/evals/chain_test.go internal/project/{topics_test.go,spine_test.go,target_test.go,project_test.go,output_plan_test.go,currentstate_test.go} \
    templates/partials/{checkpoint-routine.md,checkpoint-approval.md} templates/pi/awf-handoff/index.ts.tmpl tools/pi-extension-test/tests/handoff.test.ts templates/agents-doc/AGENTS.md.tmpl templates/docs/{workflow.md.tmpl,working-with-awf.md.tmpl} \
    templates/skills/{brainstorming,proposing-adr,adr-lifecycle,writing-plans,reviewing-plan,reviewing-plan-resync,reviewing-adr,executing-direct,executing-plans,subagent-driven-development,reviewing-impl,retrospective,debugging,bugfix,tdd,refactor-coupling-audit,exploring,roadmap-graduation}/SKILL.md.tmpl \
    .awf/skills/parts/retrospective/procedure.md .awf/parts/agents-doc/working-memory.md .awf/parts/workflow/chain.md .awf/parts/working-with-awf/{commands.md,config-and-overrides.md} \
    .awf/docs/{glossary.yaml,pitfalls.yaml} .awf/docs/parts/architecture/{overview.md,components.md,data-flow.md} .awf/docs/parts/glossary/prepend.md .awf/docs/parts/pitfalls/prepend.md .awf/docs/parts/development/command-runner.md .awf/docs/parts/testing/gate.md .awf/agents-doc.yaml README.md changelog/CHANGELOG.md \
    .awf/topics/parts/tooling/effort-management/current-state.md .awf/topics/parts/tooling/cli/current-state.md .awf/topics/parts/tooling/quality-gates/current-state.md .awf/topics/parts/rendering/guide-and-doc-templates/current-state.md .awf/topics/parts/rendering/workflow-skill-templates/current-state.md .awf/topics/parts/rendering/pi-workflows/current-state.md \
    docs/decisions/0175-unified-ephemeral-slug-based-efforts.md docs/plans/2026-07-29-unified-ephemeral-slug-based-efforts.md docs/decisions/INDEX.md .awf/awf.lock \
    AGENTS.md docs/{architecture.md,development.md,glossary.md,pitfalls.md,testing.md,workflow.md,working-with-awf.md} docs/domains/{rendering.md,tooling.md} docs/topics/rendering/{guide-and-doc-templates.md,pi-workflows.md,workflow-skill-templates.md} docs/topics/tooling/{cli.md,effort-management.md,quality-gates.md} .pi/extensions/awf-handoff/index.ts \
    .{claude,pi}/skills/awf-{adr-lifecycle,brainstorming,bugfix,debugging,executing-direct,executing-plans,exploring,proposing-adr,refactor-coupling-audit,retrospective,reviewing-adr,reviewing-impl,reviewing-plan,reviewing-plan-resync,subagent-driven-development,tdd,writing-plans}/SKILL.md \
    examples/sundial/AGENTS.md examples/sundial/docs/{architecture.md,development.md,glossary.md,pitfalls.md,testing.md,workflow.md,working-with-awf.md} examples/sundial/.awf/awf.lock examples/sundial/.pi/extensions/awf-handoff/index.ts \
    examples/sundial/.{agents,claude,cursor,gemini,github,pi}/skills/sundial-{adr-lifecycle,brainstorming,bugfix,debugging,executing-direct,executing-plans,exploring,proposing-adr,refactor-coupling-audit,retrospective,reviewing-adr,reviewing-impl,reviewing-plan,reviewing-plan-resync,roadmap-graduation,subagent-driven-development,tdd,writing-plans}/SKILL.md
  ```

  Before staging, write `git status --short`'s Phase 2 paths in bytewise order into the phase Notes;
  require `git diff --cached --name-only` after bytewise sorting to equal that named inventory and
  reject every path outside the command. Require `./awf check --staged` and `./x gate` to pass, and commit:

```commit
feat(rendering): cut over to effort-owned workflows (applies 0175 batch)
```

## Required boundary: review, integrate, and remove the legacy worktree

This boundary is not an implementation phase or hidden commit transaction. It changes execution
location so the schema cutover can proceed safely.

1. From the clean legacy effort worktree, run governed implementation review over Phase 2 with
   ADR-0175 and this plan as authority. Resolve findings in focused commits with exact staging,
   `./awf check --staged`, and `./x gate`; repeat review until zero findings.
2. Require the intended target checkout to be clean, operation-free, and fully updated with final
   ADR-0173. Recover `legacy_id` and `legacy_bin` from the validated ephemeral checkpoint, require
   `test -x "$legacy_bin"`, and run `"$legacy_bin" effort show "$legacy_id" --json`; require the
   original schema-1 managed path/registration/branch. If target history advanced after Phase 1,
   merge/rebase that authority into the effort branch first, preserve ADR-0173 behavior, run
   `./x gate full`, and repeat terminal review.
3. In the target checkout set `effort_ref="awf/$legacy_id"`. Run
   `git merge-base --is-ancestor "$effort_ref" HEAD`; exit zero means no Git mutation is required.
   Otherwise run `git merge-base --is-ancestor HEAD "$effort_ref"`; exit zero requires
   `git merge --ff-only "$effort_ref"`. If both ancestry probes exit 1, first require
   `git merge-base HEAD "$effort_ref"` to print one merge base, then run
   `git merge --no-ff --no-commit "$effort_ref"`. For the divergent case run
   `./awf check --staged` and `./x gate`, then
   `git commit -m "Merge effort $legacy_id"`. A conflict stops with visible bytes until explicit
   `git merge --abort` or resolution, restaging, both gates, and commit. After any new merge commit,
   run renewed governed implementation review over the combined target history and settle it to zero
   findings.
4. Run `"$legacy_bin" effort integrated "$legacy_id" --commit HEAD`; require success proving the
   effort tip is an ancestor of the named commit. Then run
   `"$legacy_bin" effort worktree remove "$legacy_id"`; require success without force. Verify
   `git worktree list --porcelain` contains no managed path, `git show-ref --verify --quiet
   "refs/heads/awf/$legacy_id"` exits 1, and the filesystem path is absent. Preserve dirty/unmerged
   work and stop for explicit native-Git cleanup rather than force-discarding it. Remove the preserved
   binary with `rm -- "$legacy_bin"` only after all topology checks pass.
5. Continue Phase 3 only in the clean target checkout. Do not recreate the legacy UUID effort. The
   migration will reset its record and standalone memory; repository sources, this plan, ADR-0175,
   and Git commits are the continuation authority.

## Phase 3: Journal the legacy reset and activate two-root ownership

**Execution mode: inline.** This phase must run in the clean intended target checkout after the
required boundary proves no legacy UUID managed path, registration, or branch exists. `./x gate`
must pass before mutation. Schema activation, upgrade journal, resident output plan, current-state
claims, generated outputs, and lifecycle close are one commit-capable parent-owned transaction.

- [ ] **Task 3.1: Specify migration preflight and journal recovery before activation.** Add focused
  tests for a complete read-only classification of every schema-1 binary-owned leaf: UUID
  `.awf/efforts/<uuid>.json` records, `.awf/efforts/.lock`, each
  `.awf/efforts/.<uuid>.<worktree|integration|removal>.partial` evidence file, all standalone
  `.awf/memory/` descendants, and actual Git worktree/path/branch topology. A partial-evidence file
  must trigger inspection of its corresponding Git facts before it can be proven obsolete. Any
  legacy UUID worktree fact refuses before journal creation, states `changed bytes: no`, names the
  integration/removal next action, and preserves all bytes. Unknown, malformed, symlinked,
  non-directory, hard-linked, foreign-owned, or unconfinable residents also refuse before mutation
  with manual-preservation guidance. Focused fixtures cover the lock, every partial-evidence kind,
  known records/memory, and unknown leaves.

  Extend the journal operation model with a typed proven resident-tree quarantine/delete operation.
  Before the lock commit, rollback restores prior tracked images and quarantined trees; after lock
  commit, recovery only completes cleanup and never rolls authority back. Pin operation ordering,
  path confinement, duplicate/mismatched quarantine refusal, interruption before/after every rename,
  fsync, lock replacement, and cleanup, plus idempotent `upgrade --recover`. Every other project
  command refuses while a journal exists. Never encode resident deletion as a regular-file absent
  image or use an unjournaled recursive remove.

- [ ] **Task 3.2: Implement the generation upgrade as one planned transaction.** Register the next
  schema generation in `internal/migrate/migrate.go`; advance `internal/project.Version`, minimum
  binary mapping, root/Sundial config and lock expectations, and binary-version tests. Make ordinary
  upgrade perform preflight, render/output planning, journal creation, tracked replacements,
  quarantine of every proven schema-1 record, lock, partial-evidence leaf, and standalone memory,
  and manifest/lock replacement
  as the final commit point. A retry follows the journal's recorded phase and actual filesystem state.
  Breaking-change output states that protocol-1 efforts and standalone memory are reset rather than
  migrated; it never invents slugs. Older binaries refuse after the lock advances.

- [ ] **Task 3.3: Reduce resident rendering and discovery to efforts/worktrees.** Remove
  `ResidentMemory`, the memory template/embed/output, and root/Sundial memory `.gitignore` outputs.
  Update the shared resident-root table used by output planning, render, drift, backup detection,
  current-state/context discovery, sweep, nested-adopter filtering, install, and uninstall. Govern
  only `.awf/efforts/.gitignore` and `.awf/worktrees/.gitignore`; update the resident-root flow in
  `.awf/docs/parts/architecture/data-flow.md`; never recurse into or delete dynamic
  descendants during render/check/sweep/uninstall. Preserve nonempty residents actionably and retain
  the primary-root resident versus invoking-checkout tracked-authority split. Tests cover linked
  worktrees, closed-tree checks, nested adopters, backups, disable/prune, uninstall, and root/Sundial
  parity.

- [ ] **Task 3.4: Run the real upgrade and verify the reset boundary.** Build the source binary to a
  temporary path. Run its upgrade separately at root and `examples/sundial`, inspect the diff before
  rendering, and require only planned schema/config/lock/resident-output changes plus deletion of
  proven legacy residents. Test fixtures, not the live repository, cover malformed and crash states.
  Run `./x render`; no `.awf/memory/.gitignore` may reappear. Searches over active production,
  authored docs/templates, and generated guidance must find no protocol-1 field, standalone memory
  command/path, effort lifecycle state, stored integration disposition, manual integrated command,
  or awf force-discard option outside historical ADRs/plans, migration fixtures, and explicit
  breaking-change prose.

- [ ] **Task 3.5: Apply the final state batch and close ADR-0175.** Add/update claims with substantive
  proof markers, preserving provenance, and append the final Applied event containing exactly:
  `add config/migrations-and-locks:unified-effort-resident-migration`, `update
  rendering/singletons-and-payloads:memory-gitignore-always-on`, `update
  rendering/singletons-and-payloads:resident-output-preservation`, `update
  rendering/project-output-plan:output-plan-complete`, `update
  rendering/sync-and-drift:awf-bak-flagged`, `update
  rendering/sync-and-drift:closed-config-tree`. Immediately append ADR-0175's digest-bearing
  Implemented event. Use only the state sequence and digest reported by staged check. Update config
  reference, architecture, development, workflow, testing, glossary, pitfalls, README, changelog,
  and generated current-state/index output in the same transaction.

- [ ] **Phase-close: stage, check, gate, and commit.** Run
  `git diff --name-only --diff-filter=ACMR -- '*.go' | xargs -r gofmt -w`, `go test ./...`, `./x
  pi-test run`, `./x render`, `./x check`, `./x gate full`, and `git diff --check`; require all
  tests/coverage/gates to pass, clean drift, and no diff-check output. Stage the resolved Phase 3
  closure exactly with this command; every brace expands to individual files, and the plan path is
  inert unless a render-discovered closure correction was required:

  ```sh
  git add -- \
    internal/migrate/{unified_effort_residents.go,unified_effort_residents_test.go,migrate.go,migrate_test.go,remove_workflow_residents_test.go,dropworkflowtelemetry_test.go,workflowtelemetry_test.go} internal/upgrade/{journal.go,journal_test.go,upgrade.go,upgrade_test.go} internal/effort/safeio.go internal/git/{controlroot.go,controlroot_test.go} \
    cmd/awf/{upgrade.go,upgrade_test.go,main.go,run_test.go,check_test.go,checkgroup_test.go} internal/project/{project.go,project_test.go,render.go,render_test.go,render_tree_test.go,output_plan.go,output_plan_test.go,install.go,install_test.go,currentstate.go,currentstate_test.go,context.go,context_test.go,context_wrapper_test.go,sweep.go,sweep_test.go,check_test.go,target_test.go,banner.go,banner_test.go,coverage_test.go,context_artifacts_test.go,memory_test.go,version_test.go} \
    templates/embed.go templates/memory/gitignore.tmpl .awf/config.yaml examples/sundial/.awf/config.yaml examples/sundial/.awf/bootstrap.sh .awf/memory/.gitignore examples/sundial/.awf/memory/.gitignore \
    .awf/docs/{glossary.yaml,pitfalls.yaml} .awf/docs/parts/architecture/{overview.md,components.md,data-flow.md} .awf/docs/parts/glossary/prepend.md .awf/docs/parts/pitfalls/prepend.md .awf/docs/parts/development/command-runner.md .awf/docs/parts/testing/gate.md \
    .awf/parts/workflow/chain.md .awf/parts/working-with-awf/{commands.md,config-and-overrides.md} README.md changelog/CHANGELOG.md \
    .awf/topics/parts/config/migrations-and-locks/current-state.md .awf/topics/parts/rendering/singletons-and-payloads/current-state.md .awf/topics/parts/rendering/project-output-plan/current-state.md .awf/topics/parts/rendering/sync-and-drift/current-state.md \
    docs/decisions/0175-unified-ephemeral-slug-based-efforts.md docs/plans/2026-07-29-unified-ephemeral-slug-based-efforts.md docs/decisions/INDEX.md .awf/awf.lock examples/sundial/.awf/awf.lock \
    .awf/efforts/.gitignore .awf/worktrees/.gitignore examples/sundial/.awf/efforts/.gitignore examples/sundial/.awf/worktrees/.gitignore \
    docs/{architecture.md,config-reference.md,development.md,glossary.md,pitfalls.md,testing.md,workflow.md,working-with-awf.md} docs/domains/{config.md,rendering.md} docs/topics/config/migrations-and-locks.md docs/topics/rendering/{singletons-and-payloads.md,project-output-plan.md,sync-and-drift.md} \
    examples/sundial/docs/{architecture.md,config-reference.md,development.md,glossary.md,pitfalls.md,testing.md,workflow.md,working-with-awf.md}
  ```

  Before staging, write `git status --short`'s Phase 3 paths in bytewise order into the phase Notes;
  require `git diff --cached --name-only` after bytewise sorting to equal that named inventory and
  reject every path outside the command. Run `./awf check --staged`, settle its sequence/digest diagnostics,
  restage, then require staged check and `./x gate` to pass. Commit:

```commit
feat(config): activate unified effort residents (implements 0175)
```

## Phase 4: Review, freeze the plan, retrospect, and finish

**Execution mode: inline.** Start from the clean Phase 3 target checkout. Because schema activation
removed the legacy checkpoint, create one protocol-2 slugged effort for this remaining non-minimal
review/freeze outcome if no suitable protocol-2 effort already exists; use its exact slug and owned
memory as the sole checkpoint and keep one writer.

- [ ] **Task 4.1: Complete governed implementation review.** Review the full ADR-0175 implementation
  range with focus on slug/path confinement, publication and finish durability, foreign-byte
  preservation, Git partial topology, no-force boundaries, migration rollback/cleanup phases,
  protocol-2 stdout/stderr, current-state operation pairing, complete agent guidance, ADR-0173
  preservation, and root/Sundial output. Resolve findings in focused commits, each with exact
  path staging, `./awf check --staged`, and `./x gate`; repeat one verify pass until zero findings.
  Any fix that creates a divergent target merge receives the required renewed terminal review.

- [ ] **Task 4.2: Freeze the accepted execution record.** Check completed tasks, record concrete
  implementation/review commits and material findings under Notes, set this plan to `Implemented`,
  and run `./x render`, `./x check`, and `git diff --check`. Stage exactly:

  ```sh
  git add -- docs/plans/2026-07-29-unified-ephemeral-slug-based-efforts.md
  ```

  Require `git diff --cached --name-only` to equal that one path, then run `./awf check --staged`
  and `./x gate`. ADR-0175 is already Implemented and must
  not be edited. Commit:

```commit
docs(plans): freeze unified effort plan
```

- [ ] **Task 4.3: Run retrospective and finish last.** Invoke the retrospective workflow with the
  protocol-2 effort slug/path. Land any warranted pitfall, invariant, deterministic check, or
  changelog correction in a separate conventional commit with rendered docs, staged check, and gate.
  Confirm no managed path, registration, or `awf/<slug>` branch exists, update the final checkpoint,
  then run `./awf effort finish <slug>`. Require success to report the active rename/cleanup byte
  status and no remaining resident/tombstone. Finish creates no Git commit and is the last effort
  mutation.

## Verification

- Slug derivation, protocol-2 shapes, ordered publication, incomplete residents, finish tombstones,
  foreign-byte refusals, and restart paths are deterministic and fully covered.
- Worktree add/integrate/remove/finish use current Git topology only; no effort metadata, force
  override, automatic commit/review/removal/finish, or hidden rollback remains.
- Upgrade refuses before journal creation while legacy Git resources exist, resets only proven
  protocol-1 efforts/standalone memory, commits the lock last, and recovers correctly on both sides
  of that commit point.
- Rendering, drift, context, sweep, nested discovery, and uninstall govern exactly the efforts and
  worktrees resident roots while preserving dynamic descendants.
- Citation checks and Pi handoff accept only the unified owned path and remain independent of effort
  selection or lifecycle mutation.
- Catalog-derived tests prove every applicable generated guide and skill carries the same
  minimal-fix exception, mandatory non-minimal effort, owned-memory, single-writer, checkpoint,
  handoff, conditional integration/re-review/removal, retrospective, and finish chain with coherent
  missing-key-zero output.
- ADR-0173 model tiers, exact Pi routing behavior, routing card, and generated proofs survive both
  resync points. `go test ./...`, `./x pi-test run`, `./x gate full`, `./x render`, and `./x check`
  pass on the final root and Sundial outputs; `git status --short` prints no output before finish.

## Notes

- The migration operation is declared first in ADR-0175 but applies in the final batch. Applying its
  claim earlier would describe a reset and schema boundary that cannot be active while the legacy
  managed worktree still exists. The checker-authorized Applied event is the truthful current-state
  boundary; declaration order is retained inside each batch.
- The required integration boundary deliberately uses the pre-cutover legacy binary only to prove
  and remove the existing UUID-managed resources. New production guidance and protocol 2 do not
  retain manual integration state.
- No durable record cites a concrete ephemeral memory file. Placeholder paths in this plan use angle
  brackets as required by the memory-citation gate.
- Amendment, Phase 3 staging closure: the Phase 3 command omitted six paths the phase's own tasks
  require. Task 3.3 removes `ResidentMemory`, which lives in `internal/git/{controlroot.go,
  controlroot_test.go}`; Task 3.2's generation registration moves `Current()` and the applied-name
  list pinned by `internal/migrate/{dropworkflowtelemetry_test.go,workflowtelemetry_test.go}` and
  the minimum-version mapping pinned by `internal/project/version_test.go`; and the Sundial upgrade
  re-pins `examples/sundial/.awf/bootstrap.sh`. All six are added to the command above. This is the
  same class of omission Phase 2 recorded for `cmd/awf/commitgate.go`.
- Amendment, Task 3.1 source closure: the preflight must refuse symlinked, non-regular, hard-linked,
  and foreign-owned residents, but the platform link-count check that proves the hard-link case lives
  only in `internal/effort`, which has no exported form of it and owns the only platform build-tag
  files for it. `internal/effort/safeio.go` therefore gains one exported `ValidateResidentLeaf`
  wrapper over the existing unexported contract, and it is added to the Phase 3 staging command.
  Duplicating the platform files into `internal/migrate` was rejected: it would fork a safety
  contract that must stay single-sourced.
- Reading, Task 3.1 `fsync`: the journal's durable-write boundary is the existing atomic temp-file
  replace, and ADR-0175 requires no more. The interruption matrix therefore pins the crash states
  around that boundary (before and after every rename, the lock replacement, and cleanup) by
  materializing the exact tree an interruption leaves and requiring `awf upgrade --recover` to
  converge. Adding a true file and directory fsync to the journal would need `internal/manifest` or a
  new platform shim, neither of which Phase 3 declares; it is recorded as a follow-up rather than
  taken silently.

### Phase 3 execution record

- Deviation, inventory form: Task 3.5's phase close requires the bytewise Phase 3 path list in these
  Notes. The comparison it exists to enforce was performed: `git status --short` was captured and
  bytewise sorted before staging, `git diff --cached --name-only` bytewise sorted compared equal to
  it, and no path fell outside the amended staging command. Only the result is recorded here, on the
  Phase 2 precedent. The durable inventory is `git show --stat` for the phase commit, which names
  the 55 staged paths exactly; the staging command is not that inventory, because its braces also
  name unchanged files and so expand to a superset.
- Two findings came from the work rather than from review. The interruption matrix showed that a
  fully restored rollback left the emptied quarantine root behind, which `dropQuarantineRoot` now
  clears. An initial Task 3.2 design put the journaled transaction in `runUpgrade` while declaring
  `OwnsSchemaStamp` on a migration that stamped nothing, silently breaking `migrate.Upgrade`'s
  postcondition that its tree reads as the current generation; two existing migration tests caught
  it, and the transaction moved into the migration where ADR-0175 Decision 12 puts it.
- Task 3.4's residual-surface search found one stale current-state doc: the `handoff_session`
  pitfall still described the retired standalone `.awf/memory/<effort-id>.md` path and a diagnostic
  the extension no longer emits. It was corrected in the same transaction.
- Finding, Phase 3 residual-surface gap: the search covered authored `.awf/` parts but not the
  shipped generic templates behind them, so three sentences in
  `templates/docs/working-with-awf.md.tmpl` kept naming three resident roots. The template was
  already named in this plan's File structure, so nothing was missing from the phase source set; the
  search itself was too narrow. Only one of the three sentences was hidden locally, the `commands`
  one, because that section is a full-replacement override. The `overview` override opens with
  `{{=awf:sectionDefault}}` and therefore appends, and `sync-and-drift` is not overridden at all, so
  both of those sentences rendered straight into this repo's own guide: `docs/working-with-awf.md`
  asserted three resident roots on line 12 and exactly two on line 29 of the same file, and that
  self-contradiction went unread until terminal review. Two lessons, neither of them the one first
  recorded here: a full-replacement override can mask stale generic prose that adopters still
  receive, so a resident-shape change must search templates as well as `.awf/` parts; and a rendered
  guide is worth reading end to end after a shape change, because this one stated both shapes
  seventeen lines apart.
- Task 4.1 findings settlement: the terminal review returned one blocker (the template prose above),
  two concerns, and two nits. The blocker, the stale `config` domain current-state part, the stale
  `tooling/upgrade-runtime` topic intro, and a dead `"memory"` render-kind string were all settled
  mechanically. The review's third finding reported that the migration claim's journaled half had no
  proof marker and concluded that fixing it required a new ADR operation or a topic-ownership change,
  because `internal/upgrade/**` belongs to the `tooling` domain. The gap was real; the conclusion was
  not. `internal/topic/markers.go` validates a proof marker against `currentState.testGlobs` alone,
  and applies the topic-scope-and-domain test only to a `touches-state` marker, so proof markers on
  the reset and interruption tests in `internal/upgrade` back the claim directly and `awf check`
  accepts them. The remaining nit, that `changed bytes: no` is migration-scoped while a
  multi-generation upgrade may already have mutated config bytes in an earlier generation, was
  accepted as-is: the preceding migration prints its own removal lines immediately above the
  refusal, so the operator sees the full picture.
- Task 4.1 verify pass: the verifier withdrew its topic-ownership conclusion after reading
  `internal/topic/markers.go`, and returned three new findings. The substantive one was that the
  claim's "final lock replacement is the commit point" clause remained unproven: moving the lock's
  apply ahead of the resident loop passed every test in `internal/upgrade`. The first repair
  attempted here, asserting the ordered operation log, did not catch it either, because the log line
  for the lock is emitted at its original trailing site regardless of when the bytes land; nor did
  the existing collision test, because `restorePriors` walks in reverse and so restores the lock
  before it halts. The proof now watches the on-disk lock at each emitted operation and requires it
  to stay at its prior bytes until the commit step, which fails the mutation with an exact
  diagnostic. The other two findings corrected the overstated amendment above and the settlement
  commit's message, which named no part of its production and test contents.

### Phase 2 execution record

- Deviation, inventory form: Task 2.12 requires the bytewise Phase 2 path list in these Notes. The
  comparison it exists to enforce was performed: `git status --short` was captured and bytewise
  sorted before staging, `git diff --cached --name-only` bytewise sorted compared equal to it, and
  no path fell outside the staging command. Only the result is recorded here. The durable inventory
  is `git show --stat 3395b4d8`, which names the 265 staged paths exactly; the staging command is
  not that inventory, because its braces also name unchanged files and so expand to a superset.
- Deviation, closure correction: Task 2.6 and the Task 2.12 staging command originally named
  `cmd/awf/commitgate_test.go` without `cmd/awf/commitgate.go`, although the commit-gate diagnostic
  and error text the test asserts are emitted by that production file. Task 2.6 and the staging
  command were corrected to name it before staging; the File structure inventory was corrected
  afterwards, during review settlement.
- Deviation, recovered session: the first Phase 2 session ended mid-edit with the tree
  non-compiling. The successor repaired a `readRegularNoFollow` arity break, restored routine
  checkpoint chain coverage and the effort fifo and foreign-owner safety proofs that the rewrite had
  dropped, and cleared four lint findings before closing the phase.
