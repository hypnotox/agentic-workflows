---
type: Project Topic
title: Effort memory and change documents
description: Local continuity, create-only document starters, and canonical workflow ownership.
paths:
  - 'internal/effortfs/**'
  - 'internal/artifactfs/**'
  - '.agents/skills/awf-effort/SKILL.md'
  - '.claude/skills/awf-effort/SKILL.md'
  - 'docs/changes/**'
  - 'docs/plans/**'
  - 'docs/decisions/**'
  - 'internal/docs/effort.md'
  - 'internal/docs/changes.md'
  - 'internal/docs/adr.md'
  - 'internal/docs/completion.md'
  - '.agents/skills/awf-changes/SKILL.md'
  - '.claude/skills/awf-changes/SKILL.md'
  - '.agents/skills/awf-adr/SKILL.md'
  - '.claude/skills/awf-adr/SKILL.md'
  - '.agents/skills/awf-completion/SKILL.md'
  - '.claude/skills/awf-completion/SKILL.md'
---

# Effort memory and change documents

`internal/effortfs` owns local continuity. An active effort is `.awf/efforts/<slug>/memory.md`; `new effort` creates it, `list` lists active efforts, `show` returns the path and raw memory, and `finish` archives the entire directory without replacement. The initial memory body comes from `internal/effortfs/templates/memory.md`, not Go-authored prose. Memory owns the current checkpoint and next action. Optional `notes.md` preserves significant findings for retrospective review. Effort contents remain opaque, and finishing leaves tracked documents untouched.

In Git repositories, `new effort`, `effort list`, `show`, and `finish` resolve the primary checkout, including calls from linked checkouts. `effort worktree add <effort-slug> [--suffix <suffix>]` creates a native Git checkout for an active effort and returns absolute checkout and shared memory paths plus its branch. It uses the primary committed HEAD without copying local files, refuses path and branch collisions, and does not adopt existing trees. Checkouts are flat `.awf/worktrees/<effort-slug>.<component>` siblings: native removal deletes only the checkout directory, so a per-effort parent would outlive its worktrees, and the dot lies outside the slug alphabet, so names cannot collide across efforts. `internal/docs/effort.md` owns the naming, suffix, and sibling-checkout contract. Git use is limited to worktree creation and effort-root discovery; memory-only and other commands remain usable without Git.

`internal/artifactfs` owns create-only starters, with their embedded Markdown templates kept in `internal/artifactfs/templates`. `new intent` and `new spec` create `docs/changes/<slug>/intent.md` and `spec.md`; `new plan` creates `docs/plans/<slug>.md`; and `new adr` creates `docs/decisions/<slug>.md` with hand-maintained pending/draft decision metadata. Every concept starter includes canonical type, optional title, and required description fields; authors replace scaffold prompts. Plans, ADRs, and topics refuse reserved index/log basenames, while change directories may use those names. None requires another document, effort memory, or initialization. Tracked-document creation remains caller-checkout local, and topic creation remains path-routed. No tracked-document starter overwrites a destination or performs Git operations.

Document roles and the checkpoints that produce them belong to `internal/docs/changes.md` and `internal/docs/adr.md`; the effort guide owns continuity obligations, including the per-effort worktree decision and reconciliation with the primary checkout. Relationships between documents remain authored references rather than AWF-managed links. AWF checks shared knowledge metadata and location-bound ADR state pairs, but does not interpret document prose, derive artifacts, enforce stages, synchronize files, or manage document lifecycles. `internal/docs/knowledge.md` owns the shared authoring contract and `internal/docs/adr.md` the decision-state mapping; the active [knowledge-bundle decision](../decisions/knowledge-bundle-contract.md) preserves the rationale for these metadata boundaries.

Canonical workflow Markdown under `internal/docs/` owns the instructions, and each guide also supplies the identical generated skill body for `.agents/skills` and `.claude/skills`. Each guide opens with when it applies, and the skill templates' descriptions carry the same triggers so adopters' `AGENTS.md` need not list them. `effort.md` covers continuity, operator decision capture, notes, the worktree choice, and handoffs; `changes.md` covers settling the direction, the user confirmation before planning, premise and design review, and intent, specification, and plan roles; `adr.md` covers decision records, their review, and authority; `completion.md` covers verification, the default commit cadence, surfacing deviations, result review, confirmation before publishing or merging into the default branch, document reconciliation, the retrospective, and cleanup.

The guides state when a checkpoint happens in the repository lifecycle and which documents result; the required agentic-skills package owns how to brainstorm, design, plan, review, and run a retrospective, and the guides name its methods by bare skill name without restating its delegation, briefing, reuse, or fallback rules. Review cues and the retrospective requirement are stated once, in `completion.md` and `changes.md`, as the repository's specialization of the agentic-skills thresholds. Command argument and output rules live in `cmd/awf` help text, which the guides point to; `TestCommandHelpCarriesContractsGuidesDelegate` guards that delegation.
