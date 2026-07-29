## Current pitfalls

Treat one concrete non-minimal outcome as one immutable slugged effort with one user-managed memory writer. Repository sources and current-state documentation outrank `.awf/efforts/<slug>/memory.md`. Never infer managed-worktree integration or removal from effort state: inspect native Git topology on every retry, never use awf to force-discard dirty or unmerged work, and finish only after path, registration, and branch are absent.

## go-git status can expose files below an ignored managed-worktree root

_Domains: tooling_

`git status` correctly ignores `.awf/worktrees/`, but go-git's `Worktree().Status()` can still return tracked-looking `.gitignore` files inside a resident managed worktree below that ignored parent. The `awf audit` uncommitted-changes rule then reported a dirty primary checkout even when native Git reported clean. This surfaced during the ADR-0168 terminal audit as eight false untracked files owned by another effort. Audit cleanliness now reads native Git porcelain, so Git itself owns repository, global, and system ignore semantics. Other path-universe consumers that still use go-git status must not copy audit's old assumption that injected global excludes make its result identical to native Git.
