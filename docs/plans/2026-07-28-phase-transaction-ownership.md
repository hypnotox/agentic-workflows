---
date: 2026-07-28
adrs: [166]
status: Proposed
---
# Plan: Phase transaction ownership

## Goal

Implement ADR-0166 by making a phase the independently green implementation transaction across
plan authoring, plan review, inline execution, subagent-driven execution, recovery, review
settlement, and routine checkpoints. The implementation adds the backed phase-ownership claim,
updates the three existing claims in declaration order, and freezes the ADR and this plan only after
every operation is Applied.

This plan does not add concurrent same-checkout mutators, worktree-isolated or patch-producing
workers, a dead-code escape hatch, runtime phase enforcement, new Pi tools, or changes to Pi's
existing implementation serialization and exclusive tool-batch behavior.

## Architecture summary

Phase 1 is a subagent-driven transaction owned by one commit-capable implementer from a clean
baseline. It changes the complete authored workflow contract and its tests in one coherent concern:
plans select ownership per phase; tasks become ordered steps; a subagent-driven phase has one
commit-capable full-phase owner; inline batch helpers are sequential, commit-disabled, explicitly
partitioned helpers; the parent settles review and dirty recovery. That transaction applies ADR-0166
operations 1 and 2 together, with the new invariant's complete proof already green.

Phase 2 is parent-owned inline lifecycle settlement. ADR-0164 is already Implemented, so it applies
operations 3 and 4 in declaration order, records the next global state sequence, and performs the
final `Implementing -> Implemented` transition. The phase also freezes this plan and renders the
final index, lock, current-state docs, and adopter outputs. The two existing claims remain true after
Phase 1 and become more specific at the settled-phase boundary in Phase 2, so no false current-state
claim exists between the independently checked commits.

No new production abstraction is needed. Policy remains in the existing authored templates,
catalog reviewer data, current-state parts, and documentation data; tests render those sources
through existing project helpers. Generated root and Sundial artifacts remain outputs of
`./x render`, never hand-edited inputs.

Every repository path in this plan resolves from the exact absolute root
`/home/hypno/Projects/agentic-workflows/`. A task-bearing path written below with a shorter
repository-relative suffix means the absolute root joined to that exact suffix; shell commands run
with that absolute root as their working directory. This closed path notation supplies absolute,
unambiguous paths without repeating the same root in every list item.

## File structure

- **Created:** `internal/project/phase_transaction_ownership_test.go`.
- **Modified authored templates:** `templates/skills/writing-plans/SKILL.md.tmpl`,
  `templates/agents/plan-reviewer.md.tmpl`, `templates/skills/executing-plans/SKILL.md.tmpl`,
  `templates/skills/subagent-driven-development/SKILL.md.tmpl`,
  `templates/skills/reviewing-plan/SKILL.md.tmpl`,
  `templates/skills/reviewing-plan-resync/SKILL.md.tmpl`,
  `templates/plans-readme/README.md.tmpl`, `templates/plans-template/template.md.tmpl`,
  `templates/docs/workflow.md.tmpl`, and `templates/docs/working-with-awf.md.tmpl`.
- **Modified authored project policy/data:** `.awf/skills/parts/writing-plans/conventions-tasks.md`,
  `.awf/agents/plan-reviewer.yaml`, `.awf/docs/glossary.yaml`,
  `.awf/docs/parts/roadmap/ideas.md`,
  `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`, and
  `.awf/topics/parts/rendering/pi-workflows/current-state.md`.
- **Modified catalog and tests:** `internal/catalog/standard.go`, `internal/catalog/batch_test.go`,
  `internal/project/spine_test.go`, `internal/project/plan_detail_modes_test.go`,
  `internal/project/guide_scopes_test.go`, `internal/project/target_test.go`, and
  `internal/evals/chain_test.go`.
- **Modified lifecycle records:** `docs/decisions/0166-phase-transaction-ownership.md`, this plan,
  `docs/decisions/INDEX.md`, and `.awf/awf.lock`.
- **Generated root outputs:** `AGENTS.md`, `.claude/agents/plan-reviewer.md`,
  `.pi/agents/plan-reviewer.md`,
  `.claude/skills/awf-{writing-plans,executing-plans,subagent-driven-development,reviewing-plan,reviewing-plan-resync}/SKILL.md`,
  `.pi/skills/awf-{writing-plans,executing-plans,subagent-driven-development,reviewing-plan,reviewing-plan-resync}/SKILL.md`,
  `docs/plans/README.md`, `docs/plans/template.md`, `docs/workflow.md`,
  `docs/working-with-awf.md`, `docs/glossary.md`, `docs/roadmap.md`,
  `docs/topics/rendering/workflow-skill-templates.md`, `docs/topics/rendering/pi-workflows.md`, and
  `docs/domains/rendering.md`.
- **Generated Sundial outputs:** `examples/sundial/.awf/awf.lock`,
  `examples/sundial/.{claude,cursor,gemini,pi}/agents/plan-reviewer.md`,
  `examples/sundial/.{agents,claude,cursor,gemini,github,pi}/skills/sundial-{writing-plans,executing-plans,subagent-driven-development,reviewing-plan,reviewing-plan-resync}/SKILL.md`,
  `examples/sundial/docs/plans/README.md`, `examples/sundial/docs/plans/template.md`,
  `examples/sundial/docs/workflow.md`, `examples/sundial/docs/working-with-awf.md`, and
  `examples/sundial/AGENTS.md`.
- **Deleted:** the `coupled-phase escape` entry from `.awf/docs/glossary.yaml` and its generated
  row from `docs/glossary.md`; no file is deleted.

If `./x render` reports an additional generated output caused solely by one of the exhaustive
authored inputs above, stop before staging, add that exact path to this plan while it is Proposed,
and re-run the phase checks. Do not broaden the inventory with a catch-all path.

## Phase 1: implement and prove the phase transaction contract

**Execution mode: subagent-driven.** Before dispatch, the parent runs `git status --short` from the
absolute repository root and requires no output, then runs `./x gate` and requires success; these
commands establish the known clean and green baseline. The parent then dispatches one
commit-capable implementer for this complete phase. The implementer owns Tasks 1.1 through 1.6, the
complete staged transaction, both required gates, and the phase-closing commit. Do not dispatch an
individual task or a commit-disabled successor.

- [ ] **Task 1.1: Add the failing cross-surface phase-ownership proof.** Create
  `internal/project/phase_transaction_ownership_test.go` with
  `TestPhaseTransactionOwnershipAcrossWorkflowSurfaces`. Put
  `// invariant: rendering/workflow-skill-templates:phase-transaction-ownership` immediately above
  the test. Use existing `renderSkillGolden`, `renderAgentGolden`, and `renderGolden` helpers to
  render configured and missingkey-zero/empty-data variants of the writing, reviewing, inline, and
  subagent-driven skills plus the plan reviewer, plan README, and plan template.

  The test must assert all of these clauses on the owning surfaces:

  1. a phase is one independently green coherent implementation transaction and tasks are ordered
     steps rather than default dispatch, review, checkpoint, or commit boundaries;
  2. every phase declares `inline` or `subagent-driven` ownership independently, so a plan may mix
     the modes;
  3. a subagent-driven phase starts from a verified clean/green baseline, sends the complete phase
     to one commit-capable implementer, stages the complete transaction, runs `awf check --staged`
     and the configured gate, creates the declared phase-closing commit, then returns to parent-owned
     report-only review and focused settlement commits;
  4. inline execution keeps the parent as owner, while batch helpers are sequential,
     commit-disabled, path-disjoint, confined to declared subsets, never own shared files, and never
     commit;
  5. batch authoring retains an exact representative, exact edge unless identical everywhere,
     exhaustive affected-site set, deterministic post-check, and optional complete partition in
     which each site belongs to the parent or exactly one helper;
  6. no coupled-phase escape or dead-code exception remains; definitions land with their first
     production consumer by merging or reordering phases;
  7. phase review settles before the routine checkpoint, and a stopped dirty implementer leads to
     inventory plus exactly one explicit choice: parent completion, restore-and-full-phase restart,
     or full-context ownership transfer; a blind `continue Task X` successor is forbidden; and
  8. a representative rendered phase contains several ordered checkbox tasks followed by exactly
     one phase-closing staged-check/gate/commit boundary, in that order, with no task-level gate or
     commit boundary; and
  9. empty vars/data render coherent generic prose with no `<no value>`, unresolved-value token,
     empty inline code span, or dangling command sentence.

  The test must initially fail against the old per-task/coupled-phase templates. Run
  `go test ./internal/project -run TestPhaseTransactionOwnershipAcrossWorkflowSurfaces`; expected
  terminal state before implementation is test failure naming missing phase-level clauses.

- [ ] **Task 1.2: Replace the plan-authoring and plan-review contract.** Modify
  `templates/skills/writing-plans/SKILL.md.tmpl`,
  `.awf/skills/parts/writing-plans/conventions-tasks.md`,
  `templates/agents/plan-reviewer.md.tmpl`, `templates/skills/reviewing-plan/SKILL.md.tmpl`,
  `templates/skills/reviewing-plan-resync/SKILL.md.tmpl`,
  `templates/plans-readme/README.md.tmpl`, and `templates/plans-template/template.md.tmpl`.
  Modify the plan reviewer's `step-exactness` focus item in `internal/catalog/standard.go` and its
  focused proof in `internal/catalog/batch_test.go`. In `.awf/agents/plan-reviewer.yaml`, delete the
  complete `coupled-phase-escape` focus item and align the overridden `step-exactness` item with the
  new phase ownership and helper-partition contract; otherwise the project override would retain the
  removed exception in root rendered reviewers.

  Preserve the existing exact-diff/pseudocode contract and add the following closed behavior:

  - Each phase header or immediately following metadata declares exactly one execution mode,
    `inline` or `subagent-driven`; mode is selected independently per phase and the phase text
    contains enough context for a fresh owner.
  - The phase, not each checkbox task, is the coherent independently green transaction and has one
    declared closing subject in a `commit` fence. Tasks are ordered implementation steps. Remove all
    prose that makes a task the routine dispatch, ownership, review, checkpoint, or commit unit.
  - Delete the coupled-phase exception. Require authors to merge or reorder horizontal slices so a
    production definition lands with its first production consumer; explicitly forbid a dead-code
    escape hatch.
  - Extend the batch form with optional worker partitions. The exact contract assigns every
    affected site to the parent or exactly one helper, makes helper subsets path-disjoint, keeps
    shared files parent-owned, and confines focused mutating commands to the assigned subset.
  - Extend plan review scope-completeness, executability, application-batches, and the catalog
    `step-exactness` focus so they flag missing/plan-wide mode, incoherent phase concern, task-level
    transaction boundaries, non-green phases, coupled phases, incomplete/overlapping partitions,
    helper-owned shared files, or unconfined commands.
  - Reviewing-plan and resync briefs must preserve the phase ownership and partition fields when
    asking a reviewer to assess created/modified paths. Review remains report-only.
  - The canonical plan skeleton must show `**Execution mode: inline.**` as coherent generic prose;
    it must not prescribe a Pi tool or require one execution mode for all phases.

  Update `internal/project/plan_detail_modes_test.go`, `TestWritingPlansTemplate`,
  `TestReviewingPlanTemplate`, and `TestReviewingPlanResyncTemplate` in
  `internal/project/spine_test.go`, plus `TestPlanReviewerStepExactnessSanctionsBatch` in
  `internal/catalog/batch_test.go`, to assert the new clauses and reject `coupled phase`,
  `coupled-phase`, `one commit per task`, and plan-wide mode inference. Preserve existing assertions
  for exact content, implementation-ready pseudocode, batch representative/edge/sites/post-check,
  context grounding, and report-only review.

- [ ] **Task 1.3: Replace inline and subagent-driven task ownership with phase ownership.** Modify
  `templates/skills/executing-plans/SKILL.md.tmpl` and
  `templates/skills/subagent-driven-development/SKILL.md.tmpl` and update their golden assertions in
  `internal/project/spine_test.go`. In `internal/catalog/standard.go`, replace the
  `subagent-driven-development` workflow profile purpose and trigger with phase-level wording:
  `Implement a plan through reviewed phase owners.` and
  `Use when a plan phase benefits from delegated implementation ownership.` Update
  `internal/project/guide_scopes_test.go` to assert that exact rendered profile row and reject the
  old `reviewed subagent tasks` and `delegated implementation tasks` wording; `./x render` must then
  update root and Sundial `AGENTS.md` in the same transaction.

  Inline execution must iterate phases, not individual tasks. Before each phase it reads the
  declared mode; for `inline`, the parent owns every ordered task, integration, staged check, gate,
  closing commit, report-only phase review, focused follow-up commits, and settlement. It may execute
  an explicitly partitioned batch itself or call one or several sequential commit-disabled helpers,
  but it inventories each return, rejects out-of-subset writes, retains shared-file ownership, runs
  the batch post-check, and never delegates the phase commit. The parent checkpoints only after
  review settlement.

  Subagent-driven execution must also iterate phases and may hand an `inline` phase to the inline
  parent procedure, allowing mixed modes in one plan. For a `subagent-driven` phase it verifies the
  baseline is clean and green, then calls one implementation child alone in its parent tool batch
  with `allowCommits: true` and the complete phase: goal, ordered tasks, exact paths, semantic
  boundaries, dependencies, prior phase commits, checks, closing subject, and V2 operation batch.
  The implementer stages the entire phase, runs `awf check --staged`, runs the configured gate, and
  commits. The parent validates the reported commit and clean checkout, dispatches one report-only
  phase review, owns focused settlement commits, and checkpoints only after findings resolve.
  Preserve sequential implementation dispatch and exclusive implementation-tool batching.

  Both skills must implement the same dirty-stop state machine. Inventory `git status --short`,
  diff, completed/remaining work, prior concerns, and failed checks before choosing: complete inline;
  restore the known green baseline and redispatch the complete revised phase; or stop for required
  user input. An explicit transfer is allowed only with the complete revised phase, dirty-state
  inventory, completed and remaining work, prior concerns, and recovery verification. Ban a blind
  successor instruction for one task and ban casual dispatch over a dirty checkout. Review fixes
  remain parent-owned and do not recreate the original transaction.

  Replace the old spine assertions (`one commit per task`, `one subagent per task`, `fresh context
  per task`, and per-task review) with exact positive and negative assertions for the phase contract,
  mixed modes, `allowCommits: true`, baseline verification, full-phase brief, parent settlement,
  dirty recovery, and phase checkpoint.

- [ ] **Task 1.4: Update workflow documentation, glossary, and deferred-roadmap source.** Modify
  `templates/docs/workflow.md.tmpl` and `templates/docs/working-with-awf.md.tmpl` so their plan and
  implementation sections describe phases as green transactions, mode per phase, phase owner versus
  helper, review settlement, and dirty recovery. Replace the workflow doc's statement that long
  implementation phases checkpoint after resumable tasks with a settled-phase boundary. Remove the
  `coupled-phase escape` mapping from `.awf/docs/glossary.yaml` rather than redefining a removed
  exception.

  Append this roadmap idea to `.awf/docs/parts/roadmap/ideas.md`:

  ```text
  - Design concurrent same-checkout batch helpers only after scope enforcement, incidental-write
    attribution, failure attribution, and deterministic integration are specified; worktree-isolated
    and patch-producing parallel workers remain out of scope for the current workflow contract.
  ```

  The prose must state that runtime phase enforcement is not promised and Pi serialization/tool
  batching are unchanged. It must not imply concurrency, a worktree solution, patch-producing
  workers, or a task-level checkpoint.

- [ ] **Task 1.5: Apply ADR-0166 operations 1 and 2 atomically with the authored behavior.** In
  `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`, insert these claims in the
  topic's stable claim order. Preserve all unrelated claim text and provenance.

  Add exactly:

  ```text
  ### `invariant: phase-transaction-ownership`

  A rendered plan phase is one independently green coherent implementation transaction with an
  explicit per-phase inline or subagent-driven owner; checkbox tasks are ordered steps rather than
  default dispatch, review, checkpoint, or commit boundaries. One commit-capable implementer owns a
  complete subagent-driven phase from a known green baseline through staged check, gate, and closing
  commit, while the parent owns inline integration, sequential commit-disabled batch helpers,
  report-only review settlement, phase checkpointing, and explicit dirty-state recovery without
  blind task-level succession.
  Origin: ADR-0166
  Backing: test
  ```

  Replace the complete `plan-task-detail-modes` claim body with:

  ```text
  The rendered plan-authoring skill, plan reviewer, implementation-plans README, and plan template
  accept exact content/diffs or implementation-ready pseudocode with a closed application contract,
  require exact form for machine-consumed and other contract-bearing representations, preserve the
  specialized batch task and no-placeholder boundary, require one coherent green transaction and an
  inline or subagent-driven owner per phase, reject coupled phases, and require any optional helper
  partition to be exhaustive, path-disjoint, shared-file-safe, and command-confined. Every surface
  renders coherently with empty variables.
  Origin: ADR-0148
  Revised-by: ADR-0157, ADR-0166
  Backing: test
  ```

  Change ADR-0166 frontmatter from `Proposed` to `Implementing`. Append an `Implementing` history
  event with the checker-derived frozen content SHA-256, then one Applied event using the next global
  state sequence and exactly these declaration-order operations:

  ```text
  add `rendering/workflow-skill-templates:phase-transaction-ownership`, update `rendering/workflow-skill-templates:plan-task-detail-modes`
  ```

  Run `./x render`. The rendered root and Sundial skills, reviewer agents, plan docs, workflow docs,
  glossary, roadmap, topic/domain docs, decision index, and locks must reflect only authored changes.
  Update `internal/evals/chain_test.go` so the checkpoint proof expects settled phase boundaries in
  both implementation skills, no task boundary, and no new checkpoint in helper returns. Update
  `internal/project/target_test.go` only where its existing Pi handoff workflow assertions need the
  generated settled-phase wording; do not alter TypeScript serialization or tool-policy tests.

- [ ] **Task 1.6: Verify, stage explicitly, and create the phase implementation commit.** Run:

  ```sh
  gofmt -w internal/catalog/standard.go internal/catalog/batch_test.go internal/project/phase_transaction_ownership_test.go internal/project/spine_test.go internal/project/plan_detail_modes_test.go internal/project/guide_scopes_test.go internal/project/target_test.go internal/evals/chain_test.go
  go test ./internal/catalog ./internal/project ./internal/evals
  ./x render
  ./x check
  git diff --check
  ```

  Expected terminal state: all tests pass; render/check report no drift; `git diff --check` prints no
  output. Inspect `git status --short` and refuse any path outside File structure. Stage each changed
  authored and generated file with explicit `git add <path>` arguments; do not use `git add -A`,
  `git add .`, a directory argument, or a glob. Then run exactly:

  ```sh
  ./awf check --staged
  ./x gate
  ```

  Require `./awf check --staged` to print `awf check --staged: clean` and the full gate to pass.
  Commit exactly:

  ```commit
  feat(rendering): make phases transactional (applies 0166 batch)
  ```

  After commit, require `git status --short` to print no output. The parent then performs one
  report-only phase review over the commit, resolves any findings in focused parent-owned commits
  with the same staged check and gate, and records the settled phase checkpoint before Phase 2.

## Phase 2: settle checkpoint claims and freeze the lifecycle records

**Execution mode: inline.** The parent owns this small declaration-order lifecycle transaction. No
implementation helper is used because the remaining work is append-only status history, exact claim
text, rendering, gating, and one closing commit.

- [ ] **Task 2.1: Apply the remaining claim updates in declaration order.** In
  `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`, replace only the complete
  `memory-checkpoint-chain-coverage` claim body with:

  ```text
  Checkpoint guidance treats memory as optional local effort state and recommends outcome-specific
  `awf effort` creation when durable coordination, memory, or worktrees warrant it. Routine
  implementation checkpoints occur only after a phase's closing implementation commit has been
  reviewed and all findings are settled; checkbox tasks and batch-helper returns are not checkpoint
  boundaries. The guidance contains no selection, assignment, adoption, detour, or telemetry-
  lifecycle gate.
  Origin: ADR-0148
  Revised-by: ADR-0149, ADR-0152, ADR-0160, ADR-0164, ADR-0167, ADR-0166
  Backing: test
  ```

  In `.awf/topics/parts/rendering/pi-workflows/current-state.md`, replace only the complete
  `pi-session-handoff-workflow` claim body with:

  ```text
  Pi checkpoint guidance permits effort-independent handoff after normal persistence at a settled
  phase boundary, with optional confined memory, and never requires selection, telemetry lifecycle
  state, adoption, or structured resume. Checkbox tasks and batch-helper returns do not trigger
  routine handoff.
  Origin: ADR-0148
  Revised-by: ADR-0149, ADR-0152, ADR-0164, ADR-0167, ADR-0166
  Backing: test
  ```

  Preserve each existing proof marker on `TestMemoryCheckpointCoverage` in
  `internal/evals/chain_test.go` and `TestHandoffWorkflowWithoutEffort` in
  `internal/project/target_test.go`; Phase 1's assertions must already prove the new text. Do not add
  a proof marker for an unbacked claim or move these markers to generated files.

- [ ] **Task 2.2: Append the final Applied batch and freeze ADR-0166.** In
  `docs/decisions/0166-phase-transaction-ownership.md`, append one Applied event with the next global
  state sequence and exactly these declaration-order operations:

  ```text
  update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`, update `rendering/pi-workflows:pi-session-handoff-workflow`
  ```

  Then change ADR-0166 frontmatter from `Implementing` to `Implemented` and append its Implemented
  event with the unchanged frozen content SHA-256. Leave this plan `Proposed` until the phase-closing
  commit has undergone its required report-only review and settlement, because findings cannot be
  recorded before that review exists. If execution stops before this transaction, append `Abandoned`
  with a rationale instead: Phase 1's Applied operations remain active and these two unapplied
  operations become Canceled.

  Run `./x render` to regenerate `docs/decisions/INDEX.md`, `.awf/awf.lock`, the two topic docs,
  `docs/domains/rendering.md`, and any lock-linked adopter outputs. Do not hand-edit a generated
  output.

- [ ] **Task 2.3: Verify, stage explicitly, and create the final implementation commit.** Run:

  ```sh
  go test ./internal/project ./internal/evals
  ./x render
  ./x check
  git diff --check
  ```

  Expected terminal state: tests pass; render/check are clean; `git diff --check` prints no output.
  Inspect `git status --short`, refuse unexpected paths, and stage every changed file with explicit
  path arguments. Run exactly:

  ```sh
  ./awf check --staged
  ./x gate
  ```

  Require the staged check and full gate to pass. Commit exactly:

  ```commit
  feat(rendering): settle phase checkpoints (implements 0166)
  ```

  Require a clean worktree after commit. The parent then performs one report-only phase review over
  the Phase 2 commit. The later whole-implementation review does not substitute for this phase
  review because the phase contract requires settlement before its checkpoint.

- [ ] **Task 2.4: Settle review findings and freeze this plan.** Resolve every Phase 2 review finding
  through focused parent-owned commits that each stage the complete settlement, run
  `./awf check --staged`, pass `./x gate`, and never amend the phase-closing commit. In the final
  settlement commit, update this plan's Notes with the settled Phase 1 and Phase 2 implementation
  findings, or state explicitly that no implementation finding occurred, then change this plan's
  `status:` from `Proposed` to `Implemented`. Stage this plan explicitly, require
  `./awf check --staged` and `./x gate` to pass, and commit exactly:

  ```commit
  docs(plans): freeze phase transaction ownership plan
  ```

  Freeze the body and checkbox history at that commit, require `git status --short` to print no
  output, and record the settled phase checkpoint only after all findings resolve and the plan is
  frozen.

## Verification

- `go test ./...` passes, including the complete phase-ownership invariant and the existing Pi
  serialization and implementation-batch-exclusivity tests.
- `./x pi-test run` passes without a TypeScript behavior change.
- `./x render && ./x check` exits clean for root and Sundial generated outputs.
- `rg -n 'one commit per task|one subagent per task|fresh context per task|coupled-phase|coupled phase' templates/skills templates/agents templates/plans-readme templates/plans-template templates/docs .awf/skills/parts/writing-plans .awf/docs/glossary.yaml docs/plans/README.md docs/plans/template.md` prints no workflow-contract matches; historical ADRs and implemented plans are intentionally outside this search.
- `rg -n 'phase-transaction-ownership|plan-task-detail-modes|memory-checkpoint-chain-coverage|pi-session-handoff-workflow' .awf/topics/parts/rendering docs/topics/rendering` shows each active claim once in authored state and once in generated documentation, with ADR-0166 provenance in declaration order.
- `git status --short` prints no output.

## Notes

Concurrent same-checkout helpers remain a roadmap idea. Implementation must preserve the settled
choice against worktree isolation and patch-producing workers and must not reinterpret Pi's existing
serialization as transaction ownership. The final implementation review and retrospective follow
the two settled phase commits; they are outside the phase-closing implementation transactions and
remain parent-owned settlement work.
