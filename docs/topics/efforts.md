---
paths:
  - 'internal/effortfs/**'
  - 'internal/artifactfs/**'
  - '.pi/skills/awf-effort/SKILL.md'
  - '.claude/skills/awf-effort/SKILL.md'
  - 'docs/changes/**'
  - 'docs/decisions/**'
  - 'internal/docs/effort.md'
---

# Effort memory and change documents

`internal/effortfs` owns local continuity. An active effort is `.awf/efforts/<slug>/memory.md`; `new effort` creates it, `list` lists active efforts, `show` returns the path and raw memory, and `finish` archives the entire directory without replacement. Memory owns the current checkpoint and next action. Optional `notes.md` preserves significant findings for retrospective review. Effort contents remain opaque, and finishing leaves tracked documents untouched.

`internal/artifactfs` owns create-only starters. `new intent`, `new spec`, and `new plan` each create their corresponding Markdown file in `docs/changes/<slug>/`, without requiring the others, effort memory, or initialization. The shared change-document constructor owns this destination convention. `new adr` creates `docs/decisions/<slug>.md` with hand-maintained pending status; topic creation remains path-routed. No creation command overwrites a destination or performs Git operations.

Intent owns the problem and desired outcome; an optional specification adds necessary behavior and design detail; the plan owns the route. ADRs preserve consequential choices and rationale beyond the change, while topics explain the current system. Memory references these documents rather than repeating them. Local memory and notes belong in the primary checkout; tracked documents belong in the implementation checkout.

Existing tracked and effort-local plans remain untouched. Authors may deliberately relocate useful plans and repair references. Documents remain only while they have a concrete use; preserve still-needed knowledge before removal. AWF does not parse document content, enforce stages, synchronize files, or manage their lifecycle.

The embedded `docs effort` guide owns document roles, review comparisons, checkpoint and resume behavior, worktree conventions, ADR lifecycle, retrospectives, and completion. Generated skills only route to that guide.
