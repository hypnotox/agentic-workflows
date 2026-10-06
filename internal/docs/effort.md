# Effort continuity

Use an effort when work needs continuity across stages, sessions, or handoffs, and associate implementation worktrees with a coordinating effort. Small, self-contained changes without worktree isolation can remain effort-free. An effort does not require a change document.

Use the repository's documented AWF runner; examples below use `./awf`. Read this workflow when it becomes relevant and reuse it during the task. For change definitions and plans, use `docs changes`; for decision records, use `docs adr`. During implementation, use `docs completion` for verification, commit cadence, and final completion or integration.

## Resume or create memory

Check for a matching active effort and resume it when it covers the work:

```sh
./awf effort list
./awf effort show <slug>
```

Otherwise, create one:

```sh
./awf new effort <slug>
# .awf/efforts/<slug>/memory.md
```

In Git repositories, `new effort` and `effort list`, `show`, and `finish` resolve the primary checkout even when invoked from a linked checkout. Sibling worktrees share the same primary `.awf/efforts/<slug>/memory.md`, not separate effort memories. Memory-only use remains available without Git.

Memory owns the current continuation checkpoint, not a second definition of the change or a session log. Keep the next action prominent, with current progress and verification, blockers, and actual checkout and artifact locations. Reference tracked definitions, plans, and ADRs rather than repeating them. Tracked documents do not link back to ignored memory.

Record consequential operator decisions, requirements, corrections, and constraints promptly in memory when they are not recorded elsewhere. Distinguish proposals from agreements and operator statements from agent interpretation. Preserve verbatim excerpts when exact wording matters to scope, meaning, or authority.

On resume, reconcile the checkpoint with the current repository, applicable instructions, topics, change documents, and ADRs. Memory is continuation evidence; current source and applicable authority determine what remains valid. Reuse context already established for the task.

Replace stale state at meaningful resumable boundaries, before handoff or context replacement, and before switching away from unfinished work. When refreshing memory, retain still-binding agreements until another authoritative home preserves them, then reference it. Use one coordinating effort and memory writer for delegated work; children return findings rather than editing memory concurrently. A checkpoint or phase boundary is not a stopping point: continue authorized work unless the requested stopping point, a genuine need for user input, or a blocker beyond existing authority has been reached.

## Retain useful findings

When significant findings arise, create `notes.md` beside memory. Record issues, avoidable friction, non-obvious pitfalls, and useful findings, including resolved problems worth reviewing. Preserve what happened, its effect, and the resolution or needed follow-up with useful evidence; distinguish observations from suspected causes. Ordinary debugging attempts need no log, but repeated friction can merit a note.

Memory owns what affects continuation; notes preserve findings after they stop affecting the next action. Reference details rather than duplicating them. Create no empty notes file or separate retrospective artifact merely to satisfy a process. Notes are local and ignored, not shared follow-up tracking; material findings must reach the user at completion.

## Coordinate checkouts and handoffs

In Git repositories, create or reuse a dedicated implementation worktree for an effort before changing tracked files. Default to one worktree per effort; several sibling checkouts may serve the same effort without changing its ownership. Use the primary checkout for tracked changes only when a concrete task requirement makes it necessary or Git/worktree support is unavailable; record the reason and actual checkout in memory. Read-only efforts need no worktree until tracked changes begin.

Use `./awf effort worktree add <effort-slug> [--suffix <suffix>]` for an existing active effort. Git is required for this command only. Omit the suffix for the reserved `default` component; explicitly passing `--suffix default` is rejected. A suffix follows the effort slug rules: one component starting with an ASCII letter or number, followed by letters, numbers, hyphens, or underscores.

```sh
./awf effort worktree add ship-it
# checkout: .awf/worktrees/ship-it/default; branch: awf/ship-it/default
./awf effort worktree add ship-it --suffix tests
# checkout: .awf/worktrees/ship-it/tests; branch: awf/ship-it/tests
```

Paths are rooted in the primary checkout: `.awf/worktrees/<effort-slug>/<component>`, with branches named `awf/<effort-slug>/<component>`. Creation uses native Git from the primary checkout's current committed HEAD, even when invoked from a linked checkout. It does not copy uncommitted or ignored files. The command prints the absolute checkout and shared memory paths and the branch name. It refuses existing paths and branch conflicts, so repeating a creation can fail. Existing worktrees are neither adopted nor relocated automatically.

AWF uses Git only for this creation and effort-root discovery. Synchronization, integration, worktree removal, and branch cleanup remain agent duties using native Git under repository conventions. With a worktree:

- keep coordinating memory and notes in the primary checkout;
- keep tracked change documents, plans, ADRs, topics, and implementation in the implementation checkout;
- record actual locations in memory and handoffs;
- pass the actual implementation checkout path explicitly to delegated agents;
- run tracked-document starter commands from the checkout that should own the file; they remain checkout-local;
- perform synchronization, integration, removal, and branch cleanup using native Git from the primary checkout.

Before resuming tracked work in an existing worktree, the coordinating agent inspects the primary checkout and worktree for uncommitted changes and compares the effort branch with the primary checkout's current committed HEAD. Incorporate any missing primary-checkout commits using the repository's merge or rebase conventions before continuing dependent work or delegation. Preserve effort commits and uncommitted work, resolve conflicts, reload affected instructions and topics, and refresh verification affected by the reconciliation. A worktree may retain commits ahead of the primary checkout; synchronization does not require matching HEADs or fetching from a remote.

Uncommitted primary-checkout changes are not shared by Git worktrees. Reconcile relevant local changes explicitly before work depends on them, preserving unrelated work; do not silently copy, discard, or commit them. Record the reconciled primary-checkout commit and actual implementation checkout in memory, or a concrete blocker if reconciliation cannot safely complete. When a recorded exception requires primary-checkout implementation, the primary and implementation checkout are the same directory.

An absolute path to an AWF wrapper does not change the command's working directory. Before handing off, leave a current checkpoint with the agreements, evidence, locations, and next action the successor needs, using authoritative references for details already recorded.

## Archive after completion

Follow `./awf docs completion` before finishing the effort. Once its completion and cleanup conditions are met, run from the primary checkout:

```sh
./awf effort finish <slug>
```

This moves the whole effort directory to `.awf/effort-archive/<slug>` without replacement. AWF treats effort contents as opaque, leaves tracked documents alone, and does not assess readiness.
