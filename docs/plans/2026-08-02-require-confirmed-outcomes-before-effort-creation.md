---
format: plan-v1
date: 2026-08-02
adrs: [require-confirmed-outcomes-before-effort-creation]
status: Proposed
---
# Plan: Require confirmed outcomes before effort creation

## Goal

Render and deterministically prove an explicit user-confirmed outcome/title boundary before first
effort creation, without changing the effort CLI, effort persistence, or existing-effort resume
semantics.

## Architecture summary

The workflow document remains the canonical home of the effort-creation contract. A new shared
confirmation partial supplies one first-creation protocol to discovery paths, while guide and chain
surfaces summarize it and checkpoint partials only validate ownership already established by that
protocol. Catalog-derived projection tests own cross-target completeness and role classification;
spine tests own canonical-home and partial-shape assertions. Implementation applies the ADR's first
three claim updates in declaration order with the contract foundation, lands the remaining workflow
fan-out as a reviewed implementation commit, and leaves the final unified-workflow claim and both
artifact freezes to the established post-review deferred flip.

## Phase 1: Establish the canonical confirmation and checkpoint contract

**Execution mode: inline.**

### Task 1.1: Write the contract assertions before changing templates
Latitude: exact
Paths: ["internal/evals/chain_test.go", "internal/project/spine_test.go"]

In `internal/evals/chain_test.go`, separate the existing final-approval classification from the new
first-creation boundary rather than treating all three stops as identical. Keep the end-of-
brainstorming and settled-ADR skills in a renamed final-approval set, and add a dedicated assertion
for the brainstorming first-creation sequence. For both Pi and Claude projections, require this
strict order before detailed design: effort-free discovery; literal `Outcome:` and `Effort title:`
labels; an explicit request to confirm creation; an instruction to end the turn without mutation; a
clear response in a later turn; then `awf effort new "<confirmed title>"`. Assert that requested
changes remain in discovery, ambiguity requests clarification, direct concrete non-minimal requests
use the same boundary, existing efforts resume under fixed identity without reconfirmation, and lost
conversational evidence requires the pair to be confirmed again. Keep final approval assertions over
only the two existing final-approval skills, but update their preamble expectations so neither final
approval partial creates a missing effort.

Update the routine checkpoint ordered assertions in `TestUnifiedEffortWorkflowCoverage` to require
validated confirmed ownership and to reject `awf effort new` inside either checkpoint partial. Add
index-scoped negative assertions so a later copy of the confirmation prose cannot satisfy an earlier
first-creation site. Preserve Pi/non-Pi continuation assertions and the prohibition on approval stops
in every other skill.

In `internal/project/spine_test.go`, extend `TestWorkingMemorySingleHomeSurfaces` to prove that the
workflow document contains the detailed discovery-to-confirmed-outcome transition, that the guide
contains only its concise route, and that routine/final-approval surfaces cannot allocate an effort.
Extend `TestCheckpointDigestShape` to include the new outcome-confirmation partial with its declared
shape while preserving exactly four numbered steps for each existing checkpoint partial. Run:

```sh
go test ./internal/evals ./internal/project -run 'Test(UnifiedEffortWorkflowCoverage|MandatoryApprovalBoundaries|WorkingMemorySingleHomeSurfaces|CheckpointDigestShape)$'
```

The command must fail only because the rendered contract has not yet been changed; record the
specific missing or forbidden phrase rather than weakening an existing assertion.

### Task 1.2: Author the canonical transition and the first creation stop
Latitude: exact
Paths: ["templates/partials/outcome-confirmation.md", "templates/partials/checkpoint-routine.md", "templates/partials/checkpoint-approval.md", "templates/skills/brainstorming/SKILL.md.tmpl", "templates/docs/workflow.md.tmpl", "templates/docs/working-with-awf.md.tmpl", "templates/agents-doc/AGENTS.md.tmpl", ".awf/parts/agents-doc/working-memory.md", ".awf/parts/workflow/chain.md", ".awf/parts/working-with-awf/commands.md", ".awf/parts/working-with-awf/config-and-overrides.md", ".awf/docs/glossary.yaml"]

Create `templates/partials/outcome-confirmation.md` as the single operative first-creation protocol.
It must state that analysis, exploration, prioritization, option comparison, and selection remain
effort-free discovery; require literal `Outcome:` and `Effort title:` presentation; ask the user to
confirm creation; end the turn without mutation; accept a clear natural-language response only in a
later turn; keep requested changes and ambiguity in discovery; cover direct concrete non-minimal
requests; preserve minimal-fix and existing-fixed-effort behavior; and require re-presentation after
context loss removes confirmation evidence. It must not use the final-approval header, persist an
effort checkpoint before confirmation, or claim that a runtime can infer consent.

Insert the partial into `templates/skills/brainstorming/SKILL.md.tmpl` after approach selection and
before detailed design. Renumber subsequent procedure steps and replace the unconditional instruction
to carry a slug through every step with discovery-first prose. Detailed design, grounding, and final
brainstorm approval must run only after creation succeeds. Keep final approval distinct: it approves
the grounded design, not the already-confirmed title.

Rewrite `templates/partials/checkpoint-routine.md` and
`templates/partials/checkpoint-approval.md` so their first numbered step validates the already-
confirmed effort for non-minimal work and routes a missing effort back to outcome confirmation.
Neither partial may contain `awf effort new`. Preserve their four-step shapes, minimal-fix treatment,
writer-owned memory update, user-attention branches, target-sensitive continuation, and final-
approval persistence.

Update `templates/docs/workflow.md.tmpl` as the detailed canonical home: define discovery, the
first-creation pair and later response, fixed-identity resume, failure/context-loss behavior, and the
three mandatory stops. Keep effort schema, worktree defaults, one-writer ownership, resume
revalidation, and terminal cleanup unchanged. Compress the same route into
`templates/agents-doc/AGENTS.md.tmpl`. Make CLI examples in
`templates/docs/working-with-awf.md.tmpl` factually distinguish command mechanics from the workflow
authorization boundary.

Apply the equivalent project-authored wording in `.awf/parts/agents-doc/working-memory.md`,
`.awf/parts/workflow/chain.md`, `.awf/parts/working-with-awf/commands.md`, and
`.awf/parts/working-with-awf/config-and-overrides.md`; never edit their rendered outputs directly.
Change the `mandatory approval check-in` glossary entry in `.awf/docs/glossary.yaml` from two hard
stops to the first-creation confirmation plus the two final approval stops. Keep the meaning concise
and avoid implying that first confirmation persists a completed checkpoint.

Run the focused command from Task 1.1. It must pass before claim application.

### Task 1.3: Apply the first three claim updates and render every target
Kind: batch
Latitude: exact
Paths: [".awf/topics/parts/rendering/guide-and-doc-templates/current-state.md", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", "docs/decisions/require-confirmed-outcomes-before-effort-creation.md", ".awf/awf.lock", "AGENTS.md", "docs/workflow.md", "docs/working-with-awf.md", "docs/glossary.md", "docs/topics/rendering/guide-and-doc-templates.md", "docs/topics/rendering/workflow-skill-templates.md", "docs/decisions/INDEX.md", "pathspec:.pi/skills/awf-*", "pathspec:.claude/skills/awf-*", "pathspec:examples/sundial/AGENTS.md", "pathspec:examples/sundial/docs/*.md", "pathspec:examples/sundial/.pi/skills/sundial-*", "pathspec:examples/sundial/.claude/skills/sundial-*"]
Representative: "Update `working-memory-single-home` to make confirmed first creation, fixed-identity resume, and the workflow-doc canonical home explicit; append this ADR's retained slug to `Revised-by:` while preserving its prior provenance."
Edge: "Update `mandatory-approval-boundaries` to distinguish the first-creation stop from the two final approval protocols, and update `memory-checkpoint-chain-coverage` so a checkpoint validates confirmed ownership but never creates it; do not mutate the still-pending `unified-effort-workflow-coverage` claim."
Post-check: "`./x render && ./x check` reaches clean generated drift; `go test ./internal/evals ./internal/project -run 'Test(UnifiedEffortWorkflowCoverage|MandatoryApprovalBoundaries|WorkingMemorySingleHomeSurfaces|CheckpointDigestShape)$'` passes; `./awf context --show pending docs/decisions/require-confirmed-outcomes-before-effort-creation.md` reports exactly the final unified-workflow operation remaining."

Move the ADR from `Proposed` to `Implementing`. Append one `Implementing` content-stamp event and
then one Applied event containing, in declaration order:

```text
update `rendering/guide-and-doc-templates:working-memory-single-home`, update `rendering/workflow-skill-templates:mandatory-approval-boundaries`, update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`
```

Use the repository's digest-probe workflow: insert a temporary 64-zero stamp, run the staged check to
obtain the computed canonical content digest, replace the placeholder with that exact lowercase
hex digest, and never commit the placeholder or a backticked stamp. Mutate exactly those three claim
blocks in their `.awf/topics/parts/` sources, preserving `Origin:` and the existing `Revised-by:`
sequence before appending this ADR slug. Do not update the fourth claim or append an Applied event for
it.

Run `./x render` to generate all project Pi and Claude skills, docs, lock output,
`docs/decisions/INDEX.md`, and Sundial projections from the authoring sources. Inspect representative
brainstorming and final-approval output in both runtimes and the Sundial adopter, then run every
Post-check command.

### Phase close

Stage the complete transaction explicitly. Confirm the staged set contains the new partial, authored
templates and convention parts, the first three claim mutations, the ADR lifecycle events, and only
render-derived outputs. Run `./awf check staged` and `./x gate`, then create the single commit:

```commit
feat(rendering): establish confirmed effort creation
```

The commit body states that this is the first Applied batch for
`ADR-require-confirmed-outcomes-before-effort-creation`.

## Phase 2: Enforce confirmed ownership across every workflow path

**Execution mode: inline.**

### Task 2.1: Reclassify the complete catalog before changing skill prose
Latitude: exact
Paths: ["internal/evals/chain_test.go", "internal/project/phase_transaction_ownership_test.go"]

Refactor `TestUnifiedEffortWorkflowCoverage`'s complete catalog role table into closed semantic roles
that distinguish: first-creation discovery owners (`brainstorming`, `debugging`, and roadmap
graduation); already-confirmed downstream owners; final review and finish paths; and report-only or
never-create support paths. Keep the completeness equality against `cat.Skills`, so a newly enabled
skill without a role fails.

For first-creation discovery owners, assert the shared confirmation protocol and prohibit creation
before its later-response anchor. For every downstream, review, execution, and support role, prohibit
`awf effort new` and require either already-confirmed ownership or an explicit never-create contract.
Retain minimal-simple exceptions where applicable, fixed-identity resume wording where applicable,
repository-authority precedence, the one-writer rule, exact owned-memory continuity, and the
standalone-memory ban. Assert TDD, ADR proposal, and plan writing refuse or route back rather than
allocating a missing effort. Keep orienting's writer-owned correction semantics distinct from
exploration's report-only role.

Adjust `internal/project/phase_transaction_ownership_test.go` only if changed checkpoint prose moves
an existing ordering anchor; preserve its semantic proof that review settlement precedes one phase
checkpoint. Run:

```sh
go test ./internal/evals ./internal/project -run 'Test(UnifiedEffortWorkflowCoverage|PhaseTransactionOwnershipAcrossWorkflowSurfaces)$'
```

The command must fail on the old opportunistic creation and missing confirmation fan-out, not on an
unrelated count or weakened assertion.

### Task 2.2: Apply the shared boundary to discovery-led paths
Latitude: exact
Paths: ["templates/skills/debugging/SKILL.md.tmpl", "templates/skills/roadmap-graduation/SKILL.md.tmpl"]

In debugging, retain effort-free hypothesis investigation. When evidence identifies a concrete
non-minimal fix without existing ownership, include the shared outcome-confirmation partial and stop
before the failing test or mutation. An existing fixed effort resumes without title reconfirmation;
a minimal known-root fix stays effort-free. Ensure exploration children remain non-creating and
read-only toward parent memory.

In roadmap graduation, identify and reverify the roadmap item before presenting a non-minimal
graduation outcome/title pair, then stop before ADR or implementation mutation. Preserve minimal
simple graduation and explicit-drop behavior, but do not let either reaching the graduation step or
citing a roadmap item imply confirmation. Include the same partial rather than duplicating its
operative prose.

### Task 2.3: Remove opportunistic creation from downstream paths
Kind: batch
Latitude: exact
Paths: ["templates/skills/tdd/SKILL.md.tmpl", "templates/skills/bugfix/SKILL.md.tmpl", "templates/skills/proposing-adr/SKILL.md.tmpl", "templates/skills/writing-plans/SKILL.md.tmpl", "templates/skills/executing-direct/SKILL.md.tmpl", "templates/skills/adr-lifecycle/SKILL.md.tmpl", "templates/skills/reviewing-adr/SKILL.md.tmpl", "templates/skills/reviewing-plan/SKILL.md.tmpl", "templates/skills/reviewing-plan-resync/SKILL.md.tmpl", "templates/skills/executing-plans/SKILL.md.tmpl", "templates/skills/subagent-driven-development/SKILL.md.tmpl", "templates/skills/reviewing-impl/SKILL.md.tmpl", "templates/skills/retrospective/SKILL.md.tmpl", "templates/skills/refactor-coupling-audit/SKILL.md.tmpl", "templates/skills/exploring/SKILL.md.tmpl", "templates/skills/orienting/SKILL.md.tmpl"]
Representative: "Change TDD, ADR proposal, and plan writing from create-or-resume language to requiring an already-confirmed effort for non-minimal work and routing a missing effort back to the confirmation owner before tests or durable authoring."
Edge: "Keep minimal known-root bug fixes effort-free; keep existing effort resume validation; keep orienting able to correct stale memory as the one writer; keep explorers and reviewers report-only toward memory; do not add the shared creation partial to a downstream or support path."
Post-check: "`rg -n 'create or resume|creating the effort first|awf effort new' templates/skills templates/partials` returns matches only inside the shared outcome-confirmation partial or factual text explicitly asserting that another path must not create; `go test ./internal/evals ./internal/project -run 'Test(UnifiedEffortWorkflowCoverage|MandatoryApprovalBoundaries|PhaseTransactionOwnershipAcrossWorkflowSurfaces)$'` passes."

Rewrite each path's ownership preamble at its true semantic boundary. Downstream paths must validate
and carry existing confirmed ownership instead of allocating it. Review, exploration, orientation,
coupling-audit, lifecycle, execution, and retrospective paths must retain their existing authority,
writer, path, and report-only distinctions while making never-create behavior unambiguous. Do not
spread the full discovery protocol beyond the three first-creation discovery owners.

Run the Post-check after the batch and inspect every remaining grep match for the intended ownership
class rather than suppressing it with alternate wording.

### Task 2.4: Publish adopter guidance and render the complete fan-out
Kind: batch
Latitude: exact
Paths: ["changelog/CHANGELOG.md", ".awf/awf.lock", "AGENTS.md", "docs/workflow.md", "docs/working-with-awf.md", "docs/glossary.md", "pathspec:.pi/skills/awf-*", "pathspec:.claude/skills/awf-*", "pathspec:examples/sundial/AGENTS.md", "pathspec:examples/sundial/docs/*.md", "pathspec:examples/sundial/.pi/skills/sundial-*", "pathspec:examples/sundial/.claude/skills/sundial-*"]
Representative: "Add one `[Unreleased]` feature entry stating that discovery remains effort-free until the user confirms the labeled outcome/title pair and that first creation occurs only after a later response."
Edge: "Name the upgrade obligation for adopter-owned full replacements of workflow, guide, checkpoint, or affected skill parts; state that existing efforts resume unchanged and that no CLI or schema migration occurs."
Post-check: "`./x render && ./x check` reaches clean drift; representative Pi, Claude, and Sundial discovery/downstream outputs satisfy the role assertions; `go test ./internal/evals ./internal/project` passes; `git diff --check` reports no whitespace errors."

Run `./x render` once after all authored skill changes; never hand-edit generated target or Sundial
outputs. Inspect the generated brainstorming, debugging, roadmap, TDD, ADR proposal, plan writing,
final approval, routine checkpoint, orientation, and exploration bodies for both targets. Add the
changelog entry with the adopter replacement warning required by the ADR. Keep the ADR at
`Implementing`: the final `unified-effort-workflow-coverage` operation remains unapplied until
terminal implementation review settles.

### Phase close

Stage the complete workflow fan-out and changelog transaction explicitly. Verify that no current-
state claim or ADR Applied event for `unified-effort-workflow-coverage` is staged. Run
`./awf check staged` and `./x gate`, then create the single commit:

```commit
feat(rendering): require confirmed ownership across workflow paths
```

## Definition of done

- Discovery, direct non-minimal requests, ambiguity, requested revisions, existing fixed efforts,
  minimal fixes, creation failure, and lost confirmation evidence all render according to the ADR in
  Pi, Claude, this project, and Sundial.
- Only the shared outcome-confirmation protocol authorizes `awf effort new`; checkpoints and every
  downstream or support skill cannot create a missing effort.
- `working-memory-single-home`, `mandatory-approval-boundaries`, and
  `memory-checkpoint-chain-coverage` are Applied in declaration order with matching claim/proof
  changes; `unified-effort-workflow-coverage` remains the exact final operation pending terminal
  review.
- The implementation commits pass focused tests, `./x render`, `./x check`, `./awf check staged`, and
  `./x gate`, with generated outputs committed beside their authoring sources.
- Independent terminal implementation review settles with no findings. Its deferred flip transaction
  updates `unified-effort-workflow-coverage` and provenance, appends the final Applied event and
  `Implemented` content stamp, flips the ADR and this plan to `Implemented`, runs `./x render`, stages
  `docs/decisions/INDEX.md`, and passes the staged check and full gate before commit.
- The changelog tells adopters with full-replacement workflow, guide, checkpoint, or affected skill
  parts to re-derive the boundary and states that existing efforts and CLI/schema behavior do not
  migrate.

## Notes

- ADR state-change partition: Phase 1 is the first Applied batch containing operations 1 through 3
  in declaration order. Phase 2 lands the full remaining implementation but no claim mutation. After
  terminal code review, the established deferred flip owns operation 4 and both terminal artifact
  status changes in one transaction.
- No spike or preparatory refactor is required. The current partial include mechanism, catalog-derived
  evaluation matrix, canonical workflow-doc ownership, and render fan-out are the intended seams.
- The tests prove rendered ordering and prohibitions. They do not execute an interactive or golden
  agent evaluation and must not introduce one.
- If implementation reveals an affected creation-capable skill outside the closed catalog role table,
  add it to the semantically correct batch and role before proceeding; do not weaken completeness or
  defer it as optional work.
