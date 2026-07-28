## Current pitfalls

Keep local effort, memory, and managed-worktree state confined to the primary control root. Native Pi skills are independently discoverable, and handoff must remain independent of effort selection.

## go-git status can expose files below an ignored managed-worktree root

_Domains: tooling_

`git status` correctly ignores `.awf/worktrees/`, but go-git's `Worktree().Status()` can still return tracked-looking `.gitignore` files inside a resident managed worktree below that ignored parent. The `awf audit` uncommitted-changes rule then reported a dirty primary checkout even when native Git reported clean. This surfaced during the ADR-0168 terminal audit as eight false untracked files owned by another effort. Audit cleanliness now reads native Git porcelain, so Git itself owns repository, global, and system ignore semantics. Other path-universe consumers that still use go-git status must not copy audit's old assumption that injected global excludes make its result identical to native Git.
