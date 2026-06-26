---
name: awf-adr-lifecycle
description: >
  Use when transitioning a awf ADR between lifecycle states.
  Encodes the 4-state lifecycle, supersedence
  flavours, and amendment-while-Proposed scope. Self-contained.
---

# awf-adr-lifecycle

A task skill for mechanical ADR lifecycle transitions — status transitions, supersedence, and amendment-while-Proposed. The authoritative source is `docs/workflow.md` and `docs/decisions/README.md`; this skill is a procedural pointer that surfaces the right rule for the status transition at hand.

## The states

| State | Meaning | Mutability |
|---|---|---|
| `Proposed` | ADR is written and under review; content is freely mutable | Freely mutable — body and status may both change |
| `Accepted` | Design is finalised; implementation authorised but not yet complete | Status field only — body is frozen |
| `Implemented` | Design and implementation have both landed in the repository | Status field only — body is frozen |
| `Superseded` | Replaced by a later ADR; kept for historical record | Status field only — body is frozen |


## Transitions

- `Proposed → Accepted` lands in the commit that finalises the design.
- `Accepted → Implemented` lands in the final commit of the implementation sequence.
- `Proposed → Implemented` directly (skipping `Accepted`) is allowed when design and implementation land together in a single commit.
- Any live state → `Superseded by ADR-NNNN` is an in-place status edit at the moment the successor ADR is committed. The predecessor's status flip and the successor's creation happen in the **same commit**.

## Supersedence flavours

### Full supersedence

The successor replaces the predecessor wholesale. Apply when most of the predecessor is mechanically obsolete.

- Successor frontmatter: `supersedes: [predecessor-number]`.
- Predecessor frontmatter: `status: Superseded by ADR-NNNN` (in-place edit; the only allowed edit on a non-Proposed ADR).
- `docs/decisions/ACTIVE.md` records the chain in its "Supersedence chains" section (auto-generated; never hand-edit).

### Partial-item supersedence

The successor overrides specific Decision items or Invariants of the predecessor without replacing it as a whole. Apply when most of the predecessor still holds and only one decision needs revisiting.

- Successor frontmatter: `related: [predecessor-number]` (NOT `supersedes:`).
- Predecessor's `status` field is **NOT** flipped — stays `Accepted`/`Implemented`.
- Successor's prose **explicitly cites the overridden items** (e.g. "Supersedes predecessor Decision item M and Invariant N").
- `docs/decisions/ACTIVE.md` continues to list both ADRs as live; the override information lives in the successor's prose and `related:` linkage.

## Procedure

Pick the status transition, then:

1. **Edit the ADR's frontmatter `status` field** in place. This is the only allowed in-place edit on a live ADR — body remains append-only once the ADR leaves `Proposed`.

1. **If full supersedence:** update the predecessor's `status` field to `Superseded by ADR-NNNN` in the **same commit**. Partial-item supersedence preserves the predecessor's status.

1. **Update the README index.** Find the ADR's row in `docs/decisions/README.md` under the right area table and update its Status column. If the new state warrants a Notes change (supersession target, deferral reason), update Notes in the same edit.

1. **Update any domain doc** under `docs/domains` whose domain this ADR materially shifts: refresh the Current state prose if the domain's position has moved. The `## Decisions` index is generated from each ADR's `domains:` field — set that field; do not hand-maintain a decisions table. Include any prose change in the same commit.

1. **Regenerate ACTIVE.md.** Run `./x sync` to regenerate `docs/decisions/ACTIVE.md`. Stage the result. Do not hand-edit `ACTIVE.md` — always regenerate and commit it alongside any ADR status change.

1. **Run `./x gate`.** The gate's drift test validates that `ACTIVE.md` is in sync with the current ADR frontmatter. If it fails, regenerate and re-stage `ACTIVE.md` before retrying.

## Commit subject templates

Use these subjects for each transition type:

- `docs(adr): accept NNNN <short title>` — `Proposed → Accepted`
- `<feat|fix|refactor>(<scope>): <subject> (implements NNNN)` — `Accepted → Implemented` in the final implementation commit
- `docs(adr): supersede NNNN with MMMM <short title>` — in the successor's introducing commit; predecessor flip lands here
- `docs(adr): defer NNNN; <one-line reason>` — `Proposed → Deferred`
- `docs(adr): decline NNNN; <one-line reason>` — `Proposed → Declined`

## Amendment-while-Proposed

While `status: Proposed`, all sections may be amended freely as edge cases or scope refinements surface. Two flavours:

- **Plain amendment.** Edit the relevant section; commit with `docs(adr): amend Context for <title>` or fold into a co-located implementation commit when the change is small.
- **Deferral.** When scope shrinks mid-flight, open a `docs(adr): defer <title>; <reason>` commit that updates the ADR Context with what was learned. Deferred work lands in a follow-up ADR or the roadmap.

Once `Accepted` or `Implemented`, the body is frozen — only the `status` field is editable in place.

## Notes

- **Authoritative source:** `docs/workflow.md` and `docs/decisions/README.md`. This skill is a procedural pointer, not a contract restatement.
- **Append-only rule:** once any live state is reached, only the `status` field is editable in place. The body is the historical record.
- **`docs/decisions/ACTIVE.md` is auto-generated** by `./x sync` and is **never hand-edited**. Always regenerate and commit it alongside any ADR status change.
- Does not commit on your behalf; surfaces the right edits for you to land.
