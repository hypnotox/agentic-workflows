# Efforts, plans, decisions, and worktrees

AWF provides optional ignored effort memory and create-only tracked Markdown starters. Its completion and integration guidance applies whether or not the work uses effort memory, a plan, an ADR, or a worktree. AWF does not decide when work is ready, interpret authored content, manage Git, or discover worktrees. Starter headings and prompts are suggestions rather than a schema: adapt or omit irrelevant sections, remove instructional placeholders, and do not manufacture or duplicate content merely to fill them.

## Keep effort memory

Create an effort from the primary checkout when durable continuity materially helps:

```sh
./awf new effort <slug>
# .awf/efforts/<slug>/memory.md
```

Use the same primary checkout to inspect or finish it:

```sh
./awf effort list
./awf effort show <slug>
./awf effort finish <slug>
```

`finish` only moves the complete resident to `.awf/effort-archive/<slug>` without replacement. It does not judge readiness, compare criteria, interpret memory, or operate Git. Existing effort-local plans and any other resident files remain opaque and move with the resident.

Create or resume an effort only when continuity is worth maintaining. Keep one coordinating memory writer when delegating; children return findings instead of editing the checkpoint concurrently. Refresh memory at meaningful resumable boundaries, before handoff or context replacement, and before switching away from unfinished work. Retain the outcome and constraints, current state and verification, actual artifact and checkout locations, blockers, immediate next action, and only useful attributed decision evidence. Reference plans and ADRs rather than copying them, and do not turn memory into a session log.

On resume, inspect the current repository, applicable instructions, topics, and ADRs. Reconcile the checkpoint with reality before continuing: memory is continuation evidence, while current source and applicable authority determine what remains valid. Reuse context already established in the task rather than repeatedly resolving and rereading it.

A checkpoint or completed phase is not a stopping boundary. Continue the next authorized work unless user input is genuinely needed, an unresolved blocker exceeds existing authority, or the requested stopping point has been reached.

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

- `pending` means the decision is under consideration or accepted but not yet in effect. The prose must distinguish an open proposal from an agreed choice; do not invent acceptance.
- `active` means the accepted decision currently governs the repository and its applicable implementation has been verified.

Use one ADR for one durable decision, including tightly related commitments that share a rationale and can be reconsidered together. Split choices with independent reasons to change, and do not record routine implementation details as ADRs.

Before implementation that changes an active decision, identify relevant active ADRs and intended supersession. This responsibility still applies when work began without a new ADR or the conflict is discovered during implementation. A new ADR should reference affected decisions and explain what it intends to replace. When the decision takes effect, manually activate it, retire only the decisions actually superseded, and update topic links in the same coherent change. For partial supersession, preserve still-binding substance in an authoritative surviving record before revising or deleting the older ADR. Retire withdrawn pending decisions and decisions that cease to apply.

Keep pending and active ADRs together in `docs/decisions/`. Active ADRs own enduring choices and rationale; topics describe current practical implications and link to that authority. Discover relevant ADRs through topics and ordinary repository inspection. When retiring an ADR, update every surviving reference to its replacement or, where history is the point, to an ordinary committed historical version. There is no roster, query command, parsed lifecycle, or clause-level status.

## Keep files in the correct checkout

Without a worktree, the primary and implementation checkout are the same directory. With a worktree:

- keep local effort memory in the primary checkout;
- keep tracked plans, ADRs, topics, and implementation in the implementation checkout;
- record the actual locations in memory and handoffs;
- run creation commands from the checkout that should own the resulting file;
- run worktree creation, integration, removal, and branch cleanup from the primary checkout.

An absolute path to a repository wrapper does not change the current working directory. AWF does not create or inspect worktrees. When isolation helps, manage one with native Git:

```sh
git worktree add -b awf/<slug> .awf/worktrees/<slug>
```

The location is a convention rather than AWF-managed topology.

## Learn selectively at completion

Before finishing, consider whether the work exposed a concrete reusable lesson. A completion may yield none. Persist only knowledge likely to help a future change avoid a demonstrated mistake or understand a non-obvious constraint.

A focused fix or behavioral test may capture the lesson completely. Where judgment or context remains, improve the most specific existing topic and consolidate stale or redundant advice. If no topic fits and the lesson merits persistence, create a focused topic with appropriate selectors. Do not copy general methods already supplied by project doctrine or optional external skills or create a mandatory retrospective artifact. Surface substantial follow-up work separately rather than silently expanding the effort.

## Complete and integrate

Compare actual results with the original outcome and criteria. Where memory exists, record relevant evidence, unmet criteria, material deviations, documentation updates, and understandable replacements or historical references for retired artifacts. Keep related implementation, documentation, and verified commits coherent under repository conventions.

After integration, confirm that evidence covers the combined result in the target checkout. Reuse still-applicable evidence; refresh checks affected by divergence, conflict resolution, or a changed integration context. Request additional review only when material uncertainty warrants it. Reconcile affected ADRs and topic links with the combined result before cleanup.

Review whether the plan still has a concrete use and whether a reusable lesson warrants capture. Then use ordinary Git for deliberate worktree and branch cleanup and, when local memory is present, archive it with `effort finish`. The CLI never judges or automates these completion conditions.
