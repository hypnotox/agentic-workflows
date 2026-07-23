---
date: 2026-07-23
adrs: [152]
status: Implemented
---
# Plan: Checkpoint, check-in, and handoff semantics

## Goal

Implement ADR-0152: replace the single "every checkpoint summary is an intervention point" protocol with two rendered boundary protocols (mandatory approval check-in, routine autonomous checkpoint), embed the routine protocol at the implementation skills' per-task sections, add the Pi post-queue handoff recovery path, and land the four declared claim operations in two checked batches. Non-goals: no change to the subagent child tools, the dashboard, the ledger protocol, or any Pi API surface beyond the awf-handoff extension's failure handling.

## Architecture summary

Design rationale lives in ADR-0152. Mechanically: `templates/partials/memory-checkpoint.md` is replaced by two partials, `checkpoint-approval.md` (rendered into `brainstorming` and `reviewing-adr`, whose terminal boundaries are the two mandatory approval stops) and `checkpoint-routine.md` (rendered into every other checkpoint-carrying skill, and additionally embedded at the per-task checkpoint points of `executing-plans` and `subagent-driven-development`). Both partials keep the `targetSessionHandoff` conditional split: only Pi output names `handoff_session`. The workflow doc, agent guide, working-with-awf doc, and rendering domain overview are rewritten to the two-protocol semantics. The awf-handoff extension gains a post-queue failure recovery path (visible notice, editor recovery content, no auto-retry, no extension-initiated model turn). Claim batches are serialized behind ADR-0149 per ADR-0152 Decision item 9: batch 1 applies `add mandatory-approval-boundaries` + `update memory-checkpoint-chain-coverage` + `update pi-session-handoff-workflow`; the final batch applies `update pi-session-handoff-lifecycle` and flips the ADR to Implemented.

Each batch phase closes in a single commit because a V2 Applied batch, its claim mutations, the backing-test changes, and the re-rendered outputs form one staged transaction that `awf check --staged` validates atomically; splitting them would leave an intermediate commit whose claims and proofs disagree.

## File structure

- **Created:**
  - `templates/partials/checkpoint-approval.md`
  - `templates/partials/checkpoint-routine.md`
- **Modified:**
  - `templates/skills/<skill>/SKILL.md.tmpl` for every skill that includes `memory-checkpoint` at execution time (enumerated by the Phase 1 batch task), plus `templates/skills/executing-direct/SKILL.md.tmpl` when Phase 0 finds it on the rebased base with a checkpoint reference
  - `templates/agents-doc/AGENTS.md.tmpl`, `templates/docs/workflow.md.tmpl`, `templates/docs/working-with-awf.md.tmpl`
  - `templates/pi/awf-handoff/index.ts.tmpl`
  - `.awf/domains/parts/rendering/current-state.md`
  - `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`
  - `.awf/topics/parts/rendering/pi-workflows/current-state.md`
  - `internal/evals/chain_test.go`, `internal/project/target_test.go`
  - `tools/pi-extension-test/tests/handoff.test.ts`
  - `docs/decisions/0152-checkpoint-check-in-and-handoff-semantics.md` (Status history appends only), `docs/decisions/INDEX.md` (regenerated), this plan's `status:`, and all re-rendered outputs via `./x sync`
- **Deleted:** `templates/partials/memory-checkpoint.md`

## Phase 0: Sequencing precondition (no commit)

- [x] **Task 0.1: Verify ADR-0149's shared-claim updates are Applied.** Run `grep -n 'Applied\|^status:' docs/decisions/0149-deterministic-effort-lifecycle-accounting-and-pi-workflow-routing.md`. Proceed only when, for each of `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`, `rendering/pi-workflows:pi-session-handoff-workflow`, and `rendering/pi-workflows:pi-session-handoff-lifecycle`, either ADR-0149 declares no operation on that claim or its Applied events cover the operation (a `status: Implemented` line satisfies all three at once). If any relevant operation is still Remaining, stop: ADR-0152 Decision item 9 forbids proceeding. If ADR-0149 lands Abandoned with any of those updates Canceled, its post-0149 architecture never arrives and the serialization condition is permanently unsatisfiable: stop and raise a user check-in on whether to re-baseline this plan against the pre-0149 architecture; that fork is a design decision ADR-0152 did not make.
- [x] **Task 0.2: Rebase, renumber this effort's ADR, and confirm a clean baseline.** Rebase this branch onto current `main` (local `main` when it is ahead of `origin/main`). Never resolve a conflict by hand-editing a rendered file; change the config-tree source and regenerate with `./x sync`. A concurrent effort merges its own ADR numbered 0151 to main first (user decision, 2026-07-23), so immediately after the rebase, as the first new commit, renumber this effort's ADR from 0151 to the next free number (expected 0152; the contiguity check makes this commit impossible before the concurrent 0151 exists on the tree, which is why the renumber lives here). Exact steps, adjusting 0152 if a different number is the next free one:
  - `git mv docs/decisions/0152-checkpoint-check-in-and-handoff-semantics.md docs/decisions/0152-checkpoint-check-in-and-handoff-semantics.md`
  - `sed -i 's/ADR-0152/ADR-0152/g' docs/decisions/0152-checkpoint-check-in-and-handoff-semantics.md docs/plans/2026-07-23-checkpoint-check-in-and-handoff-semantics.md`
  - `sed -i 's/adrs: \[151\]/adrs: [152]/' docs/plans/2026-07-23-checkpoint-check-in-and-handoff-semantics.md`
  - `sed -i 's/0152-checkpoint-check-in-and-handoff-semantics/0152-checkpoint-check-in-and-handoff-semantics/g' docs/plans/2026-07-23-checkpoint-check-in-and-handoff-semantics.md`
  - Post-check: `grep -rn '0151' docs/decisions/0152-checkpoint-check-in-and-handoff-semantics.md docs/plans/2026-07-23-checkpoint-check-in-and-handoff-semantics.md` returns no matches outside this task's own renumber instructions; every later reference to this effort's ADR then reads ADR-0152.
  - Run `./x sync` (INDEX.md regenerates), stage the rename and edits, run `awf check --staged`, then `./x gate`; both must pass, then commit.

  ```commit
  docs(adr): renumber 0151 to 0152 for concurrent merge order
  ```

  Acceptance: `./x check` reports clean and `./x gate` exits 0 on the rebased, renumbered branch before any Phase 1 edit.
- [x] **Task 0.3: Pin the routine-partial site set.** Run `grep -rln 'awf:include memory-checkpoint' templates/skills/` and record the output as the Phase 1 batch-task site set. Run `ls templates/skills/executing-direct/ 2>/dev/null`: if the skill exists and its template references the working-memory checkpoint, it joins the site set; if it does not exist, it is out of scope and no later task may create it.
- [x] **Task 0.4: Reconcile this plan's quoted texts against the rebased base.** ADR-0149's landed batches may have rewritten the same surfaces this plan quotes. Diff the rebased base's `templates/partials/memory-checkpoint.md`, the three shared claim bodies (`memory-checkpoint-chain-coverage`, `pi-session-handoff-workflow`, `pi-session-handoff-lifecycle`), and the two marked invariant tests against the versions quoted in Tasks 1.1, 1.2, 1.8, 1.9, 2.3, and 2.5. Fold any 0149-added semantics (for example effort-identity lines in memory files, memory-backed handoff preconditions, or governed-loader phrasing) into the partial texts and replacement claim bodies before executing those tasks; keep every existing `Revised-by:` line and append `Revised-by: ADR-0152` after them; record each fold under Notes. ADR-0152's semantics govern the merge; nothing 0149 added may be silently dropped.

## Phase 1: Two protocols, skill classification, docs, claim batch 1

One closing commit (see the transaction rationale in the Architecture summary). ADR-0152 goes `Proposed -> Implementing` and applies the first batch: `add rendering/workflow-skill-templates:mandatory-approval-boundaries`, `update rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`, `update rendering/pi-workflows:pi-session-handoff-workflow`.

- [x] **Task 1.1: Create `templates/partials/checkpoint-routine.md`** with exactly this content, as amended by any Task 0.4 reconciliation folds (`targetSessionHandoff` is the existing var already used by `templates/partials/memory-checkpoint.md`; preserve its spelling):

  ```
  **Routine checkpoint.** At this boundary:
  1. Complete the memory update in its own tool batch. In `.awf/memory/<effort-slug>.md` (create it if missing), set `Phase:` to the completed phase, set `Next:` to the immediate next action, append one line to `## Handoff log`, and refresh `Updated:`.
  2. Decide whether user attention is required: material authority drift, a materially different choice than the approved design, significant scope expansion, an unresolved correctness or safety concern, a blocker, or failed required verification. If any apply, raise a check-in that names the issue, the options, a recommendation, and the blocked next action, then stop and wait.
  3. Otherwise state a one-line continuity notice (completed phase, immediate next action, exact memory path) and continue immediately; the notice is informational, never a stop.{{if .targetSessionHandoff}} In the next tool batch, invoke `handoff_session` alone with the exact memory path and a kickoff that states the immediate successor action. Continue automatically in the fresh session unless the user cancels during the five-second window. A failed handoff leaves the checkpoint valid and becomes a check-in, never a silent retry.{{else}} Continue through the target-native successor without claiming session replacement.{{end}} Mechanical corrections and authority-determined implementation details stay autonomous. The file skeleton and ground rules live in the agent guide's working-memory section.
  ```

- [x] **Task 1.2: Create `templates/partials/checkpoint-approval.md`** with exactly this content, as amended by any Task 0.4 reconciliation folds:

  ```
  **Mandatory approval check-in.** This boundary requires explicit user approval:
  1. Complete the memory update in its own tool batch. In `.awf/memory/<effort-slug>.md` (create it if missing), set `Phase:` to the completed phase, set `Next:` to the immediate next action pending approval, append one line to `## Handoff log`, and refresh `Updated:`.
  2. Present the completed work summary, explicitly request approval, and end the turn. Stop even when there is no concern to raise; this stop is the protocol, not a judgment call.
  3. If the user rejects or requests changes: revise, persist and commit as applicable, regenerate the summary, and request approval again. After explicit approval, persist the approval and next action before continuing.{{if .targetSessionHandoff}} Then invoke `handoff_session` alone with the exact memory path and a kickoff that states the approved successor action; continue automatically in the fresh session unless the user cancels during the five-second window. A failed handoff leaves the checkpoint valid and becomes a check-in, never a silent retry.{{else}} Then continue through the target-native successor without claiming session replacement.{{end}} The file skeleton and ground rules live in the agent guide's working-memory section.
  ```

- [x] **Task 1.3: Delete `templates/partials/memory-checkpoint.md`.** Deterministic backstop: the renderer fails loudly on an unknown partial (`internal/render/include.go` returns `awf:include: unknown partial`), so any include site missed by Task 1.4 breaks `./x sync` instead of silently rendering stale prose.

- [x] **Task 1.4: Batch task: swap the include line in every checkpoint-carrying skill template.**
  - Affected-site set: exactly the files recorded by Task 0.3 (on the pre-rebase base: brainstorming, proposing-adr, reviewing-adr, writing-plans, reviewing-plan, reviewing-plan-resync, executing-plans, subagent-driven-development, reviewing-impl, bugfix, debugging; the rebase may add executing-direct).
  - Representative (routine; identical at every site except the two approval sites): in `templates/skills/writing-plans/SKILL.md.tmpl` replace the line `<!-- awf:include memory-checkpoint -->` with `<!-- awf:include checkpoint-routine -->`.
  - Edge (approval; exactly two sites): in `templates/skills/brainstorming/SKILL.md.tmpl` and `templates/skills/reviewing-adr/SKILL.md.tmpl` replace that same line with `<!-- awf:include checkpoint-approval -->`.
  - Constraint: existing `<!-- awf:section memory-checkpoint -->` anchors (bugfix, debugging) keep their section name; only include targets change (see Notes).
  - Post-check: `grep -rn 'awf:include memory-checkpoint' templates/` returns no output and `./x sync` succeeds.

- [x] **Task 1.5: Embed the complete routine protocol at the per-task boundaries** (ADR-0152 Decision item 5):
  - `templates/skills/executing-plans/SKILL.md.tmpl`: replace the one-line per-task bullet (currently `- **Checkpoint.** After each independently resumable committed task, complete the working-memory checkpoint protocol and its visible summary before starting the next task.`) with a bullet stating that after each independently resumable committed and reviewed task the complete routine protocol below runs before the next task starts, followed on the next line by `<!-- awf:include checkpoint-routine -->` so the full protocol text sits at the point of use.
  - `templates/skills/subagent-driven-development/SKILL.md.tmpl`: apply the same shape to its per-task sentence (currently `After each implemented and reviewed task, complete the working-memory checkpoint protocol and its visible summary before advancing.`).
  - If Task 0.3 added `templates/skills/executing-direct/SKILL.md.tmpl`, give its checkpoint boundary the same embedded include.
  - Both existing files keep their terminal `<!-- awf:include checkpoint-routine -->`; the partial legally renders more than once per file (`internal/render/include.go` replaces every occurrence).

- [x] **Task 1.6: Adjust boundary prose in the reclassified skills** (qualifying, non-contractual prose; edit only inside the named sections, preserving each template's section structure):
  - `templates/skills/brainstorming/SKILL.md.tmpl`: the terminal/handoff prose states that the end of brainstorming (after the grounding check) is a mandatory approval check-in governed by the included approval protocol, and the successor skill is invoked only after explicit approval. Remove any sentence implying automatic continuation into the successor without approval. The rewritten prose also states that when the user requests changes, the design is revised and re-presented for approval without repeating the single-pass grounding check (ADR-0152 Decision item 3).
  - `templates/skills/reviewing-adr/SKILL.md.tmpl`: the hand-off section states that once review converges the settled-ADR summary goes to the user as the second mandatory approval check-in, and resync, plan writing, or implementation starts only after approval. The review loop itself (dispatch, fix application, verify pass) stays autonomous.
  - `templates/skills/proposing-adr/SKILL.md.tmpl`: scope the autonomous-continuation sentence to "continue into ADR review"; the mandatory stop lives at the end of review, not after this skill's commit.
  - Constraints for all three: no Pi tool name outside the shared partials; every conditional branch renders coherent generic prose with unset vars (publication safety).

- [x] **Task 1.7: Rewrite the three doc surfaces** (the quoted fragments are required literal prose because Phase 1 tests assert them; surrounding prose is qualifying):
  - `templates/agents-doc/AGENTS.md.tmpl` working-memory bullet (currently starting `- **Make checkpoints durable and visible.**`): rewrite to a bullet starting `- **Checkpoints are durable; check-ins are deliberate.**` stating the three-step order (persist, classify, then check-in or continue), that a routine summary is a continuity notice and not an intervention point, and that the two mandatory approval boundaries are the end of brainstorming and the settled ADR review. Keep the `targetSessionHandoff` conditional with the same branch semantics as `checkpoint-routine.md`.
  - `templates/docs/workflow.md.tmpl` checkpoint passage (the sentence ending `that summary is the user's intervention point.` and its two conditional continuations): rewrite to the two-protocol semantics, including the literal sentences `A routine checkpoint summary is a continuity notice, not a stop.` and `The end of brainstorming and the settled ADR review are the two mandatory approval check-ins.`
  - `.awf/domains/parts/rendering/current-state.md` checkpoint narrative (the paragraph opening `ADR-0145 makes every rendered workflow checkpoint`): rewrite to name ADR-0152's two protocols and their boundary assignment; its Pi sentence stays accurate because this same commit updates `pi-session-handoff-workflow`.
  - `templates/docs/working-with-awf.md.tmpl` handoff paragraph: update the sentence `After a durable checkpoint's visible summary, workflow guidance calls it alone...` to the new invocation semantics (clear branch of the routine protocol only, after persistence and the continuity notice; approval boundaries invoke it only after explicit approval). Qualifying prose; the recovery-path addition stays in Task 2.4.

- [x] **Task 1.8: Author the claim mutations for batch 1.**
  - In `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`, add (alphabetical position among the `###` entries):

    ```
    ### `invariant: mandatory-approval-boundaries`

    The rendered brainstorming and ADR-review skills close with the mandatory approval protocol: persist memory, present the completed summary, explicitly request approval, and stop; continuation and any session handoff begin only after explicit approval is persisted. No other chain skill renders an approval stop.
    Origin: ADR-0152
    Backing: test
    ```

  - Same file: replace the `memory-checkpoint-chain-coverage` claim body with:

    ```
    Every checkpoint-carrying skill outside the two mandatory approval boundaries renders the routine protocol: memory persistence first, then the attention classification, then either a raised check-in that stops or a continuity notice that continues without ending the turn. The implementation skills embed the complete routine protocol at their per-task sections, retrospective alone carries the memory-file deletion step, and a non-trivial brainstormed effort becomes memory-backed when its first settled decision is persisted while small checkpoint-less efforts stay memory-free.
    ```

    keeping its existing `Origin:` line and every existing `Revised-by:` line, appending `Revised-by: ADR-0152` after them.
  - In `.awf/topics/parts/rendering/pi-workflows/current-state.md`, replace the `pi-session-handoff-workflow` claim body with:

    ```
    Pi-rendered routine-checkpoint guidance invokes handoff_session alone only on the clear branch, after persistence and the continuity notice, at phase and per-task implementation boundaries; approval boundaries hand off only after explicit approval, a failed handoff is portrayed as a check-in rather than a success, and non-Pi targets render both protocols without naming the unsupported tool.
    ```

    keeping its existing `Origin:` line and every existing `Revised-by:` line, appending `Revised-by: ADR-0152` after them.

- [x] **Task 1.9: Rewrite the workflow-template tests to the boundary classification** (ADR-0152 Decision item 11):
  - `internal/evals/chain_test.go`: split the test carrying `// invariant: rendering/workflow-skill-templates:memory-checkpoint-chain-coverage` into:
    - a routine-coverage test keeping that marker, asserting for every routine skill, in rendered order: the memory-update instruction, the attention-classification instruction with its trigger list, the check-in stop branch, and the continuity-notice continue branch; plus per-task-section scoping for the implementation skills (locate the per-task section's text region by its heading and run the ordered assertions inside that region only, so a distant terminal include cannot satisfy them) and the retrospective deletion step;
    - a new test carrying `// invariant: rendering/workflow-skill-templates:mandatory-approval-boundaries`, asserting the two approval skills render, in order: memory persistence, the explicit approval request, the stop instruction, the post-approval persistence instruction, and only then continuation; and asserting no routine skill contains the approval-request literal `explicitly request approval`.
  - `internal/project/target_test.go` `TestPiSessionHandoffWorkflow` (marker `// invariant: rendering/pi-workflows:pi-session-handoff-workflow`): rebuild the ordered-phrase table from the two new partials. Pi routine skills assert the clear-branch order (continuity notice, then the backquoted `handoff_session` invoked alone, the five-second window, and the failed-handoff-becomes-check-in sentence); Pi approval skills assert the approval request appears before any `handoff_session` mention; non-Pi output for both classes contains no `handoff_session`. Keep the existing enabled-skill config fixture, extending the skill lists only if executing-direct joined the site set.
  - Acceptance: `go test ./internal/evals/ ./internal/project/` exits 0. Sanity-check discriminating power locally (for example by temporarily swapping the two include lines in one skill and observing both tests fail); never commit the sanity edit.

- [x] **Task 1.10: Apply the lifecycle transition and close the phase.** Following `awf-adr-lifecycle`: append to ADR-0152's Status history the `Implementing; content-sha256: <digest>` event and the first `Applied` event listing exactly `add rendering/workflow-skill-templates:mandatory-approval-boundaries`, `update rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`, `update rendering/pi-workflows:pi-session-handoff-workflow` (declaration order preserved; `pi-session-handoff-lifecycle` stays Remaining). Run `./x sync` (INDEX.md and every rendered output regenerate), stage the complete transaction, run `awf check --staged`, then `./x gate`; both must pass before the commit.

  ```commit
  feat(rendering): render two-protocol checkpoint semantics
  ```

## Phase 2: Pi post-queue recovery path, final claim batch, Implemented flip

One closing commit (same transaction rationale). Applies the final batch (`update rendering/pi-workflows:pi-session-handoff-lifecycle`), flips ADR-0152 to Implemented, and freezes this plan.

- [x] **Task 2.1: Add the post-queue recovery path to `templates/pi/awf-handoff/index.ts.tmpl`** in the `awf-handoff-continue` handler (pseudocode; the user-facing strings are exact contracts):
  - Move `const wrapper = buildKickoffWrapper(request.memoryPath, request.kickoff);` to immediately after the pending-request check, before the `try` block (`buildKickoffWrapper` is pure and both inputs were validated at queue time), so the identifier is in scope inside the `catch` block and recovery content exists for every failure the catch can observe.
  - In the handler's existing `catch (error)` block, keep the failure observation and the rethrow, and add before the rethrow: call `ctx.ui.notify` with severity `"error"` and the exact text `Fresh-session handoff failed; the durable checkpoint remains valid. Recovery text is in the editor.`, then `ctx.ui.setEditorText(wrapper)` with the prepared wrapper. This branch covers every failure thrown while the old session is still active (revalidation failure, lost persistence, `newSession` rejection before replacement commits).
  - Forbidden: retrying `queueCommand` or `newSession`; any extension-initiated model turn (no `sendUserMessage` into the old session); any session or memory deletion; any change to the countdown-cancellation branch (cancellation keeps its existing `Fresh-session handoff canceled.` notice, gets no editor write and no error severity, and returns without throwing).
  - Post-teardown failures (after Pi begins disposing the old session) stay on ADR-0145's truthful boundary: no new promise, no behavior change.
- [x] **Task 2.2: Extend the extension tests** in `tools/pi-extension-test/tests/handoff.test.ts`: prove that a revalidation failure after queueing notifies with the exact error text and places the exact wrapper in the old session's editor while consuming the pending request; that a `newSession` rejection does the same; that cancellation still produces only its cancellation notice, with no editor write and no error severity; and that no branch invokes `queueCommand` twice for one request. Acceptance: the container lane passes with the extension files at 100% line/function/branch coverage (its c8 check-coverage thresholds fail below that).
- [x] **Task 2.3: Update `internal/project/target_test.go` `TestPiSessionHandoffLifecycle`** (marker `// invariant: rendering/pi-workflows:pi-session-handoff-lifecycle`): add the two new contract literals (the failure notify text from Task 2.1 and the recovery `setEditorText(wrapper)` call in the catch path) to the ordered-source assertions, and add an ordered-source check anchored inside the continue handler proving the wrapper is built before the countdown and revalidation, with the phrase sequence `const wrapper = buildKickoffWrapper(request.memoryPath`, then `const proceed = await countdown(ctx, deps)`, then `await validateMemoryPath(request.memoryPath, deps)`.
- [x] **Task 2.4: Update `templates/docs/working-with-awf.md.tmpl`** handoff paragraph: after the existing kickoff-failure sentence, add qualifying prose stating that a post-queue failure that leaves the old session active raises a visible failure notice and places the prepared kickoff wrapper in the editor as the recovery path, never retrying automatically; make no new promise about post-teardown crashes.
- [x] **Task 2.5: Author the final claim mutation** in `.awf/topics/parts/rendering/pi-workflows/current-state.md`: replace the `pi-session-handoff-lifecycle` claim body with:

  ```
  The Pi handoff lifecycle queues a single-use continuation after model settlement, presents and cleans up a cancellable five-second countdown, revalidates the memory path, replaces with a persisted parent-linked session, submits kickoff only through the replacement context, retains an editor fallback, and on a post-queue failure that leaves the old session active surfaces a visible failure notice with the prepared kickoff wrapper in the editor, never auto-retrying and never initiating a model turn, while stating the truthful nontransactional teardown boundary without deleting sessions or memory.
  ```

  keeping its existing `Origin:` line and every existing `Revised-by:` line, appending `Revised-by: ADR-0152` after them; the body is subject to Task 0.4 reconciliation like Tasks 1.1/1.2.
- [x] **Task 2.6: Apply the final lifecycle transition and close.** Append the final `Applied` event listing exactly `update rendering/pi-workflows:pi-session-handoff-lifecycle`, then the `Implemented; content-sha256: <digest>` event. Flip this plan's frontmatter to `status: Implemented` in the same commit, recording any surfaced findings under Notes. Run `./x sync`, stage the complete transaction, run `awf check --staged`, then `./x gate full`; both must pass before the commit.

  ```commit
  feat(rendering): add pi post-queue handoff recovery path
  ```

## Verification

- `./x check` clean and `./x gate full` exits 0 on the final commit.
- `grep -rn "user's intervention point" templates/ .awf/domains/` returns no output.
- `grep -rln 'awf:include checkpoint-approval' templates/skills/` matches exactly the brainstorming and reviewing-adr templates.
- `go test ./internal/evals/ ./internal/project/` exits 0; per its assertions, non-Pi renders contain no `handoff_session` and Pi routine renders place the clear-branch handoff after the continuity notice.
- `awf topic rendering/workflow-skill-templates:mandatory-approval-boundaries` shows the new claim with Origin ADR-0152 and test backing; `awf topic rendering/pi-workflows` shows both revised handoff claims carrying `Revised-by: ADR-0152`.

## Notes

- Task 0.3/1.5 execution (2026-07-23): the landed `executing-direct` template carried no checkpoint reference, so Task 0.3's literal condition kept it out of the include-swap set; ADR-0152 Decision item 5 nevertheless names the direct route when memory-backed, so per the moved-surface rule below the skill gained a memory-backed-conditional embedded routine protocol as a new procedure step.
- Task 0.4 execution (2026-07-23): folds applied from ADR-0149's landed revisions: working memory optionality with no ceremony creation, the exact active `Effort: <active-effort-id>` line, validated-memory-only Pi handoff with in-session or structured-resume continuation for checkpoint-less efforts, and the lifecycle claim's effort-identity and association revalidation. Revision metadata uses the corpus's single comma-separated line (`Revised-by: ADR-0149, ADR-0152`), not a second `Revised-by:` line as the task wording assumed.
- Phase execution (2026-07-23): the `examples/sundial` rendered outputs regenerate with every sync and must stage inside each batch transaction; `awf check --staged` also loads the HEAD snapshot, which is what forces the renumber into the rebase replay (first Notes entry below).
- Task 0.2 execution (2026-07-23): the ADR file rename to 0152 was folded into the rebase replay of the ADR's introducing commit instead of landing as a separate post-rebase commit. Reason: `awf check --staged` loads the HEAD snapshot too, and a post-rebase HEAD carrying both the concurrent 0151 and this effort's not-yet-renamed 0151 fails the duplicate-number check, making the separate commit impossible; the plan-reference updates land in the post-rebase completion commit as planned. Additionally, the renumber post-check as originally written expected zero `0151` matches, but Task 0.2's own instructions necessarily mention 0151 while describing the transition; the post-check is scoped to matches outside those instructions.
- The `awf:section memory-checkpoint` override anchors in bugfix and debugging keep their historical name; renaming override anchors would break adopter part files for zero semantic gain. Record here if a later effort renames them.
- If ADR-0149's landed architecture moved a surface this plan names (for example the brainstorming template's terminal section or a skill's section list in `internal/catalog/standard.go`), follow ADR-0152's semantics against the moved surface and note the deviation here; the ADR, not this plan's line quotes, is authoritative.
- Explicit five-second countdown cancellation remains an intentional stop with notification (ADR-0152 Decision item 7); no test may treat it as a failure branch.
