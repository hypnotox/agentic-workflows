---
name: awf-subagent-driven-development
description: Use to execute a written awf plan by dispatching one subagent per task (fresh context per task) when the plan's tasks are mostly independent. Sequential dispatch only — never parallel. Per-task review after each implementer. Terminal handoff to awf-reviewing-impl. Companion to awf-executing-plans.
---

# awf-subagent-driven-development

The `implementation` chain node, subagent-dispatch shape. Wraps `AGENTS.md` step 6 ("Implementation") when a plan's tasks are independent enough to dispatch one subagent per task. Companion to `awf-executing-plans`.

**Per-task review is the recommended discipline.** After each implementer subagent reports `DONE`, dispatch one review subagent (spec-adherence + code quality combined) before advancing to the next task. A project that relies solely on the terminal `awf-reviewing-impl` review can drop this section; otherwise keep it — catching issues per task is cheaper than catching them in the final pass. Dropping it leaves the terminal `awf-reviewing-impl` as the only quality gate — there is no panel backstop in this model, so the whole-branch review absorbs everything per-task review would have caught.

## When to invoke

Invoke when:
- A plan exists under `docs/plans/YYYY-MM-DD-<topic>.md`, AND
- Plan tasks are mostly independent — each task can be implemented without seeing prior tasks' context, AND
- Fresh context per task is valuable (each task touches a distinct subsystem; main-session context is preserved for coordination).

The agent picks between this skill and `awf-executing-plans` by inspecting the plan's phase structure and task coupling. For tightly-coupled sequential plans where state flows between tasks, use `awf-executing-plans` instead.

If no plan exists, implement directly, then invoke `awf-reviewing-impl` at the end.

## Procedure

1. **Resolve the plan path.** Use the most-recent mutable plan under `docs/plans/` that this chain run is implementing, or the path the user passes explicitly. A plan is mutable until its corresponding ADR flips to `Implemented` or it gains a `# Implementation complete (YYYY-MM-DD)` header (non-ADR plans).

1. **Read the plan, raise concerns before dispatching.** Critical gaps — missing file content, unclear commands, contradictory steps, placeholders ("TBD", "similar to task N") — surface to the user before any subagent runs. Do not guess.

1. **Extract each task's full text + scene-setting context.** The dispatched subagent does NOT see this conversation. Capture, in the `Agent` prompt:
   - The task's exact file paths, content, and commands from the plan.
   - The plan phase the task belongs to (one sentence locating the task in the larger work).
   - Any prior-task outputs the task depends on (commit SHAs, file paths created earlier).
   - The project conventions the subagent must follow (see next step).

1. **Per task — dispatch one implementer subagent** via the `Agent` tool. Bake these conventions into the prompt verbatim:
   - **Conventional Commits.** `<type>(<scope>): <subject>`, subject under 72 chars, body explains the *why*.
   - **`./x gate` per commit.** Fast tier by default; `./x gate full` for the pre-push tier when a pre-push-only surface is involved. See `AGENTS.md`.
   - **No amending prior commits.** Fixes land as new commits on top.
   - **Docs travel with the change.** Any commit changing reality updates the corresponding docs or `AGENTS.md` in the same commit.
   - **Status report.** On completion, report one of: `DONE`, `DONE_WITH_CONCERNS`, `NEEDS_CONTEXT`, or `BLOCKED`.



1. **Handle the implementer's status:**
   - **`DONE`** → run the per-task review if present, then advance to the next task.
   - **`DONE_WITH_CONCERNS`** → read the concerns before proceeding. If they are about correctness or scope, address them before review. If they are observations, note them and proceed.
   - **`NEEDS_CONTEXT`** → provide the missing context and re-dispatch the implementer.
   - **`BLOCKED`** → assess: context gap → re-dispatch with more context; task too large → escalate; reasoning failure → re-dispatch with a more capable model, or escalate. Never blindly retry without changes.

1. **After a `DONE` implementer reports — dispatch one review subagent.** Dispatch ONE review subagent covering spec-adherence and code quality before marking the task done. Pass it: the task's requirements, the commit SHA(s) just created, and any invariants the project enforces. Apply mechanical findings directly; escalate genuine blockers. The whole-branch review at the terminal step covers the current-session diff; this per-task review is the gate before advancing.

1. **Final task: ADR status flip and/or plan freeze.** The final subagent's commit includes the ADR `Proposed → Accepted`/`Implemented` flip, named explicitly in the dispatched task prompt. The prompt must also instruct running `./x sync` to regenerate `docs/decisions/ACTIVE.md` and stage it — the ADR commit runs the gate, so the drift test must pass. Same rule for the `# Implementation complete (YYYY-MM-DD)` header on non-ADR plans.

1. **Terminal step: invoke `awf-reviewing-impl`** via the `Skill` tool. That skill dispatches an implementation-review subagent against the current-session SHA range, classifies findings, and applies fixes as new commits on top.

## Notes

- **Sequential dispatch only — never parallel.** File-level conflicts and ADR-flip ordering require tasks to run one at a time.
- The whole-branch review at the terminal step covers the current-session diff and is the final quality gate; any per-task review in play is a lighter check before advancing to the next task.
- One concern per commit; auto-commit when green.
