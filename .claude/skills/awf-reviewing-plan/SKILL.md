---
name: awf-reviewing-plan
description: >
  Use after awf-writing-plans writes a plan under docs/the skills framework/plans.
  Dispatches the plan-reviewer subagent, routes findings
  (mechanical / reasoned / user-decision), applies fixes, and hands off to
  awf-reviewing-plan-resync when an ADR exists, or to implementation
  when none does.
---

# awf-reviewing-plan

## When this skill fires

Terminal step of `awf-writing-plans`. Invoked once the plan file is written and committed under `docs/the skills framework/plans/`. The `plan-reviewer` subagent owns the review discipline: it reads the plan, runs its internal lenses (scope-completeness, executability, doc-currency, convention-alignment, testing-discipline), classifies each finding as mechanical / reasoned / user-decision, applies fixes with a 3-round soft cap, and returns a digest. This skill dispatches that agent, surfaces the digest, gates on user-decision findings, and chains to the next node.

This skill owns the post-write **full** plan review only. The plan↔ADR resync pass after a linked ADR review converges is owned by the separate `awf-reviewing-plan-resync` skill.

## Procedure

1. **Identify the plan path.** If the user named it explicitly, use that path. Otherwise, use the most recently-modified file under `docs/the skills framework/plans/` matching the `YYYY-MM-DD-*.md` pattern.

1. **Path detection detail.** When no explicit path is given: list `docs/the skills framework/plans/YYYY-MM-DD-*.md` sorted by modification time (newest last). Take the last entry. If no files match, stop and ask the user for the path.

1. **Dispatch the `plan-reviewer` subagent.** Invoke the agent tool with subagent type `plan-reviewer` and a brief that includes:
   - The absolute plan path.
   - The instruction to run in full mode (all five lenses: scope-completeness, executability, doc-currency, convention-alignment, testing-discipline).
   - The instruction to return findings as `[{focus, severity, location, issue, suggested_fix, classification}]`.
   - The commit convention: apply fixes as new commits (never `--amend`) using the `awf` scope.

   The agent handles lens application, finding classification, fix application, and the re-review loop internally. Do not re-describe those steps here.

1. **Surface the digest, then route the findings.** Display the digest the `plan-reviewer` agent returns to the user. Then route the classified findings by classification kind, not severity:
   - **mechanical** — agent applies directly.
   - **reasoned** — agent applies with one-line rationale.
   - **user-decision** — present to the user and wait.

1. **Commit applied fixes.** Fixes are committed as new commits (never `--amend`) using `awf` scope. The agent handles the Edit calls; this skill ensures the commit convention is followed. Only the plan file is edited; no other repository files are touched.

1. **Re-review loop.** The `plan-reviewer` agent manages the re-review loop (3-round soft cap) and escalates residual structural findings as `user-decision` items. Do not issue further dispatch without explicit user direction.

1. **Hand off after review settles.** Once the review converges (no user-decision findings, or all user decisions resolved):
   - If a linked ADR exists (named in the plan header or the session context), invoke `awf-reviewing-plan-resync` to catch plan-vs-finalised-ADR drift.
   - If no ADR exists, the chain proceeds directly to implementation.

## Notes

- If the user asks to skip review, comply but warn that a chain step is being skipped.
- See `AGENTS.md` for full plan lifecycle rules and the canonical workflow chain.
- The `plan-reviewer` agent is lens-diverse internally; this skill does not orchestrate a panel and does not specify per-lens model routing.
- The plan-resync pass (post-ADR-review drift check) is a separate skill: `awf-reviewing-plan-resync`.
