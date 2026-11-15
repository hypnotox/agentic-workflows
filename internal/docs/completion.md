# Complete and integrate work

Use this workflow throughout implementation, not only at the end: to verify and commit each coherent unit, review the result, and before reporting completion, integrating, publishing, or cleaning up. It applies with or without an effort, change documents, or a worktree.

Commands use `./awf`; substitute the repository's AWF runner. Other workflows are cited as `awf docs <page>`, the same text as the skill `awf-<page>`. Methods named `agentic-…` come from the required agentic-skills package: this workflow says where in the work they apply, and the method says how, including when independent review is warranted. `agentic-implementing` owns implementing, verifying, result review, and work-history review.

The coordinating agent owns commits, review of the combined result, integration, and cleanup; delegated agents return their work, checks, and findings within their brief. When delegating, choose each agent's model and reasoning effort to fit the judgment that piece of work needs, as `agentic-subagents` describes: gathering evidence usually needs less capability than deciding on it.

## Review cues

The review cues are the situations listed in `agentic-reviewing` under "When independent review is warranted"; read that list before deciding. `agentic-reviewing` owns it, together with choosing the reviewer, briefing, reuse, and fallback. Give the reviewer the target and its basis, not your conclusion.

## Verify each coherent unit

Before implementation relies on a code structure that meets the review cues, review it under `awf docs changes`, reusing earlier review of a settled design.

Compare the result with the agreed outcome and success criteria, from the intent, specification, and ADRs when they exist, not only the plan's steps. Verify meaningful behavior and contracts with checks that tolerate incidental changes, keeping checks that catch real defects. Inspect the combined diff and update affected documentation and topics.

When the result would materially deviate from the agreed outcome, such as an unmet requirement, changed scope, or different user-visible behavior, stop dependent work and bring the deviation to the user with the options. Record what is agreed in its authoritative home; never rewrite the agreed outcome or criteria to match the implementation.

With an effort, update its memory with evidence, unmet criteria, deviations, and the next action, referencing documents rather than copying them.

## Commit verified units

By default, commit each verified, coherent unit without waiting to be asked; repository and user instructions override this. Commits land on the current branch of the checkout doing the work. Committing does not include pushing, opening a pull request, or merging.

A small change is normally one commit. For larger work, commit units as they finish, usually at phase boundaries, not all at the end. Keep related implementation, tests, and documentation together. If committing is blocked, report what remains uncommitted and why.

## Review the result

When the review cues apply, obtain independent review of the actual result before reporting completion or integrating, reusing directly applicable review. Review against the agreed outcome, constraints, and project guidance, with lenses relevant to the work. Include affected documentation, also documents that were not edited, checking claims against the delivered behavior, the agreed outcome, and active decisions; resolve conflicts with authoritative agreements instead of rewriting them. Address material findings and refresh affected verification.

## Report

Report what was checked, what that evidence establishes, what remains unassessed, uncertain, or unmet, and material deviations. Say when a required review could not be independent, and why.

## Reconcile documents

Use `awf docs adr` for affected decision records; lasting choices belong in ADRs and current guidance in topics. Keep each change definition and plan only while it serves implementation, verification, review, handoff, or a maintained reference. Before removing one, preserve still-needed requirements and knowledge elsewhere; an ADR does not replace a behavior specification. Repair surviving links with replacements or a committed reference such as `git show <commit>:<path>`.

## Integrate and publish

Confirm with the user before pushing, opening a pull request, tagging, or releasing, and before merging into the default branch, unless the request already authorized that step. These steps are visible outside the checkout or hard to undo. While a step awaits confirmation, finish and report everything that does not depend on it, and leave the worktree and branch in place. A retrospective waits for a merge the task is expected to perform, so it covers the integration; if the merge is declined or deferred, run it then.

Integrate with native Git under the repository's conventions; AWF does not integrate. With a worktree, integrate and clean up from the primary checkout as `awf docs effort` describes. Afterwards, confirm verification covers the combined result in the target checkout, refreshing checks affected by divergence or conflict resolution, and reconcile affected ADR and topic links.

## Retrospective

A retrospective is required when the work used an effort or met a meaningful issue, such as a failed approach, a misunderstanding, a user correction, or friction with tools or guidance, even if resolved. Routine effort-free changes need none. This applies the retrospective rule in `agentic-implementing`, which owns how to run it and when to make it independent; in AWF, an effort marks long-running or multi-stage work.

Run it once verification, review findings, documentation, and any integration within the task are reconciled, before the final report and cleanup; a merge awaiting confirmation in the task comes first, while a pull request handoff does not wait for its later merge. Supply the history as evidence: effort memory and notes, decisions, review findings, and verification and integration outcomes. Report material findings to the user, including resolved issues worth tracking.

Put a reusable lesson in the most specific topic under `awf docs topics`; topics hold current guidance, not incident history. Surface substantial follow-up instead of silently expanding the task, and track it in repository issues; effort notes are local.

## Clean up

When all of the above is done and the work is integrated or the user has said to discard it, remove any worktree and branch with Git, and archive effort memory from the primary checkout:

```sh
./awf effort finish <slug>
```
