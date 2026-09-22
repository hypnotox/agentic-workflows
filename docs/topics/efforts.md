---
type: Project Topic
title: Effort memory and change documents
description: Local continuity, create-only document starters, and canonical workflow ownership.
paths:
  - 'internal/effortfs/**'
  - 'internal/artifactfs/**'
  - '.pi/skills/awf-effort/SKILL.md'
  - '.claude/skills/awf-effort/SKILL.md'
  - 'docs/changes/**'
  - 'docs/plans/**'
  - 'docs/decisions/**'
  - 'internal/docs/effort.md'
  - 'internal/docs/changes.md'
  - 'internal/docs/adr.md'
  - 'internal/docs/completion.md'
  - '.pi/skills/awf-changes/SKILL.md'
  - '.claude/skills/awf-changes/SKILL.md'
  - '.pi/skills/awf-adr/SKILL.md'
  - '.claude/skills/awf-adr/SKILL.md'
  - '.pi/skills/awf-completion/SKILL.md'
  - '.claude/skills/awf-completion/SKILL.md'
---

# Effort memory and change documents

`internal/effortfs` owns local continuity. An active effort is `.awf/efforts/<slug>/memory.md`; `new effort` creates it, `list` lists active efforts, `show` returns the path and raw memory, and `finish` archives the entire directory without replacement. The initial memory body comes from `internal/effortfs/templates/memory.md`, not Go-authored prose. Memory owns the current checkpoint and next action. Optional `notes.md` preserves significant findings for retrospective review. Effort contents remain opaque, and finishing leaves tracked documents untouched.

`internal/artifactfs` owns create-only starters, with their embedded Markdown templates kept in `internal/artifactfs/templates`. `new intent` and `new spec` create `docs/changes/<slug>/intent.md` and `spec.md`; `new plan` creates `docs/plans/<slug>.md`; and `new adr` creates `docs/decisions/<slug>.md` with hand-maintained pending/draft decision metadata. Every concept starter includes canonical type, optional title, and required description fields; authors replace scaffold prompts. Plans, ADRs, and topics refuse reserved index/log basenames, while change directories may use those names. None requires another document, effort memory, or initialization. Topic creation remains path-routed. No creation command overwrites a destination or performs Git operations.

Intent owns the problem and desired outcome, and an optional specification adds necessary behavior and design detail. Authors use that change definition as the basis for each plan or ADR the change needs: plans own implementation routes, while ADRs preserve consequential choices and rationale beyond the change. Relationships remain authored references rather than AWF-managed links. Topics explain the current system, and memory references tracked documents rather than repeating them. Local memory and notes belong in the primary checkout; tracked documents belong in the implementation checkout.

Tracked plans remain in `docs/plans/`. Existing effort-local plans remain untouched; authors may deliberately relocate useful ones into `docs/plans/` and repair references. Documents remain only while they have a concrete use; preserve still-needed knowledge before removal. AWF checks shared knowledge metadata and location-bound ADR state pairs, but does not interpret document prose, derive artifacts, enforce stages, synchronize files, or manage their lifecycle. `internal/docs/knowledge.md` owns the shared authoring contract and `internal/docs/adr.md` owns the decision-state mapping. The active [knowledge-bundle decision](../decisions/knowledge-bundle-contract.md) preserves the rationale for these metadata boundaries.

Canonical workflow Markdown under `internal/docs/` owns the instructions: `effort.md` covers continuity, notes, worktrees, and handoffs; `changes.md` covers premise challenges, intent/specification/plan roles, and design review before dependent structural commitments; `adr.md` covers decision records, their review, and authority; `completion.md` covers verification, coherent commits, combined-result and affected-documentation review, integration, historical retrospective, and cleanup. Each also supplies its generated skill body for both harnesses. Review depends on complexity, and an applicable assessment can cover related questions. A premise challenge targets intent when present and needs neither an effort nor a document. Retrospective independence depends on consequential history or issues, with routine mechanical work remaining proportionate. Completion entrypoints apply during implementation so verified intermediate units receive commit guidance before final completion, and completion applies with or without an effort. ADR lifecycle guidance loads only when decision records are affected.
