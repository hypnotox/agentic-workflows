# Efforts and change documents

AWF provides ignored local effort memory and create-only tracked Markdown starters. Documents are author-owned; AWF does not interpret their content, manage Git, or decide readiness. Adapt or omit starter sections and remove unused prompts. Completion guidance applies with or without these aids.

## Define a change

Keep a change's intent and optional specification together in `docs/changes/<slug>/`. Create only the definition documents the work needs:

```sh
./awf new intent <slug>  # docs/changes/<slug>/intent.md
./awf new spec <slug>    # docs/changes/<slug>/spec.md
```

Run each command from the implementation checkout. It creates only that file, refuses replacement, and requires neither the other document nor an effort. Use a descriptive slug for the change. A related effort may use the same slug.

**Intent** preserves the problem, desired outcome, scope, non-goals, constraints, and what success looks like. Use it to establish substantial work before choosing the implementation route. Distinguish actual requirements from proposed mechanisms, and agreements from open questions. Reuse an adequate existing statement rather than duplicating it.

**Specification** is optional. Add one when important behavior or design remains unclear after intent is established. Make the intended result precise through necessary interactions, boundaries, examples, and acceptance conditions. Refine the intent rather than repeating it, and reference applicable ADRs instead of copying their rationale. Omit incidental implementation details and speculative cases.

Use the agreed intent and any specification as the basis for each plan or ADR the change needs:

```sh
./awf new plan <plan-slug>       # docs/plans/<plan-slug>.md
./awf new adr <decision-slug>    # docs/decisions/<decision-slug>.md
```

These relationships are authored, not automated: AWF creates each requested file independently and does not infer, link, or synchronize documents. Use distinct slugs when one change needs multiple plans or decisions, and reference the originating intent or specification from each derived document.

**Plan** owns one implementation route: coherent changes, real dependencies, integration points, and proportionate verification. Reference the agreed outcome, criteria, and decisions; do not introduce unresolved requirements or design choices inside steps. A plan may state its basis briefly when separate intent or specification documents add no value. Revise the route as evidence changes, without silently changing the agreed result.

**ADR** extracts a consequential choice whose rationale should remain useful after the change is complete. A change may need several ADRs when choices have independent reasons to change. Keep ADRs organized by decision area in `docs/decisions/`; their full lifecycle is described below.

Review a specification against intent, each plan and proposed ADR against the agreed change and applicable active decisions, and implementation against those agreements rather than only completed plan steps. Keep these comparisons proportionate; they do not require separate review documents or approval stages.

Keep one authoritative home for each requirement and decision. Preserve the original outcome and criteria while work proceeds; record agreed changes and material deviations explicitly rather than rewriting success to match the implementation. Commit documents while they guide implementation or review so Git retains that basis. Tracked documents do not link back to ignored effort memory.

Tracked plans remain in `docs/plans/`. Existing effort-local plans remain untouched. When useful, deliberately move an effort-local plan into `docs/plans/`, reconcile any existing destination, and repair references while preserving one authoritative copy. No bulk migration is required.

## Keep effort memory

Use an effort for work that needs continuity across stages, sessions, or handoffs. Small, self-contained changes without worktree isolation do not require one. An effort does not require a change definition, plan, ADR, or worktree.

From the primary checkout, check for a matching active effort and resume it when it covers the work:

```sh
./awf effort list
./awf effort show <slug>
```

Otherwise, create an effort from the primary checkout:

```sh
./awf new effort <slug>
# .awf/efforts/<slug>/memory.md
```

Use the same primary checkout to finish it:

```sh
./awf effort finish <slug>
```

`finish` moves the whole effort directory to `.awf/effort-archive/<slug>` without replacement. It leaves tracked documents alone and does not assess completion.

Memory owns the current continuation checkpoint, not a second copy of requirements or a session log. Keep current progress and verification, blockers, the immediate next action, and actual checkout and artifact locations. Reference change definitions, plans, and ADRs rather than restating them. Retain consequential requirements and agreements not recorded elsewhere; distinguish proposals from agreements.

Replace stale state at meaningful resumable boundaries, before handoff or context replacement, and before switching away from unfinished work. Keep the next action prominent. Use one coordinating effort and memory writer for delegated work; children return findings rather than editing memory concurrently.

On resume, inspect the current repository, applicable instructions, change definitions, plans, topics, and ADRs. Reconcile the checkpoint with reality before continuing: memory is continuation evidence, while current source and applicable authority determine what remains valid. Reuse context already established in the task rather than repeatedly resolving and rereading it.

A checkpoint or completed phase is not a stopping boundary. Continue the next authorized work unless user input is genuinely needed, an unresolved blocker exceeds existing authority, or the requested stopping point has been reached.

## Keep effort notes

When significant findings arise, create `.awf/efforts/<slug>/notes.md` beside memory in the primary checkout. Record issues, avoidable friction, non-obvious pitfalls, and useful findings as they occur, including resolved problems worth reviewing. Preserve what happened, its effect, and its resolution or needed follow-up, with useful evidence; distinguish observed problems from suspected causes. Ordinary debugging attempts need no log, but repeated small friction can be worth recording.

Memory owns the current continuation state; notes preserve findings after they stop affecting the next action. Put a current blocker in memory and reference its details in notes instead of duplicating them. Keep entries concise without a mandatory reporting template. Create no empty notes file or separate retrospective document merely to satisfy a process. Notes remain ignored local files and archive with the effort; material findings must also reach the user at completion.

## Use durable ADRs

An ADR extracts a consequential choice whose rationale should remain useful after the originating change is complete. It is not a summary of the specification. Keep it in `docs/decisions/`, organized by decision area rather than by change. Create one without requiring an effort:

```sh
./awf new adr <slug>
# docs/decisions/<slug>.md
```

New ADRs begin with hand-maintained `status: pending` frontmatter. AWF does not parse, validate, activate, retire, or otherwise automate ADR content or status.

- `pending` means the decision is proposed or under consideration; agreement has not been established.
- `accepted` means the direction is agreed but not yet fully in effect.
- `active` means the agreed decision currently governs the repository and its applicable implementation has been verified.

Status records established agreement and implementation state, not a new approval gate. Do not invent acceptance. A decision may become accepted and active in the same change. For existing records, use their documented agreement and implementation state when updating status.

Use one ADR for one coherent decision area. Keep commitments together when they share a rationale and can be reconsidered together; split choices with independent reasons to change, including when the domain has evolved since the original ADR was written. Do not record routine implementation details as ADRs.

For substantive supersession, prefer replacing the containing record even when only part of its policy changes. Restate the resulting decisions in one or more ADRs, each understandable on its own within its scope, so readers need not reconstruct current authority from a chain of predecessors. Carry forward still-binding commitments with their original meaning and the rationale needed to understand them. Explain what changes and why, distinguishing inherited commitments from new choices through ordinary prose; reframing must not silently alter an accepted commitment. Reuse a suitable existing authoritative record where that keeps ownership clear. After retirement, each still-binding decision must have one authoritative active home.

Before implementation that changes an active decision, identify relevant active ADRs and intended replacements. This responsibility still applies when work began without a new ADR or the conflict is discovered during implementation. A new ADR should reference affected decisions and explain what it intends to replace. Pending and accepted records do not displace active authority. An accepted replacement guides implementation but does not retire the predecessor's still-governing commitments. Retire the predecessor only when its remaining commitments have active homes and the accepted changes are in effect with applicable verification; reuse still-valid evidence for unchanged implementation. If some replacements are not yet active, preserve the predecessor's still-governing content even if other replacements are active.

Complete activation, retirement, and reference updates in the same coherent change. Update every surviving reference, including AWF topic links, to the relevant replacements or, where the old decision itself is the subject, to an ordinary committed historical version such as `git show <commit>:docs/decisions/<slug>.md`. A successor's own predecessor reference must also survive deletion. Once no content in the predecessor still governs, it can be deleted with history retained through Git. Ordinary corrections need not create new ADRs, and revising a record remains useful where appropriate; neither revision nor deletion may discard still-binding content without an authoritative surviving home. Retire withdrawn pending or accepted decisions and decisions that cease to apply; if no commitments remain binding, no replacement is needed.

Keep pending, accepted, and active ADRs together in `docs/decisions/`. Active ADRs own enduring choices and rationale; AWF topics describe current practical implications and link to that authority. Discover relevant ADRs through topics and ordinary repository inspection. There is no roster, query command, parsed lifecycle, or clause-level status.

## Keep files in the correct checkout

Associate implementation worktrees with a new or existing coordinating effort. Without a worktree, the primary and implementation checkout are the same directory. With a worktree:

- keep local effort memory and notes in the primary checkout;
- keep tracked change definitions, plans, ADRs, topics, and implementation in the implementation checkout;
- record the actual locations in memory and handoffs;
- run creation commands from the checkout that should own the resulting file;
- run worktree creation, integration, removal, and branch cleanup from the primary checkout.

An absolute path to a repository wrapper does not change the current working directory. AWF does not create or inspect worktrees. When isolation helps, manage one with native Git:

```sh
git worktree add -b awf/<slug> .awf/worktrees/<slug>
```

The location is a convention rather than AWF-managed topology.

## Complete a retrospective

At implementation completion, review the work and recorded findings for significant issues, avoidable friction, and useful lessons. Report material findings in the user-facing completion summary, including resolved issues worth tracking, and identify unresolved follow-ups. Review is required; findings are not. Do not manufacture findings or investigate unrelated areas to fill a report. For small work without an effort, review the available context without creating memory, notes, or a separate retrospective document.

Update the most specific existing topic when a reusable lesson warrants guidance, consolidating stale or redundant advice. A focused fix or behavioral test may fully capture the lesson without more guidance, but does not replace reporting a material issue encountered. Create a focused topic only when no existing one fits and the lesson merits persistence. Topics carry current guidance, not incident history; do not copy general methods already supplied by doctrine or optional skills.

Surface substantial follow-up work rather than silently expanding the implementation. Use ordinary repository issues for durable tracking when appropriate; ignored notes and their archive are not a shared tracking system.

## Complete and integrate

Compare actual results with the agreed intent, specification, and applicable ADRs, or the established outcome and criteria when separate documents were unnecessary. Where memory exists, update the current checkpoint with relevant evidence, unmet criteria, material deviations, documentation updates, and understandable replacements or historical references for retired artifacts. A separate completion-evidence section is optional, not required. Keep related implementation, documentation, and verified commits coherent under repository conventions.

Retain lasting choices in ADRs and current practical guidance in topics. Keep change definitions and plans while they serve implementation, verification, review, handoff, or a maintained reference. Remove each when that use ends, preserving still-needed requirements and knowledge first; an ADR does not replace a behavior specification. Repair surviving links with current replacements or a historical reference such as `git show <commit>:<path>`. Preserve document commits and later deletions through ordinary non-squash integration.

After integration, confirm that evidence covers the combined result in the target checkout. Reuse still-applicable evidence; refresh checks affected by divergence, conflict resolution, or a changed integration context. Request additional review only when material uncertainty warrants it. Reconcile affected ADRs and topic links with the combined result before cleanup.

Complete the retrospective before reporting completion and revisit any material findings from integration before cleanup. Review whether each change definition and plan still has a concrete use. Then use ordinary Git for deliberate worktree and branch cleanup and, when local memory is present, archive it with `effort finish`. The CLI never judges or automates these completion conditions.
