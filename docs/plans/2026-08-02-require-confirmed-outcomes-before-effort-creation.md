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
Paths: ["internal/evals/chain_test.go", "internal/project/spine_test.go", "internal/project/example_wiring_test.go"]

In `internal/evals/chain_test.go`, separate the existing final-approval classification from the new
first-creation boundary rather than treating all three stops as identical. Keep the end-of-
brainstorming and settled-ADR skills in a renamed final-approval set, and add a dedicated assertion
for the brainstorming first-creation sequence. For both Pi and Claude projections, require this
strict order before detailed design: effort-free discovery; literal `Outcome:` and `Effort title:`
labels; an explicit request to confirm creation; an instruction to end the turn without mutation; a
clear response in a later turn; then `awf effort new "<confirmed title>"`. Assert that requested
changes remain in discovery, ambiguity requests clarification, direct concrete non-minimal requests
use the same boundary, and existing efforts resume under fixed identity without reconfirmation.
Require both creation-failure branches: retained pair-and-response evidence reports the concrete
failure and recovery action and permits retry without another confirmation, while lost conversational
evidence requires the pair to be confirmed again. Keep final approval assertions over
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
shape while preserving exactly four numbered steps for each existing checkpoint partial.

In `internal/project/example_wiring_test.go`, add `TestSundialConfirmedEffortBoundary`. Read the
actual committed `examples/sundial/AGENTS.md`, Pi and Claude brainstorming skills, and workflow doc.
Apply the same ordered assertion method to the labeled pair, stop, later response, and creation; prove
the guide and workflow distinguish discovery from fixed-identity resume; and reject `awf effort new`
inside the rendered routine and final-approval blocks. This test must exercise Sundial's real adopted
configuration and committed projections, not the generic catalog fixture. Run:

```sh
go test ./internal/evals ./internal/project -run 'Test(UnifiedEffortWorkflowCoverage|MandatoryApprovalBoundaries|WorkingMemorySingleHomeSurfaces|CheckpointDigestShape|SundialConfirmedEffortBoundary)$'
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

Use this exact body for `templates/partials/outcome-confirmation.md`:

```markdown
**Mandatory first-creation confirmation.** Discovery creates no effort. Analysis, exploration,
prioritization, option comparison, and selection remain discovery until one concrete non-minimal
outcome can be named. A direct concrete non-minimal request follows the same boundary. A minimal
simple fix remains effort-free, and an existing effort resumes under its fixed identity and existing
validation rules without title reconfirmation.

When no existing effort owns the outcome, present both fields:

`Outcome: <concrete non-minimal outcome>`
`Effort title: <proposed title>`

Ask the user to confirm creation, then end the turn without creating an effort, memory, branch, or
managed worktree. Only a clear response in a later turn confirms the pair and permits
`awf effort new "<confirmed title>"`. Agreement before the pair was presented does not confirm it.
A requested change stays in discovery and receives a revised pair; an ambiguous response receives a
focused clarification.

If creation fails while the pair and its later confirming response remain available in conversational
context, report the concrete failure and recovery action and retry without another confirmation. If
context loss or session replacement makes that evidence unavailable, present and confirm the pair
again before retrying creation.
```

Apply these exact source replacements around that shared body:

- In `templates/skills/brainstorming/SKILL.md.tmpl`, replace the procedure preamble with
  `A minimal simple fix uses no effort. Discovery creates no effort or shared memory. After the user confirms the labeled outcome and effort title and creation succeeds, carry the one effort slug and exact .awf/efforts/<slug>/memory.md path through the remaining steps; children receive them read-only and never edit shared memory. Repository sources and current-state documentation outrank checkpoint prose; standalone memory is forbidden and one user-managed writer remains responsible. The full protocol lives below.` Keep the memory path in backticks in the file. Insert step 4 as `4. **Confirm first effort creation.** Complete the mandatory first-creation confirmation below before detailed design.` followed immediately by `<!-- awf:include outcome-confirmation -->`; renumber old steps 4 through 7 to 5 through 8.
- In `templates/docs/workflow.md.tmpl`, replace the chain's effort sentence with
  `Discovery creates no effort. Before first creation for a concrete non-minimal outcome, the agent presents a labeled outcome and effort title, stops, and waits for a clear later user response; after creation every chain stage carries the one immutable slug and owned memory path. Existing efforts resume under their fixed identity without title reconfirmation.` Replace the working-memory opening with `Session context is volatile; repository authority is not. Analysis, exploration, prioritization, option comparison, and selection remain effort-free discovery. Before first creation for one concrete non-minimal outcome, present Outcome: and Effort title:, ask the user to confirm creation, and end the turn without mutation; only a clear response in a later turn permits awf effort new "<confirmed title>". A minimal simple fix uses neither an effort nor memory, while an existing effort resumes under its fixed identity and existing validation rules without title reconfirmation.` Keep command names, field labels, and placeholders backticked in the file. Replace the checkpoint opening with `Checkpoints are durable; check-ins are deliberate. A checkpoint never creates an effort: it validates already-confirmed ownership for non-minimal work, updates the owned file, and routes missing ownership back to mandatory first-creation confirmation.`
- In `templates/agents-doc/AGENTS.md.tmpl` and `.awf/parts/agents-doc/working-memory.md`, begin the working-memory paragraph with `Discovery creates no effort. Before first creation for a concrete non-minimal outcome, present the labeled outcome and proposed effort title, ask the user to confirm creation, and end the turn without mutation; only a clear response in a later turn permits awf effort new "<confirmed title>". A minimal simple fix uses no effort, and an existing effort resumes under its fixed identity without title reconfirmation.` Keep the command and placeholder backticked, then retain the existing worktree, path, writer, checkpoint, authority, and finish clauses.
- In `.awf/parts/workflow/chain.md`, replace the current effort sentence with the exact chain sentence specified for `templates/docs/workflow.md.tmpl`, using the project path spelling already present.
- In `templates/docs/working-with-awf.md.tmpl` and `.awf/parts/working-with-awf/commands.md`, prefix the `awf effort new` mechanics with `Workflow authorization precedes this command: discovery creates no effort, and first creation follows a clear later user response confirming the labeled outcome and proposed title. Existing efforts resume without title reconfirmation.` Do not imply the CLI verifies confirmation.
- In `.awf/parts/working-with-awf/config-and-overrides.md`, replace its first creation summary with `Discovery creates no effort. A concrete non-minimal outcome uses exactly one immutable slugged effort only after the labeled outcome and proposed title receive clear later user confirmation; an existing effort resumes under its fixed identity without title reconfirmation.` Retain the writer, authority, integration, removal, retrospective, and finish clauses. Add `Full-replacement workflow, guide, checkpoint, or affected skill parts must re-derive this confirmation boundary; default-template projection tests cannot inspect replacement prose.` to the override responsibility paragraph.
- Set the glossary meaning exactly to `The chain's three hard stops: first-creation outcome/title confirmation, final grounded brainstorming approval, and settled ADR approval. Each stops for a later explicit user response; only the two final approvals persist a completed-phase summary.`

After authoring, run `git diff --check` and inspect each include site. Do not run a terminal
actual-Sundial assertion against stale generated files; Task 1.3 renders those files before requiring
the assertion to pass.

### Task 1.3: Render the confirmation boundary before asserting generated output
Kind: batch
Latitude: exact
Paths: [".awf/awf.lock", "AGENTS.md", "docs/workflow.md", "docs/working-with-awf.md", "docs/glossary.md", "pathspec:.pi/skills/awf-*", "pathspec:.claude/skills/awf-*", "examples/sundial/.awf/awf.lock", "pathspec:examples/sundial/AGENTS.md", "pathspec:examples/sundial/docs/*.md", "pathspec:examples/sundial/.pi/skills/sundial-*", "pathspec:examples/sundial/.claude/skills/sundial-*"]
Representative: "Run `./x render` after every Phase 1 authoring source is complete, then inspect the generated Pi, Claude, project, and Sundial brainstorming confirmation sequence."
Edge: "Verify final-approval and routine-checkpoint output contains no creation command, existing-effort resume needs no reconfirmation, and no generated file contains an unresolved-value token."
Post-check: "Only after `./x render` completes, `go test ./internal/evals ./internal/project -run 'Test(UnifiedEffortWorkflowCoverage|MandatoryApprovalBoundaries|WorkingMemorySingleHomeSurfaces|CheckpointDigestShape|SundialConfirmedEffortBoundary)$'` passes and `./x check` reports clean drift."

Run `./x render` before invoking the Post-check. This task owns the first generated-output update and
establishes that both generic temporary projections and the actual committed Sundial projections
satisfy the authored contract. A failing actual-Sundial assertion after render is a template or
render-fan-out defect to fix in this task, never an accepted intermediate terminal state.

### Task 1.4: Apply the first three claim updates and render every target
Kind: batch
Latitude: exact
Paths: [".awf/topics/parts/rendering/guide-and-doc-templates/current-state.md", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", "docs/decisions/require-confirmed-outcomes-before-effort-creation.md", ".awf/awf.lock", "AGENTS.md", "docs/workflow.md", "docs/working-with-awf.md", "docs/glossary.md", "docs/topics/rendering/guide-and-doc-templates.md", "docs/topics/rendering/workflow-skill-templates.md", "docs/decisions/INDEX.md", "pathspec:.pi/skills/awf-*", "pathspec:.claude/skills/awf-*", "examples/sundial/.awf/awf.lock", "pathspec:examples/sundial/AGENTS.md", "pathspec:examples/sundial/docs/*.md", "pathspec:examples/sundial/.pi/skills/sundial-*", "pathspec:examples/sundial/.claude/skills/sundial-*"]
Representative: "Update `working-memory-single-home` to make confirmed first creation, fixed-identity resume, and the workflow-doc canonical home explicit; append this ADR's retained slug to `Revised-by:` while preserving its prior provenance."
Edge: "Update `mandatory-approval-boundaries` to distinguish the first-creation stop from the two final approval protocols, and update `memory-checkpoint-chain-coverage` so a checkpoint validates confirmed ownership but never creates it; do not mutate the still-pending `unified-effort-workflow-coverage` claim."
Post-check: "`./x render && ./x check` reaches clean generated drift; `go test ./internal/evals ./internal/project -run 'Test(UnifiedEffortWorkflowCoverage|MandatoryApprovalBoundaries|WorkingMemorySingleHomeSurfaces|CheckpointDigestShape|SundialConfirmedEffortBoundary)$'` passes; the pending context reports the first three named operations Applied, `unified-effort-workflow-coverage` Remaining, and no undeclared or Canceled operation."

Replace the three claim blocks with these exact future bodies and metadata:

```markdown
### `invariant: working-memory-single-home`

Working-memory guidance has one canonical workflow-doc home. Discovery creates no effort; before first creation for a concrete non-minimal outcome, the agent presents a labeled outcome and proposed effort title, stops without mutation, and waits for a clear later user response. A minimal simple fix uses no effort, while an existing effort resumes under its fixed identity and existing validation rules without title reconfirmation. A confirmed outcome creates exactly one immutable slugged effort that always owns `.awf/efforts/<slug>/memory.md`. Guides carry confirmation routing, slug/path, repository-authority, one-user-managed-writer, the worktree-default execution location, conditional worktree integration/removal, retrospective, and finish routing without duplicating the detailed skeleton; standalone memory and concrete durable-record citations are forbidden, and children never become a second memory writer. Resume verification is procedurally homed in the orienting skill's resume-revalidation section; the workflow doc keeps the memory and confirmation contract and routes to it.
Origin: ADR-0157
Revised-by: ADR-0160, ADR-0161, ADR-0164, ADR-0167, ADR-0175, ADR-0187, ADR-0189, ADR-require-confirmed-outcomes-before-effort-creation
Backing: test

### `invariant: mandatory-approval-boundaries`

The rendered brainstorming skill carries mandatory first-creation confirmation before detailed design: it presents labeled `Outcome:` and `Effort title:` fields, asks the user to confirm creation, ends the turn without mutation, and permits first creation only after a clear response in a later turn. Brainstorming also closes with final grounded-design approval, and ADR review closes with settled-ADR approval; each final approval persists memory, presents the completed summary, explicitly requests approval, and stops. Continuation and handoff begin only after the applicable later response is persisted when an effort exists. No other chain skill renders a final approval stop, and no checkpoint creates missing ownership.
Origin: ADR-0152
Revised-by: ADR-0160, ADR-0167, ADR-require-confirmed-outcomes-before-effort-creation
Backing: test

### `invariant: memory-checkpoint-chain-coverage`

Checkpoint guidance renders the four-step digest: it creates no effort for a minimal simple fix, because a boundary was reached, or because work was classified non-minimal. For non-minimal work it validates one already-confirmed immutable slug and `.awf/efforts/<slug>/memory.md`, confirms `Effort: <slug>`, carries continuation in the effort's managed worktree when one exists with the owned path spelled primary-root-relative, and updates phase, next action, time, any unrecorded settled decision, and any observation in one writer-owned batch. Missing ownership routes back to mandatory first-creation confirmation. It appends a handoff-log entry only after a fresh-session boundary actually exists. Routine implementation checkpoints remain after the phase-closing commit and settled report-only review, never after heading-identified tasks or helper returns; an executable plan projection does not create a checkpoint boundary; an additional checkpoint is permitted at any safe point whose next action is independently resumable, and every checkpoint points at the workflow doc's working-memory section for authority precedence, the one-writer contract, the skeleton, and the full protocol.
Origin: ADR-0148
Revised-by: ADR-0149, ADR-0152, ADR-0160, ADR-0164, ADR-0166, ADR-0167, ADR-0175, ADR-0186, ADR-0189, ADR-0197, ADR-0209, ADR-0213, ADR-require-confirmed-outcomes-before-effort-creation
Backing: test
```

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
Paths: ["internal/evals/chain_test.go", "internal/project/example_wiring_test.go"]

Refactor `TestUnifiedEffortWorkflowCoverage`'s complete catalog role table into closed semantic roles
that distinguish: first-creation discovery owners (`brainstorming`, `debugging`, and roadmap
graduation); already-confirmed downstream owners; final review and finish paths; and report-only or
never-create support paths. Keep the completeness equality against `cat.Skills`, so a newly enabled
skill without a role fails.

Use this exact closed role mapping:

- `first-creation-discovery`: `brainstorming`, `debugging`, `roadmap-graduation`.
- `confirmed-downstream-minimal`: `bugfix`, `tdd`, `executing-direct`.
- `confirmed-downstream`: `proposing-adr`, `adr-lifecycle`, `writing-plans`, `reviewing-plan`, `reviewing-plan-resync`, `reviewing-adr`, `executing-plans`, `subagent-driven-development`, `reviewing-impl`, `retrospective`.
- `never-create-support`: `refactor-coupling-audit`, `exploring`, `orienting`.

Require every `first-creation-discovery` body to contain the shared confirmation protocol and prove
that `awf effort new` occurs after the later-response anchor. Require every other role to contain no
`awf effort new`. Both downstream roles must say ownership is already confirmed and route absence
back to first-creation confirmation; only `confirmed-downstream-minimal` must retain a minimal-simple
exception. Support roles must say they never create an effort; `refactor-coupling-audit` and
`exploring` remain report-only toward parent memory, while `orienting` retains its one-writer stale-
checkpoint correction. Across all roles retain fixed-identity resume wording where applicable,
repository-authority precedence, the one-writer rule, exact owned-memory continuity, and the
standalone-memory ban. Keep the existing orthogonal reviewer-memory set for never-edit assertions.

Extend `TestSundialConfirmedEffortBoundary` in `internal/project/example_wiring_test.go` to read the
actual generated debugging, roadmap-graduation, TDD, ADR-proposal, plan-writing, orienting, and
exploring skills for both enabled target directories. Apply the same role assertions as the generic
catalog proof: discovery owners contain the shared boundary; downstream and support paths contain no
`awf effort new`; fixed efforts resume without reconfirmation; and every rendered file remains free
of unresolved-value tokens. Leave `internal/project/phase_transaction_ownership_test.go` unchanged:
the checkpoint's review-settlement and single-phase-boundary anchors are not moved by this plan.
Run:

```sh
go test ./internal/evals ./internal/project -run 'Test(UnifiedEffortWorkflowCoverage|SundialConfirmedEffortBoundary)$'
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
Post-check: "`rg -n 'create or resume|creating the effort first|awf effort new' templates/skills templates/partials` returns matches only inside the shared outcome-confirmation partial or factual text explicitly asserting that another path must not create; `go test ./internal/evals -run 'Test(UnifiedEffortWorkflowCoverage|MandatoryApprovalBoundaries)$'` passes against temporary projections rendered from the authored sources. Actual committed Sundial assertions are deferred to Task 2.4 after its required render."

Apply these exact transformation shapes and exhaustive assignments:

- `tdd`, `bugfix`, and `executing-direct`: use `A minimal simple <work kind> uses no effort. Non-minimal work requires one already-confirmed effort with owned memory before this skill starts. This skill never creates a missing effort; if ownership is absent, stop and return to mandatory first-creation outcome/title confirmation before writing a test or mutating files.` Substitute only the skill-specific work kind and retain each existing repository-authority, writer, helper, and path clauses.
- `proposing-adr`, `writing-plans`, and `adr-lifecycle`: use `This skill requires the one existing confirmed effort and its exact owned memory path before durable authoring or lifecycle mutation. It never creates a missing effort; if ownership is absent, stop and return to mandatory first-creation outcome/title confirmation.` Then retain each existing authority, writer, reviewer, and artifact-specific clauses.
- `reviewing-plan`, `reviewing-plan-resync`, `reviewing-adr`, `reviewing-impl`, `executing-plans`, `subagent-driven-development`, and `retrospective`: prefix the existing carry/validate contract with `This skill operates only inside an existing confirmed effort and never creates a missing effort.` Retain all report-only-child, execution-owner, integration, checkpoint, and finish semantics unchanged.
- `refactor-coupling-audit` and `exploring`: state `This support path never creates an effort. When a parent effort exists, its slug and owned memory path are read-only context; never create a second effort, become a second writer, or edit shared memory.` Retain their report-only and authority contracts.
- `orienting`: state `Orienting never creates an effort. When resuming an existing confirmed effort, validate its fixed identity and owned memory; only the effort's one user-managed writer may correct stale checkpoint prose.` Retain the existing resume-revalidation procedure and prohibit children from editing memory.

Do not spread the full discovery protocol beyond `brainstorming`, `debugging`, and
`roadmap-graduation`.

Run the Post-check after the batch and inspect every remaining grep match for the intended ownership
class rather than suppressing it with alternate wording.

### Task 2.4: Render the complete fan-out, then publish and prove it
Kind: batch
Latitude: exact
Paths: ["changelog/CHANGELOG.md", ".awf/awf.lock", "AGENTS.md", "docs/workflow.md", "docs/working-with-awf.md", "docs/glossary.md", "pathspec:.pi/skills/awf-*", "pathspec:.claude/skills/awf-*", "examples/sundial/.awf/awf.lock", "pathspec:examples/sundial/AGENTS.md", "pathspec:examples/sundial/docs/*.md", "pathspec:examples/sundial/.pi/skills/sundial-*", "pathspec:examples/sundial/.claude/skills/sundial-*"]
Representative: "Add one `[Unreleased]` feature entry stating that discovery remains effort-free until the user confirms the labeled outcome/title pair and that first creation occurs only after a later response."
Edge: "Name the upgrade obligation for adopter-owned full replacements of workflow, guide, checkpoint, or affected skill parts; state that existing efforts resume unchanged and that no CLI or schema migration occurs."
Post-check: "`./x render && ./x check` reaches clean drift; representative Pi, Claude, and Sundial discovery/downstream outputs satisfy the role assertions; `go test ./internal/evals ./internal/project` passes; `git diff --check` reports no whitespace errors."

Add this exact bullet under `[Unreleased]` Features:

```markdown
- Discovery now remains effort-free until the agent presents a labeled concrete outcome and proposed effort title, stops without mutation, and receives clear confirmation in a later user response; existing efforts resume under their fixed identity without reconfirmation, and no CLI or schema migration occurs. Adopters with full-replacement workflow, guide, checkpoint, or affected skill parts must re-derive this first-creation boundary because default-template projection tests cannot inspect replacement prose.
```

Add the changelog entry first, then run `./x render` once after all authored skill changes; never
hand-edit generated target or Sundial outputs. Only after that render, run the Task 2.4 Post-check and
inspect the generated brainstorming, debugging, roadmap, TDD, ADR proposal, plan writing, final
approval, routine checkpoint, orientation, and exploration bodies for both targets. A failing actual-
Sundial assertion after render is fixed here before the task completes. Keep the ADR at
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
