---
paths:
  - 'internal/effortfs/**'
  - 'internal/adrfs/**'
  - '.pi/skills/awf-effort/SKILL.md'
  - '.claude/skills/awf-effort/SKILL.md'
  - 'docs/decisions/**'
---

# Effort memory

An active effort is `.awf/efforts/<slug>/memory.md`. `new` creates a small editable continuity and evidence skeleton, `list` lists active residents, `show` returns the path and raw memory, and `finish` moves the complete resident to `.awf/effort-archive/<slug>` without replacement. Memory and extra resident files are opaque; `finish` does not interpret readiness or content.

Complexity can warrant a plan; a material choice can warrant an ADR; an effort can need either, both, or neither. `plan new <effort-slug>` exclusively creates `.awf/efforts/<effort-slug>/plan.md` in an existing active effort. `adr new <slug>` exclusively creates `docs/decisions/<slug>.md`. These optional scaffolds are author-owned Markdown, do not participate in projection, and receive no content validation or automatic Git action.

Memory and plans stay in the primary checkout. An ADR has one working copy in the implementation checkout, is committed with its chosen decision and rationale, and is removed in a later implementation/topic-update commit after verification and incorporation of its durable substance into applicable topics. Preserve both commits through non-squash integration; Git history is the historical record.

AWF performs no Git operation and has no worktree topology. The generated effort guidance owns the native worktree location convention, completion comparison, decision evidence, topic update, and ADR retirement instructions.
