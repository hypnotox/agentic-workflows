---
name: awf-reviewing-adr
description: >
  Use after awf-proposing-adr writes an ADR under docs/decisions
  (status Proposed). Dispatches the adr-reviewer subagent, routes findings
  (mechanical / reasoned / user-decision), applies fixes, and hands off to
  awf-reviewing-plan-resync when a plan exists.
---

# awf-reviewing-adr

## When this skill fires

Terminal step of `awf-proposing-adr`. Invoked once the ADR file is written and committed (status `Proposed`). The `adr-reviewer` subagent owns the review discipline: it reads the ADR, runs its internal lenses, classifies each finding as mechanical / reasoned / user-decision, applies fixes with a 3-round soft cap, and returns a digest. This skill dispatches that agent, surfaces the digest, gates on user-decision findings, and chains to the next node.

## Procedure

1. **Identify the ADR path.** If the user named it explicitly, use that path. Otherwise, use the most recently-modified file under `docs/decisions/` matching the `NNNN-*.md` pattern.

1. **Path detection detail.** When no explicit path is given: list `docs/decisions/NNNN-*.md` sorted by modification time (newest last). Take the last entry. If no files match, stop and ask the user for the path.

1. **Dispatch the `adr-reviewer` subagent.** Invoke the agent tool with subagent type `adr-reviewer` and a brief that includes:
   - The absolute ADR path.
   - The instruction to return findings as `[{focus, severity, location, issue, suggested_fix, classification}]`.
   - The commit convention: apply fixes as new commits (never `--amend`) using the `awf` scope.

   The agent handles lens application, finding classification, fix application, and the re-review loop internally. Do not re-describe those steps here.

1. **Surface the digest, then route the findings.** Display the digest the `adr-reviewer` agent returns to the user. Then route the classified findings by classification kind, not severity:
   - **mechanical** — agent applies directly.
   - **reasoned** — agent applies with one-line rationale.
   - **user-decision** — present to the user and wait.

1. **Commit applied fixes.** Fixes are committed as new commits (never `--amend`) using `awf` scope. The agent handles the Edit calls; this skill ensures the commit convention is followed.

1. **Re-review loop.** The `adr-reviewer` agent manages the re-review loop (3-round soft cap) and escalates residual structural findings as `user-decision` items. Do not issue further dispatch without explicit user direction.

1. **Flip ADR status when finalised.** After the review settles (no structural findings, or user decisions resolved), flip the ADR `status:` frontmatter from `Proposed` to `Accepted`. Flip to `Implemented` instead if this ADR ships its own code in the same commit series. Commit the flip.

1. **Hand off to plan resync.** After the ADR review converges and the status is flipped, check whether a plan exists (a `docs/plans/YYYY-MM-DD-*.md` file named or implied by the ADR). If a plan exists, invoke `awf-reviewing-plan-resync` against that plan. If no plan exists, the chain proceeds directly to implementation.

## Notes

- If the user asks to skip review, comply but warn that a chain step is being skipped.
- See `docs/workflow.md` for full ADR lifecycle rules and the canonical workflow chain.
- The `adr-reviewer` agent is lens-diverse internally; this skill does not orchestrate a panel and does not specify per-lens model routing.
