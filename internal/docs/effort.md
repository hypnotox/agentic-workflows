# Effort continuity

Use an effort when work spans several stages, sessions, or handoffs, or should run in its own worktree. Small, self-contained changes stay effort-free. An effort needs no change document.

Commands use `./awf`; substitute the repository's AWF runner. Other workflows are cited as `awf docs <page>`, the same text as the skill `awf-<page>`. Use `awf docs completion` for verification, commits, and finishing.

## Resume or create memory

Check for a matching active effort and resume it when it covers the work; otherwise create one:

```sh
./awf effort list
./awf effort show <slug>
./awf new effort <slug>   # .awf/efforts/<slug>/memory.md
```

In a Git repository, memory lives in the primary checkout and all of the effort's worktrees share it. Memory works without Git.

## Keep memory a continuation checkpoint

Memory is the current continuation checkpoint, not a session log or a second definition of the change. Keep the next action prominent, with progress, verification, blockers, and actual checkout and artifact locations. Reference tracked documents instead of repeating them; tracked documents do not link back to memory.

Record consequential operator decisions, requirements, corrections, and constraints promptly when no other home holds them. Distinguish proposals from agreements, and operator statements from your interpretation. Quote verbatim when exact wording matters to scope, meaning, or authority.

On resume, reconcile memory with the current repository, instructions, topics, and documents; current source and authority win. Refresh memory at resumable boundaries, before handoff or context replacement, and before switching away from unfinished work, keeping still-binding agreements until another authoritative home holds them.

One coordinating agent writes memory; delegated agents return findings instead. A checkpoint is not a stopping point: continue until the requested stopping point, a steering point in `awf docs changes` or `awf docs completion`, a genuine need for user input, or a blocker.

## Retain significant findings

Record significant findings in `notes.md` beside memory: issues, avoidable friction, non-obvious pitfalls, and resolved problems worth reviewing. Note what happened, its effect, and the resolution or follow-up, separating observations from suspected causes. Ordinary debugging needs no log; create no empty notes file. Notes feed the retrospective; they are local, so material findings must still reach the user.

## Decide whether to use a worktree

Decide before the effort first changes tracked files, and record the choice and reason in memory. A worktree helps when work runs in parallel with other work, runs long, should leave the primary checkout usable, or makes risky changes that may be abandoned. Short sequential work can use the primary checkout; read-only work needs none.

Create one for an active effort; only this command needs Git:

```sh
./awf effort worktree add ship-it                # .awf/worktrees/ship-it.default, branch awf/ship-it/default
./awf effort worktree add ship-it --suffix tests # .awf/worktrees/ship-it.tests, branch awf/ship-it/tests
```

One worktree per effort is usual; siblings may serve the same effort. `awf help effort` gives creation details. With a worktree:

- keep memory and notes in the primary checkout, and tracked documents, topics, and implementation in the worktree;
- record actual locations in memory and handoffs, and pass the worktree path explicitly to delegated agents;
- run document starters from the checkout that should own the file; an absolute path to the runner does not change the working directory;
- synchronize, integrate, remove the worktree, and delete its branch with native Git from the primary checkout; AWF does none of these. Integrate only as `awf docs completion` describes, which includes confirming with the user first.

Before resuming tracked work in an existing worktree, check both checkouts for uncommitted changes and compare the effort branch with the primary checkout's committed HEAD. Bring in missing primary commits by the repository's merge or rebase convention before dependent work or delegation, preserving effort commits and uncommitted work, then reload affected instructions and topics and refresh affected verification. Synchronizing needs neither matching HEADs nor a fetch from a remote. Uncommitted primary-checkout changes are not shared with worktrees: reconcile relevant ones explicitly, never silently copying, discarding, or committing them. Record the reconciled commit, or the blocker, in memory.

## Hand off and archive

Before a handoff, leave a current checkpoint with the agreements, evidence, locations, and next action the successor needs. Once `awf docs completion` is satisfied, archive from the primary checkout:

```sh
./awf effort finish <slug>
```
