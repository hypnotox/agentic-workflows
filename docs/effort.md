# Efforts, plans, decisions, and worktrees

AWF provides ignored local effort memory and create-only tracked Markdown starters. Its completion and integration guidance applies whether or not the work uses effort memory, a plan, an ADR, or a worktree. AWF does not decide when work is ready, interpret authored content, manage Git, or discover worktrees. Starter headings and prompts are suggestions rather than a schema: adapt or omit irrelevant sections, remove instructional placeholders, and do not manufacture or duplicate content merely to fill them.

## Keep effort memory

Use an effort for work that needs continuity across stages, sessions, or handoffs. Small, self-contained changes without worktree isolation do not require one. An effort does not require a plan, ADR, or worktree.

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

`finish` only moves the complete resident to `.awf/effort-archive/<slug>` without replacement. It does not judge readiness, compare criteria, interpret memory, or operate Git. Existing effort-local plans and any other resident files remain opaque and move with the resident.

Keep delegated work within the coordinating effort rather than creating an effort for each child. Keep one coordinating memory writer when delegating; children return findings instead of editing the checkpoint concurrently. Refresh memory at meaningful resumable boundaries, before handoff or context replacement, and before switching away from unfinished work. Retain the outcome and constraints, current state and verification, actual artifact and checkout locations, blockers, immediate next action, and only useful attributed decision evidence. Reference plans and ADRs rather than copying them. Preserve consequential constraints and agreements not recorded elsewhere. Replace stale checkpoint state instead of appending a session log; keep the immediate next action prominent and locations with artifact references.

On resume, inspect the current repository, applicable instructions, topics, and ADRs. Reconcile the checkpoint with reality before continuing: memory is continuation evidence, while current source and applicable authority determine what remains valid. Reuse context already established in the task rather than repeatedly resolving and rereading it.

A checkpoint or completed phase is not a stopping boundary. Continue the next authorized work unless user input is genuinely needed, an unresolved blocker exceeds existing authority, or the requested stopping point has been reached.

## Keep effort notes

When significant findings arise, create `.awf/efforts/<slug>/notes.md` beside memory in the primary checkout. Record issues, avoidable friction, non-obvious pitfalls, and useful findings as they occur, including resolved problems worth reviewing. Preserve what happened, its effect, and its resolution or needed follow-up, with useful evidence; distinguish observed problems from suspected causes. Ordinary debugging attempts need no log, but repeated small friction can be worth recording.

Memory owns the current continuation state; notes preserve findings after they stop affecting the next action. Put a current blocker in memory and reference its details in notes instead of duplicating them. Keep entries concise without a mandatory reporting template. Create no empty notes file or separate retrospective document merely to satisfy a process. Notes remain ignored local files and archive with the effort; material findings must also reach the user at completion.

## Use tracked plans

A plan is a tracked implementation route and success criteria, independent of an effort:

```sh
./awf new plan <slug>
# docs/plans/<slug>.md
```

Run the command from the implementation checkout that should own the file. Creation is exclusive and has no Git side effect. The Markdown is author-owned and unparsed; adapt the starter, remove instructional placeholders, and omit irrelevant sections rather than manufacturing content.

Use one authoritative plan. If an effort adopts an existing plan, reference it from memory instead of copying it into the resident; durable documents do not link back to ignored memory. Keep the original outcome and criteria available while work proceeds; record changed goals and deviations explicitly instead of rewriting success. Commit the plan while it guides execution so Git retains its useful form.

At completion, compare the delivered result with the plan, retain lasting decisions in ADRs, update current implementation guidance and practical implications in topics, and delete the plan once it no longer serves execution, verification, review, or handoff. Retain it only for a concrete continuing use. Update surviving references to a replacement or, when history is what matters, an ordinary committed reference such as `git show <commit>:<path>`. Preserve the plan commit and its later deletion through ordinary non-squash integration.

Existing effort-local plans and other authored documents are never moved, rewritten, or deleted automatically. When resuming older work, an agent may deliberately move a plan into `docs/plans/` and update references while preserving one authoritative copy. No bulk migration is required.

## Use durable ADRs

Create an architecture decision record without requiring an effort:

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
- keep tracked plans, ADRs, topics, and implementation in the implementation checkout;
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

Compare actual results with the original outcome and criteria. Where memory exists, update the current checkpoint with relevant evidence, unmet criteria, material deviations, documentation updates, and understandable replacements or historical references for retired artifacts. A separate completion-evidence section is optional, not required. Keep related implementation, documentation, and verified commits coherent under repository conventions.

After integration, confirm that evidence covers the combined result in the target checkout. Reuse still-applicable evidence; refresh checks affected by divergence, conflict resolution, or a changed integration context. Request additional review only when material uncertainty warrants it. Reconcile affected ADRs and topic links with the combined result before cleanup.

Complete the retrospective before reporting completion and revisit any material findings from integration before cleanup. Review whether the plan still has a concrete use. Then use ordinary Git for deliberate worktree and branch cleanup and, when local memory is present, archive it with `effort finish`. The CLI never judges or automates these completion conditions.
