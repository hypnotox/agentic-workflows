---
name: awf-writing-plans
description: Use to write an implementation plan for a complex awf task under docs/plans. Enforces project plan conventions (bite-sized tasks, exact file paths, exact content, exact commands). Terminal step invokes awf-reviewing-plan or routes to an execution skill.
---

# awf-writing-plans

Writes a plan to `docs/plans/YYYY-MM-DD-<topic>.md` per the awf plan convention. The plan is the execution record; the design lives in the linked ADR (when one exists). Do not duplicate rationale — link.

## When to invoke

Per `AGENTS.md`: complex ADR-driven work (multi-commit implementation) and complex non-ADR work where the implementation steps benefit from upfront enumeration — multi-commit work, interdependent steps, refactors applying an already-decided pattern across many sites, or work destined for subagent dispatch. Skip for one-line bugfixes and changes that follow established patterns. When in doubt, write the plan.

## Conventions enforced

- **Path:** `docs/plans/YYYY-MM-DD-<kebab-topic>.md`. The date is today (ISO-8601).

- **Required header:** goal, architecture summary, tech stack (language version, key packages touched), file structure (created / modified / deleted).

- **Tasks:** bite-sized (~2-5 min each), checkbox syntax (`- [ ]`), grouped into phases. Each task specifies exact file paths, the exact content for new files or exact diff for modifications, the exact commands with expected output, and a commit step at the end of each phase. The plan must be executable by an agent with no prior conversation context.

- **No placeholders:** no "TBD", "implement later", or "similar to task N". If a step changes a file, the step shows the change verbatim. If a verify step runs a command, the expected output is exact.

- **Gate cost:** `./x gate` (~~15s) runs on every code-touching commit. Batch closely-related same-shape changes that share one rationale into a single commit; keep genuinely independent concerns separate. Docs-only commits outside the decisions directory skip the gate automatically. See `AGENTS.md` "Commit granularity vs gate cost".

- **Test-first for bugs:** add a failing test as its own task before the fix task.

## Procedure

1. **Confirm scope with the user** if the brainstorm did not already pin down the file structure and phase shape. Resolve any open questions before writing the plan.



1. **Write the plan file in one go** using `Write`. Do not commit yet. The plan must be self-contained — every step executable by an agent with no prior conversation context.







1. **Terminal step: invoke `awf-reviewing-plan`** via the `Skill` tool, passing the plan path. The reviewer runs the lens panel and reports findings; route them per the reviewing skill's procedure. After review findings are resolved, commit the plan: `docs(plans): add YYYY-MM-DD-<topic>`.

## Notes

- Plans are mutable while the corresponding ADR is `Proposed` (or while non-ADR implementation is in flight); they freeze when the ADR flips to `Accepted`/`Implemented` or when a `# Implementation complete (YYYY-MM-DD)` line is added at the top for non-ADR work.



- The plan is the execution record. The design lives in the linked ADR. Do not duplicate design rationale in the plan — link to the ADR instead.
- Plans stay in the repo permanently after freezing. They are the historical record of how a complex change rolled out.
