{{=awf:sectionDefault}}

The rendered `commit-msg` payload makes older-format ADR merge authorization definitive only after Git exposes the assembled index, incoming parents, and final message. A refusal preserves the merge for trailer correction and `git commit` retry; `pre-merge-commit` continues to check only its earlier staged evidence.

Core `effort-workflow` renders for both built-in targets and directs native persistent checkout or context tooling to the exact existing `.awf/worktrees/<slug>` worktree. It does not create a parallel harness-owned worktree or standalone memory.
