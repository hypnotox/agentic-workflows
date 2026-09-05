---
paths:
  - 'internal/effortfs/**'
  - '.pi/skills/awf-effort/SKILL.md'
  - '.claude/skills/awf-effort/SKILL.md'
---

# Effort memory

An active effort is `.awf/efforts/<slug>/memory.md`. `new` creates a small editable skeleton, `list` lists active residents, `show` returns the path and raw memory, and `finish` moves the complete resident to `.awf/effort-archive/<slug>` without replacement. Memory and extra resident files are opaque.

AWF performs no Git operation and has no worktree topology. The generated effort guidance may recommend native Git conventions using branch `awf/<slug>` and `.awf/worktrees/<slug>`, but users create, integrate, and remove those worktrees themselves.
