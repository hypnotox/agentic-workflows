Generated files are owned by awf. Edit `.awf/`, then render and check.

The `commit-msg` payload authorizes older-format ADR merges only after Git exposes the final message and incoming parents. Correct a refusal's trailers and run `git commit`; `pre-merge-commit` checks only earlier staged evidence.

`effort-workflow` uses the existing `.awf/worktrees/<slug>` worktree. It creates neither a parallel worktree nor standalone memory.
