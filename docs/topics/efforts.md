---
paths:
  - 'internal/effortfs/**'
  - 'internal/artifactfs/**'
  - '.pi/skills/awf-effort/SKILL.md'
  - '.claude/skills/awf-effort/SKILL.md'
  - 'docs/changes/**'
  - 'docs/plans/**'
  - 'docs/decisions/**'
  - 'internal/docs/effort.md'
---

# Effort memory and change documents

`internal/effortfs` owns local continuity. An active effort is `.awf/efforts/<slug>/memory.md`; `new effort` creates it, `list` lists active efforts, `show` returns the path and raw memory, and `finish` archives the entire directory without replacement. Memory owns the current checkpoint and next action. Optional `notes.md` preserves significant findings for retrospective review. Effort contents remain opaque, and finishing leaves tracked documents untouched.

`internal/artifactfs` owns create-only starters. `new intent` and `new spec` create `docs/changes/<slug>/intent.md` and `spec.md`; `new plan` creates `docs/plans/<slug>.md`; and `new adr` creates `docs/decisions/<slug>.md` with hand-maintained pending status. None requires another document, effort memory, or initialization. Topic creation remains path-routed. No creation command overwrites a destination or performs Git operations.

Intent owns the problem and desired outcome, and an optional specification adds necessary behavior and design detail. Authors use that change definition as the basis for each plan or ADR the change needs: plans own implementation routes, while ADRs preserve consequential choices and rationale beyond the change. Relationships remain authored references rather than AWF-managed links. Topics explain the current system, and memory references tracked documents rather than repeating them. Local memory and notes belong in the primary checkout; tracked documents belong in the implementation checkout.

Tracked plans remain in `docs/plans/`. Existing effort-local plans remain untouched; authors may deliberately relocate useful ones into `docs/plans/` and repair references. Documents remain only while they have a concrete use; preserve still-needed knowledge before removal. AWF does not parse document content, derive artifacts, enforce stages, synchronize files, or manage their lifecycle.

The embedded `docs effort` guide owns document roles, review comparisons, checkpoint and resume behavior, worktree conventions, ADR lifecycle, retrospectives, and completion. Generated skills only route to that guide.
