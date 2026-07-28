## Current pitfalls

Keep local effort, memory, and managed-worktree state confined to the primary control root. Native Pi skills are independently discoverable, and handoff must remain independent of effort selection.

## go-git status can expose files below an ignored managed-worktree root

_Domains: tooling_

`git status` correctly ignores `.awf/worktrees/`, but go-git's `Worktree().Status()` can still return tracked-looking `.gitignore` files inside a resident managed worktree below that ignored parent. The `awf audit` uncommitted-changes rule then reports a dirty primary checkout even when native Git reports clean. This surfaced during the ADR-0168 terminal audit as eight false untracked files owned by another effort. Do not delete, move, or commit another effort's managed worktree to satisfy the audit. Confirm native Git is clean, run the audit with `.awf/worktrees/` added to an isolated global-exclude configuration, and treat a permanent fix as a status-semantics bug: go-git-backed cleanliness must exclude the entire resident root exactly as native Git does.
