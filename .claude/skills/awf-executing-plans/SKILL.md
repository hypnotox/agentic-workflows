---
name: awf-executing-plans
description: Use to execute a written awf plan inline (one task at a time, in the current session) when the plan's tasks are tightly coupled or sequential. Codifies bite-sized tasks, one commit per task, ./x gate per commit, ADR status flip in the final commit, terminal handoff to awf-reviewing-impl. Companion to awf-subagent-driven-development.
---

# awf-executing-plans

The `implementation` chain node, inline shape. Wraps `AGENTS.md` step 6 ("Implementation") when a plan exists and the agent has determined that tasks are tightly coupled or sequential. Companion to `awf-subagent-driven-development`.

## When to invoke

Invoke when:
- A plan exists under `docs/the skills framework/plans/YYYY-MM-DD-<topic>.md`, AND
- Plan tasks are tightly coupled or sequential — state flows between tasks, each task informs the next, or the plan is short enough that subagent-per-task overhead is not justified, AND
- Full main-session visibility into each step is valuable (debugging, exploratory iteration, or work that may need mid-flight redirection).

The agent picks between this skill and `awf-subagent-driven-development` by inspecting the plan's phase structure and task coupling. For plans whose tasks are mostly independent and benefit from fresh context per task, use `awf-subagent-driven-development` instead.

If no plan exists, implement directly without a chain skill, then invoke `awf-reviewing-impl` at the end.

## Procedure

1. **Resolve the plan path.** Use the most-recent mutable plan under `docs/the skills framework/plans/` that this chain run is implementing, or the path the user passes explicitly. A plan is mutable until its corresponding ADR flips to `Implemented` or it gains a `# Implementation complete (YYYY-MM-DD)` header (non-ADR plans).

1. **Read the plan, raise concerns before starting.** Critical gaps — missing file content, unclear commands, contradictory steps, placeholders ("TBD", "similar to task N") — surface to the user before touching code. Do not guess.

1. **Per task — execute, verify, commit (one commit per task):**
   - **Implement** following the plan's exact file paths, content, and diff. No drift from the plan; raise to the user if the plan needs an amendment.
   - **Verify** with `./x gate` (fast tier). See `AGENTS.md` for the tier split and when to run the full tier.
   - **Commit.** Conventional Commits (`<type>(<scope>): <subject>`), subject under 72 chars, body explains the *why*. Auto-commit when green (tests pass + lint clean).







1. **Final commit for ADR-driven plans.** Flip the ADR `status:` frontmatter from `Proposed → Accepted` (design finalised, implementation may continue in further commits) or `Proposed → Implemented` (direct flip when no separate Accepted phase is needed) in the same commit. Then run `go test ./internal/adrtools/` to regenerate `docs/decisions/ACTIVE.md` and stage it — the commit touches the decisions directory, so the gate's drift test must pass.

1. **Final commit for non-ADR plans.** Add a `# Implementation complete (YYYY-MM-DD)` header line at the top of the plan file (freezes the plan per `AGENTS.md` "Planning files / Lifecycle").

1. **Terminal step: invoke `awf-reviewing-impl`** via the `Skill` tool. That skill dispatches an implementation-review subagent against the current-session SHA range, classifies findings, and applies fixes as new commits on top.

## Notes



- Gates are mandatory; if red, fix the root cause. Do not bypass with `--no-verify` except in genuine emergencies, and follow up with a fix commit.

- Auto-commit when green: tests pass + lint clean → commit without asking (per `AGENTS.md` "Auto-commit when green").

- One concern per commit; if a task accumulates unrelated tweaks, split it.

- Docs travel with the change: any commit that changes reality updates the corresponding docs or `AGENTS.md` in the same commit.
