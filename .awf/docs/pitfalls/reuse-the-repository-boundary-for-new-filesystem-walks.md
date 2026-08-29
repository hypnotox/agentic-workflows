---
title: "Reuse the repository boundary for new filesystem walks"
domains: ["tooling"]
---
New tests should use `testsupport.WalkRepoFiles`; production code should use the Git-derived
selected file set or implement the same nested-checkout pruning. A plain recursive walk can
include nested repositories through either `.git` directories or `gitdir:` pointer files.
Add boundary cases for both forms, and test linked-worktree handling separately through the
Git opener.
