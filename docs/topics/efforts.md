---
paths:
  - 'internal/effortfs/**'
  - 'internal/artifactfs/**'
  - '.pi/skills/awf-effort/SKILL.md'
  - '.claude/skills/awf-effort/SKILL.md'
  - 'docs/plans/**'
  - 'docs/decisions/**'
  - 'internal/docs/effort.md'
---

# Effort memory and tracked artifacts

An active effort is `.awf/efforts/<slug>/memory.md`. `new effort` creates a small editable continuation starter, `list` lists active residents, `show` returns the path and raw memory, and `finish` moves the complete resident to `.awf/effort-archive/<slug>` without replacement. Memory and extra resident files are opaque; `finish` does not interpret readiness or content. Memory holds the current checkpoint and immediate continuation. An agent may create sibling `notes.md` when significant findings arise; notes preserve implementation issues for retrospective review and archive with the resident. AWF does not create or interpret notes.

Complexity can warrant a plan; a material durable choice can warrant an ADR; an effort can need either, both, or neither. `new plan <slug>` exclusively creates the standalone tracked `docs/plans/<slug>.md`. `new adr <slug>` exclusively creates `docs/decisions/<slug>.md` with hand-maintained pending status. `internal/artifactfs` owns these starters and topic creation; `internal/effortfs` owns only effort residency and archive behavior. Created Markdown stays author-owned, outside projection and semantic enforcement, and receives no automatic Git action.

Keep local memory and notes in the primary checkout and tracked plans, ADRs, topics, and implementation in the checkout that owns them. Existing effort-local plans remain opaque and archive with their resident; AWF never migrates them. Memory references authoritative tracked documents without reverse links from durable artifacts to ignored state.

Plans remain while they concretely guide execution, verification, review, or handoff and are deliberately removed when that use ends. Pending, accepted, and active ADRs remain together: active records own enduring choices and rationale, while topics explain implemented behavior and practical implications and link to relevant ADRs. Status changes and retirement are manual file/Git operations; AWF parses no lifecycle.

The embedded `docs effort` guide owns effort adoption, checkpoint/resume/continue behavior, coordinating worktree associations and native Git conventions, plan and ADR lifecycle, implementation retrospectives and selective reusable guidance, post-integration assurance, and effort finishing instructions. Generated skills only route to that guide.
